package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"net"
	"testing"
	"time"
	"sync"
	"net/http"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"

	"golang.org/x/net/http2"
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
