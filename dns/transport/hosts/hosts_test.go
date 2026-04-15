package hosts

import (
	"net/netip"
	"os"
	"runtime"
	"testing"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/stretchr/testify/require"
)

func TestHosts(t *testing.T) {
	t.Parallel()
	require.Equal(t, []netip.Addr{netip.AddrFrom4([4]byte{127, 0, 0, 1}), netip.IPv6Loopback()}, NewFile("testdata/hosts").Lookup("localhost"))
	if runtime.GOOS != "windows" {
		defaultPathResolved, err := defaultPath()
		if err != nil {
			t.Fatal(E.Cause(err, "resolve default hosts path"))
		}
		content, readErr := os.ReadFile(defaultPathResolved)
		require.NoError(t, readErr)
		hFile := NewFile(defaultPathResolved)
		if len(hFile.Lookup("localhost")) == 0 {
			t.Fatal("failed to resolve localhost: ", defaultPathResolved, ": \n", string(content))
		}
	}
}
