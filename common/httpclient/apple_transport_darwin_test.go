//go:build darwin && cgo

package httpclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/proxybridge"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common/json/badoption"
	commonLogger "github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const appleHTTPTestTimeout = 5 * time.Second

const appleHTTPRecoveryLoops = 5

type appleHTTPTestDialer struct {
	dialer   net.Dialer
	listener net.ListenConfig
	hostMap  map[string]string
}

type appleHTTPObservedRequest struct {
	method     string
	body       string
	host       string
	values     []string
	protoMajor int
}

type appleHTTPTestServer struct {
	server         *httptest.Server
	baseURL        string
	dialHost       string
	certificate    stdtls.Certificate
	certificatePEM string
	publicKeyHash  []byte
}

type appleTestAnchors struct {
	ref      unsafe.Pointer
	releases int
}

func (a *appleTestAnchors) Retain() adapter.AppleAnchors {
	return a
}

func (a *appleTestAnchors) Release() {
	a.releases++
}

func (a *appleTestAnchors) Ref() unsafe.Pointer {
	return a.ref
}

func TestNewAppleSessionConfig(t *testing.T) {
	serverCertificate, serverCertificatePEM := newAppleHTTPTestCertificate(t, "localhost")
	serverHash := certificatePublicKeySHA256(t, serverCertificate.Certificate[0])
	otherHash := bytes.Repeat([]byte{0x7f}, applePinnedHashSize)

	testCases := []struct {
		name    string
		options option.HTTPClientOptions
		check   func(t *testing.T, config appleSessionConfig)
		wantErr string
	}{
		{
			name: "success with certificate anchors",
			options: option.HTTPClientOptions{
				Version: 2,
				DialerOptions: option.DialerOptions{
					ConnectTimeout: badoption.Duration(2 * time.Second),
				},
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Enabled:     true,
						ServerName:  "localhost",
						MinVersion:  "1.2",
						MaxVersion:  "1.3",
						Certificate: badoption.Listable[string]{serverCertificatePEM},
					},
				},
			},
			check: func(t *testing.T, config appleSessionConfig) {
				t.Helper()
				if config.serverName != "localhost" {
					t.Fatalf("unexpected server name: %q", config.serverName)
				}
				if config.minVersion != stdtls.VersionTLS12 {
					t.Fatalf("unexpected min version: %x", config.minVersion)
				}
				if config.maxVersion != stdtls.VersionTLS13 {
					t.Fatalf("unexpected max version: %x", config.maxVersion)
				}
				if config.insecure {
					t.Fatal("unexpected insecure flag")
				}
				if !config.anchorOnly {
					t.Fatal("expected anchor_only")
				}
				if config.userAnchors == nil {
					t.Fatal("expected user anchors")
				}
				if config.userAnchors.Ref() == nil {
					t.Fatal("expected non-empty user anchors")
				}
				if config.store != nil {
					t.Fatal("unexpected store reference")
				}
				if len(config.pinnedPublicKeySHA256s) != 0 {
					t.Fatalf("unexpected pinned hashes: %d", len(config.pinnedPublicKeySHA256s))
				}
			},
		},
		{
			name: "success with flattened pins",
			options: option.HTTPClientOptions{
				Version: 2,
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Enabled:                    true,
						Insecure:                   true,
						CertificatePublicKeySHA256: badoption.Listable[[]byte]{serverHash, otherHash},
					},
				},
			},
			check: func(t *testing.T, config appleSessionConfig) {
				t.Helper()
				if !config.insecure {
					t.Fatal("expected insecure flag")
				}
				if len(config.pinnedPublicKeySHA256s) != 2*applePinnedHashSize {
					t.Fatalf("unexpected flattened pin length: %d", len(config.pinnedPublicKeySHA256s))
				}
				if !bytes.Equal(config.pinnedPublicKeySHA256s[:applePinnedHashSize], serverHash) {
					t.Fatal("unexpected first pin")
				}
				if !bytes.Equal(config.pinnedPublicKeySHA256s[applePinnedHashSize:], otherHash) {
					t.Fatal("unexpected second pin")
				}
				if config.userAnchors != nil {
					t.Fatal("unexpected user anchors")
				}
				if config.anchorOnly {
					t.Fatal("unexpected anchor_only")
				}
			},
		},
		{
			name:    "http11 unsupported",
			options: option.HTTPClientOptions{Version: 1},
			wantErr: "HTTP/1.1 is unsupported in Apple HTTP engine",
		},
		{
			name:    "http3 unsupported",
			options: option.HTTPClientOptions{Version: 3},
			wantErr: "HTTP/3 is unsupported in Apple HTTP engine",
		},
		{
			name:    "unknown version",
			options: option.HTTPClientOptions{Version: 9},
			wantErr: "unknown HTTP version: 9",
		},
		{
			name: "disable version fallback unsupported",
			options: option.HTTPClientOptions{
				DisableVersionFallback: true,
			},
			wantErr: "disable_version_fallback is unsupported in Apple HTTP engine",
		},
		{
			name: "http2 options unsupported",
			options: option.HTTPClientOptions{
				HTTP2Options: option.HTTP2Options{
					IdleTimeout: badoption.Duration(time.Second),
				},
			},
			wantErr: "HTTP/2 options are unsupported in Apple HTTP engine",
		},
		{
			name: "quic options unsupported",
			options: option.HTTPClientOptions{
				HTTP3Options: option.QUICOptions{
					InitialPacketSize: 1200,
				},
			},
			wantErr: "QUIC options are unsupported in Apple HTTP engine",
		},
		{
			name: "tls engine unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{Engine: "go"},
				},
			},
			wantErr: "tls.engine is unsupported in Apple HTTP engine",
		},
		{
			name: "disable sni unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{DisableSNI: true},
				},
			},
			wantErr: "disable_sni is unsupported in Apple HTTP engine",
		},
		{
			name: "alpn unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						ALPN: badoption.Listable[string]{"h2"},
					},
				},
			},
			wantErr: "tls.alpn is unsupported in Apple HTTP engine",
		},
		{
			name: "cipher suites unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						CipherSuites: badoption.Listable[string]{"TLS_AES_128_GCM_SHA256"},
					},
				},
			},
			wantErr: "cipher_suites is unsupported in Apple HTTP engine",
		},
		{
			name: "curve preferences unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						CurvePreferences: badoption.Listable[option.CurvePreference]{option.CurvePreference(option.X25519)},
					},
				},
			},
			wantErr: "curve_preferences is unsupported in Apple HTTP engine",
		},
		{
			name: "client certificate unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						ClientCertificate: badoption.Listable[string]{"client-certificate"},
						ClientKey:         badoption.Listable[string]{"client-key"},
					},
				},
			},
			wantErr: "client certificate is unsupported in Apple HTTP engine",
		},
		{
			name: "tls fragment unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{Fragment: true},
				},
			},
			wantErr: "tls fragment is unsupported in Apple HTTP engine",
		},
		{
			name: "ktls unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{KernelTx: true},
				},
			},
			wantErr: "ktls is unsupported in Apple HTTP engine",
		},
		{
			name: "ech unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						ECH: &option.OutboundECHOptions{Enabled: true},
					},
				},
			},
			wantErr: "ech is unsupported in Apple HTTP engine",
		},
		{
			name: "utls unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						UTLS: &option.OutboundUTLSOptions{Enabled: true},
					},
				},
			},
			wantErr: "utls is unsupported in Apple HTTP engine",
		},
		{
			name: "reality unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Reality: &option.OutboundRealityOptions{Enabled: true},
					},
				},
			},
			wantErr: "reality is unsupported in Apple HTTP engine",
		},
		{
			name: "pin and certificate conflict",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Certificate:                badoption.Listable[string]{serverCertificatePEM},
						CertificatePublicKeySHA256: badoption.Listable[[]byte]{serverHash},
					},
				},
			},
			wantErr: "certificate_public_key_sha256 is conflict with certificate or certificate_path",
		},
		{
			name: "invalid min version",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{MinVersion: "bogus"},
				},
			},
			wantErr: "parse min_version",
		},
		{
			name: "invalid max version",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{MaxVersion: "bogus"},
				},
			},
			wantErr: "parse max_version",
		},
		{
			name: "invalid pin length",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						CertificatePublicKeySHA256: badoption.Listable[[]byte]{{0x01, 0x02}},
					},
				},
			},
			wantErr: "invalid certificate_public_key_sha256 length: 2",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			config, err := newAppleSessionConfig(context.Background(), testCase.options)
			if testCase.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if testCase.check != nil {
				testCase.check(t, config)
			}
		})
	}
}

func TestAppleTransportVerifyPublicKeySHA256(t *testing.T) {
	serverCertificate, _ := newAppleHTTPTestCertificate(t, "localhost")
	goodHash := certificatePublicKeySHA256(t, serverCertificate.Certificate[0])
	badHash := append([]byte(nil), goodHash...)
	badHash[0] ^= 0xff

	err := verifyApplePinnedPublicKeySHA256(goodHash, serverCertificate.Certificate[0])
	if err != nil {
		t.Fatalf("expected correct pin to succeed: %v", err)
	}

	err = verifyApplePinnedPublicKeySHA256(badHash, serverCertificate.Certificate[0])
	if err == nil {
		t.Fatal("expected incorrect pin to fail")
	}
	if !strings.Contains(err.Error(), "unrecognized remote public key") {
		t.Fatalf("unexpected pin mismatch error: %v", err)
	}

	err = verifyApplePinnedPublicKeySHA256(goodHash[:applePinnedHashSize-1], serverCertificate.Certificate[0])
	if err == nil {
		t.Fatal("expected malformed pin list to fail")
	}
	if !strings.Contains(err.Error(), "invalid pinned public key list") {
		t.Fatalf("unexpected malformed pin error: %v", err)
	}
}

func TestNewAppleTransportClosesSessionConfigOnBridgeFailure(t *testing.T) {
	_, serverCertificatePEM := newAppleHTTPTestCertificate(t, "localhost")
	restoreAppleTransportFactories(t)
	testAnchors := &appleTestAnchors{ref: unsafe.Pointer(new(int))}
	newAppleUserAnchors = func([]byte) (adapter.AppleAnchors, error) {
		return testAnchors, nil
	}
	newAppleProxyBridge = func(context.Context, commonLogger.ContextLogger, string, N.Dialer) (*proxybridge.Bridge, error) {
		return nil, errors.New("bridge boom")
	}

	_, err := newAppleTransport(newAppleHTTPTestContext(), log.NewNOPFactory().NewLogger("httpclient"), &appleHTTPTestDialer{}, appleTransportAnchorOptions(serverCertificatePEM))
	if err == nil || !strings.Contains(err.Error(), "bridge boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if testAnchors.releases != 1 {
		t.Fatalf("expected 1 anchor release, got %d", testAnchors.releases)
	}
}

func TestNewAppleTransportClosesSessionConfigOnSessionFailure(t *testing.T) {
	_, serverCertificatePEM := newAppleHTTPTestCertificate(t, "localhost")
	restoreAppleTransportFactories(t)
	testAnchors := &appleTestAnchors{ref: unsafe.Pointer(new(int))}
	newAppleUserAnchors = func([]byte) (adapter.AppleAnchors, error) {
		return testAnchors, nil
	}
	newAppleTransportSession = func(*appleTransportShared) (unsafe.Pointer, error) {
		return nil, errors.New("session boom")
	}

	_, err := newAppleTransport(newAppleHTTPTestContext(), log.NewNOPFactory().NewLogger("httpclient"), &appleHTTPTestDialer{}, appleTransportAnchorOptions(serverCertificatePEM))
	if err == nil || !strings.Contains(err.Error(), "session boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if testAnchors.releases != 1 {
		t.Fatalf("expected 1 anchor release, got %d", testAnchors.releases)
	}
}

func TestAppleTransportRoundTripHTTPS(t *testing.T) {
	requests := make(chan appleHTTPObservedRequest, 1)
	server := startAppleHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		requests <- appleHTTPObservedRequest{
			method:     r.Method,
			body:       string(body),
			host:       r.Host,
			values:     append([]string(nil), r.Header.Values("X-Test")...),
			protoMajor: r.ProtoMajor,
		}
		w.Header().Set("X-Reply", "apple")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("response body"))
	})

	transport := newAppleHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: appleHTTPServerTLSOptions(server),
		},
	})

	request, err := http.NewRequest(http.MethodPost, server.URL("/roundtrip"), bytes.NewReader([]byte("request body")))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Add("X-Test", "one")
	request.Header.Add("X-Test", "two")
	request.Host = "custom.example"

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	responseBody := readResponseBody(t, response)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}
	if response.Status != "201 Created" {
		t.Fatalf("unexpected status: %q", response.Status)
	}
	if response.Header.Get("X-Reply") != "apple" {
		t.Fatalf("unexpected response header: %q", response.Header.Get("X-Reply"))
	}
	if responseBody != "response body" {
		t.Fatalf("unexpected response body: %q", responseBody)
	}
	if response.ContentLength != int64(len(responseBody)) {
		t.Fatalf("unexpected content length: %d", response.ContentLength)
	}

	observed := waitObservedRequest(t, requests)
	if observed.method != http.MethodPost {
		t.Fatalf("unexpected method: %q", observed.method)
	}
	if observed.body != "request body" {
		t.Fatalf("unexpected request body: %q", observed.body)
	}
	if observed.host != "custom.example" {
		t.Fatalf("unexpected host: %q", observed.host)
	}
	if observed.protoMajor != 2 {
		t.Fatalf("expected HTTP/2 request, got HTTP/%d", observed.protoMajor)
	}
	var normalizedValues []string
	for _, value := range observed.values {
		for _, part := range strings.Split(value, ",") {
			normalizedValues = append(normalizedValues, strings.TrimSpace(part))
		}
	}
	slices.Sort(normalizedValues)
	if !slices.Equal(normalizedValues, []string{"one", "two"}) {
		t.Fatalf("unexpected header values: %#v", observed.values)
	}
}

func TestAppleTransportPinnedPublicKey(t *testing.T) {
	server := startAppleHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pinned"))
	})

	goodTransport := newAppleHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:                    true,
				ServerName:                 "localhost",
				Insecure:                   true,
				CertificatePublicKeySHA256: badoption.Listable[[]byte]{server.publicKeyHash},
			},
		},
	})

	response, err := goodTransport.RoundTrip(newAppleHTTPRequest(t, http.MethodGet, server.URL("/good"), nil))
	if err != nil {
		t.Fatalf("expected pinned request to succeed: %v", err)
	}
	response.Body.Close()

	badHash := append([]byte(nil), server.publicKeyHash...)
	badHash[0] ^= 0xff
	badTransport := newAppleHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:                    true,
				ServerName:                 "localhost",
				Insecure:                   true,
				CertificatePublicKeySHA256: badoption.Listable[[]byte]{badHash},
			},
		},
	})

	response, err = badTransport.RoundTrip(newAppleHTTPRequest(t, http.MethodGet, server.URL("/bad"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected incorrect pinned public key to fail")
	}
}

func TestAppleTransportGuardrails(t *testing.T) {
	testCases := []struct {
		name          string
		options       option.HTTPClientOptions
		buildRequest  func(t *testing.T) *http.Request
		wantErrSubstr string
	}{
		{
			name: "websocket upgrade rejected",
			options: option.HTTPClientOptions{
				Version: 2,
			},
			buildRequest: func(t *testing.T) *http.Request {
				t.Helper()
				request := newAppleHTTPRequest(t, http.MethodGet, "https://localhost/socket", nil)
				request.Header.Set("Connection", "Upgrade")
				request.Header.Set("Upgrade", "websocket")
				return request
			},
			wantErrSubstr: "HTTP upgrade requests are unsupported in Apple HTTP engine",
		},
		{
			name: "missing url rejected",
			options: option.HTTPClientOptions{
				Version: 2,
			},
			buildRequest: func(t *testing.T) *http.Request {
				t.Helper()
				return &http.Request{Method: http.MethodGet}
			},
			wantErrSubstr: "missing request URL",
		},
		{
			name: "unsupported scheme rejected",
			options: option.HTTPClientOptions{
				Version: 2,
			},
			buildRequest: func(t *testing.T) *http.Request {
				t.Helper()
				return newAppleHTTPRequest(t, http.MethodGet, "ftp://localhost/file", nil)
			},
			wantErrSubstr: "unsupported URL scheme: ftp",
		},
		{
			name: "server name mismatch rejected",
			options: option.HTTPClientOptions{
				Version: 2,
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Enabled:    true,
						ServerName: "example.com",
					},
				},
			},
			buildRequest: func(t *testing.T) *http.Request {
				t.Helper()
				return newAppleHTTPRequest(t, http.MethodGet, "https://localhost/path", nil)
			},
			wantErrSubstr: "tls.server_name is unsupported in Apple HTTP engine unless it matches request host",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			transport := newAppleHTTPTestTransport(t, nil, testCase.options)
			response, err := transport.RoundTrip(testCase.buildRequest(t))
			if err == nil {
				response.Body.Close()
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), testCase.wantErrSubstr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAppleTransportCancellationRecovery(t *testing.T) {
	server := startAppleHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/block":
			select {
			case <-r.Context().Done():
				return
			case <-time.After(appleHTTPTestTimeout):
				http.Error(w, "request was not canceled", http.StatusGatewayTimeout)
			}
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})

	transport := newAppleHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: appleHTTPServerTLSOptions(server),
		},
	})

	for index := 0; index < appleHTTPRecoveryLoops; index++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		request := newAppleHTTPRequestWithContext(t, ctx, http.MethodGet, server.URL("/block"), nil)
		response, err := transport.RoundTrip(request)
		cancel()
		if err == nil {
			response.Body.Close()
			t.Fatalf("iteration %d: expected cancellation error", index)
		}
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			t.Fatalf("iteration %d: unexpected cancellation error: %v", index, err)
		}

		response, err = transport.RoundTrip(newAppleHTTPRequest(t, http.MethodGet, server.URL("/ok"), nil))
		if err != nil {
			t.Fatalf("iteration %d: follow-up request failed: %v", index, err)
		}
		if body := readResponseBody(t, response); body != "ok" {
			response.Body.Close()
			t.Fatalf("iteration %d: unexpected follow-up body: %q", index, body)
		}
		response.Body.Close()
	}
}

func TestAppleTransportLifecycle(t *testing.T) {
	server := startAppleHTTPTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	transport := newAppleHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: appleHTTPServerTLSOptions(server),
		},
	})

	assertAppleHTTPSucceeds(t, transport, server.URL("/original"))

	transport.CloseIdleConnections()
	assertAppleHTTPSucceeds(t, transport, server.URL("/reset"))

	innerTransport := transport.(*appleTransport)
	err := innerTransport.Close()
	if err != nil {
		t.Fatal(err)
	}

	response, err := innerTransport.RoundTrip(newAppleHTTPRequest(t, http.MethodGet, server.URL("/closed"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected closed transport to fail")
	}
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("unexpected closed transport error: %v", err)
	}
}

func startAppleHTTPTestServer(t *testing.T, handler http.HandlerFunc) *appleHTTPTestServer {
	t.Helper()

	serverCertificate, serverCertificatePEM := newAppleHTTPTestCertificate(t, "localhost")
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.TLS = &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
	}
	server.StartTLS()
	t.Cleanup(server.Close)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	baseURL := *parsedURL
	baseURL.Host = net.JoinHostPort("localhost", parsedURL.Port())

	return &appleHTTPTestServer{
		server:         server,
		baseURL:        baseURL.String(),
		dialHost:       parsedURL.Hostname(),
		certificate:    serverCertificate,
		certificatePEM: serverCertificatePEM,
		publicKeyHash:  certificatePublicKeySHA256(t, serverCertificate.Certificate[0]),
	}
}

func (s *appleHTTPTestServer) URL(path string) string {
	if path == "" {
		return s.baseURL
	}
	if strings.HasPrefix(path, "/") {
		return s.baseURL + path
	}
	return s.baseURL + "/" + path
}

func newAppleHTTPTestTransport(t *testing.T, server *appleHTTPTestServer, options option.HTTPClientOptions) innerTransport {
	t.Helper()

	ctx := newAppleHTTPTestContext()
	dialer := &appleHTTPTestDialer{
		hostMap: make(map[string]string),
	}
	if server != nil {
		dialer.hostMap["localhost"] = server.dialHost
	}

	transport, err := newAppleTransport(ctx, log.NewNOPFactory().NewLogger("httpclient"), dialer, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = transport.Close()
	})
	return transport
}

func newAppleHTTPTestContext() context.Context {
	return service.ContextWith[adapter.ConnectionManager](
		context.Background(),
		route.NewConnectionManager(log.NewNOPFactory().NewLogger("connection")),
	)
}

func appleTransportAnchorOptions(certificatePEM string) option.HTTPClientOptions {
	return option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:     true,
				ServerName:  "localhost",
				MinVersion:  "1.2",
				Certificate: badoption.Listable[string]{certificatePEM},
			},
		},
	}
}

func restoreAppleTransportFactories(t *testing.T) {
	t.Helper()
	oldAnchors := newAppleUserAnchors
	oldBridge := newAppleProxyBridge
	oldSession := newAppleTransportSession
	t.Cleanup(func() {
		newAppleUserAnchors = oldAnchors
		newAppleProxyBridge = oldBridge
		newAppleTransportSession = oldSession
	})
}

func (d *appleHTTPTestDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	host := destination.AddrString()
	if destination.IsDomain() {
		host = destination.Fqdn
		if mappedHost, loaded := d.hostMap[host]; loaded {
			host = mappedHost
		}
	}
	return d.dialer.DialContext(ctx, network, net.JoinHostPort(host, strconv.Itoa(int(destination.Port))))
}

func (d *appleHTTPTestDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	host := destination.AddrString()
	if destination.IsDomain() {
		host = destination.Fqdn
		if mappedHost, loaded := d.hostMap[host]; loaded {
			host = mappedHost
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return d.listener.ListenPacket(ctx, N.NetworkUDP, net.JoinHostPort(host, strconv.Itoa(int(destination.Port))))
}

func newAppleHTTPTestCertificate(t *testing.T, serverName string) (stdtls.Certificate, string) {
	t.Helper()

	privateKeyPEM, certificatePEM, err := boxTLS.GenerateCertificate(nil, nil, time.Now, serverName, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := stdtls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, string(certificatePEM)
}

func certificatePublicKeySHA256(t *testing.T, certificateDER []byte) []byte {
	t.Helper()

	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatal(err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	hashValue := sha256.Sum256(publicKeyDER)
	return append([]byte(nil), hashValue[:]...)
}

func appleHTTPServerTLSOptions(server *appleHTTPTestServer) *option.OutboundTLSOptions {
	return &option.OutboundTLSOptions{
		Enabled:     true,
		ServerName:  "localhost",
		Certificate: badoption.Listable[string]{server.certificatePEM},
	}
}

func newAppleHTTPRequest(t *testing.T, method string, rawURL string, body []byte) *http.Request {
	t.Helper()
	return newAppleHTTPRequestWithContext(t, context.Background(), method, rawURL, body)
}

func newAppleHTTPRequestWithContext(t *testing.T, ctx context.Context, method string, rawURL string, body []byte) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func waitObservedRequest(t *testing.T, requests <-chan appleHTTPObservedRequest) appleHTTPObservedRequest {
	t.Helper()

	select {
	case request := <-requests:
		return request
	case <-time.After(appleHTTPTestTimeout):
		t.Fatal("timed out waiting for observed request")
		return appleHTTPObservedRequest{}
	}
}

func readResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertAppleHTTPSucceeds(t *testing.T, transport http.RoundTripper, rawURL string) {
	t.Helper()

	response, err := transport.RoundTrip(newAppleHTTPRequest(t, http.MethodGet, rawURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if body := readResponseBody(t, response); body != "ok" {
		t.Fatalf("unexpected response body: %q", body)
	}
}
