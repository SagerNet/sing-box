package local

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDNSReadConfig(t *testing.T) {
	t.Parallel()
	require.NoError(t, dnsReadConfig(context.Background(), "/etc/resolv.conf").err)
}
