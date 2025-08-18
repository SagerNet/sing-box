//go:build darwin

package local

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/logger"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*ResolvTransport)(nil)

type ResolvTransport struct {
	dns.TransportAdapter
	ctx    context.Context
	logger logger.ContextLogger
}

func NewResolvTransport(ctx context.Context, logger log.ContextLogger, tag string) (adapter.DNSTransport, error) {
	return &ResolvTransport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeLocal, tag, nil),
		ctx:              ctx,
		logger:           logger,
	}, nil
}

func (t *ResolvTransport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *ResolvTransport) Close() error {
	return nil
}

func (t *ResolvTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	return doBlockingWithCtx(ctx, func() (*mDNS.Msg, error) {
		return cgoResSearch(question.Name, int(question.Qtype), int(question.Qclass))
	})
}
