//go:build darwin && cgo

package tls

import (
	"bytes"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/json/badoption"
	N "github.com/sagernet/sing/common/network"
)

const (
	appleTLSBenchmarkReadPayloadSize  = 16 * 1024
	appleTLSBenchmarkWritePayloadSize = 48 * 1024
)

func BenchmarkAppleClientReadBuffer(b *testing.B) {
	payload := bytes.Repeat([]byte{'r'}, appleTLSBenchmarkReadPayloadSize)
	start := make(chan struct{})
	clientConn, serverResult := newAppleBenchmarkClientConn(b, func(conn *stdtls.Conn) error {
		<-start
		for range b.N {
			if err := writeBenchmarkPayload(conn, payload); err != nil {
				return err
			}
		}
		return nil
	})
	defer clientConn.Close()

	extendedConn := clientConn.(N.ExtendedConn)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ReportMetric(float64(len(payload)), "payload_B")
	b.ResetTimer()
	close(start)
	target := b.N * len(payload)
	var received int
	for received < target {
		buffer := buf.NewSize(len(payload))
		err := extendedConn.ReadBuffer(buffer)
		if err != nil {
			buffer.Release()
			b.Fatal(err)
		}
		received += buffer.Len()
		buffer.Release()
	}
	b.StopTimer()
	if err := <-serverResult; err != nil {
		b.Fatal(err)
	}
}

func BenchmarkAppleClientReadWaiter(b *testing.B) {
	payload := bytes.Repeat([]byte{'w'}, appleTLSBenchmarkReadPayloadSize)
	start := make(chan struct{})
	clientConn, serverResult := newAppleBenchmarkClientConn(b, func(conn *stdtls.Conn) error {
		<-start
		for range b.N {
			if err := writeBenchmarkPayload(conn, payload); err != nil {
				return err
			}
		}
		return nil
	})
	defer clientConn.Close()

	readWaiter, ok := clientConn.(N.ReadWaitCreator).CreateReadWaiter()
	if !ok {
		b.Fatal("expected read waiter")
	}
	readWaiter.InitializeReadWaiter(N.ReadWaitOptions{
		MTU: appleTLSBenchmarkReadPayloadSize,
	})
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ReportMetric(float64(len(payload)), "payload_B")
	b.ResetTimer()
	close(start)
	target := b.N * len(payload)
	var received int
	for received < target {
		buffer, err := readWaiter.WaitReadBuffer()
		if err != nil {
			if errors.Is(err, io.ErrNoProgress) {
				continue
			}
			b.Fatal(err)
		}
		received += buffer.Len()
		buffer.Release()
	}
	b.StopTimer()
	if err := <-serverResult; err != nil {
		b.Fatal(err)
	}
}

func BenchmarkAppleClientWriteBuffer(b *testing.B) {
	payload := bytes.Repeat([]byte{'x'}, appleTLSBenchmarkWritePayloadSize)
	start := make(chan struct{})
	clientConn, serverResult := newAppleBenchmarkClientConn(b, func(conn *stdtls.Conn) error {
		<-start
		_, err := io.CopyN(io.Discard, conn, int64(b.N*len(payload)))
		return err
	})
	defer clientConn.Close()

	extendedConn := clientConn.(N.ExtendedConn)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ReportMetric(float64(len(payload)), "payload_B")
	b.ReportMetric(float64(appleTLSWriteChunkSize), "write_chunk_B")
	b.ResetTimer()
	close(start)
	for range b.N {
		buffer := buf.NewSize(len(payload))
		_, err := buffer.Write(payload)
		if err != nil {
			buffer.Release()
			b.Fatal(err)
		}
		err = extendedConn.WriteBuffer(buffer)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if err := <-serverResult; err != nil {
		b.Fatal(err)
	}
}

func BenchmarkStdlibClientReadBuffer(b *testing.B) {
	payload := bytes.Repeat([]byte{'r'}, appleTLSBenchmarkReadPayloadSize)
	start := make(chan struct{})
	clientConn, serverResult := newStdlibBenchmarkClientConn(b, func(conn *stdtls.Conn) error {
		<-start
		for range b.N {
			if err := writeBenchmarkPayload(conn, payload); err != nil {
				return err
			}
		}
		return nil
	})
	defer clientConn.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ReportMetric(float64(len(payload)), "payload_B")
	b.ResetTimer()
	close(start)
	target := b.N * len(payload)
	var received int
	for received < target {
		buffer := buf.NewSize(len(payload))
		n, err := clientConn.Read(buffer.FreeBytes())
		if n > 0 {
			buffer.Truncate(buffer.Len() + n)
		}
		received += buffer.Len()
		buffer.Release()
		if err != nil && received < target {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if err := <-serverResult; err != nil {
		b.Fatal(err)
	}
}

func BenchmarkStdlibClientWriteBuffer(b *testing.B) {
	payload := bytes.Repeat([]byte{'x'}, appleTLSBenchmarkWritePayloadSize)
	start := make(chan struct{})
	clientConn, serverResult := newStdlibBenchmarkClientConn(b, func(conn *stdtls.Conn) error {
		<-start
		_, err := io.CopyN(io.Discard, conn, int64(b.N*len(payload)))
		return err
	})
	defer clientConn.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ReportMetric(float64(len(payload)), "payload_B")
	b.ResetTimer()
	close(start)
	for range b.N {
		buffer := buf.NewSize(len(payload))
		_, err := buffer.Write(payload)
		if err != nil {
			buffer.Release()
			b.Fatal(err)
		}
		_, err = clientConn.Write(buffer.Bytes())
		buffer.Release()
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	if err := <-serverResult; err != nil {
		b.Fatal(err)
	}
}

func newAppleBenchmarkClientConn(b *testing.B, handler func(*stdtls.Conn) error) (Conn, <-chan error) {
	b.Helper()

	serverCertificate, serverCertificatePEM := newAppleTestCertificate(b, "localhost")
	serverResult, serverAddress := startAppleTLSIOTestServer(b, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	}, handler)

	clientConn, err := newAppleTestClientConn(b, serverAddress, option.OutboundTLSOptions{
		Enabled:     true,
		Engine:      "apple",
		ServerName:  "localhost",
		MinVersion:  "1.2",
		MaxVersion:  "1.2",
		Certificate: badoption.Listable[string]{serverCertificatePEM},
	})
	if err != nil {
		b.Fatal(err)
	}
	return clientConn, serverResult
}

func newStdlibBenchmarkClientConn(b *testing.B, handler func(*stdtls.Conn) error) (*stdtls.Conn, <-chan error) {
	b.Helper()

	serverCertificate, serverCertificatePEM := newAppleTestCertificate(b, "localhost")
	serverResult, serverAddress := startAppleTLSIOTestServer(b, &stdtls.Config{
		Certificates: []stdtls.Certificate{serverCertificate},
		MinVersion:   stdtls.VersionTLS12,
		MaxVersion:   stdtls.VersionTLS12,
	}, handler)

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(serverCertificatePEM)) {
		b.Fatal("parse benchmark certificate")
	}
	dialer := &net.Dialer{
		Timeout: appleTLSTestTimeout,
	}
	clientConn, err := stdtls.DialWithDialer(dialer, "tcp", serverAddress, &stdtls.Config{
		ServerName: "localhost",
		RootCAs:    roots,
		MinVersion: stdtls.VersionTLS12,
		MaxVersion: stdtls.VersionTLS12,
	})
	if err != nil {
		b.Fatal(err)
	}
	return clientConn, serverResult
}

func writeBenchmarkPayload(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if err != nil {
			return err
		}
		payload = payload[n:]
	}
	return nil
}
