//go:build with_gvisor

package tailscale

import (
	"context"
	"crypto/tls"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/certificate"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/tailscale/client/local"
)

func RegisterCertificateProvider(registry *certificate.Registry) {
	certificate.Register[option.TailscaleCertificateProviderOptions](registry, C.TypeTailscale, NewCertificateProvider)
}

var _ adapter.CertificateProviderService = (*CertificateProvider)(nil)

type CertificateProvider struct {
	certificate.Adapter
	endpointTag string
	endpoint    *Endpoint
	localClient *local.Client
}

func NewCertificateProvider(ctx context.Context, _ log.ContextLogger, tag string, options option.TailscaleCertificateProviderOptions) (adapter.CertificateProviderService, error) {
	if options.Endpoint == "" {
		return nil, E.New("missing tailscale endpoint tag")
	}
	endpointManager := service.FromContext[adapter.EndpointManager](ctx)
	if endpointManager == nil {
		return nil, E.New("missing endpoint manager in context")
	}
	rawEndpoint, loaded := endpointManager.Get(options.Endpoint)
	if !loaded {
		return nil, E.New("endpoint not found: ", options.Endpoint)
	}
	endpoint, isTailscale := rawEndpoint.(*Endpoint)
	if !isTailscale {
		return nil, E.New("endpoint is not Tailscale: ", options.Endpoint)
	}
	return &CertificateProvider{
		Adapter:     certificate.NewAdapter(C.TypeTailscale, tag),
		endpointTag: options.Endpoint,
		endpoint:    endpoint,
	}, nil
}

func (p *CertificateProvider) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	localClient, err := p.endpoint.Server().LocalClient()
	if err != nil {
		return E.Cause(err, "initialize tailscale local client for endpoint ", p.endpointTag)
	}
	p.localClient = localClient
	return nil
}

func (p *CertificateProvider) Close() error {
	return nil
}

func (p *CertificateProvider) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	localClient := p.localClient
	if localClient == nil {
		return nil, E.New("Tailscale is not ready yet")
	}
	return localClient.GetCertificate(clientHello)
}
