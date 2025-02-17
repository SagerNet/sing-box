package rule

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

func NewRuleAction(ctx context.Context, logger logger.ContextLogger, action option.RuleAction) (adapter.RuleAction, error) {
	switch action.Action {
	case "":
		return nil, nil
	case C.RuleActionTypeRoute:
		return &RuleActionRoute{
			Outbound: action.RouteOptions.Outbound,
			RuleActionRouteOptions: RuleActionRouteOptions{
				OverrideAddress:           M.ParseSocksaddrHostPort(action.RouteOptions.OverrideAddress, 0),
				OverridePort:              action.RouteOptions.OverridePort,
				NetworkStrategy:           (*C.NetworkStrategy)(action.RouteOptions.NetworkStrategy),
				FallbackDelay:             time.Duration(action.RouteOptions.FallbackDelay),
				UDPDisableDomainUnmapping: action.RouteOptions.UDPDisableDomainUnmapping,
				UDPConnect:                action.RouteOptions.UDPConnect,
				TLSFragment:               action.RouteOptions.TLSFragment,
				TLSFragmentFallbackDelay:  time.Duration(action.RouteOptions.TLSFragmentFallbackDelay),
				TLSRecordFragment:         action.RouteOptions.TLSRecordFragment,
			},
		}, nil
	case C.RuleActionTypeRouteOptions:
		return &RuleActionRouteOptions{
			OverrideAddress:           M.ParseSocksaddrHostPort(action.RouteOptionsOptions.OverrideAddress, 0),
			OverridePort:              action.RouteOptionsOptions.OverridePort,
			NetworkStrategy:           (*C.NetworkStrategy)(action.RouteOptionsOptions.NetworkStrategy),
			FallbackDelay:             time.Duration(action.RouteOptionsOptions.FallbackDelay),
			UDPDisableDomainUnmapping: action.RouteOptionsOptions.UDPDisableDomainUnmapping,
			UDPConnect:                action.RouteOptionsOptions.UDPConnect,
			UDPTimeout:                time.Duration(action.RouteOptionsOptions.UDPTimeout),
			TLSFragment:               action.RouteOptionsOptions.TLSFragment,
			TLSFragmentFallbackDelay:  time.Duration(action.RouteOptionsOptions.TLSFragmentFallbackDelay),
			TLSRecordFragment:         action.RouteOptionsOptions.TLSRecordFragment,
		}, nil
	case C.RuleActionTypeDirect:
		directDialer, err := dialer.New(ctx, option.DialerOptions(action.DirectOptions), false)
		if err != nil {
			return nil, err
		}
		var description string
		descriptions := action.DirectOptions.Descriptions()
		switch len(descriptions) {
		case 0:
		case 1:
			description = F.ToString("(", descriptions[0], ")")
		case 2:
			description = F.ToString("(", descriptions[0], ",", descriptions[1], ")")
		default:
			description = F.ToString("(", descriptions[0], ",", descriptions[1], ",...)")
		}
		return &RuleActionDirect{
			Dialer:      directDialer,
			description: description,
		}, nil
	case C.RuleActionTypeReject:
		return &RuleActionReject{
			Method: action.RejectOptions.Method,
			NoDrop: action.RejectOptions.NoDrop,
			logger: logger,
		}, nil
	case C.RuleActionTypeHijackDNS:
		return &RuleActionHijackDNS{}, nil
	case C.RuleActionTypeSniff:
		sniffAction := &RuleActionSniff{
			SnifferNames: action.SniffOptions.Sniffer,
			Timeout:      time.Duration(action.SniffOptions.Timeout),
		}
		return sniffAction, sniffAction.build()
	case C.RuleActionTypeResolve:
		return &RuleActionResolve{
			Server:       action.ResolveOptions.Server,
			Strategy:     C.DomainStrategy(action.ResolveOptions.Strategy),
			DisableCache: action.ResolveOptions.DisableCache,
			RewriteTTL:   action.ResolveOptions.RewriteTTL,
			ClientSubnet: action.ResolveOptions.ClientSubnet.Build(netip.Prefix{}),
		}, nil
	default:
		panic(F.ToString("unknown rule action: ", action.Action))
	}
}

func NewDNSRuleAction(logger logger.ContextLogger, action option.DNSRuleAction) adapter.RuleAction {
	switch action.Action {
	case "":
		return nil
	case C.RuleActionTypeRoute:
		return &RuleActionDNSRoute{
			Server: action.RouteOptions.Server,
			RuleActionDNSRouteOptions: RuleActionDNSRouteOptions{
				Strategy:     C.DomainStrategy(action.RouteOptions.Strategy),
				DisableCache: action.RouteOptions.DisableCache,
				RewriteTTL:   action.RouteOptions.RewriteTTL,
				ClientSubnet: netip.Prefix(common.PtrValueOrDefault(action.RouteOptions.ClientSubnet)),
			},
		}
	case C.RuleActionTypeRouteOptions:
		return &RuleActionDNSRouteOptions{
			Strategy:     C.DomainStrategy(action.RouteOptionsOptions.Strategy),
			DisableCache: action.RouteOptionsOptions.DisableCache,
			RewriteTTL:   action.RouteOptionsOptions.RewriteTTL,
			ClientSubnet: netip.Prefix(common.PtrValueOrDefault(action.RouteOptionsOptions.ClientSubnet)),
		}
	case C.RuleActionTypeReject:
		return &RuleActionReject{
			Method: action.RejectOptions.Method,
			NoDrop: action.RejectOptions.NoDrop,
			logger: logger,
		}
	case C.RuleActionTypePredefined:
		return &RuleActionPredefined{
			Rcode:  action.PredefinedOptions.Rcode.Build(),
			Answer: common.Map(action.PredefinedOptions.Answer, option.DNSRecordOptions.Build),
			Ns:     common.Map(action.PredefinedOptions.Ns, option.DNSRecordOptions.Build),
			Extra:  common.Map(action.PredefinedOptions.Extra, option.DNSRecordOptions.Build),
		}
	default:
		panic(F.ToString("unknown rule action: ", action.Action))
	}
}

type RuleActionRoute struct {
	Outbound string
	RuleActionRouteOptions
}

func (r *RuleActionRoute) Type() string {
	return C.RuleActionTypeRoute
}

func (r *RuleActionRoute) String() string {
	var descriptions []string
	descriptions = append(descriptions, r.Outbound)
	descriptions = append(descriptions, r.Descriptions()...)
	return F.ToString("route(", strings.Join(descriptions, ","), ")")
}

type RuleActionRouteOptions struct {
	OverrideAddress           M.Socksaddr
	OverridePort              uint16
	NetworkStrategy           *C.NetworkStrategy
	NetworkType               []C.InterfaceType
	FallbackNetworkType       []C.InterfaceType
	FallbackDelay             time.Duration
	UDPDisableDomainUnmapping bool
	UDPConnect                bool
	UDPTimeout                time.Duration
	TLSFragment               bool
	TLSFragmentFallbackDelay  time.Duration
	TLSRecordFragment         bool
}

func (r *RuleActionRouteOptions) Type() string {
	return C.RuleActionTypeRouteOptions
}

func (r *RuleActionRouteOptions) String() string {
	return F.ToString("route-options(", strings.Join(r.Descriptions(), ","), ")")
}

func (r *RuleActionRouteOptions) Descriptions() []string {
	var descriptions []string
	if r.OverrideAddress.IsValid() {
		descriptions = append(descriptions, F.ToString("override-address=", r.OverrideAddress.AddrString()))
	}
	if r.OverridePort > 0 {
		descriptions = append(descriptions, F.ToString("override-port=", r.OverridePort))
	}
	if r.NetworkStrategy != nil {
		descriptions = append(descriptions, F.ToString("network-strategy=", r.NetworkStrategy))
	}
	if r.NetworkType != nil {
		descriptions = append(descriptions, F.ToString("network-type=", strings.Join(common.Map(r.NetworkType, C.InterfaceType.String), ",")))
	}
	if r.FallbackNetworkType != nil {
		descriptions = append(descriptions, F.ToString("fallback-network-type="+strings.Join(common.Map(r.NetworkType, C.InterfaceType.String), ",")))
	}
	if r.FallbackDelay > 0 {
		descriptions = append(descriptions, F.ToString("fallback-delay=", r.FallbackDelay.String()))
	}
	if r.UDPDisableDomainUnmapping {
		descriptions = append(descriptions, "udp-disable-domain-unmapping")
	}
	if r.UDPConnect {
		descriptions = append(descriptions, "udp-connect")
	}
	if r.UDPTimeout > 0 {
		descriptions = append(descriptions, "udp-timeout")
	}
	if r.TLSFragment {
		descriptions = append(descriptions, "tls-fragment")
	}
	if r.TLSFragmentFallbackDelay > 0 {
		descriptions = append(descriptions, F.ToString("tls-fragment-fallback-delay=", r.TLSFragmentFallbackDelay.String()))
	}
	if r.TLSRecordFragment {
		descriptions = append(descriptions, "tls-record-fragment")
	}
	return descriptions
}

type RuleActionDNSRoute struct {
	Server string
	RuleActionDNSRouteOptions
}

func (r *RuleActionDNSRoute) Type() string {
	return C.RuleActionTypeRoute
}

func (r *RuleActionDNSRoute) String() string {
	var descriptions []string
	descriptions = append(descriptions, r.Server)
	if r.DisableCache {
		descriptions = append(descriptions, "disable-cache")
	}
	if r.RewriteTTL != nil {
		descriptions = append(descriptions, F.ToString("rewrite-ttl=", *r.RewriteTTL))
	}
	if r.ClientSubnet.IsValid() {
		descriptions = append(descriptions, F.ToString("client-subnet=", r.ClientSubnet))
	}
	return F.ToString("route(", strings.Join(descriptions, ","), ")")
}

type RuleActionDNSRouteOptions struct {
	Strategy     C.DomainStrategy
	DisableCache bool
	RewriteTTL   *uint32
	ClientSubnet netip.Prefix
}

func (r *RuleActionDNSRouteOptions) Type() string {
	return C.RuleActionTypeRouteOptions
}

func (r *RuleActionDNSRouteOptions) String() string {
	var descriptions []string
	if r.DisableCache {
		descriptions = append(descriptions, "disable-cache")
	}
	if r.RewriteTTL != nil {
		descriptions = append(descriptions, F.ToString("rewrite-ttl=", *r.RewriteTTL))
	}
	if r.ClientSubnet.IsValid() {
		descriptions = append(descriptions, F.ToString("client-subnet=", r.ClientSubnet))
	}
	return F.ToString("route-options(", strings.Join(descriptions, ","), ")")
}

type RuleActionDirect struct {
	Dialer      N.Dialer
	description string
}

func (r *RuleActionDirect) Type() string {
	return C.RuleActionTypeDirect
}

func (r *RuleActionDirect) String() string {
	return "direct" + r.description
}

type RejectedError struct {
	Cause error
}

func (r *RejectedError) Error() string {
	return "rejected"
}

func (r *RejectedError) Unwrap() error {
	return r.Cause
}

func IsRejected(err error) bool {
	var rejected *RejectedError
	return errors.As(err, &rejected)
}

type RuleActionReject struct {
	Method      string
	NoDrop      bool
	logger      logger.ContextLogger
	dropAccess  sync.Mutex
	dropCounter []time.Time
}

func (r *RuleActionReject) Type() string {
	return C.RuleActionTypeReject
}

func (r *RuleActionReject) String() string {
	if r.Method == C.RuleActionRejectMethodDefault {
		return "reject"
	}
	return F.ToString("reject(", r.Method, ")")
}

func (r *RuleActionReject) Error(ctx context.Context) error {
	var returnErr error
	switch r.Method {
	case C.RuleActionRejectMethodDefault:
		returnErr = &RejectedError{tun.ErrReset}
	case C.RuleActionRejectMethodDrop:
		return &RejectedError{tun.ErrDrop}
	case C.RuleActionRejectMethodReply:
		return nil
	default:
		panic(F.ToString("unknown reject method: ", r.Method))
	}
	if r.NoDrop {
		return returnErr
	}
	r.dropAccess.Lock()
	defer r.dropAccess.Unlock()
	timeNow := time.Now()
	r.dropCounter = common.Filter(r.dropCounter, func(t time.Time) bool {
		return timeNow.Sub(t) <= 30*time.Second
	})
	r.dropCounter = append(r.dropCounter, timeNow)
	if len(r.dropCounter) > 50 {
		if ctx != nil {
			r.logger.DebugContext(ctx, "dropped due to flooding")
		}
		return &RejectedError{tun.ErrDrop}
	}
	return returnErr
}

type RuleActionHijackDNS struct{}

func (r *RuleActionHijackDNS) Type() string {
	return C.RuleActionTypeHijackDNS
}

func (r *RuleActionHijackDNS) String() string {
	return "hijack-dns"
}

type RuleActionSniff struct {
	SnifferNames   []string
	StreamSniffers []sniff.StreamSniffer
	PacketSniffers []sniff.PacketSniffer
	Timeout        time.Duration
	// Deprecated
	OverrideDestination bool
}

func (r *RuleActionSniff) Type() string {
	return C.RuleActionTypeSniff
}

func (r *RuleActionSniff) build() error {
	for _, name := range r.SnifferNames {
		switch name {
		case C.ProtocolTLS:
			r.StreamSniffers = append(r.StreamSniffers, sniff.TLSClientHello)
		case C.ProtocolHTTP:
			r.StreamSniffers = append(r.StreamSniffers, sniff.HTTPHost)
		case C.ProtocolQUIC:
			r.PacketSniffers = append(r.PacketSniffers, sniff.QUICClientHello)
		case C.ProtocolDNS:
			r.StreamSniffers = append(r.StreamSniffers, sniff.StreamDomainNameQuery)
			r.PacketSniffers = append(r.PacketSniffers, sniff.DomainNameQuery)
		case C.ProtocolSTUN:
			r.PacketSniffers = append(r.PacketSniffers, sniff.STUNMessage)
		case C.ProtocolBitTorrent:
			r.StreamSniffers = append(r.StreamSniffers, sniff.BitTorrent)
			r.PacketSniffers = append(r.PacketSniffers, sniff.UTP)
			r.PacketSniffers = append(r.PacketSniffers, sniff.UDPTracker)
		case C.ProtocolDTLS:
			r.PacketSniffers = append(r.PacketSniffers, sniff.DTLSRecord)
		case C.ProtocolSSH:
			r.StreamSniffers = append(r.StreamSniffers, sniff.SSH)
		case C.ProtocolRDP:
			r.StreamSniffers = append(r.StreamSniffers, sniff.RDP)
		case C.ProtocolNTP:
			r.PacketSniffers = append(r.PacketSniffers, sniff.NTP)
		default:
			return E.New("unknown sniffer: ", name)
		}
	}
	return nil
}

func (r *RuleActionSniff) String() string {
	if len(r.SnifferNames) == 0 && r.Timeout == 0 {
		return "sniff"
	} else if len(r.SnifferNames) > 0 && r.Timeout == 0 {
		return F.ToString("sniff(", strings.Join(r.SnifferNames, ","), ")")
	} else if len(r.SnifferNames) == 0 && r.Timeout > 0 {
		return F.ToString("sniff(", r.Timeout.String(), ")")
	} else {
		return F.ToString("sniff(", strings.Join(r.SnifferNames, ","), ",", r.Timeout.String(), ")")
	}
}

type RuleActionResolve struct {
	Server       string
	Strategy     C.DomainStrategy
	DisableCache bool
	RewriteTTL   *uint32
	ClientSubnet netip.Prefix
}

func (r *RuleActionResolve) Type() string {
	return C.RuleActionTypeResolve
}

func (r *RuleActionResolve) String() string {
	var options []string
	if r.Server != "" {
		options = append(options, r.Server)
	}
	if r.Strategy != C.DomainStrategyAsIS {
		options = append(options, F.ToString(option.DomainStrategy(r.Strategy)))
	}
	if r.DisableCache {
		options = append(options, "disable_cache")
	}
	if r.RewriteTTL != nil {
		options = append(options, F.ToString("rewrite_ttl=", *r.RewriteTTL))
	}
	if r.ClientSubnet.IsValid() {
		options = append(options, F.ToString("client_subnet=", r.ClientSubnet))
	}
	if len(options) == 0 {
		return "resolve"
	} else {
		return F.ToString("resolve(", strings.Join(options, ","), ")")
	}
}

type RuleActionPredefined struct {
	Rcode  int
	Answer []dns.RR
	Ns     []dns.RR
	Extra  []dns.RR
}

func (r *RuleActionPredefined) Type() string {
	return C.RuleActionTypePredefined
}

func (r *RuleActionPredefined) String() string {
	var options []string
	options = append(options, dns.RcodeToString[r.Rcode])
	options = append(options, common.Map(r.Answer, dns.RR.String)...)
	options = append(options, common.Map(r.Ns, dns.RR.String)...)
	options = append(options, common.Map(r.Extra, dns.RR.String)...)
	return F.ToString("predefined(", strings.Join(options, ","), ")")
}

func (r *RuleActionPredefined) Response(request *dns.Msg) *dns.Msg {
	return &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 request.Id,
			Response:           true,
			Authoritative:      true,
			RecursionDesired:   true,
			RecursionAvailable: true,
			Rcode:              r.Rcode,
		},
		Question: request.Question,
		Answer:   rewriteRecords(r.Answer, request.Question[0]),
		Ns:       rewriteRecords(r.Ns, request.Question[0]),
		Extra:    rewriteRecords(r.Extra, request.Question[0]),
	}
}

func rewriteRecords(records []dns.RR, question dns.Question) []dns.RR {
	return common.Map(records, func(it dns.RR) dns.RR {
		if strings.HasPrefix(it.Header().Name, "*") {
			if strings.HasSuffix(question.Name, it.Header().Name[1:]) {
				it = dns.Copy(it)
				it.Header().Name = question.Name
			}
		}
		return it
	})
}
