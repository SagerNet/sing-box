//go:build with_quic

package v2rayxhttp_test

import (
	"bytes"
	"context"
	stdtls "crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sagernet/quic-go/http3"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

func TestXHTTPXRayHTTP3Interop(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	for _, mode := range []string{"stream-one", "stream-up", "packet-up"} {
		t.Run(mode+"/sing-box-client-to-xray-server", func(t *testing.T) {
			testSingBoxClientToXRayHTTP3(t, xrayBinary, mode)
		})
		t.Run(mode+"/xray-client-to-sing-box-server", func(t *testing.T) {
			testXRayClientToSingBoxHTTP3(t, xrayBinary, mode)
		})
	}
}

func testSingBoxClientToXRayHTTP3(t *testing.T, xrayBinary, mode string) {
	certificate, key, ca := interopCertificate(t)
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)

	startXRayHTTP3(t, xrayBinary, serverPort, ca, xrayVMessServerConfig(serverPort, userID, certificate, key, "h3", mode))
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{"h3"}}},
				Transport:                   interopTransportOptions(option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode}),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxHTTP3(t *testing.T, xrayBinary, mode string) {
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
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: &option.InboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: certificate, KeyPath: key, ALPN: []string{"h3"}}},
				Transport:                  interopTransportOptions(option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: mode}),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessClientConfig(proxyPort, serverPort, userID, ca, "h3", mode))
	interopPingPong(t, proxyPort, targetPort)
}

func startXRayHTTP3(t testing.TB, binary string, port uint16, ca string, config any) {
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

	certificate, err := os.ReadFile(ca)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(certificate))
	for range 50 {
		transport := &http3.Transport{TLSClientConfig: &stdtls.Config{RootCAs: pool, ServerName: "xhttp.test", NextProtos: []string{http3.NextProtoH3}}}
		response, requestErr := (&http.Client{Transport: transport}).Do(&http.Request{Method: http.MethodOptions, URL: mustHTTP3URL(port), Header: make(http.Header)})
		if requestErr == nil {
			_ = response.Body.Close()
			_ = transport.Close()
			return
		}
		_ = transport.Close()
		if command.ProcessState != nil {
			require.Failf(t, "Xray exited before becoming ready", "%s", output.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Failf(t, "Xray HTTP/3 did not become ready", "%s", output.String())
}

func mustHTTP3URL(port uint16) *url.URL {
	return &url.URL{Scheme: "https", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), Path: "/xhttp/"}
}
