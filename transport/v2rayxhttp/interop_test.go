package v2rayxhttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// TestXHTTPXRayInterop verifies the XHTTP wire protocol against a locally
// built Xray binary. Set XHTTP_XRAY_BINARY to that binary's path to run it.
// Keeping the binary opt-in makes the regular unit-test suite hermetic.
func TestXHTTPXRayInterop(t *testing.T) {
	xrayBinary := requireXRayBinary(t)

	for _, httpVersion := range []string{"http/1.1", "h2"} {
		for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
			t.Run(httpVersion+"/sing-box-client-to-xray-server/"+mode, func(t *testing.T) {
				testSingBoxClientToXRayServer(t, xrayBinary, httpVersion, mode)
			})
			t.Run(httpVersion+"/xray-client-to-sing-box-server/"+mode, func(t *testing.T) {
				testXRayClientToSingBoxServer(t, xrayBinary, httpVersion, mode)
			})
		}
	}
}

// TestXHTTPXRayInteropParameters expands the wire baseline to the XHTTP
// metadata, payload-placement, and obfuscated-padding variants. Packet-up is
// used where a sequence and an explicit uplink payload placement are present.
func TestXHTTPXRayInteropParameters(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	profiles := []struct {
		name    string
		options option.V2RayXHTTPOptions
	}{
		{
			name: "obfs-header-tokenish-stream-one",
			options: option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: "stream-one",
				XPaddingBytes:     option.V2RayXHTTPRange{From: 32, To: 64},
				XPaddingObfsMode:  true,
				XPaddingPlacement: "header", XPaddingHeader: "X-Interop-Pad", XPaddingMethod: "tokenish",
			},
		},
		{
			name: "session-header-sequence-query-body",
			options: option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: "packet-up",
				SessionIDPlacement: "header", SessionIDKey: "X-Interop-Session",
				SeqPlacement: "query", SeqKey: "x_interop_seq", UplinkDataPlacement: "body",
			},
		},
		{
			name: "session-query-sequence-header-data-header",
			options: option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: "packet-up",
				SessionIDPlacement: "query", SessionIDKey: "x_interop_session",
				SeqPlacement: "header", SeqKey: "X-Interop-Seq",
				UplinkDataPlacement: "header", UplinkDataKey: "X-Interop-Data",
				UplinkChunkSize: option.V2RayXHTTPRange{From: 64, To: 96},
			},
		},
		{
			name: "session-cookie-sequence-cookie-data-cookie",
			options: option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: "packet-up",
				SessionIDPlacement: "cookie", SessionIDKey: "x_interop_session",
				SeqPlacement: "cookie", SeqKey: "x_interop_seq",
				UplinkDataPlacement: "cookie", UplinkDataKey: "x_interop_data",
				UplinkChunkSize: option.V2RayXHTTPRange{From: 64, To: 96},
			},
		},
		{
			name: "session-path-sequence-path-data-auto",
			options: option.V2RayXHTTPOptions{
				Path: "/xhttp/", Mode: "packet-up", UplinkDataPlacement: "auto",
			},
		},
	}
	for _, profile := range profiles {
		t.Run(profile.name+"/sing-box-client-to-xray-server", func(t *testing.T) {
			testSingBoxClientToXRayServerWithXHTTP(t, xrayBinary, "h2", profile.options)
		})
		t.Run(profile.name+"/xray-client-to-sing-box-server", func(t *testing.T) {
			testXRayClientToSingBoxServerWithXHTTP(t, xrayBinary, "h2", profile.options)
		})
	}
}

// TestXHTTPXRayOuterProtocols verifies that the shared XHTTP transport is
// usable by all three TCP proxy protocols that expose it in sing-box.
func TestXHTTPXRayOuterProtocols(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	for _, protocol := range []string{"vless", "trojan"} {
		for _, httpVersion := range []string{"http/1.1", "h2"} {
			for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
				t.Run(protocol+"/"+httpVersion+"/sing-box-client-to-xray-server/"+mode, func(t *testing.T) {
					testSingBoxClientToXRayServerOuter(t, xrayBinary, protocol, httpVersion, mode)
				})
				t.Run(protocol+"/"+httpVersion+"/xray-client-to-sing-box-server/"+mode, func(t *testing.T) {
					testXRayClientToSingBoxServerOuter(t, xrayBinary, protocol, httpVersion, mode)
				})
			}
		}
	}
}

// TestXHTTPXRayDownloadSettingsInterop verifies that XHTTP's upload and
// download streams may use different client targets while retaining a shared
// server-side session. The second local port forwards to the same XHTTP
// server, modelling distinct CDN front doors with one backend.
func TestXHTTPXRayDownloadSettingsInterop(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	t.Run("sing-box-client-to-xray-server", func(t *testing.T) {
		testSingBoxClientToXRayDownloadSettings(t, xrayBinary)
	})
	t.Run("xray-client-to-sing-box-server", func(t *testing.T) {
		testXRayClientToSingBoxDownloadSettings(t, xrayBinary)
	})
}

func testSingBoxClientToXRayDownloadSettings(t *testing.T, xrayBinary string) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	downloadPort, closeDownload := interopTCPForwarder(t, serverPort)
	defer closeDownload()
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	xhttp := option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "packet-up"}

	startXRay(t, xrayBinary, serverPort, xrayVMessServerConfigWithXHTTP(serverPort, userID, certificate, key, "h2", xhttp))
	downloadXHTTP := xhttp
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{"h2"}}},
				Transport: interopTransportOptions(option.V2RayXHTTPOptions{
					Path: "/xhttp/", Mode: "packet-up",
					DownloadSettings: &option.V2RayXHTTPDownloadSettings{
						ServerOptions:               option.ServerOptions{Server: "127.0.0.1", ServerPort: downloadPort},
						OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{"h2"}}},
						V2RayXHTTPOptions:           downloadXHTTP,
					},
				}),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxDownloadSettings(t *testing.T, xrayBinary string) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	downloadPort, closeDownload := interopTCPForwarder(t, serverPort)
	defer closeDownload()
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	xhttp := option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "packet-up"}

	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeVMess,
			Options: &option.VMessInboundOptions{
				ListenOptions: interopListen(serverPort), Users: []option.VMessUser{{UUID: userID}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{"h2"}}},
				Transport:                  interopTransportOptions(xhttp),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessClientConfigWithDownloadSettings(proxyPort, serverPort, downloadPort, userID, ca, "h2", xhttp))
	interopPingPong(t, proxyPort, targetPort)
}

func requireXRayBinary(t testing.TB) string {
	t.Helper()
	xrayBinary := os.Getenv("XHTTP_XRAY_BINARY")
	if xrayBinary == "" {
		t.Skip("set XHTTP_XRAY_BINARY to run Xray XHTTP interoperability tests")
	}
	_, err := os.Stat(xrayBinary)
	require.NoError(t, err)
	return xrayBinary
}

func testSingBoxClientToXRayServer(t *testing.T, xrayBinary, alpn, mode string) {
	certificate, key, ca := interopCertificate(t)
	serverPort := interopPort(t)
	proxyPort := interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)

	startXRay(t, xrayBinary, serverPort, xrayVMessServerConfig(serverPort, userID, certificate, key, alpn, mode))
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type:    C.TypeSOCKS,
			Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)},
		}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort},
				UUID:          userID,
				Security:      "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{
					Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{alpn},
				}},
				Transport: interopTransport(mode),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxServer(t *testing.T, xrayBinary, alpn, mode string) {
	certificate, key, ca := interopCertificate(t)
	serverPort := interopPort(t)
	proxyPort := interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)

	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeVMess,
			Options: &option.VMessInboundOptions{
				ListenOptions: interopListen(serverPort),
				Users:         []option.VMessUser{{UUID: userID}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{
					Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{alpn},
				}},
				Transport: interopTransport(mode),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessClientConfig(proxyPort, serverPort, userID, ca, alpn, mode))
	interopPingPong(t, proxyPort, targetPort)
}

func testSingBoxClientToXRayServerWithXHTTP(t *testing.T, xrayBinary, alpn string, xhttp option.V2RayXHTTPOptions) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)

	startXRay(t, xrayBinary, serverPort, xrayVMessServerConfigWithXHTTP(serverPort, userID, certificate, key, alpn, xhttp))
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{alpn}}},
				Transport:                   interopTransportOptions(xhttp),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxServerWithXHTTP(t *testing.T, xrayBinary, alpn string, xhttp option.V2RayXHTTPOptions) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)

	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeVMess,
			Options: &option.VMessInboundOptions{
				ListenOptions: interopListen(serverPort), Users: []option.VMessUser{{UUID: userID}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{alpn}}},
				Transport:                  interopTransportOptions(xhttp),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessClientConfigWithXHTTP(proxyPort, serverPort, userID, ca, alpn, xhttp))
	interopPingPong(t, proxyPort, targetPort)
}

func testSingBoxClientToXRayServerOuter(t *testing.T, xrayBinary, protocol, alpn, mode string) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	credential := interopCredential(t, protocol)

	startXRay(t, xrayBinary, serverPort, xrayOuterServerConfig(protocol, serverPort, credential, certificate, key, alpn, mode))
	startInteropBox(t, option.Options{
		Inbounds:  []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{interopOuterClientOutbound(protocol, serverPort, credential, ca, alpn, mode)},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxServerOuter(t *testing.T, xrayBinary, protocol, alpn, mode string) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	credential := interopCredential(t, protocol)

	startInteropBox(t, option.Options{
		Inbounds:  []option.Inbound{interopOuterServerInbound(protocol, serverPort, credential, certificate, key, alpn, mode)},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayOuterClientConfig(protocol, proxyPort, serverPort, credential, ca, alpn, mode))
	interopPingPong(t, proxyPort, targetPort)
}

func interopCredential(t *testing.T, protocol string) string {
	t.Helper()
	if protocol == "vless" {
		return interopUUID(t)
	}
	return "xhttp-interop-password"
}

func interopOuterClientOutbound(protocol string, serverPort uint16, credential, ca, alpn, mode string) option.Outbound {
	tlsOptions := option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{alpn}}}
	switch protocol {
	case "vless":
		return option.Outbound{Type: C.TypeVLESS, Options: &option.VLESSOutboundOptions{
			ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: credential,
			OutboundTLSOptionsContainer: tlsOptions, Transport: interopTransport(mode),
		}}
	case "trojan":
		return option.Outbound{Type: C.TypeTrojan, Options: &option.TrojanOutboundOptions{
			ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, Password: credential,
			OutboundTLSOptionsContainer: tlsOptions, Transport: interopTransport(mode),
		}}
	default:
		panic("unknown interop protocol: " + protocol)
	}
}

func interopOuterServerInbound(protocol string, serverPort uint16, credential, certificate, key, alpn, mode string) option.Inbound {
	tlsOptions := option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{alpn}}}
	switch protocol {
	case "vless":
		return option.Inbound{Type: C.TypeVLESS, Options: &option.VLESSInboundOptions{
			ListenOptions: interopListen(serverPort), Users: []option.VLESSUser{{UUID: credential}},
			InboundTLSOptionsContainer: tlsOptions, Transport: interopTransport(mode),
		}}
	case "trojan":
		return option.Inbound{Type: C.TypeTrojan, Options: &option.TrojanInboundOptions{
			ListenOptions: interopListen(serverPort), Users: []option.TrojanUser{{Password: credential}},
			InboundTLSOptionsContainer: tlsOptions, Transport: interopTransport(mode),
		}}
	default:
		panic("unknown interop protocol: " + protocol)
	}
}

func interopTransport(mode string) *option.V2RayTransportOptions {
	return interopTransportOptions(option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode})
}

func interopTransportOptions(xhttp option.V2RayXHTTPOptions) *option.V2RayTransportOptions {
	return &option.V2RayTransportOptions{
		Type: C.V2RayTransportTypeXHTTP,
		// Xray normalizes a path to include a trailing slash when its default
		// path placement is active. Use that canonical form on both sides.
		XHTTPOptions: xhttp,
	}
}

func interopListen(port uint16) option.ListenOptions {
	return option.ListenOptions{
		Listen:     common.Ptr(badoption.Addr(netip.MustParseAddr("127.0.0.1"))),
		ListenPort: port,
	}
}

func interopCertificate(t testing.TB) (certificatePath, keyPath, caPath string) {
	t.Helper()
	directory := t.TempDir()
	key, certificate, err := tls.GenerateCertificate(nil, nil, time.Now, "xhttp.test", time.Now().Add(time.Hour))
	require.NoError(t, err)
	certificatePath = filepath.Join(directory, "certificate.pem")
	keyPath = filepath.Join(directory, "key.pem")
	caPath = filepath.Join(directory, "ca.pem")
	require.NoError(t, os.WriteFile(certificatePath, certificate, 0o600))
	require.NoError(t, os.WriteFile(keyPath, key, 0o600))
	require.NoError(t, os.WriteFile(caPath, certificate, 0o600))
	return
}

func interopPort(t testing.TB) uint16 {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

func interopEchoServer(t testing.TB) (uint16, func()) {
	t.Helper()
	_, benchmark := t.(*testing.B)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			if !benchmark {
				t.Log("echo accepted connection")
			}
			go func() {
				defer connection.Close()
				n, copyErr := io.Copy(connection, connection)
				if !benchmark {
					t.Logf("echo connection completed: bytes=%d error=%v", n, copyErr)
				}
			}()
		}
	}()
	return uint16(listener.Addr().(*net.TCPAddr).Port), func() {
		_ = listener.Close()
		<-done
	}
}

func interopTCPForwarder(t testing.TB, targetPort uint16) (uint16, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer connection.Close()
				upstream, dialErr := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(targetPort))))
				if dialErr != nil {
					return
				}
				defer upstream.Close()
				doneCopy := make(chan struct{})
				go func() { _, _ = io.Copy(upstream, connection); close(doneCopy) }()
				_, _ = io.Copy(connection, upstream)
				<-doneCopy
			}()
		}
	}()
	return uint16(listener.Addr().(*net.TCPAddr).Port), func() {
		_ = listener.Close()
		<-done
	}
}

func interopUUID(t testing.TB) string {
	t.Helper()
	userID, err := uuid.DefaultGenerator.NewV4()
	require.NoError(t, err)
	return userID.String()
}

func interopPingPong(t testing.TB, proxyPort, targetPort uint16) {
	interopPingPongPayload(t, proxyPort, targetPort, []byte("xhttp interop"))
}

func interopPingPongPayload(t testing.TB, proxyPort, targetPort uint16, payload []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(proxyPort))))
	require.NoError(t, err)
	defer connection.Close()
	require.NoError(t, connection.SetDeadline(time.Now().Add(10*time.Second)))
	_, err = socks.ClientHandshake5(connection, socks5.CommandConnect, M.ParseSocksaddrHostPort("127.0.0.1", targetPort), "", "")
	require.NoError(t, err)
	_, err = connection.Write(payload)
	require.NoError(t, err)
	response := make([]byte, len(payload))
	_, err = io.ReadFull(connection, response)
	require.NoError(t, err)
	require.Equal(t, payload, response)
}

func startInteropBox(t testing.TB, options option.Options) {
	t.Helper()
	if _, benchmark := t.(*testing.B); benchmark {
		options.Log = &option.LogOptions{Disabled: true}
	}
	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	require.NoError(t, err)
	require.NoError(t, instance.Start())
	t.Cleanup(func() {
		_ = instance.Close()
		cancel()
	})
}

func startXRay(t testing.TB, binary string, port uint16, config any) {
	t.Helper()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	configPath := filepath.Join(t.TempDir(), "xray.json")
	require.NoError(t, os.WriteFile(configPath, data, 0o600))
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, binary, "run", "-c", configPath)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	require.NoError(t, command.Start())
	t.Cleanup(func() {
		cancel()
		_ = command.Wait()
		if t.Failed() {
			t.Logf("Xray output:\n%s", output.String())
		}
	})
	// Xray has no readiness endpoint. Wait for the listener before connecting
	// its peer, while keeping the retry window short.
	for range 50 {
		if command.ProcessState != nil {
			require.Failf(t, "Xray exited before becoming ready", "%s", output.String())
		}
		connection, dialErr := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), 20*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Failf(t, "Xray did not become ready", "%s", output.String())
}

func xrayVMessServerConfig(port uint16, userID, certificate, key, alpn, mode string) map[string]any {
	return xrayVMessServerConfigWithXHTTP(port, userID, certificate, key, alpn, option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode})
}

func xrayVMessServerConfigWithXHTTP(port uint16, userID, certificate, key, alpn string, xhttp option.V2RayXHTTPOptions) map[string]any {
	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": "vmess",
			"settings": map[string]any{"clients": []any{map[string]any{"id": userID}}},
			"streamSettings": xrayXHTTPStreamSettingsWithOptions(alpn, xhttp, map[string]any{
				"certificates": []any{map[string]any{"certificateFile": certificate, "keyFile": key}},
			}),
		}},
		"outbounds": []any{map[string]any{
			"protocol": "freedom",
			// Xray v26 blocks private destinations by default for VMess inbounds.
			// The regression peer is intentionally a loopback echo server.
			"settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}},
		}},
	}
}

func xrayVMessClientConfig(proxyPort, serverPort uint16, userID, ca, alpn, mode string) map[string]any {
	return xrayVMessClientConfigWithXHTTP(proxyPort, serverPort, userID, ca, alpn, option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode})
}

func xrayVMessClientConfigWithXHTTP(proxyPort, serverPort uint16, userID, ca, alpn string, xhttp option.V2RayXHTTPOptions) map[string]any {
	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": proxyPort, "protocol": "socks",
			"settings": map[string]any{"udp": false},
		}},
		"outbounds": []any{map[string]any{
			"protocol": "vmess",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": "127.0.0.1", "port": serverPort,
				"users": []any{map[string]any{"id": userID, "security": "aes-128-gcm"}},
			}}},
			"streamSettings": xrayXHTTPStreamSettingsWithOptions(alpn, xhttp, map[string]any{
				"serverName":   "xhttp.test",
				"certificates": []any{map[string]any{"certificateFile": ca, "usage": "verify"}},
			}),
		}},
	}
}

func xrayVMessClientConfigWithDownloadSettings(proxyPort, serverPort, downloadPort uint16, userID, ca, alpn string, xhttp option.V2RayXHTTPOptions) map[string]any {
	config := xrayVMessClientConfigWithXHTTP(proxyPort, serverPort, userID, ca, alpn, xhttp)
	streamSettings := config["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
	downloadSettings := xrayXHTTPStreamSettingsWithOptions(alpn, xhttp, map[string]any{
		"serverName": "xhttp.test", "certificates": []any{map[string]any{"certificateFile": ca, "usage": "verify"}},
	})
	downloadSettings["address"] = "127.0.0.1"
	downloadSettings["port"] = downloadPort
	streamSettings["xhttpSettings"].(map[string]any)["downloadSettings"] = downloadSettings
	return config
}

func xrayOuterServerConfig(protocol string, port uint16, credential, certificate, key, alpn, mode string) map[string]any {
	var settings map[string]any
	switch protocol {
	case "vless":
		settings = map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": credential}}}
	case "trojan":
		settings = map[string]any{"clients": []any{map[string]any{"password": credential}}}
	default:
		panic("unknown interop protocol: " + protocol)
	}
	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": protocol, "settings": settings,
			"streamSettings": xrayXHTTPStreamSettings(alpn, mode, map[string]any{
				"certificates": []any{map[string]any{"certificateFile": certificate, "keyFile": key}},
			}),
		}},
		"outbounds": []any{map[string]any{
			"protocol": "freedom",
			"settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}},
		}},
	}
}

func xrayOuterClientConfig(protocol string, proxyPort, serverPort uint16, credential, ca, alpn, mode string) map[string]any {
	var settings map[string]any
	switch protocol {
	case "vless":
		settings = map[string]any{"vnext": []any{map[string]any{
			"address": "127.0.0.1", "port": serverPort,
			"users": []any{map[string]any{"id": credential, "encryption": "none"}},
		}}}
	case "trojan":
		settings = map[string]any{"servers": []any{map[string]any{
			"address": "127.0.0.1", "port": serverPort, "password": credential,
		}}}
	default:
		panic("unknown interop protocol: " + protocol)
	}
	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": proxyPort, "protocol": "socks",
			"settings": map[string]any{"udp": false},
		}},
		"outbounds": []any{map[string]any{
			"protocol": protocol, "settings": settings,
			"streamSettings": xrayXHTTPStreamSettings(alpn, mode, map[string]any{
				"serverName": "xhttp.test", "certificates": []any{map[string]any{"certificateFile": ca, "usage": "verify"}},
			}),
		}},
	}
}

func xrayXHTTPStreamSettings(alpn, mode string, tlsSettings map[string]any) map[string]any {
	return xrayXHTTPStreamSettingsWithOptions(alpn, option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode}, tlsSettings)
}

func xrayXHTTPStreamSettingsWithOptions(alpn string, options option.V2RayXHTTPOptions, tlsSettings map[string]any) map[string]any {
	tlsSettings["alpn"] = []string{alpn}
	xhttpSettings := map[string]any{"path": options.Path, "mode": options.Mode}
	if options.XPaddingBytes.To != 0 {
		xhttpSettings["xPaddingBytes"] = xrayRange(options.XPaddingBytes)
	}
	if options.XPaddingObfsMode {
		xhttpSettings["xPaddingObfsMode"] = true
		xhttpSettings["xPaddingKey"] = options.XPaddingKey
		xhttpSettings["xPaddingHeader"] = options.XPaddingHeader
		xhttpSettings["xPaddingPlacement"] = options.XPaddingPlacement
		xhttpSettings["xPaddingMethod"] = options.XPaddingMethod
	}
	set := func(key, value string) {
		if value != "" {
			xhttpSettings[key] = value
		}
	}
	set("sessionIDPlacement", options.SessionIDPlacement)
	set("sessionIDKey", options.SessionIDKey)
	set("seqPlacement", options.SeqPlacement)
	set("seqKey", options.SeqKey)
	set("uplinkDataPlacement", options.UplinkDataPlacement)
	set("uplinkDataKey", options.UplinkDataKey)
	if options.UplinkChunkSize.To != 0 {
		xhttpSettings["uplinkChunkSize"] = xrayRange(options.UplinkChunkSize)
	}
	if options.XMUX.MaxConcurrency.To != 0 || options.XMUX.MaxConnections.To != 0 || options.XMUX.CMaxReuseTimes.To != 0 || options.XMUX.HMaxRequestTimes.To != 0 || options.XMUX.HMaxReusableSecs.To != 0 || options.XMUX.HKeepAlivePeriod != 0 {
		xmux := map[string]any{}
		if options.XMUX.MaxConcurrency.To != 0 {
			xmux["maxConcurrency"] = xrayRange(options.XMUX.MaxConcurrency)
		}
		if options.XMUX.MaxConnections.To != 0 {
			xmux["maxConnections"] = xrayRange(options.XMUX.MaxConnections)
		}
		if options.XMUX.CMaxReuseTimes.To != 0 {
			xmux["cMaxReuseTimes"] = xrayRange(options.XMUX.CMaxReuseTimes)
		}
		if options.XMUX.HMaxRequestTimes.To != 0 {
			xmux["hMaxRequestTimes"] = xrayRange(options.XMUX.HMaxRequestTimes)
		}
		if options.XMUX.HMaxReusableSecs.To != 0 {
			xmux["hMaxReusableSecs"] = xrayRange(options.XMUX.HMaxReusableSecs)
		}
		if options.XMUX.HKeepAlivePeriod != 0 {
			xmux["hKeepAlivePeriod"] = options.XMUX.HKeepAlivePeriod
		}
		xhttpSettings["xmux"] = xmux
	}
	return map[string]any{
		"network": "xhttp", "security": "tls", "tlsSettings": tlsSettings,
		"xhttpSettings": xhttpSettings,
	}
}

func xrayRange(value option.V2RayXHTTPRange) string {
	if value.From == value.To {
		return strconv.FormatInt(int64(value.From), 10)
	}
	return strconv.FormatInt(int64(value.From), 10) + "-" + strconv.FormatInt(int64(value.To), 10)
}
