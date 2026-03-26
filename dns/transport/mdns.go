package transport

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	mDNS "github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

var (
	mdnsIPv4Addr = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	mdnsIPv6Addr = &net.UDPAddr{IP: net.ParseIP("ff02::fb"), Port: 5353}
)

const mdnsDefaultTimeout = 10 * time.Second

type mdnsResult struct {
	response *mDNS.Msg
	err      error
}

func RegisterMDNS(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.MDNSDNSServerOptions](registry, C.DNSTypeMDNS, NewMDNS)
}

var _ adapter.DNSTransport = (*MDNSTransport)(nil)

type MDNSTransport struct {
	dns.TransportAdapter
	ctx    context.Context
	logger logger.ContextLogger
	iface  *net.Interface
}

func NewMDNS(ctx context.Context, logger log.ContextLogger, tag string, options option.MDNSDNSServerOptions) (adapter.DNSTransport, error) {
	var iface *net.Interface
	if options.Interface != "" {
		var err error
		iface, err = net.InterfaceByName(options.Interface)
		if err != nil {
			return nil, E.Cause(err, "find interface ", options.Interface)
		}
	}
	return &MDNSTransport{
		TransportAdapter: dns.NewTransportAdapter(C.DNSTypeMDNS, tag, nil),
		ctx:              ctx,
		logger:           logger,
		iface:            iface,
	}, nil
}

func (t *MDNSTransport) Start(_ adapter.StartStage) error {
	return nil
}

func (t *MDNSTransport) Close() error {
	return nil
}

func (t *MDNSTransport) Reset() {
}

func (t *MDNSTransport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	// Filter questions to only A/AAAA types
	var questions []mDNS.Question
	for _, q := range message.Question {
		switch q.Qtype {
		case mDNS.TypeA, mDNS.TypeAAAA:
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		return nil, dns.RcodeRefused
	}

	query := message.Copy()
	query.Question = questions
	query.RecursionDesired = false
	// Set QU (unicast-response) bit for direct reply
	for i := range query.Question {
		query.Question[i].Qclass |= 1 << 15
	}

	rawQuery, err := query.Pack()
	if err != nil {
		t.logger.Error("mDNS pack query: ", err)
		return nil, dns.RcodeRefused
	}

	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(mdnsDefaultTimeout)
	}

	queryCtx, queryCancel := context.WithDeadline(ctx, deadline)
	defer queryCancel()

	resultCh := make(chan mdnsResult, 4)

	go t.mdnsQuery(queryCtx, "udp4", rawQuery, questions, message.Id, resultCh)
	go t.mdnsQuery(queryCtx, "udp6", rawQuery, questions, message.Id, resultCh)

	var errors []error
	for range 2 {
		select {
		case <-queryCtx.Done():
			t.logger.Error("mDNS query canceled: ", queryCtx.Err())
			return nil, dns.RcodeRefused
		case r := <-resultCh:
			if r.response != nil {
				return r.response, nil
			}
			errors = append(errors, r.err)
		}
	}
	err = E.Errors(errors...)
	t.logger.Error("mDNS query failed: ", err)
	return nil, dns.RcodeRefused
}

func (t *MDNSTransport) mdnsQuery(ctx context.Context, network string, rawQuery []byte, questions []mDNS.Question, msgId uint16, resultCh chan<- mdnsResult) {
	var bindIP net.IP
	var multicastAddr *net.UDPAddr
	if network == "udp4" {
		bindIP = net.IPv4zero
		multicastAddr = mdnsIPv4Addr
	} else {
		bindIP = net.IPv6zero
		multicastAddr = mdnsIPv6Addr
	}

	// Unicast socket on random port for sending and receiving QU responses
	unicastConn, err := net.ListenUDP(network, &net.UDPAddr{IP: bindIP, Port: 0})
	if err != nil {
		resultCh <- mdnsResult{err: err}
		return
	}
	defer func() { _ = unicastConn.Close() }()

	// Multicast socket on port 5353 for receiving multicast responses
	multicastConn, err := mdnsListenUDP(network, &net.UDPAddr{IP: bindIP, Port: 5353})
	if err != nil {
		resultCh <- mdnsResult{err: err}
		return
	}
	defer func() { _ = multicastConn.Close() }()

	// Join multicast group
	t.mdnsJoinGroup(network, multicastConn, multicastAddr)

	// Close connections when context is canceled to unblock reads
	go func() {
		<-ctx.Done()
		_ = unicastConn.Close()
		_ = multicastConn.Close()
	}()

	// Send query from unicast socket (random port)
	if _, err := unicastConn.WriteToUDP(rawQuery, multicastAddr); err != nil {
		resultCh <- mdnsResult{err: E.Cause(err, "send mDNS ", network, " query")}
		return
	}

	// Listen on both sockets, first response wins
	go t.mdnsReadLoop(multicastConn, questions, msgId, resultCh)
	t.mdnsReadLoop(unicastConn, questions, msgId, resultCh)
}

func (t *MDNSTransport) mdnsJoinGroup(network string, conn *net.UDPConn, multicastAddr *net.UDPAddr) {
	joinOnIfaces := func(join func(iface *net.Interface) error) {
		if t.iface != nil {
			_ = join(t.iface)
		} else if ifaces, err := net.Interfaces(); err == nil {
			for i := range ifaces {
				if ifaces[i].Flags&net.FlagUp != 0 && ifaces[i].Flags&net.FlagMulticast != 0 {
					_ = join(&ifaces[i])
				}
			}
		}
	}

	if network == "udp4" {
		p := ipv4.NewPacketConn(conn)
		joinOnIfaces(func(iface *net.Interface) error {
			return p.JoinGroup(iface, multicastAddr)
		})
	} else {
		p := ipv6.NewPacketConn(conn)
		joinOnIfaces(func(iface *net.Interface) error {
			return p.JoinGroup(iface, multicastAddr)
		})
	}
}

func mdnsListenUDP(network string, addr *net.UDPAddr) (*net.UDPConn, error) {
	var lc net.ListenConfig
	lc.Control = control.Append(lc.Control, control.ReuseAddr())
	pc, err := lc.ListenPacket(context.Background(), network, addr.String())
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
}

func (t *MDNSTransport) mdnsReadLoop(conn *net.UDPConn, questions []mDNS.Question, msgId uint16, resultCh chan<- mdnsResult) {
	buf := make([]byte, 65536)
	for {
		n, _, readErr := conn.ReadFromUDP(buf)
		if readErr != nil {
			resultCh <- mdnsResult{err: readErr}
			return
		}

		var response mDNS.Msg
		if err := response.Unpack(buf[:n]); err != nil {
			continue
		}

		if !response.Response {
			continue
		}

		filtered := mdnsFilterResponse(&response, questions)
		if filtered != nil {
			filtered.Id = msgId
			resultCh <- mdnsResult{response: filtered}
			return
		}
	}
}

func mdnsFilterResponse(response *mDNS.Msg, questions []mDNS.Question) *mDNS.Msg {
	type key struct {
		name  string
		qtype uint16
	}
	match := make(map[key]bool, len(questions))
	for _, q := range questions {
		match[key{strings.ToLower(q.Name), q.Qtype}] = true
	}

	var answers []mDNS.RR
	for _, rr := range response.Answer {
		if match[key{strings.ToLower(rr.Header().Name), rr.Header().Rrtype}] {
			answers = append(answers, rr)
		}
	}
	if len(answers) == 0 {
		return nil
	}
	return &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Response:      true,
			Authoritative: true,
			Rcode:         mDNS.RcodeSuccess,
		},
		Question: questions,
		Answer:   answers,
	}
}
