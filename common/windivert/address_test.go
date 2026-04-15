package windivert

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestAddressSize(t *testing.T) {
	t.Parallel()
	require.Equal(t, uintptr(80), unsafe.Sizeof(Address{}))
}

func TestAddressIPv6(t *testing.T) {
	t.Parallel()
	var addr Address
	require.False(t, addr.IPv6())
	addr.bits = 1 << addrBitIPv6
	require.True(t, addr.IPv6())
}

func TestAddressSetIPChecksum(t *testing.T) {
	t.Parallel()
	var addr Address
	addr.SetIPChecksum(true)
	require.Equal(t, uint32(1<<addrBitIPChecksum), addr.bits)
	addr.SetIPChecksum(false)
	require.Equal(t, uint32(0), addr.bits)
}

func TestAddressSetTCPChecksum(t *testing.T) {
	t.Parallel()
	var addr Address
	addr.SetTCPChecksum(true)
	require.Equal(t, uint32(1<<addrBitTCPChecksum), addr.bits)
	addr.SetTCPChecksum(false)
	require.Equal(t, uint32(0), addr.bits)
}

// Setters must not disturb sibling bits.
func TestAddressFlagBitsIndependent(t *testing.T) {
	t.Parallel()
	var addr Address
	addr.SetIPChecksum(true)
	addr.SetTCPChecksum(true)
	addr.bits |= 1 << addrBitIPv6

	addr.SetIPChecksum(false)
	require.False(t, addr.bits&(1<<addrBitIPChecksum) != 0)
	require.True(t, addr.bits&(1<<addrBitTCPChecksum) != 0)
	require.True(t, addr.bits&(1<<addrBitIPv6) != 0)
}
