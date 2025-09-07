package tls

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/common/ktls"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	aTLS "github.com/sagernet/sing/common/tls"
)

type KTLSClientConfig struct {
	Config
	logger             logger.ContextLogger
	kernelTx, kernelRx bool
}

func (w *KTLSClientConfig) ClientHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
	tlsConn, err := aTLS.ClientHandshake(ctx, conn, w.Config)
	if err != nil {
		return nil, err
	}
	kConn, err := ktls.NewConn(ctx, w.logger, tlsConn, w.kernelTx, w.kernelRx)
	if err != nil {
		tlsConn.Close()
		return nil, E.Cause(err, "initialize kernel TLS")
	}
	return kConn, nil
}

func (w *KTLSClientConfig) Clone() Config {
	return &KTLSClientConfig{
		w.Config.Clone(),
		w.logger,
		w.kernelTx,
		w.kernelRx,
	}
}

type KTlSServerConfig struct {
	ServerConfig
	logger             logger.ContextLogger
	kernelTx, kernelRx bool
}

func (w *KTlSServerConfig) ServerHandshake(ctx context.Context, conn net.Conn) (aTLS.Conn, error) {
	tlsConn, err := aTLS.ServerHandshake(ctx, conn, w.ServerConfig)
	if err != nil {
		return nil, err
	}
	kConn, err := ktls.NewConn(ctx, w.logger, tlsConn, w.kernelTx, w.kernelRx)
	if err != nil {
		tlsConn.Close()
		return nil, E.Cause(err, "initialize kernel TLS")
	}
	return kConn, nil
}

func (w *KTlSServerConfig) Clone() Config {
	return &KTlSServerConfig{
		w.ServerConfig.Clone().(ServerConfig),
		w.logger,
		w.kernelTx,
		w.kernelRx,
	}
}
