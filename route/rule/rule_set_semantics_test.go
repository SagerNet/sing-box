package rule

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/convertor/adguard"
	C "github.com/sagernet/sing-box/constant"
	slogger "github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestRouteRuleSetMergeDestinationAddressGroup(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		metadata adapter.InboundContext
		inner    adapter.HeadlessRule
	}{
		{
			name:     "domain",
			metadata: testMetadata("www.example.com"),
			inner:    headlessDefaultRule(t, func(rule *abstractDefaultRule) { addDestinationAddressItem(t, rule, []string{"www.example.com"}, nil) }),
		},
		{
			name:     "domain_suffix",
			metadata: testMetadata("www.example.com"),
			inner:    headlessDefaultRule(t, func(rule *abstractDefaultRule) { addDestinationAddressItem(t, rule, nil, []string{"example.com"}) }),
		},
		{
			name:     "domain_keyword",
			metadata: testMetadata("www.example.com"),
			inner:    headlessDefaultRule(t, func(rule *abstractDefaultRule) { addDestinationKeywordItem(rule, []string{"example"}) }),
		},
		{
			name:     "domain_regex",
			metadata: testMetadata("www.example.com"),
			inner:    headlessDefaultRule(t, func(rule *abstractDefaultRule) { addDestinationRegexItem(t, rule, []string{`^www\.example\.com$`}) }),
		},
		{
			name: "ip_cidr",
			metadata: func() adapter.InboundContext {
				metadata := testMetadata("lookup.example")
				metadata.DestinationAddresses = []netip.Addr{netip.MustParseAddr("8.8.8.8")}
				return metadata
			}(),
			inner: headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationIPCIDRItem(t, rule, []string{"8.8.8.0/24"})
			}),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ruleSet := newLocalRuleSetForTest("merge-destination", testCase.inner)
			rule := routeRuleForTest(func(rule *abstractDefaultRule) {
				addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			})
			require.True(t, rule.Match(&testCase.metadata))
		})
	}
}

func TestRouteRuleSetMergeSourceAndPortGroups(t *testing.T) {
	t.Parallel()
	t.Run("source address", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-source-address", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addSourceAddressItem(t, rule, []string{"10.0.0.0/8"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addSourceAddressItem(t, rule, []string{"198.51.100.0/24"})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("source address via ruleset ipcidr match source", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-source-address-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationIPCIDRItem(t, rule, []string{"10.0.0.0/8"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{
				setList:           []adapter.RuleSet{ruleSet},
				ipCidrMatchSource: true,
			})
			addSourceAddressItem(t, rule, []string{"198.51.100.0/24"})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("destination port", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-destination-port", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationPortItem(rule, []uint16{443})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationPortItem(rule, []uint16{8443})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("destination port range", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-destination-port-range", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationPortRangeItem(t, rule, []string{"400:500"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationPortItem(rule, []uint16{8443})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("source port", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-source-port", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addSourcePortItem(rule, []uint16{1000})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addSourcePortItem(rule, []uint16{2000})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("source port range", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("merge-source-port-range", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addSourcePortRangeItem(t, rule, []string{"900:1100"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addSourcePortItem(rule, []uint16{2000})
		})
		require.True(t, rule.Match(&metadata))
	})
}

func TestRouteRuleSetOuterGroupedStateMergesIntoSameGroup(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		metadata   adapter.InboundContext
		buildOuter func(*testing.T, *abstractDefaultRule)
		buildInner func(*testing.T, *abstractDefaultRule)
	}{
		{
			name:     "destination address",
			metadata: testMetadata("www.example.com"),
			buildOuter: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			},
			buildInner: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationAddressItem(t, rule, nil, []string{"google.com"})
			},
		},
		{
			name:     "source address",
			metadata: testMetadata("www.example.com"),
			buildOuter: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addSourceAddressItem(t, rule, []string{"10.0.0.0/8"})
			},
			buildInner: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addSourceAddressItem(t, rule, []string{"198.51.100.0/24"})
			},
		},
		{
			name:     "source port",
			metadata: testMetadata("www.example.com"),
			buildOuter: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addSourcePortItem(rule, []uint16{1000})
			},
			buildInner: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addSourcePortItem(rule, []uint16{2000})
			},
		},
		{
			name:     "destination port",
			metadata: testMetadata("www.example.com"),
			buildOuter: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationPortItem(rule, []uint16{443})
			},
			buildInner: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationPortItem(rule, []uint16{8443})
			},
		},
		{
			name: "destination ip cidr",
			metadata: func() adapter.InboundContext {
				metadata := testMetadata("lookup.example")
				metadata.DestinationAddresses = []netip.Addr{netip.MustParseAddr("203.0.113.1")}
				return metadata
			}(),
			buildOuter: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			},
			buildInner: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"198.51.100.0/24"})
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ruleSet := newLocalRuleSetForTest("outer-merge-"+testCase.name, headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				testCase.buildInner(t, rule)
			}))
			rule := routeRuleForTest(func(rule *abstractDefaultRule) {
				testCase.buildOuter(t, rule)
				addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			})
			require.True(t, rule.Match(&testCase.metadata))
		})
	}
}

func TestRouteRuleSetOtherFieldsStayAnd(t *testing.T) {
	t.Parallel()
	metadata := testMetadata("www.example.com")
	ruleSet := newLocalRuleSetForTest("other-fields-and", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		addDestinationAddressItem(t, rule, nil, []string{"example.com"})
	}))
	rule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
	})
	require.False(t, rule.Match(&metadata))
}

func TestRouteRuleSetMergedBranchKeepsAndConstraints(t *testing.T) {
	t.Parallel()
	t.Run("outer group does not bypass inner non grouped condition", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("network-and", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("outer group does not satisfy different grouped branch", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("different-group", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"google.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addSourcePortItem(rule, []uint16{1000})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestRouteRuleSetOrSemantics(t *testing.T) {
	t.Parallel()
	t.Run("later ruleset can satisfy outer group", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		emptyStateSet := newLocalRuleSetForTest("network-only", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
		}))
		destinationStateSet := newLocalRuleSetForTest("domain-only", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{emptyStateSet, destinationStateSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("multi rule set does not satisfy outer group", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest(
			"rule-set-and",
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			}),
		)
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("cross ruleset union is not allowed", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		sourceStateSet := newLocalRuleSetForTest("source-only", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addSourcePortItem(rule, []uint16{1000})
		}))
		destinationStateSet := newLocalRuleSetForTest("destination-only", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{sourceStateSet, destinationStateSet}})
			addSourcePortItem(rule, []uint16{2000})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestRouteRuleSetLogicalSemantics(t *testing.T) {
	t.Parallel()
	t.Run("logical set does not satisfy outer group", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("logical-or", headlessLogicalRule(
			C.LogicalTypeOr,
			false,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			}),
		))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("logical branch does not lift outer group requirements", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("logical-and", headlessLogicalRule(
			C.LogicalTypeAnd,
			false,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addSourcePortItem(rule, []uint16{1000})
			}),
		))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			addSourcePortItem(rule, []uint16{2000})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("logical branch matches on its own conditions", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("logical-and-self", headlessLogicalRule(
			C.LogicalTypeAnd,
			false,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addSourcePortItem(rule, []uint16{1000})
			}),
		))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addSourcePortItem(rule, []uint16{1000})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("inverted set does not satisfy outer group", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("invert", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			rule.invert = true
			addDestinationAddressItem(t, rule, nil, []string{"cn"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestRuleSetShapeBoundary(t *testing.T) {
	t.Parallel()
	buildOuter := func(ruleSet *LocalRuleSet) *DefaultRule {
		return routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, []string{"extra.example.org"}, nil)
			addDestinationPortItem(rule, []uint16{443})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
	}
	singleShape := buildOuter(newLocalRuleSetForTest("flat-single", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		addDestinationAddressItem(t, rule, nil, []string{"a.example.com", "b.example.com"})
	})))
	multiShape := buildOuter(newLocalRuleSetForTest(
		"flat-multi",
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"a.example.com"})
		}),
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"b.example.com"})
		}),
	))
	queries := []struct {
		name         string
		domain       string
		port         uint16
		singleResult bool
		multiResult  bool
	}{
		{"in set", "www.b.example.com", 443, true, false},
		{"outer own domain", "extra.example.org", 443, true, false},
		{"neither", "other.example.net", 443, false, false},
		{"in set with wrong port", "www.b.example.com", 80, false, false},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			t.Parallel()
			singleMetadata := testMetadata(query.domain)
			singleMetadata.Destination.Port = query.port
			multiMetadata := testMetadata(query.domain)
			multiMetadata.Destination.Port = query.port
			require.Equal(t, query.singleResult, singleShape.Match(&singleMetadata))
			require.Equal(t, query.multiResult, multiShape.Match(&multiMetadata))
		})
	}
}

func TestRuleSetCaseBoundary(t *testing.T) {
	t.Parallel()
	t.Run("single cross group rule merges into outer", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("port-single", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationPortRangeItem(t, rule, []string{"400:500"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationPortItem(rule, []uint16{8080})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("multi rule set keeps outer group absolute", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest(
			"port-multi",
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationPortRangeItem(t, rule, []string{"400:500"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"never.example"})
			}),
		)
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationPortItem(rule, []uint16{8080})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestRuleSetLogicalBranchSelfContained(t *testing.T) {
	t.Parallel()
	newRuleSet := func() *LocalRuleSet {
		return newLocalRuleSetForTest("and-branch", headlessLogicalRule(
			C.LogicalTypeAnd,
			false,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"b.example.com"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationPortRangeItem(t, rule, []string{"800:900"})
			}),
		))
	}
	t.Run("branch matches only on its own conditions", func(t *testing.T) {
		t.Parallel()
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{newRuleSet()}})
		})
		matchedMetadata := testMetadata("www.b.example.com")
		matchedMetadata.Destination.Port = 850
		require.True(t, rule.Match(&matchedMetadata))
		unmatchedMetadata := testMetadata("www.b.example.com")
		require.False(t, rule.Match(&unmatchedMetadata))
	})
	t.Run("outer condition and set are both required", func(t *testing.T) {
		t.Parallel()
		matchedRule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addSourcePortItem(rule, []uint16{1000})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{newRuleSet()}})
		})
		metadata := testMetadata("www.b.example.com")
		metadata.Destination.Port = 850
		require.True(t, matchedRule.Match(&metadata))
		setMissMetadata := testMetadata("other.example.net")
		setMissMetadata.Destination.Port = 850
		require.False(t, matchedRule.Match(&setMissMetadata))
		outerMissRule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addSourcePortItem(rule, []uint16{2000})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{newRuleSet()}})
		})
		outerMissMetadata := testMetadata("www.b.example.com")
		outerMissMetadata.Destination.Port = 850
		require.False(t, outerMissRule.Match(&outerMissMetadata))
	})
}

func TestRuleSetMixedReference(t *testing.T) {
	t.Parallel()
	t.Run("single rule set merges as or", func(t *testing.T) {
		t.Parallel()
		ruleSet := newLocalRuleSetForTest("mixed-single", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"set.example.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, []string{"extra.example.org"}, nil)
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		extraMetadata := testMetadata("extra.example.org")
		require.True(t, rule.Match(&extraMetadata))
		setMetadata := testMetadata("www.set.example.com")
		require.True(t, rule.Match(&setMetadata))
		otherMetadata := testMetadata("other.example.net")
		require.False(t, rule.Match(&otherMetadata))
	})
	t.Run("multi rule set is an independent condition", func(t *testing.T) {
		t.Parallel()
		ruleSet := newLocalRuleSetForTest(
			"mixed-multi",
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"set.example.com"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"set2.example.com"})
			}),
		)
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationPortItem(rule, []uint16{443})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		matchedMetadata := testMetadata("www.set2.example.com")
		require.True(t, rule.Match(&matchedMetadata))
		portMissMetadata := testMetadata("www.set2.example.com")
		portMissMetadata.Destination.Port = 80
		require.False(t, rule.Match(&portMissMetadata))
		setMissMetadata := testMetadata("other.example.net")
		require.False(t, rule.Match(&setMissMetadata))
	})
}

func TestRuleSetStandaloneReference(t *testing.T) {
	t.Parallel()
	ruleSet := newLocalRuleSetForTest(
		"standalone",
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"a.example.net"})
		}),
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationPortRangeItem(t, rule, []string{"400:500"})
		}),
	)
	rule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})
	domainMetadata := testMetadata("www.a.example.net")
	domainMetadata.Destination.Port = 80
	require.True(t, rule.Match(&domainMetadata))
	portMetadata := testMetadata("other.example.org")
	require.True(t, rule.Match(&portMetadata))
	missMetadata := testMetadata("other.example.org")
	missMetadata.Destination.Port = 80
	require.False(t, rule.Match(&missMetadata))
}

func TestRuleSetInvertedSingleRuleIsBoolean(t *testing.T) {
	t.Parallel()
	ruleSet := newLocalRuleSetForTest("inverted-single", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		rule.invert = true
		addDestinationAddressItem(t, rule, nil, []string{"blocked.example"})
	}))
	rule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})
	allowedMetadata := testMetadata("good.example.org")
	require.True(t, rule.Match(&allowedMetadata))
	blockedMetadata := testMetadata("www.blocked.example")
	require.False(t, rule.Match(&blockedMetadata))
}

func TestRuleSetEmptySetNeverMatches(t *testing.T) {
	t.Parallel()
	emptySet := newLocalRuleSetForTest("empty")
	t.Run("outer own group does not bypass the set", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{emptySet}})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("standalone reference does not match", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{emptySet}})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestRouteRuleSetInvertBranchSemantics(t *testing.T) {
	t.Parallel()
	t.Run("inverted default branch acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("invert-grouped", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			rule.invert = true
			addDestinationAddressItem(t, rule, nil, []string{"google.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("inverted default branch with non grouped condition acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("invert-network", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			rule.invert = true
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("inverted logical branch acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("logical-invert-grouped", headlessLogicalRule(
			C.LogicalTypeOr,
			true,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"google.com"})
			}),
		))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("inverted logical branch with non grouped condition acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("logical-invert-network", headlessLogicalRule(
			C.LogicalTypeOr,
			true,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
			}),
		))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
}

func TestRouteRuleSetNoLeakageRegressions(t *testing.T) {
	t.Parallel()
	t.Run("same ruleset failed branch does not leak", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest(
			"same-set",
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
				addSourcePortItem(rule, []uint16{1})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
				addSourcePortItem(rule, []uint16{1000})
			}),
		)
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("adguard exclusion remains isolated across rulesets", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("im.qq.com")
		excludeSet := newLocalRuleSetForTest("adguard", mustAdGuardRule(t, "@@||im.qq.com^\n||whatever1.com^\n"))
		otherSet := newLocalRuleSetForTest("other", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"whatever2.com"})
		}))
		rule := routeRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{excludeSet, otherSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
}

func TestDefaultRuleDoesNotReuseGroupedMatchCacheAcrossEvaluations(t *testing.T) {
	t.Parallel()
	metadata := testMetadata("www.example.com")
	rule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addDestinationAddressItem(t, rule, nil, []string{"example.com"})
	})
	require.True(t, rule.Match(&metadata))

	metadata.Destination.Fqdn = "www.example.org"
	require.False(t, rule.Match(&metadata))
}

func TestRouteRuleSetRemoteUsesSameSemantics(t *testing.T) {
	t.Parallel()
	ruleSet := newRemoteRuleSetForTest(
		"remote",
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
		}),
		headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
		}),
	)
	standaloneRule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})
	standaloneMetadata := testMetadata("www.example.com")
	require.True(t, standaloneRule.Match(&standaloneMetadata))
	combinedRule := routeRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
	})
	combinedMetadata := testMetadata("www.example.com")
	require.False(t, combinedRule.Match(&combinedMetadata))
}

func TestDNSRuleSetSemantics(t *testing.T) {
	t.Parallel()
	t.Run("outer destination group merges into matching ruleset branch", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.baidu.com")
		ruleSet := newLocalRuleSetForTest("dns-merged-branch", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"google.com"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"baidu.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("outer destination group does not bypass ruleset non grouped condition", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("dns-network-and", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("inverted branch acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.baidu.com")
		ruleSet := newLocalRuleSetForTest("dns-invert-grouped", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			rule.invert = true
			addDestinationAddressItem(t, rule, nil, []string{"google.com"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"baidu.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("inverted logical branch acts as boolean term", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("dns-logical-invert-network", headlessLogicalRule(
			C.LogicalTypeOr,
			true,
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addOtherItem(rule, NewNetworkItem([]string{N.NetworkUDP}))
			}),
		))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("match address limit merges destination group", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("dns-merge", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.True(t, rule.MatchAddressLimit(&metadata, dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))))
	})
	t.Run("dns keeps ruleset or semantics", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		emptyStateSet := newLocalRuleSetForTest("dns-empty", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
		}))
		destinationStateSet := newLocalRuleSetForTest("dns-destination", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationAddressItem(t, rule, nil, []string{"example.com"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{emptyStateSet, destinationStateSet}})
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.True(t, rule.MatchAddressLimit(&metadata, dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))))
	})
	t.Run("ruleset ip cidr flags stay scoped", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest("dns-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{
				setList:           []adapter.RuleSet{ruleSet},
				ipCidrAcceptEmpty: true,
			})
		})
		require.True(t, rule.MatchAddressLimit(&metadata, dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))))
		require.False(t, rule.MatchAddressLimit(&metadata, dnsResponseForTest(netip.MustParseAddr("8.8.8.8"))))
		require.True(t, rule.MatchAddressLimit(&metadata, dnsResponseForTest()))
		require.False(t, metadata.IPCIDRMatchSource)
		require.False(t, metadata.IPCIDRAcceptEmpty)
	})
	t.Run("pre lookup ruleset only deferred fields fail closed", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("lookup.example")
		ruleSet := newLocalRuleSetForTest("dns-prelookup-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		// This is accepted without match_response so mixed rule_set deployments keep
		// working; the destination-IP-only branch simply cannot match before a DNS
		// response is available.
		require.False(t, rule.Match(&metadata))
	})
	t.Run("pre lookup ruleset destination cidr does not fall back to other predicates", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("lookup.example")
		ruleSet := newLocalRuleSetForTest("dns-prelookup-network-and-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.False(t, rule.Match(&metadata))
	})
	t.Run("pre lookup mixed ruleset still matches non response branch", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("www.example.com")
		ruleSet := newLocalRuleSetForTest(
			"dns-prelookup-mixed",
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			}),
			headlessDefaultRule(t, func(rule *abstractDefaultRule) {
				addDestinationAddressItem(t, rule, nil, []string{"example.com"})
			}),
		)
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		// Destination-IP predicates inside rule_set fail closed before the DNS response,
		// but they must not force validation errors or suppress sibling non-response
		// branches.
		require.True(t, rule.Match(&metadata))
	})
}

func TestDNSMatchResponseRuleSetDestinationCIDRUsesDNSResponse(t *testing.T) {
	t.Parallel()

	ruleSet := newLocalRuleSetForTest("dns-response-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
	}))
	rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})
	rule.matchResponse = true

	matchedMetadata := testMetadata("lookup.example")
	matchedMetadata.DNSResponse = dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))
	require.True(t, rule.Match(&matchedMetadata))
	require.Empty(t, matchedMetadata.DestinationAddresses)

	unmatchedMetadata := testMetadata("lookup.example")
	unmatchedMetadata.DNSResponse = dnsResponseForTest(netip.MustParseAddr("8.8.8.8"))
	require.False(t, rule.Match(&unmatchedMetadata))
}

func TestDNSMatchResponseMissingResponseUsesBooleanSemantics(t *testing.T) {
	t.Parallel()

	t.Run("plain rule remains false", func(t *testing.T) {
		t.Parallel()

		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {})
		rule.matchResponse = true

		metadata := testMetadata("lookup.example")
		require.False(t, rule.Match(&metadata))
	})

	t.Run("invert rule becomes true", func(t *testing.T) {
		t.Parallel()

		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			rule.invert = true
		})
		rule.matchResponse = true

		metadata := testMetadata("lookup.example")
		require.True(t, rule.Match(&metadata))
	})

	t.Run("logical wrapper respects inverted child", func(t *testing.T) {
		t.Parallel()

		nestedRule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			rule.invert = true
		})
		nestedRule.matchResponse = true

		logicalRule := &LogicalDNSRule{
			abstractLogicalRule: abstractLogicalRule{
				rules: []adapter.HeadlessRule{nestedRule},
				mode:  C.LogicalTypeAnd,
			},
		}

		metadata := testMetadata("lookup.example")
		require.True(t, logicalRule.Match(&metadata))
	})
}

func TestDNSAddressLimitIgnoresDestinationAddresses(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		build             func(*testing.T, *abstractDefaultRule)
		matchedResponse   *mDNS.Msg
		unmatchedResponse *mDNS.Msg
	}{
		{
			name: "ip_cidr",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("203.0.113.1")),
			unmatchedResponse: dnsResponseForTest(netip.MustParseAddr("8.8.8.8")),
		},
		{
			name: "ip_is_private",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPIsPrivateItem(rule)
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("10.0.0.1")),
			unmatchedResponse: dnsResponseForTest(netip.MustParseAddr("8.8.8.8")),
		},
		{
			name: "ip_accept_any",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPAcceptAnyItem(rule)
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("203.0.113.1")),
			unmatchedResponse: dnsResponseForTest(),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
				testCase.build(t, rule)
			})

			mismatchMetadata := testMetadata("lookup.example")
			mismatchMetadata.DestinationAddresses = []netip.Addr{netip.MustParseAddr("203.0.113.1")}
			require.False(t, rule.MatchAddressLimit(&mismatchMetadata, testCase.unmatchedResponse))

			matchMetadata := testMetadata("lookup.example")
			matchMetadata.DestinationAddresses = []netip.Addr{netip.MustParseAddr("8.8.8.8")}
			require.True(t, rule.MatchAddressLimit(&matchMetadata, testCase.matchedResponse))
		})
	}
}

func TestDNSLegacyAddressLimitPreLookupDefersDirectRules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		build             func(*testing.T, *abstractDefaultRule)
		matchedResponse   *mDNS.Msg
		unmatchedResponse *mDNS.Msg
	}{
		{
			name: "ip_cidr",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("203.0.113.1")),
			unmatchedResponse: dnsResponseForTest(netip.MustParseAddr("8.8.8.8")),
		},
		{
			name: "ip_is_private",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPIsPrivateItem(rule)
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("10.0.0.1")),
			unmatchedResponse: dnsResponseForTest(netip.MustParseAddr("8.8.8.8")),
		},
		{
			name: "ip_accept_any",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPAcceptAnyItem(rule)
			},
			matchedResponse:   dnsResponseForTest(netip.MustParseAddr("203.0.113.1")),
			unmatchedResponse: dnsResponseForTest(),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
				testCase.build(t, rule)
			})

			preLookupMetadata := testMetadata("lookup.example")
			require.True(t, rule.LegacyPreMatch(&preLookupMetadata))

			matchedMetadata := testMetadata("lookup.example")
			require.True(t, rule.MatchAddressLimit(&matchedMetadata, testCase.matchedResponse))

			unmatchedMetadata := testMetadata("lookup.example")
			require.False(t, rule.MatchAddressLimit(&unmatchedMetadata, testCase.unmatchedResponse))
		})
	}
}

func TestDNSLegacyAddressLimitPreLookupDefersRuleSetDestinationCIDR(t *testing.T) {
	t.Parallel()

	ruleSet := newLocalRuleSetForTest("dns-legacy-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
	}))
	rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})

	preLookupMetadata := testMetadata("lookup.example")
	require.True(t, rule.LegacyPreMatch(&preLookupMetadata))

	matchedMetadata := testMetadata("lookup.example")
	require.True(t, rule.MatchAddressLimit(&matchedMetadata, dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))))

	unmatchedMetadata := testMetadata("lookup.example")
	require.False(t, rule.MatchAddressLimit(&unmatchedMetadata, dnsResponseForTest(netip.MustParseAddr("8.8.8.8"))))
}

func TestDNSLegacyLogicalAddressLimitPreLookupDefersNestedRules(t *testing.T) {
	t.Parallel()

	nestedRule := dnsRuleForTest(func(rule *abstractDefaultRule) {
		addDestinationIPIsPrivateItem(rule)
	})
	logicalRule := &LogicalDNSRule{
		abstractLogicalRule: abstractLogicalRule{
			rules: []adapter.HeadlessRule{nestedRule},
			mode:  C.LogicalTypeAnd,
		},
	}

	preLookupMetadata := testMetadata("lookup.example")
	require.True(t, logicalRule.LegacyPreMatch(&preLookupMetadata))

	matchedMetadata := testMetadata("lookup.example")
	require.True(t, logicalRule.MatchAddressLimit(&matchedMetadata, dnsResponseForTest(netip.MustParseAddr("10.0.0.1"))))

	unmatchedMetadata := testMetadata("lookup.example")
	require.False(t, logicalRule.MatchAddressLimit(&unmatchedMetadata, dnsResponseForTest(netip.MustParseAddr("8.8.8.8"))))
}

func TestDNSLegacyInvertAddressLimitPreLookupRegression(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		build          func(*testing.T, *abstractDefaultRule)
		matchedAddrs   []netip.Addr
		unmatchedAddrs []netip.Addr
	}{
		{
			name: "ip_cidr",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			},
			matchedAddrs:   []netip.Addr{netip.MustParseAddr("203.0.113.1")},
			unmatchedAddrs: []netip.Addr{netip.MustParseAddr("8.8.8.8")},
		},
		{
			name: "ip_is_private",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPIsPrivateItem(rule)
			},
			matchedAddrs:   []netip.Addr{netip.MustParseAddr("10.0.0.1")},
			unmatchedAddrs: []netip.Addr{netip.MustParseAddr("8.8.8.8")},
		},
		{
			name: "ip_accept_any",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPAcceptAnyItem(rule)
			},
			matchedAddrs: []netip.Addr{netip.MustParseAddr("203.0.113.1")},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
				rule.invert = true
				testCase.build(t, rule)
			})

			preLookupMetadata := testMetadata("lookup.example")
			require.True(t, rule.LegacyPreMatch(&preLookupMetadata))

			matchedMetadata := testMetadata("lookup.example")
			require.False(t, rule.MatchAddressLimit(&matchedMetadata, dnsResponseForTest(testCase.matchedAddrs...)))

			unmatchedMetadata := testMetadata("lookup.example")
			require.True(t, rule.MatchAddressLimit(&unmatchedMetadata, dnsResponseForTest(testCase.unmatchedAddrs...)))
		})
	}
}

func TestDNSLegacyInvertLogicalAddressLimitPreLookupRegression(t *testing.T) {
	t.Parallel()

	t.Run("inverted deferred child does not suppress branch", func(t *testing.T) {
		t.Parallel()

		logicalRule := &LogicalDNSRule{
			abstractLogicalRule: abstractLogicalRule{
				rules: []adapter.HeadlessRule{
					dnsRuleForTest(func(rule *abstractDefaultRule) {
						rule.invert = true
						addDestinationIPIsPrivateItem(rule)
					}),
				},
				mode: C.LogicalTypeAnd,
			},
		}

		preLookupMetadata := testMetadata("lookup.example")
		require.True(t, logicalRule.LegacyPreMatch(&preLookupMetadata))
	})
}

func TestDNSLegacyInvertRuleSetAddressLimitPreLookupRegression(t *testing.T) {
	t.Parallel()

	ruleSet := newLocalRuleSetForTest("dns-legacy-invert-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
		rule.invert = true
		addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
	}))
	rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
		addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
	})

	preLookupMetadata := testMetadata("lookup.example")
	require.True(t, rule.LegacyPreMatch(&preLookupMetadata))

	matchedMetadata := testMetadata("lookup.example")
	require.False(t, rule.MatchAddressLimit(&matchedMetadata, dnsResponseForTest(netip.MustParseAddr("203.0.113.1"))))

	unmatchedMetadata := testMetadata("lookup.example")
	require.True(t, rule.MatchAddressLimit(&unmatchedMetadata, dnsResponseForTest(netip.MustParseAddr("8.8.8.8"))))
}

func TestDNSInvertAddressLimitPreLookupRegression(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		build          func(*testing.T, *abstractDefaultRule)
		matchedAddrs   []netip.Addr
		unmatchedAddrs []netip.Addr
	}{
		{
			name: "ip_cidr",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
			},
			matchedAddrs:   []netip.Addr{netip.MustParseAddr("203.0.113.1")},
			unmatchedAddrs: []netip.Addr{netip.MustParseAddr("8.8.8.8")},
		},
		{
			name: "ip_is_private",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPIsPrivateItem(rule)
			},
			matchedAddrs:   []netip.Addr{netip.MustParseAddr("10.0.0.1")},
			unmatchedAddrs: []netip.Addr{netip.MustParseAddr("8.8.8.8")},
		},
		{
			name: "ip_accept_any",
			build: func(t *testing.T, rule *abstractDefaultRule) {
				t.Helper()
				addDestinationIPAcceptAnyItem(rule)
			},
			matchedAddrs: []netip.Addr{netip.MustParseAddr("203.0.113.1")},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
				rule.invert = true
				testCase.build(t, rule)
			})

			preLookupMetadata := testMetadata("lookup.example")
			require.True(t, rule.Match(&preLookupMetadata))

			matchedMetadata := testMetadata("lookup.example")
			matchedMetadata.DestinationAddresses = testCase.matchedAddrs
			require.False(t, rule.MatchAddressLimit(&matchedMetadata, dnsResponseForTest(testCase.matchedAddrs...)))

			unmatchedMetadata := testMetadata("lookup.example")
			unmatchedMetadata.DestinationAddresses = testCase.unmatchedAddrs
			require.True(t, rule.MatchAddressLimit(&unmatchedMetadata, dnsResponseForTest(testCase.unmatchedAddrs...)))
		})
	}
	t.Run("mixed resolved and deferred fields invert matches pre lookup", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("lookup.example")
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			rule.invert = true
			addOtherItem(rule, NewNetworkItem([]string{N.NetworkTCP}))
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		})
		require.True(t, rule.Match(&metadata))
	})
	t.Run("ruleset only deferred fields invert matches pre lookup", func(t *testing.T) {
		t.Parallel()
		metadata := testMetadata("lookup.example")
		ruleSet := newLocalRuleSetForTest("dns-ruleset-ipcidr", headlessDefaultRule(t, func(rule *abstractDefaultRule) {
			addDestinationIPCIDRItem(t, rule, []string{"203.0.113.0/24"})
		}))
		rule := dnsRuleForTest(func(rule *abstractDefaultRule) {
			rule.invert = true
			addRuleSetItem(rule, &RuleSetItem{setList: []adapter.RuleSet{ruleSet}})
		})
		require.True(t, rule.Match(&metadata))
	})
}

func routeRuleForTest(build func(*abstractDefaultRule)) *DefaultRule {
	rule := &DefaultRule{}
	build(&rule.abstractDefaultRule)
	return rule
}

func dnsRuleForTest(build func(*abstractDefaultRule)) *DefaultDNSRule {
	rule := &DefaultDNSRule{}
	build(&rule.abstractDefaultRule)
	return rule
}

func headlessDefaultRule(t *testing.T, build func(*abstractDefaultRule)) *DefaultHeadlessRule {
	t.Helper()
	rule := &DefaultHeadlessRule{}
	build(&rule.abstractDefaultRule)
	return rule
}

func headlessLogicalRule(mode string, invert bool, rules ...adapter.HeadlessRule) *LogicalHeadlessRule {
	return &LogicalHeadlessRule{
		abstractLogicalRule: abstractLogicalRule{
			rules:  rules,
			mode:   mode,
			invert: invert,
		},
	}
}

func newLocalRuleSetForTest(tag string, rules ...adapter.HeadlessRule) *LocalRuleSet {
	return &LocalRuleSet{
		tag:   tag,
		rules: rules,
	}
}

func newRemoteRuleSetForTest(tag string, rules ...adapter.HeadlessRule) *RemoteRuleSet {
	return &RemoteRuleSet{
		tag:   tag,
		rules: rules,
	}
}

func mustAdGuardRule(t *testing.T, content string) adapter.HeadlessRule {
	t.Helper()
	rules, err := adguard.ToOptions(strings.NewReader(content), slogger.NOP())
	require.NoError(t, err)
	require.Len(t, rules, 1)
	rule, err := NewHeadlessRule(context.Background(), rules[0])
	require.NoError(t, err)
	return rule
}

func testMetadata(domain string) adapter.InboundContext {
	return adapter.InboundContext{
		Network: N.NetworkTCP,
		Source: M.Socksaddr{
			Addr: netip.MustParseAddr("10.0.0.1"),
			Port: 1000,
		},
		Destination: M.Socksaddr{
			Fqdn: domain,
			Port: 443,
		},
	}
}

func dnsResponseForTest(addresses ...netip.Addr) *mDNS.Msg {
	response := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			Response: true,
			Rcode:    mDNS.RcodeSuccess,
		},
	}
	for _, address := range addresses {
		if address.Is4() {
			response.Answer = append(response.Answer, &mDNS.A{
				Hdr: mDNS.RR_Header{
					Name:   mDNS.Fqdn("lookup.example"),
					Rrtype: mDNS.TypeA,
					Class:  mDNS.ClassINET,
					Ttl:    60,
				},
				A: net.IP(append([]byte(nil), address.AsSlice()...)),
			})
		} else {
			response.Answer = append(response.Answer, &mDNS.AAAA{
				Hdr: mDNS.RR_Header{
					Name:   mDNS.Fqdn("lookup.example"),
					Rrtype: mDNS.TypeAAAA,
					Class:  mDNS.ClassINET,
					Ttl:    60,
				},
				AAAA: net.IP(append([]byte(nil), address.AsSlice()...)),
			})
		}
	}
	return response
}

func addRuleSetItem(rule *abstractDefaultRule, item *RuleSetItem) {
	rule.ruleSetItem = item
	rule.allItems = append(rule.allItems, item)
}

func addOtherItem(rule *abstractDefaultRule, item RuleItem) {
	rule.items = append(rule.items, item)
	rule.allItems = append(rule.allItems, item)
}

func addSourceAddressItem(t *testing.T, rule *abstractDefaultRule, cidrs []string) {
	t.Helper()
	item, err := NewIPCIDRItem(true, cidrs)
	require.NoError(t, err)
	rule.sourceAddressItems = append(rule.sourceAddressItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationAddressItem(t *testing.T, rule *abstractDefaultRule, domains []string, suffixes []string) {
	t.Helper()
	item, err := NewDomainItem(domains, suffixes)
	require.NoError(t, err)
	rule.destinationAddressItems = append(rule.destinationAddressItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationKeywordItem(rule *abstractDefaultRule, keywords []string) {
	item := NewDomainKeywordItem(keywords)
	rule.destinationAddressItems = append(rule.destinationAddressItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationRegexItem(t *testing.T, rule *abstractDefaultRule, regexes []string) {
	t.Helper()
	item, err := NewDomainRegexItem(regexes)
	require.NoError(t, err)
	rule.destinationAddressItems = append(rule.destinationAddressItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationIPCIDRItem(t *testing.T, rule *abstractDefaultRule, cidrs []string) {
	t.Helper()
	item, err := NewIPCIDRItem(false, cidrs)
	require.NoError(t, err)
	rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationIPIsPrivateItem(rule *abstractDefaultRule) {
	item := NewIPIsPrivateItem(false)
	rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationIPAcceptAnyItem(rule *abstractDefaultRule) {
	item := NewIPAcceptAnyItem()
	rule.destinationIPCIDRItems = append(rule.destinationIPCIDRItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addSourcePortItem(rule *abstractDefaultRule, ports []uint16) {
	item := NewPortItem(true, ports)
	rule.sourcePortItems = append(rule.sourcePortItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addSourcePortRangeItem(t *testing.T, rule *abstractDefaultRule, ranges []string) {
	t.Helper()
	item, err := NewPortRangeItem(true, ranges)
	require.NoError(t, err)
	rule.sourcePortItems = append(rule.sourcePortItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationPortItem(rule *abstractDefaultRule, ports []uint16) {
	item := NewPortItem(false, ports)
	rule.destinationPortItems = append(rule.destinationPortItems, item)
	rule.allItems = append(rule.allItems, item)
}

func addDestinationPortRangeItem(t *testing.T, rule *abstractDefaultRule, ranges []string) {
	t.Helper()
	item, err := NewPortRangeItem(false, ranges)
	require.NoError(t, err)
	rule.destinationPortItems = append(rule.destinationPortItems, item)
	rule.allItems = append(rule.allItems, item)
}
