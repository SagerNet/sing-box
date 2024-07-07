package sniff_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff"

	"github.com/stretchr/testify/require"
)

func TestSniffHTTP1(t *testing.T) {
	t.Parallel()
	pkt := "GET / HTTP/1.1\r\nHost: www.google.com\r\nAccept: */*\r\n\r\n"
	var metadata adapter.InboundContext
	err := sniff.HTTPHost(context.Background(), &metadata, strings.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, metadata.Domain, "www.google.com")
}

func TestSniffHTTP1WithPort(t *testing.T) {
	t.Parallel()
	pkt := "GET / HTTP/1.1\r\nHost: www.gov.cn:8080\r\nAccept: */*\r\n\r\n"
	var metadata adapter.InboundContext
	err := sniff.HTTPHost(context.Background(), &metadata, strings.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, metadata.Domain, "www.gov.cn")
}
