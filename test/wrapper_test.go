package main

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestOptionsWrapper(t *testing.T) {
	inbound := option.LegacyInbound{
		Type: C.TypeHTTP,
		HTTPOptions: option.HTTPMixedInboundOptions{
			InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
				TLS: &option.InboundTLSOptions{
					Enabled: true,
				},
			},
		},
	}
	rawOptions, err := inbound.RawOptions()
	require.NoError(t, err)
	tlsOptionsWrapper, loaded := rawOptions.(option.InboundTLSOptionsWrapper)
	require.True(t, loaded, "find inbound tls options")
	tlsOptions := tlsOptionsWrapper.TakeInboundTLSOptions()
	require.NotNil(t, tlsOptions, "find inbound tls options")
	tlsOptions.Enabled = false
	tlsOptionsWrapper.ReplaceInboundTLSOptions(tlsOptions)
	require.False(t, inbound.HTTPOptions.TLS.Enabled, "replace tls enabled")
}
