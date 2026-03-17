package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/debug"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/protocol/socks"

	"golang.org/x/net/http2"
	"github.com/stretchr/testify/require"
)

func TestAnyTLS2Self(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	t.Cleanup(func() {
		time.Sleep(7 * time.Second)
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{{
						Name:     "sekai",
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeAnyTLS2,
				Tag:  "anytls2-out",
				Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password:                 "password",
					IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
					RuleAction: option.RuleAction{
						Action:       C.RuleActionTypeRoute,
						RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"},
					},
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestAnyTLS2WebsocketSelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	t.Cleanup(func() {
		time.Sleep(7 * time.Second)
	})
	transport := &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeWebsocket,
		WebsocketOptions: option.V2RayWebsocketOptions{
			Path: "/anytls",
		},
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{{
						Name:     "sekai",
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeAnyTLS2,
				Tag:  "anytls2-out",
				Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password:                 "password",
					IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
					RuleAction: option.RuleAction{
						Action:       C.RuleActionTypeRoute,
						RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"},
					},
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

// TestAnyTLS2HTTP2ReverseProxySelf tests AnyTLS2 over HTTP/2 behind an in-process TLS-terminating
// reverse proxy. The proxy (on otherPort) accepts TLS+HTTP/2 from the AnyTLS2 client and
// forwards h2c (cleartext HTTP/2) to the AnyTLS2 backend (on serverPort, no TLS). This mirrors a
// real nginx deployment where nginx terminates TLS and proxies to the backend over h2c.
func TestAnyTLS2HTTP2ReverseProxySelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	httpPath := "/anytls-http2"
	// Run goleak cleanup last, after all connections have drained.
	t.Cleanup(func() {
		time.Sleep(7 * time.Second)
	})

	// Build the in-process TLS+HTTP/2 reverse proxy.
	cert, err := tls.LoadX509KeyPair(certPem, keyPem)
	if err != nil {
		t.Fatal(err)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	backendURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", serverPort))
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxy.FlushInterval = -1 // enable response streaming; required for bidirectional tunnel
	// Use h2c (cleartext HTTP/2) to the backend so the backend's HTTP/2 server sees a properly
	// decoded request.Body stream rather than chunked HTTP/1.1 bytes.
	//
	// golang.org/x/net/http2/h2c hijacks connections from the standard net/http.Server, so
	// httpServer.Close() cannot close those goroutines. We track every backend TCP connection
	// made by the h2c transport and force-close them in a separate cleanup step, which unblocks
	// all h2c goroutines so goleak sees no leaks.
	var backendConnsMu sync.Mutex
	var backendConns []net.Conn
	proxy.Transport = &http2.Transport{
		AllowHTTP: true, // h2c: allow HTTP/2 over cleartext
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			conn, dialErr := (&net.Dialer{}).DialContext(ctx, network, addr)
			if dialErr != nil {
				return nil, dialErr
			}
			backendConnsMu.Lock()
			backendConns = append(backendConns, conn)
			backendConnsMu.Unlock()
			return conn, nil
		},
	}
	// Force-close tracked backend conns so h2c goroutines (which httpServer.Close cannot reach)
	// exit before the 7s goleak sleep. This cleanup is registered before httpSrv.Close, so with
	// LIFO cleanup order it executes after httpSrv.Close and still guarantees h2c goroutine drain.
	t.Cleanup(func() {
		backendConnsMu.Lock()
		for _, c := range backendConns {
			c.Close()
		}
		backendConnsMu.Unlock()
	})
	httpSrv := &http.Server{Handler: proxy, TLSConfig: tlsCfg}
	if err := http2.ConfigureServer(httpSrv, nil); err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", otherPort), tlsCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { httpSrv.Close() })
	serveErrCh := make(chan error, 1)
	go func() {
		err := httpSrv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErrCh <- err
		}
	}()
	select {
	case serveErr := <-serveErrCh:
		t.Fatalf("reverse proxy serve failed: %v", serveErr)
	case <-time.After(100 * time.Millisecond):
	}

	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				// Backend: no TLS, plain HTTP — nginx proxies here.
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{{
						Name:     "sekai",
						Password: "password",
					}},
					Transport: &option.V2RayTransportOptions{
						Type: C.V2RayTransportTypeHTTP,
						HTTPOptions: option.V2RayHTTPOptions{
							Path: httpPath,
						},
					},
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeAnyTLS2,
				Tag:  "anytls2-out",
				Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{
						// Client connects to the TLS reverse proxy, not the backend directly.
						Server:     "127.0.0.1",
						ServerPort: otherPort,
					},
					Password:                 "password",
					IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Transport: &option.V2RayTransportOptions{
						Type: C.V2RayTransportTypeHTTP,
						HTTPOptions: option.V2RayHTTPOptions{
							Path: httpPath,
						},
					},
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
					RuleAction: option.RuleAction{
						Action:       C.RuleActionTypeRoute,
						RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"},
					},
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

// TestAnyTLS2QUICSelf tests AnyTLS2 over QUIC transport end-to-end.
// QUIC provides per-stream delivery without TCP head-of-line blocking, so
// AnyTLS2 session multiplexing on top of QUIC is completely free of TCP HoL blocking.
// Unlike WebSocket/HTTP/2 (which can be reverse-proxied by nginx), QUIC connects
// the client directly to the AnyTLS2 server and requires a dedicated UDP port.
// Requires build tag: with_quic.
func TestAnyTLS2QUICSelf(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	t.Cleanup(func() {
		time.Sleep(7 * time.Second)
	})
	transport := &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeQUIC,
	}
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: clientPort,
					},
				},
			},
			{
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{
						Listen:     common.Ptr(badoption.Addr(netip.IPv4Unspecified())),
						ListenPort: serverPort,
					},
					Users: []option.AnyTLSUser{{
						Name:     "sekai",
						Password: "password",
					}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{
						TLS: &option.InboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
							KeyPath:         keyPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{
				Type: C.TypeAnyTLS2,
				Tag:  "anytls2-out",
				Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{
						Server:     "127.0.0.1",
						ServerPort: serverPort,
					},
					Password:                 "password",
					IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
						TLS: &option.OutboundTLSOptions{
							Enabled:         true,
							ServerName:      "example.org",
							CertificatePath: certPem,
						},
					},
					Transport: transport,
				},
			},
		},
		Route: &option.RouteOptions{
			Rules: []option.Rule{{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultRule{
					RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
					RuleAction: option.RuleAction{
						Action:       C.RuleActionTypeRoute,
						RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"},
					},
				},
			}},
		},
	})
	testSuit(t, clientPort, testPort)
}

func TestAnyTLS2NegativeWrongPassword(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	t.Cleanup(func() {
		time.Sleep(7 * time.Second)
	})
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{
			{
				Type: C.TypeMixed,
				Tag:  "mixed-in",
				Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{
					Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort,
				}},
			},
			{
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{
						Enabled: true, ServerName: "example.org", CertificatePath: certPem, KeyPath: keyPem,
					}},
				},
			},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect},
			{Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
				ServerOptions:             option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
				Password:                  "wrong-password",
				IdleSessionCheckInterval:  badoption.Duration(6 * time.Second),
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{
					Enabled: true, ServerName: "example.org", CertificatePath: certPem,
				}},
			}},
		},
		Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}},
			RuleAction:     option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}},
		}}}},
	})
	assertAnyTLS2ProxyTCPFail(t, clientPort, testPort)
}

func TestAnyTLS2NegativeTransportPathMismatch(t *testing.T) {
	_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
	t.Run("websocket", func(t *testing.T) {
		t.Cleanup(func() {
			time.Sleep(7 * time.Second)
		})
		startInstance(t, option.Options{
			Inbounds: []option.Inbound{
				{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort}}},
				{Type: C.TypeAnyTLS2, Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem, KeyPath: keyPem}},
					Transport: &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: option.V2RayWebsocketOptions{Path: "/anytls-a"}},
				}},
			},
			Outbounds: []option.Outbound{
				{Type: C.TypeDirect},
				{Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: "password", IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem}},
					Transport:                   &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: option.V2RayWebsocketOptions{Path: "/anytls-b"}},
				}},
			},
			Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}}, RuleAction: option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}}}}}},
		})
		assertAnyTLS2ProxyTCPFail(t, clientPort, testPort)
	})

	t.Run("http", func(t *testing.T) {
		t.Cleanup(func() {
			time.Sleep(7 * time.Second)
		})
		startInstance(t, option.Options{
			Inbounds: []option.Inbound{
				{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort}}},
				{Type: C.TypeAnyTLS2, Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem, KeyPath: keyPem}},
					Transport: &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTP, HTTPOptions: option.V2RayHTTPOptions{Path: "/anytls-a"}},
				}},
			},
			Outbounds: []option.Outbound{
				{Type: C.TypeDirect},
				{Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
					ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: "password", IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
					OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem}},
					Transport:                   &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTP, HTTPOptions: option.V2RayHTTPOptions{Path: "/anytls-b"}},
				}},
			},
			Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}}, RuleAction: option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}}}}}},
		})
		assertAnyTLS2ProxyTCPFail(t, clientPort, testPort)
	})
}

func TestAnyTLS2NegativeStandaloneProbeNoResponse(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeAnyTLS2,
			Options: &option.AnyTLS2InboundOptions{
				ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
				Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", F.ToString(serverPort)), 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(2*time.Second)))
	_, err = conn.Write(bytes.Repeat([]byte{'A'}, 64))
	require.NoError(t, err)
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if n > 0 {
		require.False(t, bytes.HasPrefix(buf[:n], []byte("HTTP/")), "standalone anytls2 should not expose HTTP fingerprint")
	}
	if err == nil {
		t.Fatalf("unexpected probe response in standalone mode: %q", string(buf[:n]))
	}
}

func TestAnyTLS2NegativePathModeHTTPFailResponse(t *testing.T) {
	startInstance(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeAnyTLS2,
			Options: &option.AnyTLS2InboundOptions{
				ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
				Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
				Transport:     &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: option.V2RayWebsocketOptions{Path: "/anytls"}},
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})

	resp, err := http.Get("http://127.0.0.1:" + F.ToString(serverPort) + "/not-anytls")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAnyTLS2NegativeEdgeCases(t *testing.T) {
	t.Run("empty users behaves as deny-all", func(t *testing.T) {
		_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
		t.Cleanup(func() {
			time.Sleep(7 * time.Second)
		})
		startInstance(t, option.Options{
			Inbounds: []option.Inbound{
				{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort}}},
				{Type: C.TypeAnyTLS2, Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem, KeyPath: keyPem}},
				}},
			},
			Outbounds: []option.Outbound{{Type: C.TypeDirect}, {Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: "password", IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem}},
			}}},
			Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}}, RuleAction: option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}}}}}},
		})
		assertAnyTLS2ProxyTCPFail(t, clientPort, testPort)
	})

	t.Run("invalid transport type rejected", func(t *testing.T) {
		err := startInstanceExpectError(t, option.Options{
			Inbounds: []option.Inbound{{
				Type: C.TypeAnyTLS2,
				Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					Transport:     &option.V2RayTransportOptions{Type: "invalid_type"},
				},
			}},
			Outbounds: []option.Outbound{{Type: C.TypeDirect}},
		})
		require.Error(t, err)
	})

	t.Run("outbound without tls rejected", func(t *testing.T) {
		err := startInstanceExpectError(t, option.Options{
			Inbounds: []option.Inbound{{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort}}}},
			Outbounds: []option.Outbound{{Type: C.TypeDirect}, {Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: "password",
			}}},
			Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}}, RuleAction: option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}}}}}},
		})
		require.Error(t, err)
	})

	t.Run("json invalid field types rejected", func(t *testing.T) {
		var opts option.Options
		err := opts.UnmarshalJSONContext(globalCtx, []byte(`{
			"inbounds": [{
				"type": "anytls2",
				"listen": "127.0.0.1",
				"listen_port": "not-a-number",
				"users": [{"name": "u", "password": 123}]
			}],
			"outbounds": [{"type": "direct"}]
		}`))
		require.Error(t, err)
	})

	t.Run("overlong password mismatch fails", func(t *testing.T) {
		_, certPem, keyPem := createSelfSignedCertificate(t, "example.org")
		t.Cleanup(func() {
			time.Sleep(7 * time.Second)
		})
		startInstance(t, option.Options{
			Inbounds: []option.Inbound{
				{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: clientPort}}},
				{Type: C.TypeAnyTLS2, Options: &option.AnyTLS2InboundOptions{
					ListenOptions: option.ListenOptions{Listen: common.Ptr(badoption.Addr(netip.IPv4Unspecified())), ListenPort: serverPort},
					Users:         []option.AnyTLSUser{{Name: "sekai", Password: "password"}},
					InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem, KeyPath: keyPem}},
				}},
			},
			Outbounds: []option.Outbound{{Type: C.TypeDirect}, {Type: C.TypeAnyTLS2, Tag: "anytls2-out", Options: &option.AnyTLS2OutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: strings.Repeat("x", 4096), IdleSessionCheckInterval: badoption.Duration(6 * time.Second),
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "example.org", CertificatePath: certPem}},
			}}},
			Route: &option.RouteOptions{Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RawDefaultRule: option.RawDefaultRule{Inbound: []string{"mixed-in"}}, RuleAction: option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "anytls2-out"}}}}}},
		})
		assertAnyTLS2ProxyTCPFail(t, clientPort, testPort)
	})
}

func startInstanceExpectError(t *testing.T, options option.Options) error {
	if debug.Enabled {
		options.Log = &option.LogOptions{Level: "trace"}
	} else {
		options.Log = &option.LogOptions{Level: "warning"}
	}
	ctx, cancel := context.WithCancel(globalCtx)
	defer cancel()
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		return err
	}
	defer instance.Close()
	return instance.Start()
}

func assertAnyTLS2ProxyTCPFail(t *testing.T, proxyPort uint16, destinationPort uint16) {
	t.Helper()
	listener, err := listen("tcp", ":"+F.ToString(destinationPort))
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		accepted <- struct{}{}
		defer conn.Close()
		buf := make([]byte, 4)
		if _, readErr := io.ReadFull(conn, buf); readErr != nil {
			return
		}
		_, _ = conn.Write([]byte("pong"))
	}()

	dialer := socks.NewClient(N.SystemDialer, M.ParseSocksaddrHostPort("127.0.0.1", proxyPort), socks.Version5, "", "")
	conn, err := dialer.DialContext(context.Background(), "tcp", M.ParseSocksaddrHostPort("127.0.0.1", destinationPort))
	if err == nil {
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		_, err = conn.Write([]byte("ping"))
		if err == nil {
			buf := make([]byte, 4)
			n, readErr := io.ReadFull(conn, buf)
			if readErr == nil && n == 4 && string(buf) == "pong" {
				t.Fatalf("unexpected success: AnyTLS2 negative case reached destination")
			}
		}
	}

	select {
	case <-accepted:
		t.Fatalf("unexpected destination accept in AnyTLS2 negative case")
	case <-time.After(300 * time.Millisecond):
	}
}
