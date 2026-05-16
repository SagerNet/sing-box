//go:build linux || (darwin && cgo)

package usbip

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

func ScatterIsoResponse(dst, payload []byte, packets []IsoPacketDescriptor) {
	cursor := 0
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			continue
		}
		if cursor+length > len(payload) {
			length = len(payload) - cursor
			if length <= 0 {
				return
			}
		}
		offset := int(packets[i].Offset)
		if offset < 0 || offset >= len(dst) {
			cursor += length
			continue
		}
		end := min(offset+length, len(dst))
		copy(dst[offset:end], payload[cursor:cursor+(end-offset)])
		cursor += length
	}
}
