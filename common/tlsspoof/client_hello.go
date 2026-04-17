package tlsspoof

import (
	"bytes"
	"context"
	"crypto/tls"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
)

// buildFakeClientHello drives crypto/tls against a write-only in-memory conn
// to capture a generated ClientHello. CurvePreferences pins classical groups
// to suppress Go's default X25519MLKEM768 hybrid key share; without this the
// post-quantum public key alone (~1184 bytes) pushes the record past one MSS,
// and middleboxes do not reassemble fragmented ClientHellos. The handshake
// error is discarded because the stub conn's Read returns immediately.
func buildFakeClientHello(sni string) ([]byte, error) {
	if sni == "" {
		return nil, E.New("empty sni")
	}
	var buf bytes.Buffer
	tlsConn := tls.Client(bufio.NewWriteOnlyConn(&buf), &tls.Config{
		ServerName: sni,
		// Order matches what browsers advertised before post-quantum.
		CurvePreferences:   []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384},
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         []string{"h2", "http/1.1"},
		InsecureSkipVerify: true,
	})
	_ = tlsConn.HandshakeContext(context.Background())
	if buf.Len() == 0 {
		return nil, E.New("tls ClientHello not produced")
	}
	return buf.Bytes(), nil
}
