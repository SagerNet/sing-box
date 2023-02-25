//go:build with_utls

package tls

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	utls "github.com/sagernet/utls"

	"golang.org/x/crypto/hkdf"
)

var _ Config = (*RealityClientConfig)(nil)

type RealityClientConfig struct {
	uClient   *UTLSClientConfig
	publicKey []byte
	shortID   []byte
}

func NewRealityClient(router adapter.Router, serverAddress string, options option.OutboundTLSOptions) (*RealityClientConfig, error) {
	if options.UTLS == nil || !options.UTLS.Enabled {
		return nil, E.New("uTLS is required by reality client")
	}

	uClient, err := NewUTLSClient(router, serverAddress, options)
	if err != nil {
		return nil, err
	}

	publicKey, err := base64.RawURLEncoding.DecodeString(options.Reality.PublicKey)
	if err != nil {
		return nil, E.Cause(err, "decode public_key")
	}
	if len(publicKey) != 32 {
		return nil, E.New("invalid public_key")
	}
	shortID, err := hex.DecodeString(options.Reality.ShortID)
	if err != nil {
		return nil, E.Cause(err, "decode short_id")
	}
	if len(shortID) != 8 {
		return nil, E.New("invalid short_id")
	}
	return &RealityClientConfig{uClient, publicKey, shortID}, nil
}

func (e *RealityClientConfig) ServerName() string {
	return e.uClient.ServerName()
}

func (e *RealityClientConfig) SetServerName(serverName string) {
	e.uClient.SetServerName(serverName)
}

func (e *RealityClientConfig) NextProtos() []string {
	return e.uClient.NextProtos()
}

func (e *RealityClientConfig) SetNextProtos(nextProto []string) {
	e.uClient.SetNextProtos(nextProto)
}

func (e *RealityClientConfig) Config() (*STDConfig, error) {
	return nil, E.New("unsupported usage for reality")
}

func (e *RealityClientConfig) Client(conn net.Conn) (Conn, error) {
	verifier := &realityVerifier{
		serverName: e.uClient.ServerName(),
	}
	uConfig := e.uClient.config.Clone()
	uConfig.InsecureSkipVerify = true
	uConfig.SessionTicketsDisabled = true
	uConfig.VerifyPeerCertificate = verifier.VerifyPeerCertificate
	uConn := utls.UClient(conn, uConfig, e.uClient.id)
	verifier.UConn = uConn
	err := uConn.BuildHandshakeState()
	if err != nil {
		return nil, err
	}
	hello := uConn.HandshakeState.Hello
	hello.SessionId = make([]byte, 32)
	copy(hello.Raw[39:], hello.SessionId)

	var nowTime time.Time
	if uConfig.Time != nil {
		nowTime = uConfig.Time()
	} else {
		nowTime = time.Now()
	}
	binary.BigEndian.PutUint64(hello.SessionId, uint64(nowTime.Unix()))

	hello.SessionId[0] = 1
	hello.SessionId[1] = 7
	hello.SessionId[2] = 5
	copy(hello.SessionId[8:], e.shortID)

	if debug.Enabled {
		fmt.Printf("REALITY hello.sessionId[:16]: %v\n", hello.SessionId[:16])
	}

	authKey := uConn.HandshakeState.State13.EcdheParams.SharedKey(e.publicKey)
	if authKey == nil {
		return nil, E.New("nil auth_key")
	}
	verifier.authKey = authKey
	_, err = hkdf.New(sha256.New, authKey, hello.Random[:20], []byte("REALITY")).Read(authKey)
	if err != nil {
		return nil, err
	}
	aesBlock, _ := aes.NewCipher(authKey)
	aesGcmCipher, _ := cipher.NewGCM(aesBlock)
	aesGcmCipher.Seal(hello.SessionId[:0], hello.Random[20:], hello.SessionId[:16], hello.Raw)
	copy(hello.Raw[39:], hello.SessionId)
	if debug.Enabled {
		fmt.Printf("REALITY hello.sessionId: %v\n", hello.SessionId)
		fmt.Printf("REALITY uConn.AuthKey: %v\n", authKey)
	}

	return &utlsConnWrapper{uConn}, nil
}

func (e *RealityClientConfig) SetSessionIDGenerator(generator func(clientHello []byte, sessionID []byte) error) {
	e.uClient.config.SessionIDGenerator = generator
}

func (e *RealityClientConfig) Clone() Config {
	return &RealityClientConfig{
		e.uClient.Clone().(*UTLSClientConfig),
		e.publicKey,
		e.shortID,
	}
}

type realityVerifier struct {
	*utls.UConn
	serverName string
	authKey    []byte
	verified   bool
}

func (c *realityVerifier) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	p, _ := reflect.TypeOf(c.Conn).Elem().FieldByName("peerCertificates")
	certs := *(*([]*x509.Certificate))(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn)) + p.Offset))
	if pub, ok := certs[0].PublicKey.(ed25519.PublicKey); ok {
		h := hmac.New(sha512.New, c.authKey)
		h.Write(pub)
		if bytes.Equal(h.Sum(nil), certs[0].Signature) {
			c.verified = true
			return nil
		}
	}
	opts := x509.VerifyOptions{
		DNSName:       c.serverName,
		Intermediates: x509.NewCertPool(),
	}
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}
	if _, err := certs[0].Verify(opts); err != nil {
		return err
	}
	if !c.verified {
		return E.New("reality verification failed")
	}
	return nil
}
