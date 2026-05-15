package usbip

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRebaseFrameIsLeastUpperBound(t *testing.T) {
	samples := []uint64{
		0, 1, 127, 128, 255, 256, 300, 1024,
		1<<31 - 1, 1 << 31, 1<<32 - 1, 1 << 32, 1 << 63,
	}
	for _, current := range samples {
		for low := range 256 {
			got := RebaseFrame(current, uint8(low))
			require.GreaterOrEqualf(t, got, current, "current=%d low=%d", current, low)
			require.Lessf(t, got-current, uint64(256), "current=%d low=%d got=%d", current, low, got)
			require.Equalf(t, uint64(low), got&0xff, "current=%d low=%d got=%d", current, low, got)
		}
	}
}

func TestEncodeIsoSubmitASAP(t *testing.T) {
	command := EncodeIsoSubmit(500, SubmitCommand{
		Header:               DataHeader{Command: CmdSubmit, Direction: USBIPDirOut, Endpoint: 1},
		TransferBufferLength: 16,
	}, 99, true)
	require.Equal(t, int32(usbipTransferFlagIsoASAP), command.TransferFlags&usbipTransferFlagIsoASAP)
	require.Equal(t, int32(0), command.StartFrame)
}

func TestEncodeIsoSubmitScheduled(t *testing.T) {
	command := EncodeIsoSubmit(300, SubmitCommand{
		Header:               DataHeader{Command: CmdSubmit, Direction: USBIPDirIn, Endpoint: 0x82},
		TransferBufferLength: 64,
	}, 200, false)
	require.Equal(t, int32(0), command.TransferFlags&usbipTransferFlagIsoASAP)
	require.Equal(t, int32(456), command.StartFrame)
}

func TestScatterIsoResponseHonorsPacketOffsets(t *testing.T) {
	dst := make([]byte, 32)
	payload := []byte{1, 1, 1, 1, 2, 2, 3, 3, 3, 3, 3, 3}
	packets := []IsoPacketDescriptor{
		{Offset: 0, Length: 4, ActualLength: 4},
		{Offset: 8, Length: 2, ActualLength: 2},
		{Offset: 16, Length: 6, ActualLength: 6},
	}
	ScatterIsoResponse(dst, payload, packets)
	require.Equal(t, []byte{1, 1, 1, 1, 0, 0, 0, 0, 2, 2, 0, 0, 0, 0, 0, 0, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, dst)
}

func TestScatterIsoResponseClampsOverlongPacket(t *testing.T) {
	dst := make([]byte, 8)
	payload := []byte{9, 9, 9, 9, 9, 9, 9, 9}
	packets := []IsoPacketDescriptor{
		{Offset: 4, Length: 8, ActualLength: 8},
	}
	ScatterIsoResponse(dst, payload, packets)
	require.Equal(t, []byte{0, 0, 0, 0, 9, 9, 9, 9}, dst)
}
