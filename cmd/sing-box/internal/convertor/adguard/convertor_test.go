package adguard

import (
	"context"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/route/rule"

	"github.com/stretchr/testify/require"
)

func TestConverter(t *testing.T) {
	t.Parallel()
	rules, err := Convert(strings.NewReader(`
||example.org^
|example.com^
example.net^
||example.edu
||example.edu.tw^
|example.gov
example.arpa
@@|sagernet.example.org|
||sagernet.org^$important
@@|sing-box.sagernet.org^$important
`))
	require.NoError(t, err)
	require.Len(t, rules, 1)
	rule, err := rule.NewHeadlessRule(context.Background(), rules[0])
	require.NoError(t, err)
	matchDomain := []string{
		"example.org",
		"www.example.org",
		"example.com",
		"example.net",
		"isexample.net",
		"www.example.net",
		"example.edu",
		"example.edu.cn",
		"example.edu.tw",
		"www.example.edu",
		"www.example.edu.cn",
		"example.gov",
		"example.gov.cn",
		"example.arpa",
		"www.example.arpa",
		"isexample.arpa",
		"example.arpa.cn",
		"www.example.arpa.cn",
		"isexample.arpa.cn",
		"sagernet.org",
		"www.sagernet.org",
	}
	notMatchDomain := []string{
		"example.org.cn",
		"notexample.org",
		"example.com.cn",
		"www.example.com.cn",
		"example.net.cn",
		"notexample.edu",
		"notexample.edu.cn",
		"www.example.gov",
		"notexample.gov",
		"sagernet.example.org",
		"sing-box.sagernet.org",
	}
	for _, domain := range matchDomain {
		require.True(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
	for _, domain := range notMatchDomain {
		require.False(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
}

func TestHosts(t *testing.T) {
	t.Parallel()
	rules, err := Convert(strings.NewReader(`
127.0.0.1 localhost
::1 localhost #[IPv6]
0.0.0.0 google.com
`))
	require.NoError(t, err)
	require.Len(t, rules, 1)
	rule, err := rule.NewHeadlessRule(context.Background(), rules[0])
	require.NoError(t, err)
	matchDomain := []string{
		"google.com",
	}
	notMatchDomain := []string{
		"www.google.com",
		"notgoogle.com",
		"localhost",
	}
	for _, domain := range matchDomain {
		require.True(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
	for _, domain := range notMatchDomain {
		require.False(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
}

func TestSimpleHosts(t *testing.T) {
	t.Parallel()
	rules, err := Convert(strings.NewReader(`
example.com
www.example.org
`))
	require.NoError(t, err)
	require.Len(t, rules, 1)
	rule, err := rule.NewHeadlessRule(context.Background(), rules[0])
	require.NoError(t, err)
	matchDomain := []string{
		"example.com",
		"www.example.org",
	}
	notMatchDomain := []string{
		"example.com.cn",
		"www.example.com",
		"notexample.com",
		"example.org",
	}
	for _, domain := range matchDomain {
		require.True(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
	for _, domain := range notMatchDomain {
		require.False(t, rule.Match(&adapter.InboundContext{
			Domain: domain,
		}), domain)
	}
}
