package hosts_test

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/dns/transport/hosts"

	"github.com/stretchr/testify/require"
)

func TestHosts(t *testing.T) {
	t.Parallel()
	require.Equal(t, []netip.Addr{netip.AddrFrom4([4]byte{127, 0, 0, 1}), netip.IPv6Loopback()}, hosts.NewFile("testdata/hosts").Lookup("localhost"))
	require.NotEmpty(t, hosts.NewFile(hosts.DefaultPath).Lookup("localhost"))
}
