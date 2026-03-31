package dns

import (
	"context"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestReproLookupWithRulesUsesRequestStrategy(t *testing.T) {
	t.Parallel()

	defaultTransport := &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP}
	var qTypes []uint16
	router := newTestRouter(t, nil, &fakeDNSTransportManager{
		defaultTransport: defaultTransport,
		transports: map[string]adapter.DNSTransport{
			"default": defaultTransport,
		},
	}, &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			qTypes = append(qTypes, message.Question[0].Qtype)
			if message.Question[0].Qtype == mDNS.TypeA {
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 60), nil
			}
			return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("2001:db8::1")}, 60), nil
		},
	})

	addresses, err := router.Lookup(context.Background(), "example.com", adapter.DNSQueryOptions{
		Strategy: C.DomainStrategyIPv4Only,
	})
	require.NoError(t, err)
	require.Equal(t, []uint16{mDNS.TypeA}, qTypes)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("2.2.2.2")}, addresses)
}

func TestReproLogicalMatchResponseIPCIDR(t *testing.T) {
	t.Parallel()

	transportManager := &fakeDNSTransportManager{
		defaultTransport: &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		transports: map[string]adapter.DNSTransport{
			"upstream": &fakeDNSTransport{tag: "upstream", transportType: C.DNSTypeUDP},
			"selected": &fakeDNSTransport{tag: "selected", transportType: C.DNSTypeUDP},
			"default":  &fakeDNSTransport{tag: "default", transportType: C.DNSTypeUDP},
		},
	}
	client := &fakeDNSClient{
		exchange: func(transport adapter.DNSTransport, message *mDNS.Msg) (*mDNS.Msg, error) {
			switch transport.Tag() {
			case "upstream":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("1.1.1.1")}, 60), nil
			case "selected":
				return FixedResponse(0, message.Question[0], []netip.Addr{netip.MustParseAddr("8.8.8.8")}, 60), nil
			default:
				return nil, E.New("unexpected transport")
			}
		},
	}
	rules := []option.DNSRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultDNSRule{
				RawDefaultDNSRule: option.RawDefaultDNSRule{
					Domain: badoption.Listable[string]{"example.com"},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeEvaluate,
					RouteOptions: option.DNSRouteActionOptions{Server: "upstream"},
				},
			},
		},
		{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalDNSRule{
				RawLogicalDNSRule: option.RawLogicalDNSRule{
					Mode: C.LogicalTypeOr,
					Rules: []option.DNSRule{{
						Type: C.RuleTypeDefault,
						DefaultOptions: option.DefaultDNSRule{
							RawDefaultDNSRule: option.RawDefaultDNSRule{
								MatchResponse: true,
								IPCIDR:        badoption.Listable[string]{"1.1.1.0/24"},
							},
						},
					}},
				},
				DNSRuleAction: option.DNSRuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.DNSRouteActionOptions{Server: "selected"},
				},
			},
		},
	}
	router := newTestRouter(t, rules, transportManager, client)

	response, err := router.Exchange(context.Background(), &mDNS.Msg{
		Question: []mDNS.Question{fixedQuestion("example.com", mDNS.TypeA)},
	}, adapter.DNSQueryOptions{})
	require.NoError(t, err)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("8.8.8.8")}, MessageToAddresses(response))
}
