package v2rayxhttp_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

// BenchmarkXHTTPXMux compares sing-box and Xray clients on the same local
// Xray XHTTP server. Run with -benchmem for allocs/op and -cpuprofile /
// -memprofile for CPU and allocation profiles; see PLANS.md for commands.
func BenchmarkXHTTPXMux(b *testing.B) {
	xrayBinary := requireXRayBinary(b)
	variants := []struct {
		name  string
		xhttp option.V2RayXHTTPOptions
	}{
		{name: "no-xmux", xhttp: option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "packet-up"}},
		{name: "xmux", xhttp: option.V2RayXHTTPOptions{
			Path: "/xhttp/", Mode: "packet-up",
			XMUX: option.V2RayXHTTPXmuxOptions{
				MaxConcurrency:   option.V2RayXHTTPRange{From: 4, To: 4},
				CMaxReuseTimes:   option.V2RayXHTTPRange{From: 16, To: 16},
				HMaxRequestTimes: option.V2RayXHTTPRange{From: 64, To: 64},
				HMaxReusableSecs: option.V2RayXHTTPRange{From: 60, To: 60},
				HKeepAlivePeriod: 15,
			},
		}},
	}
	for _, variant := range variants {
		b.Run("sing-box/"+variant.name, func(b *testing.B) {
			benchmarkSingBoxXMux(b, xrayBinary, variant.xhttp)
		})
		b.Run("xray/"+variant.name, func(b *testing.B) {
			benchmarkXRayXMux(b, xrayBinary, variant.xhttp)
		})
	}
}

func benchmarkSingBoxXMux(b *testing.B, xrayBinary string, xhttp option.V2RayXHTTPOptions) {
	certificate, key, ca := interopCertificate(b)
	serverPort, proxyPort := interopPort(b), interopPort(b)
	targetPort, closeTarget := interopEchoServer(b)
	defer closeTarget()
	userID := interopUUID(b)
	serverXHTTP := option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "packet-up"}
	startXRay(b, xrayBinary, serverPort, xrayVMessServerConfigWithXHTTP(serverPort, userID, certificate, key, "h2", serverXHTTP))
	startInteropBox(b, option.Options{
		Inbounds: []option.Inbound{{Type: C.TypeSOCKS, Options: &option.SocksInboundOptions{ListenOptions: interopListen(proxyPort)}}},
		Outbounds: []option.Outbound{{
			Type: C.TypeVMess,
			Options: &option.VMessOutboundOptions{
				ServerOptions: option.ServerOptions{Server: "127.0.0.1", ServerPort: serverPort}, UUID: userID, Security: "aes-128-gcm",
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: "xhttp.test", CertificatePath: ca, ALPN: []string{"h2"}}},
				Transport:                   interopTransportOptions(xhttp),
			},
		}},
	})
	benchmarkProxyTraffic(b, proxyPort, targetPort)
}

func benchmarkXRayXMux(b *testing.B, xrayBinary string, xhttp option.V2RayXHTTPOptions) {
	certificate, key, ca := interopCertificate(b)
	serverPort, proxyPort := interopPort(b), interopPort(b)
	targetPort, closeTarget := interopEchoServer(b)
	defer closeTarget()
	userID := interopUUID(b)
	serverXHTTP := option.V2RayXHTTPOptions{Path: "/xhttp/", Mode: "packet-up"}
	startXRay(b, xrayBinary, serverPort, xrayVMessServerConfigWithXHTTP(serverPort, userID, certificate, key, "h2", serverXHTTP))
	startXRay(b, xrayBinary, proxyPort, xrayVMessClientConfigWithXHTTP(proxyPort, serverPort, userID, ca, "h2", xhttp))
	benchmarkProxyTraffic(b, proxyPort, targetPort)
}

func benchmarkProxyTraffic(b *testing.B, proxyPort, targetPort uint16) {
	payload := make([]byte, 64*1024)
	for index := range payload {
		payload[index] = byte(index)
	}
	latencies := make([]time.Duration, 0, b.N)
	var access sync.Mutex
	var firstErr error
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	started := time.Now()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			access.Lock()
			failed := firstErr != nil
			access.Unlock()
			if failed {
				continue
			}
			startedAt := time.Now()
			err := benchmarkPingPong(proxyPort, targetPort, payload)
			access.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			if err == nil {
				latencies = append(latencies, time.Since(startedAt))
			}
			access.Unlock()
		}
	})
	elapsed := time.Since(started)
	b.StopTimer()
	if firstErr != nil {
		b.Fatal(firstErr)
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		index := (len(latencies)*99+99)/100 - 1
		b.ReportMetric(float64(latencies[index].Microseconds())/1000, "p99_ms")
	}
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "connections/s")
}

func benchmarkPingPong(proxyPort, targetPort uint16, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(proxyPort))))
	if err != nil {
		return err
	}
	defer connection.Close()
	if err = connection.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if _, err = socks.ClientHandshake5(connection, socks5.CommandConnect, M.ParseSocksaddrHostPort("127.0.0.1", targetPort), "", ""); err != nil {
		return err
	}
	if _, err = connection.Write(payload); err != nil {
		return err
	}
	response := make([]byte, len(payload))
	if _, err = io.ReadFull(connection, response); err != nil {
		return err
	}
	for index := range payload {
		if response[index] != payload[index] {
			return fmt.Errorf("unexpected response byte at %d", index)
		}
	}
	return nil
}
