//go:build with_utls

package v2rayxhttp_test

import (
	"crypto/ecdh"
	"crypto/rand"
	stdtls "crypto/tls"
	"encoding/base64"
	"net"
	"strconv"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

// TestXHTTPRealityInterop verifies the H2 XHTTP path through REALITY/uTLS
// against Xray in both directions. REALITY's fallback target is local so the
// test remains self-contained; authenticated test traffic never uses it.
func TestXHTTPRealityInterop(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	privateKey, publicKey := interopRealityKeyPair(t)
	const serverName = "xhttp.test"
	const shortID = "0123456789abcdef"

	for _, mode := range []string{"stream-one", "auto"} {
		t.Run(mode+"/sing-box-client-to-xray-server", func(t *testing.T) {
			testSingBoxClientToXRayRealityServer(t, xrayBinary, privateKey, publicKey, serverName, shortID, mode)
		})
		t.Run(mode+"/xray-client-to-sing-box-server", func(t *testing.T) {
			testXRayClientToSingBoxRealityServer(t, xrayBinary, privateKey, publicKey, serverName, shortID, mode)
		})
	}
}

func TestXHTTPRealityDownloadSettingsInterop(t *testing.T) {
	xrayBinary := requireXRayBinary(t)
	privateKey, publicKey := interopRealityKeyPair(t)
	const serverName = "xhttp.test"
	const shortID = "0123456789abcdef"

	t.Run("sing-box-client-to-xray-server", func(t *testing.T) {
		testSingBoxClientToXRayRealityDownloadSettings(t, xrayBinary, privateKey, publicKey, serverName, shortID)
	})
	t.Run("xray-client-to-sing-box-server", func(t *testing.T) {
		testXRayClientToSingBoxRealityDownloadSettings(t, xrayBinary, privateKey, publicKey, serverName, shortID)
	})
}

func interopRealityKeyPair(t *testing.T) (privateKey, publicKey string) {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(key.Bytes()), base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
}

func interopRealityFallback(t *testing.T, serverName string) (uint16, func()) {
	t.Helper()
	certificatePath, keyPath, _ := interopCertificate(t)
	certificate, err := stdtls.LoadX509KeyPair(certificatePath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := stdtls.Listen("tcp", "127.0.0.1:0", &stdtls.Config{Certificates: []stdtls.Certificate{certificate}})
	if err != nil {
		t.Fatal(err)
	}
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
				if tlsConnection, ok := connection.(*stdtls.Conn); ok {
					_ = tlsConnection.Handshake()
				}
			}()
		}
	}()
	return uint16(listener.Addr().(*net.TCPAddr).Port), func() {
		_ = listener.Close()
		<-done
	}
}

func testSingBoxClientToXRayRealityServer(t *testing.T, xrayBinary, privateKey, publicKey, serverName, shortID, mode string) {
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	fallbackPort, closeFallback := interopRealityFallback(t, serverName)
	defer closeFallback()

	startXRay(t, xrayBinary, serverPort, xrayVMessRealityServerConfig(serverPort, fallbackPort, userID, privateKey, serverName, shortID, mode))
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: interopRealityClientTLS(serverName, publicKey, shortID)},
				Transport:                   interopTransport(mode),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxRealityServer(t *testing.T, xrayBinary, privateKey, publicKey, serverName, shortID, mode string) {
	serverPort, proxyPort := interopPort(t), interopPort(t)
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	fallbackPort, closeFallback := interopRealityFallback(t, serverName)
	defer closeFallback()

	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeVMess,
			Options: &option.VMessInboundOptions{
				ListenOptions: interopListen(serverPort), Users: []option.VMessUser{{UUID: userID}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: interopRealityServerTLS(fallbackPort, privateKey, serverName, shortID)},
				Transport:                  interopTransport(mode),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessRealityClientConfig(proxyPort, serverPort, userID, publicKey, serverName, shortID, mode))
	interopPingPong(t, proxyPort, targetPort)
}

func testSingBoxClientToXRayRealityDownloadSettings(t *testing.T, xrayBinary, privateKey, publicKey, serverName, shortID string) {
	serverPort, proxyPort := interopPort(t), interopPort(t)
	downloadPort, closeDownload := interopTCPForwarder(t, serverPort)
	defer closeDownload()
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	fallbackPort, closeFallback := interopRealityFallback(t, serverName)
	defer closeFallback()

	startXRay(t, xrayBinary, serverPort, xrayVMessRealityServerConfig(serverPort, fallbackPort, userID, privateKey, serverName, shortID, "auto"))
	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: interopRealityClientTLS(serverName, publicKey, shortID)},
				Transport: interopTransportOptions(option.V2RayXHTTPOptions{
					Path: "/xhttp/", Mode: "auto",
					DownloadSettings: &option.V2RayXHTTPDownloadSettings{
						ServerOptions:               option.ServerOptions{Server: "127.0.0.1", ServerPort: downloadPort},
						OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: interopRealityClientTLS(serverName, publicKey, shortID)},
						V2RayXHTTPOptions:           option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "auto"},
					},
				}),
			},
		}},
	})
	interopPingPong(t, proxyPort, targetPort)
}

func testXRayClientToSingBoxRealityDownloadSettings(t *testing.T, xrayBinary, privateKey, publicKey, serverName, shortID string) {
	serverPort, proxyPort := interopPort(t), interopPort(t)
	downloadPort, closeDownload := interopTCPForwarder(t, serverPort)
	defer closeDownload()
	targetPort, closeTarget := interopEchoServer(t)
	defer closeTarget()
	userID := interopUUID(t)
	fallbackPort, closeFallback := interopRealityFallback(t, serverName)
	defer closeFallback()

	startInteropBox(t, option.Options{
		Inbounds: []option.Inbound{{
			Type: C.TypeVMess,
			Options: &option.VMessInboundOptions{
				ListenOptions: interopListen(serverPort), Users: []option.VMessUser{{UUID: userID}},
				InboundTLSOptionsContainer: option.InboundTLSOptionsContainer{TLS: interopRealityServerTLS(fallbackPort, privateKey, serverName, shortID)},
				Transport:                  interopTransport("auto"),
			},
		}},
		Outbounds: []option.Outbound{{Type: C.TypeDirect}},
	})
	startXRay(t, xrayBinary, proxyPort, xrayVMessRealityClientConfigWithDownloadSettings(proxyPort, serverPort, downloadPort, userID, publicKey, serverName, shortID))
	interopPingPong(t, proxyPort, targetPort)
}

func interopRealityClientTLS(serverName, publicKey, shortID string) *option.OutboundTLSOptions {
	return &option.OutboundTLSOptions{
		Enabled: true, ServerName: serverName,
		Reality: &option.OutboundRealityOptions{Enabled: true, PublicKey: publicKey, ShortID: shortID},
		UTLS:    &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
	}
}

func interopRealityServerTLS(fallbackPort uint16, privateKey, serverName, shortID string) *option.InboundTLSOptions {
	return &option.InboundTLSOptions{
		Enabled: true, ServerName: serverName, ALPN: []string{"h2"},
		Reality: &option.InboundRealityOptions{
			Enabled: true, PrivateKey: privateKey, ShortID: []string{shortID},
			Handshake: option.InboundRealityHandshakeOptions{ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: fallbackPort}},
		},
	}
}

func xrayVMessRealityServerConfig(port, fallbackPort uint16, userID, privateKey, serverName, shortID, mode string) map[string]any {
	return map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"listen": "127.0.0.1", "port": port, "protocol": "vmess",
			"settings":       map[string]any{"clients": []any{map[string]any{"id": userID}}},
			"streamSettings": xrayXHTTPRealityStreamSettings("server", fallbackPort, privateKey, "", serverName, shortID, mode),
		}},
		"outbounds": []any{map[string]any{
			"protocol": "freedom",
			"settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}},
		}},
	}
}

func xrayVMessRealityClientConfig(proxyPort, serverPort uint16, userID, publicKey, serverName, shortID, mode string) map[string]any {
	return map[string]any{
		"log":      map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": proxyPort, "protocol": "socks", "settings": map[string]any{"udp": false}}},
		"outbounds": []any{map[string]any{
			"protocol":       "vmess",
			"settings":       map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": serverPort, "users": []any{map[string]any{"id": userID, "security": "aes-128-gcm"}}}}},
			"streamSettings": xrayXHTTPRealityStreamSettings("client", 0, "", publicKey, serverName, shortID, mode),
		}},
	}
}

func xrayVMessRealityClientConfigWithDownloadSettings(proxyPort, serverPort, downloadPort uint16, userID, publicKey, serverName, shortID string) map[string]any {
	config := xrayVMessRealityClientConfig(proxyPort, serverPort, userID, publicKey, serverName, shortID, "auto")
	streamSettings := config["outbounds"].([]any)[0].(map[string]any)["streamSettings"].(map[string]any)
	downloadSettings := xrayXHTTPRealityStreamSettings("client", 0, "", publicKey, serverName, shortID, "auto")
	downloadSettings["address"] = "127.0.0.1"
	downloadSettings["port"] = downloadPort
	streamSettings["xhttpSettings"].(map[string]any)["downloadSettings"] = downloadSettings
	return config
}

func xrayXHTTPRealityStreamSettings(role string, fallbackPort uint16, privateKey, publicKey, serverName, shortID, mode string) map[string]any {
	realitySettings := map[string]any{"serverName": serverName, "shortId": shortID}
	if role == "server" {
		realitySettings["dest"] = "127.0.0.1:" + strconv.Itoa(int(fallbackPort))
		realitySettings["serverNames"] = []string{serverName}
		realitySettings["privateKey"] = privateKey
		realitySettings["shortIds"] = []string{shortID}
		// sing-box follows the original REALITY client-version encoding. Xray
		// v26 defaults its server-side minimum to its own current version, so
		// make the interop peer explicitly accept compatible implementations.
		realitySettings["minClientVer"] = "0.0.0"
	} else {
		realitySettings["fingerprint"] = "chrome"
		realitySettings["publicKey"] = publicKey
	}
	return map[string]any{
		"network": "xhttp", "security": "reality", "realitySettings": realitySettings,
		"xhttpSettings": map[string]any{"path": "/xhttp/", "mode": mode},
	}
}
