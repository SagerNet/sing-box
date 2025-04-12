package local

import (
	"context"
	"math/rand"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/hosts"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
)

var _ adapter.DNSTransport = (*Transport)(nil)

type Transport struct {
	dns.TransportAdapter
	ctx    context.Context
	hosts  *hosts.File
	dialer N.Dialer
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
	transportDialer, err := dns.NewLocalDialer(ctx, options)
	if err != nil {
		return nil, err
	}
	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeLocal, tag, options),
		ctx:              ctx,
		hosts:            hosts.NewFile(hosts.DefaultPath),
		dialer:           transportDialer,
	}, nil
}

func (t *Transport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *Transport) Close() error {
	return nil
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	question := message.Question[0]
	domain := dns.FqdnToDomain(question.Name)
	if question.Qtype == mDNS.TypeA || question.Qtype == mDNS.TypeAAAA {
		addresses := t.hosts.Lookup(domain)
		if len(addresses) > 0 {
			return dns.FixedResponse(message.Id, question, addresses, C.DefaultDNSTTL), nil
		}
	}
	systemConfig := getSystemDNSConfig(t.ctx)
	if systemConfig.singleRequest || !(message.Question[0].Qtype == mDNS.TypeA || message.Question[0].Qtype == mDNS.TypeAAAA) {
		return t.exchangeSingleRequest(ctx, systemConfig, message, domain)
	} else {
		return t.exchangeParallel(ctx, systemConfig, message, domain)
	}
}

func (t *Transport) exchangeSingleRequest(ctx context.Context, systemConfig *dnsConfig, message *mDNS.Msg, domain string) (*mDNS.Msg, error) {
	var lastErr error
	for _, fqdn := range systemConfig.nameList(domain) {
		response, err := t.tryOneName(ctx, systemConfig, fqdn, message)
		if err != nil {
			lastErr = err
			continue
		}
		return response, nil
	}
	return nil, lastErr
}

func (t *Transport) exchangeParallel(ctx context.Context, systemConfig *dnsConfig, message *mDNS.Msg, domain string) (*mDNS.Msg, error) {
	returned := make(chan struct{})
	defer close(returned)
	type queryResult struct {
		response *mDNS.Msg
		err      error
	}
	results := make(chan queryResult)
	startRacer := func(ctx context.Context, fqdn string) {
		response, err := t.tryOneName(ctx, systemConfig, fqdn, message)
		if err == nil {
			addresses, _ := dns.MessageToAddresses(response)
			if len(addresses) == 0 {
				err = E.New(fqdn, ": empty result")
			}
		}
		select {
		case results <- queryResult{response, err}:
		case <-returned:
		}
	}
	queryCtx, queryCancel := context.WithCancel(ctx)
	defer queryCancel()
	var nameCount int
	for _, fqdn := range systemConfig.nameList(domain) {
		nameCount++
		go startRacer(queryCtx, fqdn)
	}
	var errors []error
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			if result.err == nil {
				return result.response, nil
			}
			errors = append(errors, result.err)
			if len(errors) == nameCount {
				return nil, E.Errors(errors...)
			}
		}
	}
}

func (t *Transport) tryOneName(ctx context.Context, config *dnsConfig, fqdn string, message *mDNS.Msg) (*mDNS.Msg, error) {
	serverOffset := config.serverOffset()
	sLen := uint32(len(config.servers))
	var lastErr error
	for i := 0; i < config.attempts; i++ {
		for j := uint32(0); j < sLen; j++ {
			server := config.servers[(serverOffset+j)%sLen]
			question := message.Question[0]
			question.Name = fqdn
			response, err := t.exchangeOne(ctx, M.ParseSocksaddr(server), question, config.timeout, config.useTCP, config.trustAD)
			if err != nil {
				lastErr = err
				continue
			}
			return response, nil
		}
	}
	return nil, E.Cause(lastErr, fqdn)
}

func (t *Transport) exchangeOne(ctx context.Context, server M.Socksaddr, question mDNS.Question, timeout time.Duration, useTCP, ad bool) (*mDNS.Msg, error) {
	if server.Port == 0 {
		server.Port = 53
	}
	var networks []string
	if useTCP {
		networks = []string{N.NetworkTCP}
	} else {
		networks = []string{N.NetworkUDP, N.NetworkTCP}
	}
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Id:                uint16(rand.Uint32()),
			RecursionDesired:  true,
			AuthenticatedData: ad,
		},
		Question: []mDNS.Question{question},
		Compress: true,
	}
	request.SetEdns0(maxDNSPacketSize, false)
	buffer := buf.Get(buf.UDPBufferSize)
	defer buf.Put(buffer)
	for _, network := range networks {
		ctx, cancel := context.WithDeadline(ctx, time.Now().Add(timeout))
		defer cancel()
		conn, err := t.dialer.DialContext(ctx, network, server)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		if deadline, loaded := ctx.Deadline(); loaded && !deadline.IsZero() {
			conn.SetDeadline(deadline)
		}
		rawMessage, err := request.PackBuffer(buffer)
		if err != nil {
			return nil, E.Cause(err, "pack request")
		}
		_, err = conn.Write(rawMessage)
		if err != nil {
			return nil, E.Cause(err, "write request")
		}
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, E.Cause(err, "read response")
		}
		var response mDNS.Msg
		err = response.Unpack(buffer[:n])
		if err != nil {
			return nil, E.Cause(err, "unpack response")
		}
		if response.Truncated && network == N.NetworkUDP {
			continue
		}
		return &response, nil
	}
	panic("unexpected")
}
