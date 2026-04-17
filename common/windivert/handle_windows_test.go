//go:build windows

package windivert

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

// CTL_CODE macro from Windows DDK:
//
//	(DeviceType<<16) | (Access<<14) | (Function<<2) | Method
func TestCtlCodeMatchesDDK(t *testing.T) {
	t.Parallel()
	// FILE_DEVICE_NETWORK=0x12, FILE_READ_DATA|FILE_WRITE_DATA=3, METHOD_OUT_DIRECT=2
	require.Equal(t, uint32(0x12E486), ctlCode(0x12, 3, 0x921, 2))
	// FILE_READ_DATA=1, METHOD_OUT_DIRECT=2
	require.Equal(t, uint32(0x12648E), ctlCode(0x12, 1, 0x923, 2))
}

// Baked-in against windivert_device.h @ v2.2.2. A mismatch here means the
// kernel will reject every ioctl with ERROR_INVALID_FUNCTION.
func TestIoctlCodesMatchUpstream(t *testing.T) {
	t.Parallel()
	require.Equal(t, uint32(0x12E486), ioctlInitialize)
	require.Equal(t, uint32(0x12E489), ioctlStartup)
	require.Equal(t, uint32(0x12648E), ioctlRecv)
	require.Equal(t, uint32(0x12E491), ioctlSend)
}

func TestBuildIoctlInitialize(t *testing.T) {
	t.Parallel()
	buf := buildIoctlInitialize(LayerNetwork, 100, FlagSendOnly)
	require.Equal(t, uint32(LayerNetwork), binary.LittleEndian.Uint32(buf[0:4]))
	// Driver expects priority+PriorityHighest(30000) so the range is non-negative.
	require.Equal(t, uint32(30100), binary.LittleEndian.Uint32(buf[4:8]))
	require.Equal(t, uint64(FlagSendOnly), binary.LittleEndian.Uint64(buf[8:16]))
}

func TestBuildIoctlInitializePriorityRange(t *testing.T) {
	t.Parallel()
	lowest := buildIoctlInitialize(LayerNetwork, PriorityLowest, 0)
	require.Equal(t, uint32(0), binary.LittleEndian.Uint32(lowest[4:8]))
	highest := buildIoctlInitialize(LayerNetwork, PriorityHighest, 0)
	require.Equal(t, uint32(60000), binary.LittleEndian.Uint32(highest[4:8]))
	zero := buildIoctlInitialize(LayerNetwork, 0, 0)
	require.Equal(t, uint32(30000), binary.LittleEndian.Uint32(zero[4:8]))
}

func TestBuildIoctlStartup(t *testing.T) {
	t.Parallel()
	flags := filterFlagOutbound | filterFlagIP
	buf := buildIoctlStartup(flags)
	require.Equal(t, flags, binary.LittleEndian.Uint64(buf[0:8]))
	// The second quad-word is unused for STARTUP.
	require.Equal(t, uint64(0), binary.LittleEndian.Uint64(buf[8:16]))
}

func TestBuildIoctlRecvEmbedsAddressPointer(t *testing.T) {
	t.Parallel()
	addr := &Address{Timestamp: 0xCAFEBABE}
	buf := buildIoctlRecv(addr)
	require.Equal(t, uint64(uintptr(unsafe.Pointer(addr))),
		binary.LittleEndian.Uint64(buf[0:8]))
	// RECV does not carry an address length; driver writes full Address back.
	require.Equal(t, uint64(0), binary.LittleEndian.Uint64(buf[8:16]))
}

func TestBuildIoctlSendEmbedsAddressPointerAndSize(t *testing.T) {
	t.Parallel()
	addr := &Address{}
	buf := buildIoctlSend(addr)
	require.Equal(t, uint64(uintptr(unsafe.Pointer(addr))),
		binary.LittleEndian.Uint64(buf[0:8]))
	require.Equal(t, uint64(unsafe.Sizeof(Address{})),
		binary.LittleEndian.Uint64(buf[8:16]))
	require.Equal(t, uint64(80), binary.LittleEndian.Uint64(buf[8:16]))
}

func TestValidateOpenArgsLayer(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateOpenArgs(LayerNetwork, 0, 0))
	require.Error(t, validateOpenArgs(Layer(1), 0, 0))
	require.Error(t, validateOpenArgs(Layer(42), 0, 0))
}

func TestValidateOpenArgsPriorityBounds(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateOpenArgs(LayerNetwork, PriorityHighest, 0))
	require.NoError(t, validateOpenArgs(LayerNetwork, PriorityLowest, 0))
	require.NoError(t, validateOpenArgs(LayerNetwork, 0, 0))
	require.Error(t, validateOpenArgs(LayerNetwork, PriorityHighest+1, 0))
	require.Error(t, validateOpenArgs(LayerNetwork, PriorityLowest-1, 0))
}

func TestValidateOpenArgsFlags(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateOpenArgs(LayerNetwork, 0, 0))
	require.NoError(t, validateOpenArgs(LayerNetwork, 0, FlagSendOnly))
	require.NoError(t, validateOpenArgs(LayerNetwork, 0, FlagSniff))
	// Sniff and send-only describe contradictory handle roles.
	require.Error(t, validateOpenArgs(LayerNetwork, 0, FlagSniff|FlagSendOnly))
	// Unknown flag bits must be rejected to surface caller mistakes early.
	require.Error(t, validateOpenArgs(LayerNetwork, 0, Flag(0x10)))
	require.Error(t, validateOpenArgs(LayerNetwork, 0, FlagSendOnly|Flag(0x10)))
}
