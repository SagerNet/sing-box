package rule

import (
	"net/netip"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/sniff"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-dns"
	"github.com/sagernet/sing-tun"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
)

func NewRuleAction(action option.RuleAction) (adapter.RuleAction, error) {
	switch action.Action {
	case C.RuleActionTypeRoute:
		return &RuleActionRoute{
			Outbound:                  action.RouteOptions.Outbound,
			UDPDisableDomainUnmapping: action.RouteOptions.UDPDisableDomainUnmapping,
		}, nil
	case C.RuleActionTypeReturn:
		return &RuleActionReturn{}, nil
	case C.RuleActionTypeReject:
		return &RuleActionReject{
			Method: action.RejectOptions.Method,
		}, nil
	case C.RuleActionTypeHijackDNS:
		return &RuleActionHijackDNS{}, nil
	case C.RuleActionTypeSniff:
		sniffAction := &RuleActionSniff{
			snifferNames: action.SniffOptions.Sniffer,
			Timeout:      time.Duration(action.SniffOptions.Timeout),
		}
		return sniffAction, sniffAction.build()
	case C.RuleActionTypeResolve:
		return &RuleActionResolve{
			Strategy: dns.DomainStrategy(action.ResolveOptions.Strategy),
			Server:   action.ResolveOptions.Server,
		}, nil
	default:
		panic(F.ToString("unknown rule action: ", action.Action))
	}
}

func NewDNSRuleAction(action option.DNSRuleAction) adapter.RuleAction {
	switch action.Action {
	case C.RuleActionTypeRoute:
		return &RuleActionDNSRoute{
			Server:       action.RouteOptions.Server,
			DisableCache: action.RouteOptions.DisableCache,
			RewriteTTL:   action.RouteOptions.RewriteTTL,
			ClientSubnet: action.RouteOptions.ClientSubnet.Build(),
		}
	case C.RuleActionTypeReturn:
		return &RuleActionReturn{}
	case C.RuleActionTypeReject:
		return &RuleActionReject{
			Method: action.RejectOptions.Method,
		}
	default:
		panic(F.ToString("unknown rule action: ", action.Action))
	}
}

type RuleActionRoute struct {
	Outbound                  string
	UDPDisableDomainUnmapping bool
}

func (r *RuleActionRoute) Type() string {
	return C.RuleActionTypeRoute
}

func (r *RuleActionRoute) String() string {
	return F.ToString("route(", r.Outbound, ")")
}

type RuleActionDNSRoute struct {
	Server       string
	DisableCache bool
	RewriteTTL   *uint32
	ClientSubnet netip.Prefix
}

func (r *RuleActionDNSRoute) Type() string {
	return C.RuleActionTypeRoute
}

func (r *RuleActionDNSRoute) String() string {
	return F.ToString("route(", r.Server, ")")
}

type RuleActionReturn struct{}

func (r *RuleActionReturn) Type() string {
	return C.RuleActionTypeReturn
}

func (r *RuleActionReturn) String() string {
	return "return"
}

type RuleActionReject struct {
	Method string
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

func (r *RuleActionReject) Error() error {
	switch r.Method {
	case C.RuleActionRejectMethodReset:
		return os.ErrClosed
	case C.RuleActionRejectMethodNetworkUnreachable:
		return syscall.ENETUNREACH
	case C.RuleActionRejectMethodHostUnreachable:
		return syscall.EHOSTUNREACH
	case C.RuleActionRejectMethodDefault, C.RuleActionRejectMethodPortUnreachable:
		return syscall.ECONNREFUSED
	case C.RuleActionRejectMethodDrop:
		return tun.ErrDrop
	default:
		panic(F.ToString("unknown reject method: ", r.Method))
	}
}

type RuleActionHijackDNS struct{}

func (r *RuleActionHijackDNS) Type() string {
	return C.RuleActionTypeHijackDNS
}

func (r *RuleActionHijackDNS) String() string {
	return "hijack-dns"
}

type RuleActionSniff struct {
	snifferNames   []string
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
	for _, name := range r.snifferNames {
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
		default:
			return E.New("unknown sniffer: ", name)
		}
	}
	return nil
}

func (r *RuleActionSniff) String() string {
	if len(r.snifferNames) == 0 && r.Timeout == 0 {
		return "sniff"
	} else if len(r.snifferNames) > 0 && r.Timeout == 0 {
		return F.ToString("sniff(", strings.Join(r.snifferNames, ","), ")")
	} else if len(r.snifferNames) == 0 && r.Timeout > 0 {
		return F.ToString("sniff(", r.Timeout.String(), ")")
	} else {
		return F.ToString("sniff(", strings.Join(r.snifferNames, ","), ",", r.Timeout.String(), ")")
	}
}

type RuleActionResolve struct {
	Strategy dns.DomainStrategy
	Server   string
}

func (r *RuleActionResolve) Type() string {
	return C.RuleActionTypeResolve
}

func (r *RuleActionResolve) String() string {
	if r.Strategy == dns.DomainStrategyAsIS && r.Server == "" {
		return F.ToString("resolve")
	} else if r.Strategy != dns.DomainStrategyAsIS && r.Server == "" {
		return F.ToString("resolve(", option.DomainStrategy(r.Strategy).String(), ")")
	} else if r.Strategy == dns.DomainStrategyAsIS && r.Server != "" {
		return F.ToString("resolve(", r.Server, ")")
	} else {
		return F.ToString("resolve(", option.DomainStrategy(r.Strategy).String(), ",", r.Server, ")")
	}
}
