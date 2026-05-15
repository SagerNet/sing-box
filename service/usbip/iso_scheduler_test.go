package usbip

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type frameOracleStub uint64

func (f frameOracleStub) CurrentFrame() uint64 { return uint64(f) }

func TestRebaseFrameAcrossWraps(t *testing.T) {
	tests := []struct {
		current uint64
		low8    uint8
		want    uint64
	}{
		{0, 0, 0},
		{0, 5, 5},
		{255, 0, 256},
		{255, 255, 255},
		{300, 0, 256},
		{300, 44, 300},
		{300, 100, 356},
		{300, 200, 200},
		{700, 10, 778},
		{700, 188, 700},
		{1<<32 - 1, 0, 1 << 32},
		{128, 0, 256},
		{127, 0, 0},
		{1024, 254, 1022},
	}
	for _, tt := range tests {
		got := RebaseFrame(tt.current, tt.low8)
		require.Equalf(t, tt.want, got, "RebaseFrame(%d, %d)", tt.current, tt.low8)
	}
}

func TestIsoSchedulerEncodeSubmitASAP(t *testing.T) {
	scheduler := NewIsoScheduler(frameOracleStub(500))
	command := scheduler.EncodeSubmit(SubmitCommand{
		Header:               DataHeader{Command: CmdSubmit, Direction: USBIPDirOut, Endpoint: 1},
		TransferBufferLength: 16,
	}, 99, true)
	require.Equal(t, int32(usbipTransferFlagIsoASAP), command.TransferFlags&usbipTransferFlagIsoASAP)
	require.Equal(t, int32(0), command.StartFrame)
}

func TestIsoSchedulerEncodeSubmitScheduled(t *testing.T) {
	scheduler := NewIsoScheduler(frameOracleStub(700))
	command := scheduler.EncodeSubmit(SubmitCommand{
		Header:               DataHeader{Command: CmdSubmit, Direction: USBIPDirIn, Endpoint: 0x82},
		TransferBufferLength: 64,
	}, 10, false)
	require.Equal(t, int32(0), command.TransferFlags&usbipTransferFlagIsoASAP)
	require.Equal(t, int32(778), command.StartFrame)
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
