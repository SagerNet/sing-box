//go:build !with_quic

package v2rayxhttp

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

func TestHTTP3RequiresQUICBuildTag(t *testing.T) {
	tlsConfig, err := tls.NewClient(context.Background(), logger.NOP(), "xhttp.test", option.OutboundTLSOptions{
		Enabled: true, ServerName: "xhttp.test", Insecure: true, ALPN: []string{"h3"},
	})
	require.NoError(t, err)
	_, err = NewClient(context.Background(), nil, M.ParseSocksaddrHostPort("127.0.0.1", 443), option.V2RayXHTTPOptions{}, tlsConfig)
	require.ErrorIs(t, err, C.ErrQUICNotIncluded)
}
