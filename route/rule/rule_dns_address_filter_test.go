package rule

import (
	"context"
	"net"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type addressFilterRouter struct {
	adapter.Router
	ruleSets map[string]adapter.RuleSet
}

func (r *addressFilterRouter) RuleSet(tag string) (adapter.RuleSet, bool) {
	ruleSet, loaded := r.ruleSets[tag]
	return ruleSet, loaded
}

func addressFilterContext(t *testing.T, ruleSetConfigs map[string]string) context.Context {
	t.Helper()
	router := &addressFilterRouter{ruleSets: make(map[string]adapter.RuleSet)}
	ctx := service.ContextWith[adapter.Router](context.Background(), router)
	for tag, config := range ruleSetConfigs {
		var plainOptions option.PlainRuleSetCompat
		err := json.UnmarshalContext(ctx, []byte(config), &plainOptions)
		require.NoError(t, err)
		ruleSet, err := NewLocalRuleSet(ctx, log.NewNOPFactory().Logger(), tag, option.RuleSet{
			Type:          C.RuleSetTypeInline,
			InlineOptions: plainOptions.Options,
		})
		require.NoError(t, err)
		router.ruleSets[tag] = ruleSet
	}
	return ctx
}

func addressFilterDNSRule(t *testing.T, ctx context.Context, config string) adapter.DNSRule {
	t.Helper()
	var ruleOptions option.DNSRule
	err := json.UnmarshalContext(ctx, []byte(config), &ruleOptions)
	require.NoError(t, err)
	rule, err := NewDNSRule(ctx, log.NewNOPFactory().NewLogger("test"), ruleOptions, true, true)
	require.NoError(t, err)
	require.NoError(t, rule.Start())
	return rule
}

func addressFilterResponse(address string) *dns.Msg {
	response := &dns.Msg{}
	response.Rcode = dns.RcodeSuccess
	response.Answer = append(response.Answer, &dns.A{
		Hdr: dns.RR_Header{Rrtype: dns.TypeA, Class: dns.ClassINET},
		A:   net.ParseIP(address).To4(),
	})
	return response
}

// addressFilterFlow mirrors dns/router.go: LegacyPreMatch under
// IgnoreDestinationIPCIDRMatch, then addressLimitResponseCheck against the
// response.
func addressFilterFlow(rule adapter.DNSRule, domain string, responseAddress string) (preMatched bool, routed bool) {
	metadata := adapter.InboundContext{
		Domain:    domain,
		QueryType: dns.TypeA,
		Source:    M.ParseSocksaddrHostPort("192.168.1.10", 5353),
	}
	metadata.ResetRuleCache()
	preMatched = rule.LegacyPreMatch(&metadata)
	if !preMatched {
		return false, false
	}
	if !rule.WithAddressLimit() {
		return true, true
	}
	checkMetadata := metadata
	return true, rule.MatchAddressLimit(&checkMetadata, addressFilterResponse(responseAddress))
}

func TestDNSAddressFilterInvert(t *testing.T) {
	t.Parallel()
	ctx := addressFilterContext(t, map[string]string{
		"mixed":     `{"version": 3, "rules": [{"domain_suffix": ["ads.example"]}, {"ip_cidr": ["1.1.1.0/24"]}]}`,
		"cn-ip":     `{"version": 3, "rules": [{"ip_cidr": ["1.1.1.0/24"]}]}`,
		"lan-ip":    `{"version": 3, "rules": [{"ip_cidr": ["192.168.0.0/16"]}]}`,
		"other-net": `{"version": 3, "rules": [{"ip_cidr": ["10.99.0.0/16"]}]}`,
		"cn-domain": `{"version": 3, "rules": [{"domain_suffix": ["cn.example"]}]}`,
	})
	testCases := []struct {
		name            string
		rule            string
		domain          string
		responseAddress string
		expectPreMatch  bool
		expectRouted    bool
	}{
		{
			name:            "direct mixed invert, domain hit",
			rule:            `{"domain_suffix": ["lookup.example"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "direct mixed invert, both miss",
			rule:            `{"domain_suffix": ["lookup.example"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "other.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "direct mixed invert, ip hit",
			rule:            `{"domain_suffix": ["lookup.example"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "other.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "ip rule-set invert, ip miss",
			rule:            `{"rule_set": ["cn-ip"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "ip rule-set invert, ip hit",
			rule:            `{"rule_set": ["cn-ip"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "mixed rule-set invert, both miss",
			rule:            `{"rule_set": ["mixed"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "mixed rule-set invert, domain hit skips pre-lookup",
			rule:            `{"rule_set": ["mixed"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "ads.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "mixed rule-set invert, ip hit",
			rule:            `{"rule_set": ["mixed"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "logical invert, ip miss",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"rule_set": ["cn-ip"]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "logical invert, ip hit",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"rule_set": ["cn-ip"]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "logical and with inverted ip rule-set, ip miss",
			rule:            `{"type": "logical", "mode": "and", "rules": [{"domain_suffix": ["lookup.example"]}, {"rule_set": ["cn-ip"], "invert": true}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "logical and with inverted ip rule-set, ip hit",
			rule:            `{"type": "logical", "mode": "and", "rules": [{"domain_suffix": ["lookup.example"]}, {"rule_set": ["cn-ip"], "invert": true}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "logical or invert, domain hit skips pre-lookup",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"domain_suffix": ["ads.example"]}, {"rule_set": ["cn-ip"]}], "action": "route", "server": "proxy"}`,
			domain:          "ads.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "logical or invert, both miss",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"domain_suffix": ["ads.example"]}, {"rule_set": ["cn-ip"]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "logical or invert, ip hit",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"domain_suffix": ["ads.example"]}, {"rule_set": ["cn-ip"]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "outer domain with ip rule-set invert, domain hit skips pre-lookup",
			rule:            `{"domain_suffix": ["cn.example"], "rule_set": ["cn-ip"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "cn.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "outer domain with ip rule-set invert, both miss",
			rule:            `{"domain_suffix": ["cn.example"], "rule_set": ["cn-ip"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "other.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "source ip with ip invert, source hit ip miss",
			rule:            `{"source_ip_cidr": ["192.168.1.0/24"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "source ip with ip invert, source hit ip hit",
			rule:            `{"source_ip_cidr": ["192.168.1.0/24"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "source ip with ip invert, source miss",
			rule:            `{"source_ip_cidr": ["10.99.0.0/16"], "ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "ip rule-set without invert, ip hit",
			rule:            `{"rule_set": ["cn-ip"], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "ip rule-set without invert, ip miss",
			rule:            `{"rule_set": ["cn-ip"], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
		},
		{
			name:            "mixed rule-set without invert, ip hit",
			rule:            `{"rule_set": ["mixed"], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "mixed rule-set without invert, both miss",
			rule:            `{"rule_set": ["mixed"], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
		},
		{
			name:            "direct ip invert, ip miss",
			rule:            `{"ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "direct ip invert, ip hit",
			rule:            `{"ip_cidr": ["1.1.1.0/24"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "nested logical invert, ip miss",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"type": "logical", "mode": "or", "rules": [{"rule_set": ["cn-ip"]}]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "nested logical invert, ip hit",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"type": "logical", "mode": "or", "rules": [{"rule_set": ["cn-ip"]}]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "nested logical and invert, domain hit ip miss",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"type": "logical", "mode": "and", "rules": [{"domain_suffix": ["lookup.example"]}, {"rule_set": ["cn-ip"]}]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "nested logical and invert, domain hit ip hit",
			rule:            `{"type": "logical", "mode": "or", "invert": true, "rules": [{"type": "logical", "mode": "and", "rules": [{"domain_suffix": ["lookup.example"]}, {"rule_set": ["cn-ip"]}]}], "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
		{
			name:            "match-source rule-set invert, source in set",
			rule:            `{"rule_set": ["lan-ip"], "rule_set_ip_cidr_match_source": true, "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "match-source rule-set invert, source not in set",
			rule:            `{"rule_set": ["other-net"], "rule_set_ip_cidr_match_source": true, "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "match-source rule-set, source in set",
			rule:            `{"rule_set": ["lan-ip"], "rule_set_ip_cidr_match_source": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "match-source rule-set, source not in set",
			rule:            `{"rule_set": ["other-net"], "rule_set_ip_cidr_match_source": true, "action": "route", "server": "proxy"}`,
			domain:          "lookup.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "direct ip with domain rule-set invert, domain hit skips pre-lookup",
			rule:            `{"ip_cidr": ["1.1.1.0/24"], "rule_set": ["cn-domain"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "cn.example",
			responseAddress: "8.8.8.8",
		},
		{
			name:            "direct ip with domain rule-set invert, both miss",
			rule:            `{"ip_cidr": ["1.1.1.0/24"], "rule_set": ["cn-domain"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "other.example",
			responseAddress: "8.8.8.8",
			expectPreMatch:  true,
			expectRouted:    true,
		},
		{
			name:            "direct ip with domain rule-set invert, ip hit",
			rule:            `{"ip_cidr": ["1.1.1.0/24"], "rule_set": ["cn-domain"], "invert": true, "action": "route", "server": "proxy"}`,
			domain:          "other.example",
			responseAddress: "1.1.1.5",
			expectPreMatch:  true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			rule := addressFilterDNSRule(t, ctx, testCase.rule)
			preMatched, routed := addressFilterFlow(rule, testCase.domain, testCase.responseAddress)
			require.Equal(t, testCase.expectPreMatch, preMatched, "pre-lookup match")
			require.Equal(t, testCase.expectRouted, routed, "routed")
		})
	}
}
