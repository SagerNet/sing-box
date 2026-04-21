//go:build windows

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
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxTLS "github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/common/winhttp"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/route"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const windowsHTTPTestTimeout = 5 * time.Second

type windowsHTTPTestDialer struct {
	dialer   net.Dialer
	listener net.ListenConfig
	hostMap  map[string]string
}

type windowsHTTPObservedRequest struct {
	method     string
	body       string
	cookie     string
	protoMajor int
}

type windowsHTTPTestServer struct {
	server         *httptest.Server
	baseURL        string
	dialHost       string
	certificatePEM string
	publicKeyHash  []byte
}

func TestNewWindowsSessionConfig(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, true, false, func(w http.ResponseWriter, r *http.Request) {})

	testCases := []struct {
		name    string
		options option.HTTPClientOptions
		check   func(t *testing.T, config windowsSessionConfig)
		wantErr string
	}{
		{
			name: "success with custom root",
			options: option.HTTPClientOptions{
				Version: 2,
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: windowsHTTPServerTLSOptions(server),
				},
			},
			check: func(t *testing.T, config windowsSessionConfig) {
				t.Helper()
				if config.version != 2 {
					t.Fatalf("unexpected version: %d", config.version)
				}
				if config.serverName != "localhost" {
					t.Fatalf("unexpected server name: %q", config.serverName)
				}
				if config.userRoots == nil {
					t.Fatal("expected user roots")
				}
				if config.insecure {
					t.Fatal("unexpected insecure flag")
				}
			},
		},
		{
			name: "success with pins",
			options: option.HTTPClientOptions{
				Version: 2,
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						Enabled:                    true,
						CertificatePublicKeySHA256: badoption.Listable[[]byte]{server.publicKeyHash},
					},
				},
			},
			check: func(t *testing.T, config windowsSessionConfig) {
				t.Helper()
				if !config.insecure {
					t.Fatal("expected insecure flag")
				}
				if len(config.pinnedPublicKeys) != 1 {
					t.Fatalf("unexpected pin count: %d", len(config.pinnedPublicKeys))
				}
			},
		},
		{
			name:    "http3 unsupported",
			options: option.HTTPClientOptions{Version: 3},
			wantErr: "HTTP/3 is unsupported in Windows HTTP engine",
		},
		{
			name: "http2 options unsupported",
			options: option.HTTPClientOptions{
				HTTP2Options: option.HTTP2Options{
					MaxConcurrentStreams: 1,
				},
			},
			wantErr: "HTTP/2 options are unsupported in Windows HTTP engine",
		},
		{
			name: "quic options unsupported",
			options: option.HTTPClientOptions{
				HTTP3Options: option.QUICOptions{
					InitialPacketSize: 1200,
				},
			},
			wantErr: "QUIC options are unsupported in Windows HTTP engine",
		},
		{
			name: "tls engine unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{Engine: C.TLSEngineGo},
				},
			},
			wantErr: "tls.engine is unsupported in Windows HTTP engine",
		},
		{
			name: "tls alpn unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{ALPN: badoption.Listable[string]{"h2"}},
				},
			},
			wantErr: "tls.alpn is unsupported in Windows HTTP engine",
		},
		{
			name: "tls handshake timeout unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{HandshakeTimeout: badoption.Duration(time.Second)},
				},
			},
			wantErr: "tls.handshake_timeout is unsupported in Windows HTTP engine",
		},
		{
			name: "disable sni unsupported",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{DisableSNI: true},
				},
			},
			wantErr: "disable_sni is unsupported in Windows HTTP engine",
		},
		{
			name: "invalid pin length",
			options: option.HTTPClientOptions{
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{
						CertificatePublicKeySHA256: badoption.Listable[[]byte]{[]byte("xx")},
					},
				},
			},
			wantErr: "invalid certificate_public_key_sha256 length: 2",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			config, err := newWindowsSessionConfig(context.Background(), testCase.options)
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
			testCase.check(t, config)
		})
	}
}

func TestWindowsSessionConfigMinimumBuildGating(t *testing.T) {
	restoreWindowsTransportTestingHooks(t)
	windowsHTTPCheckPlatform = func() error {
		return errors.New("Windows HTTP engine requires Windows build 19041 or later")
	}

	_, err := newWindowsSessionConfig(context.Background(), option.HTTPClientOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Windows HTTP engine requires Windows build 19041 or later") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWindowsSecureProtocols(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		minVersion uint16
		maxVersion uint16
		wantMask   uint32
	}{
		{
			name:     "defaults",
			wantMask: winhttp.SecureProtocolTLS12 | winhttp.SecureProtocolTLS13,
		},
		{
			name:       "max version below tls12 expands lower bound",
			maxVersion: stdtls.VersionTLS11,
			wantMask:   winhttp.SecureProtocolTLS10 | winhttp.SecureProtocolTLS11,
		},
		{
			name:       "explicit min version",
			minVersion: stdtls.VersionTLS11,
			wantMask:   winhttp.SecureProtocolTLS11 | winhttp.SecureProtocolTLS12 | winhttp.SecureProtocolTLS13,
		},
		{
			name:       "explicit range",
			minVersion: stdtls.VersionTLS11,
			maxVersion: stdtls.VersionTLS12,
			wantMask:   winhttp.SecureProtocolTLS11 | winhttp.SecureProtocolTLS12,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mask := windowsSecureProtocols(testCase.minVersion, testCase.maxVersion)
			if mask != testCase.wantMask {
				t.Fatalf("unexpected secure protocol mask: %#x", mask)
			}
		})
	}
}

func TestWindowsTransportPlainHTTP(t *testing.T) {
	t.Parallel()

	requests := make(chan windowsHTTPObservedRequest, 1)
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- windowsHTTPObservedRequest{
			method:     r.Method,
			body:       string(body),
			protoMajor: r.ProtoMajor,
		}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodPost, server.URL("/plain"), []byte("hello")))
	if err != nil {
		t.Fatal(err)
	}
	if body := readWindowsHTTPResponseBody(t, response); body != "ok" {
		t.Fatalf("unexpected response body: %q", body)
	}
	response.Body.Close()
	observed := <-requests
	if observed.method != http.MethodPost || observed.body != "hello" {
		t.Fatalf("unexpected request: %+v", observed)
	}
	if observed.protoMajor != 1 {
		t.Fatalf("unexpected proto major: %d", observed.protoMajor)
	}
}

func TestWindowsTransportHTTPSVersion1(t *testing.T) {
	t.Parallel()

	requests := make(chan windowsHTTPObservedRequest, 1)
	server := startWindowsHTTPTestServer(t, true, true, func(w http.ResponseWriter, r *http.Request) {
		requests <- windowsHTTPObservedRequest{protoMajor: r.ProtoMajor}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 1,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: windowsHTTPServerTLSOptions(server),
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/h1"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.ProtoMajor != 1 {
		t.Fatalf("unexpected response proto major: %d", response.ProtoMajor)
	}
	if response.TLS == nil {
		t.Fatal("expected TLS state")
	}
	response.Body.Close()
	if observed := <-requests; observed.protoMajor != 1 {
		t.Fatalf("unexpected request proto major: %d", observed.protoMajor)
	}
}

func TestWindowsTransportHTTPSVersion2FallbackEnabled(t *testing.T) {
	t.Parallel()

	requests := make(chan windowsHTTPObservedRequest, 1)
	server := startWindowsHTTPTestServer(t, true, true, func(w http.ResponseWriter, r *http.Request) {
		requests <- windowsHTTPObservedRequest{protoMajor: r.ProtoMajor}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: windowsHTTPServerTLSOptions(server),
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/h2"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.ProtoMajor != 2 {
		t.Fatalf("unexpected response proto major: %d", response.ProtoMajor)
	}
	if response.TLS == nil || response.TLS.NegotiatedProtocol != "h2" {
		t.Fatalf("unexpected TLS negotiated protocol: %+v", response.TLS)
	}
	response.Body.Close()
	if observed := <-requests; observed.protoMajor != 2 {
		t.Fatalf("unexpected request proto major: %d", observed.protoMajor)
	}
}

func TestWindowsTransportHTTPSVersion2FallbackDisabled(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, true, false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version:                2,
		DisableVersionFallback: true,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: windowsHTTPServerTLSOptions(server),
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/strict-h2"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected protocol mismatch")
	}
}

func TestWindowsTransportPlainHTTPVersion2FallbackDisabled(t *testing.T) {
	t.Parallel()

	requests := make(chan windowsHTTPObservedRequest, 1)
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		requests <- windowsHTTPObservedRequest{protoMajor: r.ProtoMajor}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version:                2,
		DisableVersionFallback: true,
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/strict-h2-cleartext"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.ProtoMajor != 1 {
		t.Fatalf("unexpected response proto major: %d", response.ProtoMajor)
	}
	if body := readWindowsHTTPResponseBody(t, response); body != "ok" {
		t.Fatalf("unexpected response body: %q", body)
	}
	response.Body.Close()
	if observed := <-requests; observed.protoMajor != 1 {
		t.Fatalf("unexpected request proto major: %d", observed.protoMajor)
	}
}

func TestWindowsTransportPinnedPublicKey(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, true, false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:                    true,
				CertificatePublicKeySHA256: badoption.Listable[[]byte]{server.publicKeyHash},
			},
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/pin"), nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
}

func TestWindowsTransportInsecure(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, true, false, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: "localhost",
				Insecure:   true,
			},
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/insecure"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.TLS == nil {
		t.Fatal("expected TLS state")
	}
	response.Body.Close()
}

func TestWindowsTransportRedirectReturnedToCaller(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/target", http.StatusFound)
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/redirect"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}
	if response.Header.Get("Location") != "/target" {
		t.Fatalf("unexpected location: %q", response.Header.Get("Location"))
	}
}

func TestWindowsTransportCookiesStayRequestLocal(t *testing.T) {
	t.Parallel()

	var seen []string
	var seenMu sync.Mutex
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		seenMu.Lock()
		seen = append(seen, r.Header.Get("Cookie"))
		seenMu.Unlock()
		if len(seen) == 1 {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc"})
		}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	firstResponse, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/cookie"), nil))
	if err != nil {
		t.Fatal(err)
	}
	firstResponse.Body.Close()
	secondResponse, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/cookie"), nil))
	if err != nil {
		t.Fatal(err)
	}
	secondResponse.Body.Close()
	seenMu.Lock()
	defer seenMu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("unexpected request count: %d", len(seen))
	}
	if seen[0] != "" || seen[1] != "" {
		t.Fatalf("unexpected cookies: %#v", seen)
	}
}

func TestWindowsTransportRequestBodyBuffering(t *testing.T) {
	t.Parallel()

	requests := make(chan windowsHTTPObservedRequest, 1)
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- windowsHTTPObservedRequest{body: string(body)}
		_, _ = w.Write([]byte("ok"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodPut, server.URL("/body"), []byte("buffer-me")))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if observed := <-requests; observed.body != "buffer-me" {
		t.Fatalf("unexpected request body: %q", observed.body)
	}
}

func TestWindowsTransportResponseBodyStreaming(t *testing.T) {
	t.Parallel()

	releaseSecondChunk := make(chan struct{})
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("hello"))
		flusher.Flush()
		<-releaseSecondChunk
		_, _ = w.Write([]byte("world"))
		flusher.Flush()
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/stream"), nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	firstChunk := make([]byte, 5)
	readDone := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(response.Body, firstChunk)
		readDone <- err
	}()
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(windowsHTTPTestTimeout):
		t.Fatal("timed out waiting for first response chunk")
	}
	if string(firstChunk) != "hello" {
		t.Fatalf("unexpected first chunk: %q", string(firstChunk))
	}
	close(releaseSecondChunk)
	secondChunk, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(secondChunk) != "world" {
		t.Fatalf("unexpected second chunk: %q", string(secondChunk))
	}
}

func TestWindowsTransportContextCancellation(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("slow"))
	})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	response, err := transport.RoundTrip(newWindowsHTTPRequestWithContext(t, ctx, http.MethodGet, server.URL("/cancel"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWindowsRequestStateMapError(t *testing.T) {
	t.Parallel()

	deadlineCtx, deadlineCancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer deadlineCancel()
	canceledCtx, canceledCancel := context.WithCancel(context.Background())
	canceledCancel()

	testCases := []struct {
		name    string
		ctx     context.Context
		input   error
		wantErr error
	}{
		{
			name:    "closed handle with canceled context",
			ctx:     canceledCtx,
			input:   os.ErrClosed,
			wantErr: context.Canceled,
		},
		{
			name:    "operation cancelled with canceled context",
			ctx:     canceledCtx,
			input:   winhttp.ErrorOperationCancelled,
			wantErr: context.Canceled,
		},
		{
			name:    "closed handle with expired deadline",
			ctx:     deadlineCtx,
			input:   os.ErrClosed,
			wantErr: context.DeadlineExceeded,
		},
		{
			name:    "background context keeps closed error",
			ctx:     context.Background(),
			input:   os.ErrClosed,
			wantErr: os.ErrClosed,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			state := &windowsRequestState{ctx: testCase.ctx}
			err := state.mapError(testCase.input)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWindowsTransportServerNameMismatch(t *testing.T) {
	t.Parallel()

	server := startWindowsHTTPTestServer(t, true, false, func(w http.ResponseWriter, r *http.Request) {})
	transport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{
		Version: 2,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:    true,
				ServerName: "example.com",
				Insecure:   true,
			},
		},
	})
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/mismatch"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected tls.server_name mismatch error")
	}
	if !strings.Contains(err.Error(), "tls.server_name is unsupported in Windows HTTP engine unless it matches request host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWindowsTransportLifecycle(t *testing.T) {
	t.Parallel()

	streamRelease := make(chan struct{})
	server := startWindowsHTTPTestServer(t, false, false, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stream":
			flusher := w.(http.Flusher)
			_, _ = w.Write([]byte("hello"))
			flusher.Flush()
			<-streamRelease
			_, _ = w.Write([]byte("world"))
		default:
			_, _ = w.Write([]byte("ok"))
		}
	})
	transport := newManagedWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})

	streamResponse, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/stream"), nil))
	if err != nil {
		t.Fatal(err)
	}
	firstChunk := make([]byte, 5)
	_, err = io.ReadFull(streamResponse.Body, firstChunk)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstChunk) != "hello" {
		t.Fatalf("unexpected first chunk: %q", string(firstChunk))
	}

	transport.CloseIdleConnections()
	assertWindowsHTTPSucceeds(t, transport, server.URL("/after-close-idle"))

	close(streamRelease)
	remaining, err := io.ReadAll(streamResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	streamResponse.Body.Close()
	if string(remaining) != "world" {
		t.Fatalf("unexpected remaining body: %q", string(remaining))
	}

	transport.Reset()
	assertWindowsHTTPSucceeds(t, transport, server.URL("/after-reset"))

	directTransport := newWindowsHTTPTestTransport(t, server, option.HTTPClientOptions{Version: 1})
	if err = directTransport.Close(); err != nil {
		t.Fatal(err)
	}
	response, err := directTransport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, server.URL("/closed"), nil))
	if err == nil {
		response.Body.Close()
		t.Fatal("expected closed transport to fail")
	}
}

func startWindowsHTTPTestServer(t *testing.T, tlsEnabled bool, enableHTTP2 bool, handler http.HandlerFunc) *windowsHTTPTestServer {
	t.Helper()

	var (
		server         *httptest.Server
		certificatePEM string
		publicKeyHash  []byte
	)
	if tlsEnabled {
		serverCertificate, pemText := newWindowsHTTPTestCertificate(t, "localhost")
		certificatePEM = pemText
		publicKeyHash = windowsHTTPCertificatePublicKeySHA256(t, serverCertificate.Certificate[0])
		server = httptest.NewUnstartedServer(handler)
		server.EnableHTTP2 = enableHTTP2
		server.TLS = &stdtls.Config{
			Certificates: []stdtls.Certificate{serverCertificate},
			MinVersion:   stdtls.VersionTLS12,
		}
		server.StartTLS()
	} else {
		server = httptest.NewServer(handler)
	}
	t.Cleanup(server.Close)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	baseURL := *parsedURL
	baseURL.Host = net.JoinHostPort("localhost", parsedURL.Port())

	return &windowsHTTPTestServer{
		server:         server,
		baseURL:        baseURL.String(),
		dialHost:       parsedURL.Hostname(),
		certificatePEM: certificatePEM,
		publicKeyHash:  publicKeyHash,
	}
}

func (s *windowsHTTPTestServer) URL(path string) string {
	if path == "" {
		return s.baseURL
	}
	if strings.HasPrefix(path, "/") {
		return s.baseURL + path
	}
	return s.baseURL + "/" + path
}

func newWindowsHTTPTestTransport(t *testing.T, server *windowsHTTPTestServer, options option.HTTPClientOptions) innerTransport {
	t.Helper()
	ctx, dialer := newWindowsHTTPTestContext(server)
	transport, err := newWindowsTransport(ctx, log.NewNOPFactory().NewLogger("httpclient"), dialer, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = transport.Close()
	})
	return transport
}

func newManagedWindowsHTTPTestTransport(t *testing.T, server *windowsHTTPTestServer, options option.HTTPClientOptions) *ManagedTransport {
	t.Helper()
	ctx, dialer := newWindowsHTTPTestContext(server)
	logger := log.NewNOPFactory().NewLogger("httpclient")
	inner, err := newWindowsTransport(ctx, logger, dialer, options)
	if err != nil {
		t.Fatal(err)
	}
	managed := &ManagedTransport{
		factory: func() (innerTransport, error) {
			return newWindowsTransport(ctx, logger, dialer, options)
		},
	}
	managed.epoch.Store(&transportEpoch{transport: inner})
	t.Cleanup(func() {
		_ = managed.close()
	})
	return managed
}

func newWindowsHTTPTestContext(server *windowsHTTPTestServer) (context.Context, *windowsHTTPTestDialer) {
	dialer := &windowsHTTPTestDialer{
		hostMap: make(map[string]string),
	}
	if server != nil {
		dialer.hostMap["localhost"] = server.dialHost
	}
	ctx := service.ContextWith[adapter.ConnectionManager](
		context.Background(),
		route.NewConnectionManager(log.NewNOPFactory().NewLogger("connection")),
	)
	return ctx, dialer
}

func newWindowsHTTPTestCertificate(t *testing.T, serverName string) (stdtls.Certificate, string) {
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

func windowsHTTPCertificatePublicKeySHA256(t *testing.T, certificateDER []byte) []byte {
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

func windowsHTTPServerTLSOptions(server *windowsHTTPTestServer) *option.OutboundTLSOptions {
	return &option.OutboundTLSOptions{
		Enabled:     true,
		ServerName:  "localhost",
		Certificate: badoption.Listable[string]{server.certificatePEM},
	}
}

func newWindowsHTTPRequest(t *testing.T, method string, rawURL string, body []byte) *http.Request {
	t.Helper()
	return newWindowsHTTPRequestWithContext(t, context.Background(), method, rawURL, body)
}

func newWindowsHTTPRequestWithContext(t *testing.T, ctx context.Context, method string, rawURL string, body []byte) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func readWindowsHTTPResponseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func assertWindowsHTTPSucceeds(t *testing.T, transport *ManagedTransport, rawURL string) {
	t.Helper()
	response, err := transport.RoundTrip(newWindowsHTTPRequest(t, http.MethodGet, rawURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	if body := readWindowsHTTPResponseBody(t, response); body != "ok" {
		t.Fatalf("unexpected response body: %q", body)
	}
	response.Body.Close()
}

func restoreWindowsTransportTestingHooks(t *testing.T) {
	t.Helper()
	oldCheckPlatform := windowsHTTPCheckPlatform
	t.Cleanup(func() {
		windowsHTTPCheckPlatform = oldCheckPlatform
	})
}

func (d *windowsHTTPTestDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	host := destination.AddrString()
	if destination.IsDomain() {
		host = destination.Fqdn
		if mappedHost, loaded := d.hostMap[host]; loaded {
			host = mappedHost
		}
	}
	return d.dialer.DialContext(ctx, network, net.JoinHostPort(host, strconv.Itoa(int(destination.Port))))
}

func (d *windowsHTTPTestDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
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
