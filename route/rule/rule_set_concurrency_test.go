package rule

import (
	"context"
	"sync"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

type concurrentRuleSet interface {
	String() string
	Match(metadata *adapter.InboundContext) bool
	mergeableRule() *DefaultHeadlessRule
}

func TestRuleSetConcurrentUpdate(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		ruleSet := &LocalRuleSet{
			ctx: context.Background(),
			tag: "local",
		}
		require.NoError(t, ruleSet.reloadRules(testDomainHeadlessRules("example.com")))
		testRuleSetConcurrentUpdate(t, ruleSet, func(iteration int) error {
			return ruleSet.reloadRules(testDomainHeadlessRules(testRuleSetDomain(iteration)))
		})
	})

	t.Run("remote", func(t *testing.T) {
		ruleSet := &RemoteRuleSet{
			ctx: context.Background(),
			tag: "remote",
			options: option.RuleSet{
				Format: C.RuleSetFormatSource,
			},
		}
		require.NoError(t, ruleSet.loadBytes(testRemoteRuleSetContent[0]))
		testRuleSetConcurrentUpdate(t, ruleSet, func(iteration int) error {
			return ruleSet.loadBytes(testRemoteRuleSetContent[iteration%len(testRemoteRuleSetContent)])
		})
	})
}

func testRuleSetConcurrentUpdate(t *testing.T, ruleSet concurrentRuleSet, update func(iteration int) error) {
	t.Helper()

	const readerCount = 4
	stop := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(readerCount)
	for range readerCount {
		go func() {
			defer readers.Done()
			metadata := &adapter.InboundContext{Domain: "example.com"}
			for {
				select {
				case <-stop:
					return
				default:
					ruleSet.Match(metadata)
					ruleSet.String()
					ruleSet.mergeableRule()
				}
			}
		}()
	}

	var updateErr error
	for iteration := range 500 {
		updateErr = update(iteration)
		if updateErr != nil {
			break
		}
	}
	close(stop)
	readers.Wait()
	require.NoError(t, updateErr)
}

func testDomainHeadlessRules(domain string) []option.HeadlessRule {
	return []option.HeadlessRule{{
		Type: C.RuleTypeDefault,
		DefaultOptions: option.DefaultHeadlessRule{
			Domain: badoption.Listable[string]{domain},
		},
	}}
}

func testRuleSetDomain(iteration int) string {
	if iteration%2 == 0 {
		return "example.com"
	}
	return "example.org"
}

var testRemoteRuleSetContent = [][]byte{
	[]byte(`{"version":4,"rules":[{"domain":["example.com"]}]}`),
	[]byte(`{"version":4,"rules":[{"domain":["example.org"]}]}`),
}
