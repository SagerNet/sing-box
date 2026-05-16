//go:build linux || (darwin && cgo)

package usbip

import (
	E "github.com/sagernet/sing/common/exceptions"
)

// EncodeIsoSubmit fills the isochronous SUBMIT fields on base. When asap is
// true, the wire-level ASAP flag is set and StartFrame is zeroed. Otherwise
// RebaseFrame recovers the absolute frame number from the controller's
// monotonic counter and ciFrame's 8 bits, then StartFrame carries the low 32
// bits across the wire.
func EncodeIsoSubmit(currentFrame uint64, base SubmitCommand, ciFrame uint8, asap bool) SubmitCommand {
	if asap {
		base.TransferFlags |= usbipTransferFlagIsoASAP
		base.StartFrame = 0
		return base
	}
	rebased := RebaseFrame(currentFrame, ciFrame)
	base.StartFrame = int32(uint32(rebased))
	return base
}

// RebaseFrame returns the smallest absolute USB frame number whose low 8
// bits equal low8 and is >= currentFrame. Apple's IOUSBHostCI iso messages
// only carry the low 8 bits; the host recovers the high bits against the
// controller's monotonic counter. firstFrameNumber must be in the future;
// 0 is reserved for ASAP and handled separately by the caller.
func RebaseFrame(currentFrame uint64, low8 uint8) uint64 {
	base := currentFrame&^0xff | uint64(low8)
	if base < currentFrame {
		base += 256
	}
	return base
}

var (
	errIsoDescriptorCount        = E.New("RET_SUBMIT iso descriptor count mismatch")
	errIsoDescriptorRange        = E.New("RET_SUBMIT iso descriptor range mismatch")
	errIsoDescriptorActualLength = E.New("RET_SUBMIT iso descriptor actual_length exceeds length")
	errIsoDescriptorSum          = E.New("RET_SUBMIT iso descriptor actual_length sum does not match header")
	errIsoPayloadShort           = E.New("RET_SUBMIT iso payload shorter than descriptor range")
)

// ValidateIsoResponse enforces the per-descriptor invariants of a RET_SUBMIT
// against the original CMD_SUBMIT shape. startIsoTransfer emits single-packet
// CMD_SUBMITs that cover the whole request, so the response must mirror that
// exact shape. If multi-packet ISO submits are added later, the count check
// relaxes to a non-empty count and the offset/length check becomes a per-
// descriptor walk that proves coverage is non-overlapping and in-range.
func ValidateIsoResponse(requestLen int, actualLength int, packets []IsoPacketDescriptor, payloadLen int) error {
	if len(packets) != 1 {
		return E.Extend(errIsoDescriptorCount, "expected 1, got ", len(packets))
	}
	descriptor := packets[0]
	if descriptor.Offset != 0 || int(descriptor.Length) != requestLen {
		return E.Extend(errIsoDescriptorRange, "offset ", descriptor.Offset, ", length ", descriptor.Length, ", request ", requestLen)
	}
	if descriptor.ActualLength < 0 || descriptor.ActualLength > descriptor.Length {
		return E.Extend(errIsoDescriptorActualLength, "actual_length ", descriptor.ActualLength, ", length ", descriptor.Length)
	}
	if int(descriptor.ActualLength) != actualLength {
		return E.Extend(errIsoDescriptorSum, "sum ", descriptor.ActualLength, " != header ", actualLength)
	}
	if int(descriptor.ActualLength) > payloadLen {
		return E.Extend(errIsoPayloadShort, "actual_length ", descriptor.ActualLength, " > payload ", payloadLen)
	}
	return nil
}

// ScatterIsoResponse copies frame data from payload into dst at the offsets
// declared by packets. Caller MUST have called ValidateIsoResponse against the
// same descriptors and payload before invoking; this function trusts every
// offset and length and will panic on slice bounds for malformed input.
func ScatterIsoResponse(dst, payload []byte, packets []IsoPacketDescriptor) {
	cursor := 0
	for i := range packets {
		length := int(packets[i].ActualLength)
		offset := int(packets[i].Offset)
		copy(dst[offset:offset+length], payload[cursor:cursor+length])
		cursor += length
	}
}
