package tls

import (
	"context"
	"crypto/sha256"
	stdtls "crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"

	"github.com/stretchr/testify/require"
)

const pinnedCertificateTestTimeout = 5 * time.Second

type pinnedTestCertificate struct {
	stdtls.Certificate
	certificatePEM string
	privateKeyPEM  string
}

func newPinnedTestCertificate(t *testing.T, serverName string) pinnedTestCertificate {
	t.Helper()
	privateKeyPEM, certificatePEM, err := GenerateCertificate(nil, nil, time.Now, serverName, time.Now().Add(time.Hour))
	require.NoError(t, err)
	certificate, err := stdtls.X509KeyPair(certificatePEM, privateKeyPEM)
	require.NoError(t, err)
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	return pinnedTestCertificate{certificate, string(certificatePEM), string(privateKeyPEM)}
}

func dialPinnedTestServer(t *testing.T, serverConfig *stdtls.Config, clientOptions option.OutboundTLSOptions) error {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(pinnedCertificateTestTimeout))
		stdtls.Server(conn, serverConfig).Handshake()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), pinnedCertificateTestTimeout)
	defer cancel()
	clientConfig, err := NewClientWithOptions(ClientOptions{
		Context: ctx,
		Logger:  logger.NOP(),
		Options: clientOptions,
	})
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), pinnedCertificateTestTimeout)
	require.NoError(t, err)
	defer conn.Close()
	tlsConn, err := ClientHandshake(ctx, conn, clientConfig)
	if err == nil {
		tlsConn.Close()
	}
	<-serverDone
	return err
}

func TestClientCertificateSHA256(t *testing.T) {
	t.Parallel()
	serverCertificate := newPinnedTestCertificate(t, "localhost")
	serverConfig := &stdtls.Config{Certificates: []stdtls.Certificate{serverCertificate.Certificate}}
	certificateHash := sha256.Sum256(serverCertificate.Leaf.Raw)
	wrongHash := sha256.Sum256([]byte("not the certificate"))
	publicKeyDER, err := x509.MarshalPKIXPublicKey(serverCertificate.Leaf.PublicKey)
	require.NoError(t, err)
	publicKeyHash := sha256.Sum256(publicKeyDER)

	require.NoError(t, dialPinnedTestServer(t, serverConfig, option.OutboundTLSOptions{
		Enabled:           true,
		ServerName:        "localhost",
		CertificateSHA256: badoption.Listable[[]byte]{certificateHash[:]},
	}))
	require.NoError(t, dialPinnedTestServer(t, serverConfig, option.OutboundTLSOptions{
		Enabled:                    true,
		ServerName:                 "localhost",
		CertificateSHA256:          badoption.Listable[[]byte]{wrongHash[:]},
		CertificatePublicKeySHA256: badoption.Listable[[]byte]{publicKeyHash[:]},
	}))
	require.Error(t, dialPinnedTestServer(t, serverConfig, option.OutboundTLSOptions{
		Enabled:           true,
		ServerName:        "localhost",
		CertificateSHA256: badoption.Listable[[]byte]{wrongHash[:]},
	}))
	require.Error(t, dialPinnedTestServer(t, serverConfig, option.OutboundTLSOptions{
		Enabled:                    true,
		ServerName:                 "localhost",
		CertificateSHA256:          badoption.Listable[[]byte]{wrongHash[:]},
		CertificatePublicKeySHA256: badoption.Listable[[]byte]{wrongHash[:]},
	}))
}

func acceptPinnedTestClient(t *testing.T, serverOptions option.InboundTLSOptions, clientConfig *stdtls.Config) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), pinnedCertificateTestTimeout)
	defer cancel()
	serverConfig, err := NewServer(ctx, log.NewNOPFactory().Logger(), serverOptions)
	require.NoError(t, err)
	require.NoError(t, serverConfig.Start())
	defer serverConfig.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	serverResult := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverResult <- acceptErr
			return
		}
		defer conn.Close()
		tlsConn, handshakeErr := ServerHandshake(ctx, conn, serverConfig)
		if handshakeErr == nil {
			tlsConn.Close()
		}
		serverResult <- handshakeErr
	}()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), pinnedCertificateTestTimeout)
	require.NoError(t, err)
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(pinnedCertificateTestTimeout))
	stdtls.Client(conn, clientConfig).Handshake()
	return <-serverResult
}

func TestServerClientCertificateSHA256(t *testing.T) {
	t.Parallel()
	serverCertificate := newPinnedTestCertificate(t, "localhost")
	clientCertificate := newPinnedTestCertificate(t, "client")
	otherCertificate := newPinnedTestCertificate(t, "other")
	clientHash := sha256.Sum256(clientCertificate.Leaf.Raw)
	serverOptions := option.InboundTLSOptions{
		Enabled:                 true,
		ServerName:              "localhost",
		Certificate:             badoption.Listable[string]{serverCertificate.certificatePEM},
		Key:                     badoption.Listable[string]{serverCertificate.privateKeyPEM},
		ClientAuthentication:    option.ClientAuthType(stdtls.RequireAndVerifyClientCert),
		ClientCertificateSHA256: badoption.Listable[[]byte]{clientHash[:]},
	}
	require.NoError(t, acceptPinnedTestClient(t, serverOptions, &stdtls.Config{
		InsecureSkipVerify: true,
		Certificates:       []stdtls.Certificate{clientCertificate.Certificate},
	}))
	require.Error(t, acceptPinnedTestClient(t, serverOptions, &stdtls.Config{
		InsecureSkipVerify: true,
		Certificates:       []stdtls.Certificate{otherCertificate.Certificate},
	}))
	require.Error(t, acceptPinnedTestClient(t, serverOptions, &stdtls.Config{
		InsecureSkipVerify: true,
	}))

	serverOptions.ClientAuthentication = option.ClientAuthType(stdtls.VerifyClientCertIfGiven)
	require.NoError(t, acceptPinnedTestClient(t, serverOptions, &stdtls.Config{
		InsecureSkipVerify: true,
	}))
	require.Error(t, acceptPinnedTestClient(t, serverOptions, &stdtls.Config{
		InsecureSkipVerify: true,
		Certificates:       []stdtls.Certificate{otherCertificate.Certificate},
	}))
}
