package mdns

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/sing/service"

	mDNS "github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	mdnsPort        = 5353
	mdnsClassTopBit = 1 << 15
	// mdnsTimeout is the hard cap of the response collection window.
	mdnsTimeout = time.Second
	// mdnsSettleTimeout is how long we keep collecting after the first answer
	// arrived, so answers from other responders/interfaces can be merged.
	mdnsSettleTimeout = 250 * time.Millisecond
	// mdnsHeadroom is reserved from the caller's deadline so the merged
	// response can still be delivered before upstream timeouts fire.
	mdnsHeadroom = 500 * time.Millisecond
)

var (
	mdnsGroupIPv4  = net.IPv4(224, 0, 0, 251)
	mdnsGroupIPv6  = net.ParseIP("ff02::fb")
	mdnsLocalZones = []string{
		"local.",
		"254.169.in-addr.arpa.",
		"8.e.f.ip6.arpa.",
		"9.e.f.ip6.arpa.",
		"a.e.f.ip6.arpa.",
		"b.e.f.ip6.arpa.",
	}
)

func IsLocalDomain(name string) bool {
	canonical := mDNS.CanonicalName(name)
	return common.Any(mdnsLocalZones, func(zone string) bool {
		return canonical == zone || strings.HasSuffix(canonical, "."+zone)
	})
}

func RegisterTransport(registry *dns.TransportRegistry) {
	dns.RegisterTransport[option.MDNSDNSServerOptions](registry, C.DNSTypeMDNS, NewTransport)
}

var (
	_ adapter.DNSTransport                    = (*Transport)(nil)
	_ adapter.DNSTransportWithPreferredDomain = (*Transport)(nil)
)

type Transport struct {
	dns.TransportAdapter
	ctx            context.Context
	logger         logger.ContextLogger
	networkManager adapter.NetworkManager
	interfaceNames badoption.Listable[string]
}

func NewTransport(ctx context.Context, logger log.ContextLogger, tag string, options option.MDNSDNSServerOptions) (adapter.DNSTransport, error) {
	return &Transport{
		TransportAdapter: dns.NewTransportAdapterWithLocalOptions(C.DNSTypeMDNS, tag, options.LocalDNSServerOptions),
		ctx:              ctx,
		logger:           logger,
		networkManager:   service.FromContext[adapter.NetworkManager](ctx),
		interfaceNames:   options.Interface,
	}, nil
}

func NewRawTransport(transportAdapter dns.TransportAdapter, ctx context.Context, logger log.ContextLogger) *Transport {
	return &Transport{
		TransportAdapter: transportAdapter,
		ctx:              ctx,
		logger:           logger,
		networkManager:   service.FromContext[adapter.NetworkManager](ctx),
	}
}

func (t *Transport) Start(stage adapter.StartStage) error {
	return nil
}

func (t *Transport) Close() error {
	return nil
}

func (t *Transport) Reset() {
}

func (t *Transport) PreferredDomain(domain string) bool {
	return IsLocalDomain(domain)
}

func (t *Transport) Exchange(ctx context.Context, message *mDNS.Msg) (*mDNS.Msg, error) {
	targets, err := t.queryTargets()
	if err != nil {
		return nil, E.Cause(err, "mdns: prepare interfaces")
	}
	request := makeQueryMessage(message)
	rawMessage, err := request.Pack()
	if err != nil {
		return nil, E.Cause(err, "mdns: pack request")
	}
	question := message.Question[0]
	deadline := queryDeadline(ctx, time.Now())
	exchangeCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	response := newResponse(message)
	var (
		responseAccess sync.Mutex
		seenRecords    = make(map[string]bool)
		settleTimer    *time.Timer
		lastErr        error
	)
	defer func() {
		responseAccess.Lock()
		defer responseAccess.Unlock()
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()
	// PTR and ANY questions expect shared records from multiple responders,
	// so they always use the full collection window.
	settleEligible := question.Qtype != mDNS.TypePTR && question.Qtype != mDNS.TypeANY
	onResponse := func(candidate *mDNS.Msg) {
		responseAccess.Lock()
		defer responseAccess.Unlock()
		mergeResponse(response, candidate, seenRecords)
		if settleEligible && settleTimer == nil && len(response.Answer) > 0 {
			settleTimer = time.AfterFunc(mdnsSettleTimeout, cancel)
		}
	}
	var (
		group     task.Group
		sentQuery atomic.Bool
	)
	for _, target := range targets {
		group.Append0(func(taskCtx context.Context) error {
			targetErr := t.exchangeTarget(taskCtx, target, rawMessage, question, deadline, &sentQuery, onResponse)
			if targetErr != nil {
				t.logger.TraceContext(ctx, targetErr)
				responseAccess.Lock()
				lastErr = targetErr
				responseAccess.Unlock()
			}
			return nil
		})
	}
	groupErr := group.Run(exchangeCtx)
	responseAccess.Lock()
	defer responseAccess.Unlock()
	if len(response.Answer) > 0 || len(response.Ns) > 0 || len(response.Extra) > 0 {
		return response, nil
	}
	if sentQuery.Load() {
		// The query went out but nobody answered, which is normal for mDNS
		// (e.g. unsupported record types or offline hosts). Return NODATA
		// instead of an error so the inbound connection is not torn down.
		return response, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	if groupErr != nil && ctx.Err() != nil {
		return nil, groupErr
	}
	return nil, E.New("mdns: query timeout")
}

// queryDeadline bounds the collection window: at most mdnsTimeout, and never
// closer to the caller's deadline than mdnsHeadroom, so a merged response is
// always ready before upstream contexts and connection timeouts expire.
func queryDeadline(ctx context.Context, now time.Time) time.Time {
	deadline := now.Add(mdnsTimeout)
	if ctxDeadline, loaded := ctx.Deadline(); loaded && !ctxDeadline.IsZero() {
		if clamped := ctxDeadline.Add(-mdnsHeadroom); clamped.Before(deadline) {
			deadline = clamped
		}
	}
	return deadline
}

type queryTarget struct {
	iface  control.Interface
	family string
}

func (t *Transport) exchangeTarget(ctx context.Context, target queryTarget, rawMessage []byte, question mDNS.Question, deadline time.Time, sentQuery *atomic.Bool, onResponse func(*mDNS.Msg)) error {
	packetConn, destination, err := t.listenPacket(ctx, target)
	if err != nil {
		return err
	}
	defer packetConn.Close()
	// Wake up the blocking read when the exchange is canceled, either by the
	// settle timer after the first answer or by the upstream context.
	stopAfterFunc := context.AfterFunc(ctx, func() {
		packetConn.SetReadDeadline(time.Unix(0, 1))
	})
	defer stopAfterFunc()

	_, err = packetConn.WriteTo(rawMessage, destination)
	if err != nil {
		return E.Cause(err, "mdns: write request on ", target.iface.Name, " ", target.family)
	}
	sentQuery.Store(true)
	err = packetConn.SetReadDeadline(deadline)
	if err != nil {
		return E.Cause(err, "mdns: set deadline on ", target.iface.Name, " ", target.family)
	}
	if ctx.Err() != nil {
		// The AfterFunc may have fired before the deadline above overwrote it.
		return nil
	}
	buffer := buf.Get(buf.UDPBufferSize)
	defer buf.Put(buffer)
	for {
		n, source, readErr := packetConn.ReadFrom(buffer)
		if readErr != nil {
			if ctx.Err() != nil || E.IsTimeout(readErr) {
				return nil
			}
			return E.Cause(readErr, "mdns: read response on ", target.iface.Name, " ", target.family)
		}
		if !validSource(source, target) {
			continue
		}
		var candidate mDNS.Msg
		err = candidate.Unpack(buffer[:n])
		if err != nil {
			t.logger.TraceContext(ctx, "mdns: unpack response: ", err)
			continue
		}
		if !validResponse(&candidate, question) {
			continue
		}
		normalizeResponse(&candidate, question)
		onResponse(&candidate)
	}
}

func (t *Transport) listenPacket(ctx context.Context, target queryTarget) (net.PacketConn, net.Addr, error) {
	var listenConfig net.ListenConfig
	listenConfig.Control = control.Append(listenConfig.Control, control.BindToInterface(t.networkManager.InterfaceFinder(), target.iface.Name, target.iface.Index))
	netInterface := target.iface.NetInterface()
	switch target.family {
	case "udp4":
		packetConn, err := listenConfig.ListenPacket(ctx, "udp4", "0.0.0.0:0")
		if err != nil {
			return nil, nil, E.Cause(err, "mdns: listen on ", target.iface.Name, " udp4")
		}
		ipv4Conn := ipv4.NewPacketConn(packetConn)
		err = ipv4Conn.SetMulticastInterface(&netInterface)
		if err != nil {
			packetConn.Close()
			return nil, nil, E.Cause(err, "mdns: set multicast interface on ", target.iface.Name, " udp4")
		}
		_ = ipv4Conn.SetMulticastTTL(255)
		return packetConn, &net.UDPAddr{IP: mdnsGroupIPv4, Port: mdnsPort}, nil
	case "udp6":
		packetConn, err := listenConfig.ListenPacket(ctx, "udp6", "[::]:0")
		if err != nil {
			return nil, nil, E.Cause(err, "mdns: listen on ", target.iface.Name, " udp6")
		}
		ipv6Conn := ipv6.NewPacketConn(packetConn)
		err = ipv6Conn.SetMulticastInterface(&netInterface)
		if err != nil {
			packetConn.Close()
			return nil, nil, E.Cause(err, "mdns: set multicast interface on ", target.iface.Name, " udp6")
		}
		_ = ipv6Conn.SetMulticastHopLimit(255)
		return packetConn, &net.UDPAddr{IP: mdnsGroupIPv6, Port: mdnsPort, Zone: target.iface.Name}, nil
	default:
		return nil, nil, E.New("mdns: unknown network: ", target.family)
	}
}

func (t *Transport) queryTargets() ([]queryTarget, error) {
	interfaces, err := t.fetchInterfaces()
	if err != nil {
		return nil, err
	}
	var targets []queryTarget
	for _, iface := range interfaces {
		supports4, supports6 := interfaceFamilies(iface)
		if supports4 {
			targets = append(targets, queryTarget{
				iface:  iface,
				family: "udp4",
			})
		}
		if supports6 {
			targets = append(targets, queryTarget{
				iface:  iface,
				family: "udp6",
			})
		}
	}
	if len(targets) == 0 {
		return nil, E.New("missing usable mDNS interfaces")
	}
	return targets, nil
}

func (t *Transport) fetchInterfaces() ([]control.Interface, error) {
	finder := t.networkManager.InterfaceFinder()
	var interfaces []control.Interface
	if len(t.interfaceNames) > 0 {
		for _, interfaceName := range t.interfaceNames {
			iface, err := finder.ByName(interfaceName)
			if err != nil {
				t.logger.Warn("mdns: interface ", interfaceName, " not found")
				continue
			}
			if !isUsableInterface(*iface) {
				t.logger.Warn("mdns: interface ", interfaceName, " is not usable")
				continue
			}
			interfaces = append(interfaces, *iface)
		}
	} else {
		interfaces = common.Filter(finder.Interfaces(), isUsableInterface)
	}
	if len(interfaces) == 0 {
		return nil, E.New("mdns: missing usable interface")
	}
	return interfaces, nil
}

func isUsableInterface(iface control.Interface) bool {
	return iface.Flags&net.FlagUp != 0 &&
		iface.Flags&net.FlagMulticast != 0 &&
		iface.Flags&net.FlagLoopback == 0 &&
		// mDNS is link-local; point-to-point tunnels (VPNs) cannot carry it
		// and multicast writes on them fail on some platforms.
		iface.Flags&net.FlagPointToPoint == 0
}

func interfaceFamilies(iface control.Interface) (supports4, supports6 bool) {
	for _, prefix := range iface.Addresses {
		addr := prefix.Addr()
		if addr.IsLoopback() {
			continue
		}
		if addr.Is4() {
			supports4 = true
		} else if addr.Is6() && !addr.Is4In6() {
			supports6 = true
		}
		if supports4 && supports6 {
			return
		}
	}
	return
}

func makeQueryMessage(message *mDNS.Msg) *mDNS.Msg {
	request := &mDNS.Msg{
		Question: slices.Clone(message.Question),
	}
	for i := range request.Question {
		stripQuestionClass(&request.Question[i])
	}
	return request
}

func newResponse(message *mDNS.Msg) *mDNS.Msg {
	response := newResponseFromQuestion(message.Question[0])
	response.Id = message.Id
	return response
}

func newResponseFromQuestion(question mDNS.Question) *mDNS.Msg {
	stripQuestionClass(&question)
	return &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Response:      true,
			Authoritative: true,
			Rcode:         mDNS.RcodeSuccess,
		},
		Question: []mDNS.Question{question},
	}
}

func validSource(source net.Addr, target queryTarget) bool {
	sourceUDP, isUDP := source.(*net.UDPAddr)
	if !isUDP || sourceUDP.Port != mdnsPort {
		return false
	}
	sourceAddr, loaded := netip.AddrFromSlice(sourceUDP.IP)
	if !loaded {
		return false
	}
	sourceAddr = sourceAddr.Unmap()
	if (target.family == "udp4" && !sourceAddr.Is4()) || (target.family == "udp6" && !sourceAddr.Is6()) {
		return false
	}
	for _, prefix := range target.iface.Addresses {
		if prefix.Contains(sourceAddr) {
			return true
		}
	}
	return false
}

func validResponse(response *mDNS.Msg, question mDNS.Question) bool {
	if !response.Response ||
		response.Opcode != mDNS.OpcodeQuery ||
		response.Rcode != mDNS.RcodeSuccess {
		return false
	}
	for _, responseQuestion := range response.Question {
		if questionMatches(responseQuestion, question) {
			return true
		}
	}
	return responseHasMatchingRecord(response, question)
}

func responseHasMatchingRecord(response *mDNS.Msg, question mDNS.Question) bool {
	for _, recordList := range [][]mDNS.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			if recordMatchesQuestion(record, question) {
				return true
			}
		}
	}
	return false
}

func questionMatches(left mDNS.Question, right mDNS.Question) bool {
	stripQuestionClass(&left)
	stripQuestionClass(&right)
	return left.Qtype == right.Qtype &&
		left.Qclass == right.Qclass &&
		strings.EqualFold(left.Name, right.Name)
}

func recordMatchesQuestion(record mDNS.RR, question mDNS.Question) bool {
	header := record.Header()
	return strings.EqualFold(header.Name, question.Name) &&
		(question.Qtype == mDNS.TypeANY ||
			header.Rrtype == question.Qtype ||
			header.Rrtype == mDNS.TypeCNAME)
}

func normalizeResponse(response *mDNS.Msg, question mDNS.Question) {
	response.Id = 0
	response.Question = []mDNS.Question{question}
	for i := range response.Question {
		stripQuestionClass(&response.Question[i])
	}
	for _, recordList := range [][]mDNS.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			stripRecordClass(record)
		}
	}
}

func mergeResponse(destination *mDNS.Msg, source *mDNS.Msg, seenRecords map[string]bool) {
	appendRecords := func(destinationRecords *[]mDNS.RR, sourceRecords []mDNS.RR) {
		for _, record := range sourceRecords {
			key := record.String()
			if seenRecords[key] {
				continue
			}
			seenRecords[key] = true
			*destinationRecords = append(*destinationRecords, record)
		}
	}
	appendRecords(&destination.Answer, source.Answer)
	appendRecords(&destination.Ns, source.Ns)
	appendRecords(&destination.Extra, source.Extra)
}

func stripQuestionClass(question *mDNS.Question) {
	question.Qclass &^= mdnsClassTopBit
}

func stripRecordClass(record mDNS.RR) {
	record.Header().Class &^= mdnsClassTopBit
}
