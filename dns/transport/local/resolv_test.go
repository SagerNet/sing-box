package local

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDNSReadConfig(t *testing.T) {
	t.Parallel()
	require.NoError(t, dnsReadConfig(nil, "/etc/resolv.conf").err)
}
