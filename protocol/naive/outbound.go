//go:build with_naive_outbound

package naive

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/sagernet/cronet-go"
	_ "github.com/sagernet/cronet-go/all"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.NaiveOutboundOptions](registry, C.TypeNaive, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx    context.Context
	logger logger.ContextLogger
	client *cronet.NaiveClient
}

func NewOutbound(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.NaiveOutboundOptions) (adapter.Outbound, error) {
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, C.ErrTLSRequired
	}
	if options.TLS.DisableSNI {
		return nil, E.New("disable_sni is not supported on naive outbound")
	}
	if options.TLS.Insecure {
		return nil, E.New("insecure is not supported on naive outbound")
	}
	if len(options.TLS.ALPN) > 0 {
		return nil, E.New("alpn is not supported on naive outbound")
	}
	if options.TLS.MinVersion != "" {
		return nil, E.New("min_version is not supported on naive outbound")
	}
	if options.TLS.MaxVersion != "" {
		return nil, E.New("max_version is not supported on naive outbound")
	}
	if len(options.TLS.CipherSuites) > 0 {
		return nil, E.New("cipher_suites is not supported on naive outbound")
	}
	if len(options.TLS.CurvePreferences) > 0 {
		return nil, E.New("curve_preferences is not supported on naive outbound")
	}
	if len(options.TLS.ClientCertificate) > 0 || options.TLS.ClientCertificatePath != "" {
		return nil, E.New("client_certificate is not supported on naive outbound")
	}
	if len(options.TLS.ClientKey) > 0 || options.TLS.ClientKeyPath != "" {
		return nil, E.New("client_key is not supported on naive outbound")
	}
	if options.TLS.Fragment || options.TLS.RecordFragment {
		return nil, E.New("fragment is not supported on naive outbound")
	}
	if options.TLS.KernelTx || options.TLS.KernelRx {
		return nil, E.New("kernel TLS is not supported on naive outbound")
	}
	if options.TLS.ECH != nil && options.TLS.ECH.Enabled {
		return nil, E.New("ECH is not currently supported on naive outbound")
	}
	if options.TLS.UTLS != nil && options.TLS.UTLS.Enabled {
		return nil, E.New("uTLS is not supported on naive outbound")
	}
	if options.TLS.Reality != nil && options.TLS.Reality.Enabled {
		return nil, E.New("reality is not supported on naive outbound")
	}

	serverAddress := options.ServerOptions.Build()

	var serverName string
	if options.TLS.ServerName != "" {
		serverName = options.TLS.ServerName
	} else {
		serverName = serverAddress.AddrString()
	}

	outboundDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:          ctx,
		Options:          options.DialerOptions,
		RemoteIsDomain:   true,
		ResolverOnDetour: true,
		NewDialer:        true,
	})
	if err != nil {
		return nil, err
	}

	var trustedRootCertificates string
	if len(options.TLS.Certificate) > 0 {
		trustedRootCertificates = strings.Join(options.TLS.Certificate, "\n")
	} else if options.TLS.CertificatePath != "" {
		content, err := os.ReadFile(options.TLS.CertificatePath)
		if err != nil {
			return nil, E.Cause(err, "read certificate")
		}
		trustedRootCertificates = string(content)
	}

	extraHeaders := make(map[string]string)
	for key, values := range options.ExtraHeaders.Build() {
		if len(values) > 0 {
			extraHeaders[key] = values[0]
		}
	}

	client, err := cronet.NewNaiveClient(cronet.NaiveClientConfig{
		Context:                    ctx,
		ServerAddress:              serverAddress,
		ServerName:                 serverName,
		Username:                   options.Username,
		Password:                   options.Password,
		Concurrency:                options.InsecureConcurrency,
		ExtraHeaders:               extraHeaders,
		TrustedRootCertificates:    trustedRootCertificates,
		CertificatePublicKeySHA256: options.TLS.CertificatePublicKeySHA256,
		Dialer:                     outboundDialer,
	})
	if err != nil {
		return nil, err
	}

	return &Outbound{
		Adapter: outbound.NewAdapterWithDialerOptions(C.TypeNaive, tag, []string{N.NetworkTCP}, options.DialerOptions),
		ctx:     ctx,
		logger:  logger,
		client:  client,
	}, nil
}

func (o *Outbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := o.client.Start()
	if err != nil {
		return err
	}
	o.logger.Info("NaiveProxy started, version: ", o.client.Engine().Version())
	return nil
}

func (o *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	ctx, metadata := adapter.ExtendContext(ctx)
	metadata.Outbound = o.Tag()
	metadata.Destination = destination
	o.logger.InfoContext(ctx, "outbound connection to ", destination)
	return o.client.DialContext(ctx, destination)
}

func (o *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (o *Outbound) Close() error {
	return o.client.Close()
}

func (o *Outbound) StartNetLogToFile(fileName string, logAll bool) bool {
	return o.client.Engine().StartNetLogToFile(fileName, logAll)
}

func (o *Outbound) StopNetLog() {
	o.client.Engine().StopNetLog()
}
