//go:build with_naive_outbound

package naive

import (
	"context"
	"encoding/pem"
	"net"
	"os"
	"strings"

	"github.com/sagernet/cronet-go"
	_ "github.com/sagernet/cronet-go/all"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
)

func RegisterOutbound(registry *outbound.Registry) {
	outbound.Register[option.NaiveOutboundOptions](registry, C.TypeNaive, NewOutbound)
}

type Outbound struct {
	outbound.Adapter
	ctx       context.Context
	logger    logger.ContextLogger
	client    *cronet.NaiveClient
	uotClient *uot.Client
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

	dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
	var dnsResolver cronet.DNSResolverFunc
	if dnsRouter != nil {
		dnsResolver = func(dnsContext context.Context, request *mDNS.Msg) *mDNS.Msg {
			response, err := dnsRouter.Exchange(dnsContext, request, adapter.DNSQueryOptions{})
			if err != nil {
				logger.Error("DNS exchange failed: ", err)
				return dns.FixedResponseStatus(request, mDNS.RcodeServerFailure)
			}
			return response
		}
	}

	var echEnabled bool
	var echConfigList []byte
	var echQueryServerName string
	if options.TLS.ECH != nil && options.TLS.ECH.Enabled {
		echEnabled = true
		echQueryServerName = options.TLS.ECH.QueryServerName
		var echConfig []byte
		if len(options.TLS.ECH.Config) > 0 {
			echConfig = []byte(strings.Join(options.TLS.ECH.Config, "\n"))
		} else if options.TLS.ECH.ConfigPath != "" {
			content, err := os.ReadFile(options.TLS.ECH.ConfigPath)
			if err != nil {
				return nil, E.Cause(err, "read ECH config")
			}
			echConfig = content
		}
		if len(echConfig) > 0 {
			block, rest := pem.Decode(echConfig)
			if block == nil || block.Type != "ECH CONFIGS" || len(rest) > 0 {
				return nil, E.New("invalid ECH configs pem")
			}
			echConfigList = block.Bytes
		}
	}
	var quicCongestionControl cronet.QUICCongestionControl
	switch options.QUICCongestionControl {
	case "":
		quicCongestionControl = cronet.QUICCongestionControlDefault
	case "bbr":
		quicCongestionControl = cronet.QUICCongestionControlBBR
	case "bbr2":
		quicCongestionControl = cronet.QUICCongestionControlBBRv2
	case "cubic":
		quicCongestionControl = cronet.QUICCongestionControlCubic
	case "reno":
		quicCongestionControl = cronet.QUICCongestionControlReno
	default:
		return nil, E.New("unknown quic congestion control: ", options.QUICCongestionControl)
	}
	client, err := cronet.NewNaiveClient(cronet.NaiveClientConfig{
		Context:                           ctx,
		ServerAddress:                     serverAddress,
		ServerName:                        serverName,
		Username:                          options.Username,
		Password:                          options.Password,
		InsecureConcurrency:               options.InsecureConcurrency,
		ExtraHeaders:                      extraHeaders,
		TrustedRootCertificates:           trustedRootCertificates,
		TrustedCertificatePublicKeySHA256: options.TLS.CertificatePublicKeySHA256,
		Dialer:                            outboundDialer,
		DNSResolver:                       dnsResolver,
		ECHEnabled:                        echEnabled,
		ECHConfigList:                     echConfigList,
		ECHQueryServerName:                echQueryServerName,
		QUIC:                              options.QUIC,
		QUICCongestionControl:             quicCongestionControl,
	})
	if err != nil {
		return nil, err
	}
	var uotClient *uot.Client
	uotOptions := common.PtrValueOrDefault(options.UDPOverTCP)
	if uotOptions.Enabled {
		uotClient = &uot.Client{
			Dialer:  &naiveDialer{client},
			Version: uotOptions.Version,
		}
	}
	var networks []string
	if uotClient != nil {
		networks = []string{N.NetworkTCP, N.NetworkUDP}
	} else {
		networks = []string{N.NetworkTCP}
	}
	return &Outbound{
		Adapter:   outbound.NewAdapterWithDialerOptions(C.TypeNaive, tag, networks, options.DialerOptions),
		ctx:       ctx,
		logger:    logger,
		client:    client,
		uotClient: uotClient,
	}, nil
}

func (h *Outbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := h.client.Start()
	if err != nil {
		return err
	}
	h.logger.Info("NaiveProxy started, version: ", h.client.Engine().Version())
	return nil
}

func (h *Outbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	switch N.NetworkName(network) {
	case N.NetworkTCP:
		h.logger.InfoContext(ctx, "outbound connection to ", destination)
		return h.client.DialEarly(destination)
	case N.NetworkUDP:
		if h.uotClient == nil {
			return nil, E.New("UDP is not supported unless UDP over TCP is enabled")
		}
		h.logger.InfoContext(ctx, "outbound UoT packet connection to ", destination)
		return h.uotClient.DialContext(ctx, network, destination)
	default:
		return nil, E.Extend(N.ErrUnknownNetwork, network)
	}
}

func (h *Outbound) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	if h.uotClient == nil {
		return nil, E.New("UDP is not supported unless UDP over TCP is enabled")
	}
	return h.uotClient.ListenPacket(ctx, destination)
}

func (h *Outbound) Close() error {
	return h.client.Close()
}

func (h *Outbound) Client() *cronet.NaiveClient {
	return h.client
}

type naiveDialer struct {
	*cronet.NaiveClient
}

func (d *naiveDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return d.NaiveClient.DialEarly(destination)
}
