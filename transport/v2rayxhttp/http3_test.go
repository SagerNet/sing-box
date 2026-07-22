//go:build with_quic

package v2rayxhttp_test

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestXHTTPHTTP3(t *testing.T) {
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode, func(t *testing.T) {
			certificate, key, ca := interopCertificate(t)
			serverPort, proxyPort := interopPort(t), interopPort(t)
			targetPort, closeTarget := interopEchoServer(t)
			defer closeTarget()
			userID := interopUUID(t)
			xhttp := option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: mode,
				QUIC: option.QUICOptions{
					HTTP2Options:            option.HTTP2Options{MaxConcurrentStreams: 8},
					DisablePathMTUDiscovery: true,
				},
			}

			startInteropBox(t, option.Options{
				Inbounds: []option.Inbound{{
					Type: C.TypeVMess,
					Options: &option.VMessInboundOptions{
						ListenOptions: interopListen(serverPort), Users: []option.VMessUser{{UUID: userID}},
						InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{"h3"}}},
						Transport:                  interopTransportOptions(xhttp),
					},
				}},
				Outbounds: []option.Outbound{{Type: C.TypeDirect}},
			})
			startInteropBox(t, option.Options{
				Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
				Outbounds: []option.Outbound{{
					Type: C.TypeVMess,
					Options: &option.VMessOutboundOptions{
						ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
						OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{"h3"}}},
						Transport:                   interopTransportOptions(xhttp),
					},
				}},
			})
			interopPingPong(t, proxyPort, targetPort)
		})
	}
}
