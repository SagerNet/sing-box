package rule

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type fakeDNSRuleSetUpdateValidator struct {
	validate func(tag string, metadata adapter.RuleSetMetadata) error
}

func (v *fakeDNSRuleSetUpdateValidator) ValidateRuleSetMetadataUpdate(tag string, metadata adapter.RuleSetMetadata) error {
	if v.validate == nil {
		return nil
	}
	return v.validate(tag, metadata)
}

func TestLocalRuleSetReloadRulesRejectsInvalidUpdateBeforeCommit(t *testing.T) {
	t.Parallel()

	var callbackCount atomic.Int32
	ctx := service.ContextWith[adapter.DNSRuleSetUpdateValidator](context.Background(), &fakeDNSRuleSetUpdateValidator{
		validate: func(tag string, metadata adapter.RuleSetMetadata) error {
			require.Equal(t, "dynamic-set", tag)
			if metadata.ContainsDNSQueryTypeRule {
				return E.New("dns conflict")
			}
			return nil
		},
	})
	ruleSet := &LocalRuleSet{
		ctx:        ctx,
		tag:        "dynamic-set",
		fileFormat: C.RuleSetFormatSource,
	}
	_ = ruleSet.callbacks.PushBack(func(adapter.RuleSet) {
		callbackCount.Add(1)
	})

	err := ruleSet.reloadRules([]option.HeadlessRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			Domain: badoption.Listable[string]{"example.com"},
		},
	}})
	require.NoError(t, err)
	require.Equal(t, int32(1), callbackCount.Load())
	require.False(t, ruleSet.metadata.ContainsDNSQueryTypeRule)
	require.True(t, ruleSet.Match(&adapter.InboundContext{Domain: "example.com"}))

	err = ruleSet.reloadRules([]option.HeadlessRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			QueryType: badoption.Listable[option.DNSQueryType]{option.DNSQueryType(1)},
		},
	}})
	require.ErrorContains(t, err, "dns conflict")
	require.Equal(t, int32(1), callbackCount.Load())
	require.False(t, ruleSet.metadata.ContainsDNSQueryTypeRule)
	require.True(t, ruleSet.Match(&adapter.InboundContext{Domain: "example.com"}))
}

func TestRemoteRuleSetLoadBytesRejectsInvalidUpdateBeforeCommit(t *testing.T) {
	t.Parallel()

	var callbackCount atomic.Int32
	ctx := service.ContextWith[adapter.DNSRuleSetUpdateValidator](context.Background(), &fakeDNSRuleSetUpdateValidator{
		validate: func(tag string, metadata adapter.RuleSetMetadata) error {
			require.Equal(t, "dynamic-set", tag)
			if metadata.ContainsDNSQueryTypeRule {
				return E.New("dns conflict")
			}
			return nil
		},
	})
	ruleSet := &RemoteRuleSet{
		ctx: ctx,
		options: option.RuleSet{
			Tag:    "dynamic-set",
			Format: C.RuleSetFormatSource,
		},
		callbacks: list.List[adapter.RuleSetUpdateCallback]{},
	}
	_ = ruleSet.callbacks.PushBack(func(adapter.RuleSet) {
		callbackCount.Add(1)
	})

	err := ruleSet.loadBytes([]byte(`{"version":4,"rules":[{"domain":["example.com"]}]}`))
	require.NoError(t, err)
	require.Equal(t, int32(1), callbackCount.Load())
	require.False(t, ruleSet.metadata.ContainsDNSQueryTypeRule)
	require.True(t, ruleSet.Match(&adapter.InboundContext{Domain: "example.com"}))

	err = ruleSet.loadBytes([]byte(`{"version":4,"rules":[{"query_type":["A"]}]}`))
	require.ErrorContains(t, err, "dns conflict")
	require.Equal(t, int32(1), callbackCount.Load())
	require.False(t, ruleSet.metadata.ContainsDNSQueryTypeRule)
	require.True(t, ruleSet.Match(&adapter.InboundContext{Domain: "example.com"}))
}
