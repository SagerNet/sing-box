package warp

import (
	"fmt"
	"io"
	"net"
	"net/netip"

	"github.com/sagernet/sing-box/iputils"

	"github.com/noql-net/certpool"
	tls "github.com/sagernet/utls"
)

// Dialer is a struct that holds various options for custom dialing.
type Dialer struct{}

const utlsExtensionSNICurve uint16 = 0x15

// SNICurveExtension implements SNICurve (0x15) extension
type SNICurveExtension struct {
	*tls.GenericExtension
	SNICurveLen int
	WillPad     bool // set false to disable extension
}

// Len returns the length of the SNICurveExtension.
func (e *SNICurveExtension) Len() int {
	if e.WillPad {
		return 4 + e.SNICurveLen
	}
	return 0
}

// Read reads the SNICurveExtension.
func (e *SNICurveExtension) Read(b []byte) (n int, err error) {
	if !e.WillPad {
		return 0, io.EOF
	}
	if len(b) < e.Len() {
		return 0, io.ErrShortBuffer
	}
	// https://tools.ietf.org/html/rfc7627
	b[0] = byte(utlsExtensionSNICurve >> 8)
	b[1] = byte(utlsExtensionSNICurve)
	b[2] = byte(e.SNICurveLen >> 8)
	b[3] = byte(e.SNICurveLen)
	y := make([]byte, 1200)
	copy(b[4:], y)
	return e.Len(), io.EOF
}

// makeTLSHelloPacketWithSNICurve creates a TLS hello packet with SNICurve.
func (d *Dialer) makeTLSHelloPacketWithSNICurve(plainConn net.Conn, config *tls.Config, sni string) (*tls.UConn, error) {
	SNICurveSize := 1200

	utlsConn := tls.UClient(plainConn, config, tls.HelloCustom)
	spec := tls.ClientHelloSpec{
		TLSVersMax: tls.VersionTLS12,
		TLSVersMin: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.GREASE_PLACEHOLDER,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_AES_128_GCM_SHA256, // tls 1.3
			tls.FAKE_TLS_DHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		Extensions: []tls.TLSExtension{
			&SNICurveExtension{
				SNICurveLen: SNICurveSize,
				WillPad:     true,
			},
			&tls.SupportedCurvesExtension{Curves: []tls.CurveID{tls.X25519, tls.CurveP256}},
			&tls.SupportedPointsExtension{SupportedPoints: []byte{0}}, // uncompressed
			&tls.SessionTicketExtension{},
			&tls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&tls.SignatureAlgorithmsExtension{
				SupportedSignatureAlgorithms: []tls.SignatureScheme{
					tls.ECDSAWithP256AndSHA256,
					tls.ECDSAWithP384AndSHA384,
					tls.ECDSAWithP521AndSHA512,
					tls.PSSWithSHA256,
					tls.PSSWithSHA384,
					tls.PSSWithSHA512,
					tls.PKCS1WithSHA256,
					tls.PKCS1WithSHA384,
					tls.PKCS1WithSHA512,
					tls.ECDSAWithSHA1,
					tls.PKCS1WithSHA1,
				},
			},
			&tls.KeyShareExtension{KeyShares: []tls.KeyShare{
				{Group: tls.CurveID(tls.GREASE_PLACEHOLDER), Data: []byte{0}},
				{Group: tls.X25519},
			}},
			&tls.PSKKeyExchangeModesExtension{Modes: []uint8{1}}, // pskModeDHE
			&tls.SNIExtension{ServerName: sni},
		},
		GetSessionID: nil,
	}
	err := utlsConn.ApplyPreset(&spec)
	if err != nil {
		return nil, fmt.Errorf("uTlsConn.Handshake() error: %w", err)
	}

	err = utlsConn.Handshake()
	if err != nil {
		return nil, fmt.Errorf("uTlsConn.Handshake() error: %w", err)
	}

	return utlsConn, nil
}

// TLSDial dials a TLS connection.
func (d *Dialer) TLSDial(plainDialer *net.Dialer, network, addr string) (net.Conn, error) {
	sni, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip, err := iputils.RandomIPFromPrefix(netip.MustParsePrefix("141.101.113.0/24"))
	if err != nil {
		return nil, err
	}
	plainConn, err := plainDialer.Dial(network, ip.String()+":443")
	if err != nil {
		return nil, err
	}

	config := tls.Config{
		ServerName: sni,
		MinVersion: tls.VersionTLS12,
		RootCAs:    certpool.Roots(),
	}

	utlsConn, handshakeErr := d.makeTLSHelloPacketWithSNICurve(plainConn, &config, sni)
	if handshakeErr != nil {
		_ = plainConn.Close()
		return nil, handshakeErr
	}
	return utlsConn, nil
}
