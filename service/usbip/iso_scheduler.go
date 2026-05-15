package usbip

type FrameOracle interface {
	CurrentFrame() uint64
}

type IsoScheduler struct {
	oracle FrameOracle
}

func NewIsoScheduler(oracle FrameOracle) *IsoScheduler {
	return &IsoScheduler{oracle: oracle}
}

func (s *IsoScheduler) EncodeSubmit(base SubmitCommand, ciFrame uint8, asap bool) SubmitCommand {
	if asap {
		base.TransferFlags |= usbipTransferFlagIsoASAP
		base.StartFrame = 0
		return base
	}
	rebased := RebaseFrame(s.oracle.CurrentFrame(), ciFrame)
	base.StartFrame = int32(uint32(rebased))
	return base
}

// RebaseFrame returns the absolute USB frame number whose low 8 bits equal
// low8 and is closest to currentFrame. Apple's IOUSBHostCI iso messages only
// carry the low 8 bits of the frame number; this rebases against the
// controller's monotonic frame counter so the wire start_frame survives the
// 256-frame wrap.
func RebaseFrame(currentFrame uint64, low8 uint8) uint64 {
	target := uint64(low8)
	if currentFrame < target {
		return target
	}
	delta := currentFrame - target
	high := delta / 256
	base := high*256 + target
	nextBase := base + 256
	if nextBase-currentFrame <= currentFrame-base {
		return nextBase
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
