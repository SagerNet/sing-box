package v2raygrpc

import (
	"context"
	"net"
	"os"

	"github.com/sagernet/sing-box/common/tls"
	internal_credentials "github.com/sagernet/sing-box/transport/v2raygrpc/credentials"

	"google.golang.org/grpc/credentials"
)

type TLSTransportCredentials struct {
	config tls.Config
}

func NewTLSTransportCredentials(config tls.Config) credentials.TransportCredentials {
	return &TLSTransportCredentials{config}
}

func (c *TLSTransportCredentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.2",
		ServerName:       c.config.ServerName(),
	}
}

func (c *TLSTransportCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	cfg := c.config.Clone()
	if cfg.ServerName() == "" {
		serverName, _, err := net.SplitHostPort(authority)
		if err != nil {
			serverName = authority
		}
		cfg.SetServerName(serverName)
	}
	conn, err := tls.ClientHandshake(ctx, rawConn, cfg)
	if err != nil {
		return nil, nil, err
	}
	tlsInfo := credentials.TLSInfo{
		State: conn.ConnectionState(),
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}
	id := internal_credentials.SPIFFEIDFromState(conn.ConnectionState())
	if id != nil {
		tlsInfo.SPIFFEID = id
	}
	return internal_credentials.WrapSyscallConn(rawConn, conn), tlsInfo, nil
}

func (c *TLSTransportCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	serverConfig, isServer := c.config.(tls.ServerConfig)
	if !isServer {
		return nil, nil, os.ErrInvalid
	}
	conn, err := tls.ServerHandshake(context.Background(), rawConn, serverConfig)
	if err != nil {
		rawConn.Close()
		return nil, nil, err
	}
	tlsInfo := credentials.TLSInfo{
		State: conn.ConnectionState(),
		CommonAuthInfo: credentials.CommonAuthInfo{
			SecurityLevel: credentials.PrivacyAndIntegrity,
		},
	}
	id := internal_credentials.SPIFFEIDFromState(conn.ConnectionState())
	if id != nil {
		tlsInfo.SPIFFEID = id
	}
	return internal_credentials.WrapSyscallConn(rawConn, conn), tlsInfo, nil
}

func (c *TLSTransportCredentials) Clone() credentials.TransportCredentials {
	return NewTLSTransportCredentials(c.config)
}

func (c *TLSTransportCredentials) OverrideServerName(serverNameOverride string) error {
	c.config.SetServerName(serverNameOverride)
	return nil
}
