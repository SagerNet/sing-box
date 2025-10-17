package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"

	"github.com/sagernet/sing-box/common/badtls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
)

func NewDialerFromOptions(ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, serverAddress string, options option.OutboundTLSOptions) (N.Dialer, error) {
	if !options.Enabled {
		return dialer, nil
	}
	config, err := NewClientWithOptions(ClientOptions{
		Context:       ctx,
		Logger:        logger,
		ServerAddress: serverAddress,
		Options:       options,
	})
	if err != nil {
		return nil, err
	}
	return NewDialer(dialer, config), nil
}

func NewClient(ctx context.Context, logger logger.ContextLogger, serverAddress string, options option.OutboundTLSOptions) (Config, error) {
	return NewClientWithOptions(ClientOptions{
		Context:       ctx,
		Logger:        logger,
		ServerAddress: serverAddress,
		Options:       options,
	})
}

type ClientOptions struct {
	Context        context.Context
	Logger         logger.ContextLogger
	ServerAddress  string
	Options        option.OutboundTLSOptions
	KTLSCompatible bool
}

func NewClientWithOptions(options ClientOptions) (Config, error) {
	if !options.Options.Enabled {
		return nil, nil
	}
	if !options.KTLSCompatible {
		if options.Options.KernelTx {
			options.Logger.Warn("enabling kTLS TX in current scenarios will definitely reduce performance, please checkout https://sing-box.sagernet.org/configuration/shared/tls/#kernel_tx")
		}
	}
	if options.Options.KernelRx {
		options.Logger.Warn("enabling kTLS RX will definitely reduce performance, please checkout https://sing-box.sagernet.org/configuration/shared/tls/#kernel_rx")
	}
	if options.Options.Reality != nil && options.Options.Reality.Enabled {
		return NewRealityClient(options.Context, options.Logger, options.ServerAddress, options.Options)
	} else if options.Options.UTLS != nil && options.Options.UTLS.Enabled {
		return NewUTLSClient(options.Context, options.Logger, options.ServerAddress, options.Options)
	}
	return NewSTDClient(options.Context, options.Logger, options.ServerAddress, options.Options)
}

func ClientHandshake(ctx context.Context, conn net.Conn, config Config) (Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, C.TCPTimeout)
	defer cancel()
	tlsConn, err := aTLS.ClientHandshake(ctx, conn, config)
	if err != nil {
		return nil, err
	}
	readWaitConn, err := badtls.NewReadWaitConn(tlsConn)
	if err == nil {
		return readWaitConn, nil
	} else if err != os.ErrInvalid {
		return nil, err
	}
	return tlsConn, nil
}

type Dialer interface {
	N.Dialer
	DialTLSContext(ctx context.Context, destination M.Socksaddr) (Conn, error)
}

type defaultDialer struct {
	dialer N.Dialer
	config Config
}

func NewDialer(dialer N.Dialer, config Config) Dialer {
	return &defaultDialer{dialer, config}
}

func (d *defaultDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if N.NetworkName(network) != N.NetworkTCP {
		return nil, os.ErrInvalid
	}
	return d.DialTLSContext(ctx, destination)
}

func (d *defaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (d *defaultDialer) DialTLSContext(ctx context.Context, destination M.Socksaddr) (Conn, error) {
	return d.dialContext(ctx, destination, true)
}

func (d *defaultDialer) dialContext(ctx context.Context, destination M.Socksaddr, echRetry bool) (Conn, error) {
	conn, err := d.dialer.DialContext(ctx, N.NetworkTCP, destination)
	if err != nil {
		return nil, err
	}
	tlsConn, err := aTLS.ClientHandshake(ctx, conn, d.config)
	if err != nil {
		conn.Close()
		var echErr *tls.ECHRejectionError
		if echRetry && errors.As(err, &echErr) && len(echErr.RetryConfigList) > 0 {
			if echConfig, isECH := d.config.(ECHCapableConfig); isECH {
				echConfig.SetECHConfigList(echErr.RetryConfigList)
				return d.dialContext(ctx, destination, false)
			}
		}
		return nil, err
	}
	return tlsConn, nil
}

func (d *defaultDialer) Upstream() any {
	return d.dialer
}
