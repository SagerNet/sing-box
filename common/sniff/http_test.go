package sniff_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/common/sniff"

	"github.com/stretchr/testify/require"
)

func TestHTTP1(t *testing.T) {
	t.Parallel()
	pkt := "GET / HTTP/1.1\r\nHost: www.google.com\r\nAccept: */*\r\n\r\n"
	metadata, err := sniff.HTTPHost(context.Background(), strings.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, metadata.Domain, "www.google.com")
}

func TestHTTP1WithPort(t *testing.T) {
	t.Parallel()
	pkt := "GET / HTTP/1.1\r\nHost: www.gov.cn:8080\r\nAccept: */*\r\n\r\n"
	metadata, err := sniff.HTTPHost(context.Background(), strings.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, metadata.Domain, "www.gov.cn")
}
