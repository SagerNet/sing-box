//go:build unix

package srs

import (
	"bytes"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

func TestMmapMatchesSource(t *testing.T) {
	t.Parallel()
	source := option.PlainRuleSet{Rules: []option.HeadlessRule{
		{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultHeadlessRule{
				Domain:       badoption.Listable[string]{"example.com"},
				DomainSuffix: badoption.Listable[string]{".example.org"},
				IPCIDR:       badoption.Listable[string]{"10.0.0.0/8", "2001:db8::/32", "192.0.2.1"},
			},
		},
		{
			Type: C.RuleTypeLogical,
			LogicalOptions: option.LogicalHeadlessRule{
				Mode: C.LogicalTypeOr,
				Rules: []option.HeadlessRule{{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultHeadlessRule{
						AdGuardDomain: badoption.Listable[string]{"||ads.example.net^"},
						SourceIPCIDR:  badoption.Listable[string]{"172.16.0.0/12"},
					},
				}},
			},
		},
	}}
	var binary bytes.Buffer
	require.NoError(t, Write(&binary, source, C.RuleSetVersionCurrent))
	parsed, err := Read(bytes.NewReader(binary.Bytes()), false)
	require.NoError(t, err)

	mmapPath := filepath.Join(t.TempDir(), "rule-set.mmap")
	mmapFile, err := os.Create(mmapPath)
	require.NoError(t, err)
	defer mmapFile.Close()
	require.NoError(t, WriteMmap(mmapFile, parsed))
	_, err = mmapFile.Seek(0, io.SeekStart)
	require.NoError(t, err)
	mmap, err := ReadMmap(mmapFile)
	require.NoError(t, err)
	_, err = os.Stat(mmapPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Equal(t, parsed.Version, mmap.Version)
	require.Len(t, mmap.Options.Rules, 2)

	first := mmap.Options.Rules[0].DefaultOptions
	require.True(t, first.DomainMatcher.Match("example.com"))
	require.True(t, first.DomainMatcher.Match("www.example.org"))
	require.False(t, first.DomainMatcher.Match("example.org"))
	require.False(t, first.DomainMatcher.Match("example.net"))
	require.True(t, first.IPSet.Contains(netip.MustParseAddr("10.255.0.1")))
	require.True(t, first.IPSet.Contains(netip.MustParseAddr("192.0.2.1")))
	require.False(t, first.IPSet.Contains(netip.MustParseAddr("192.0.2.2")))
	require.True(t, first.IPSet.Contains(netip.MustParseAddr("2001:db8::1")))
	require.False(t, first.IPSet.Contains(netip.MustParseAddr("2001:db9::1")))

	second := mmap.Options.Rules[1].LogicalOptions.Rules[0].DefaultOptions
	require.True(t, second.AdGuardDomainMatcher.Match("ads.example.net"))
	require.False(t, second.AdGuardDomainMatcher.Match("example.net"))
	require.True(t, second.SourceIPSet.Contains(netip.MustParseAddr("172.31.255.255")))
	require.False(t, second.SourceIPSet.Contains(netip.MustParseAddr("172.32.0.0")))
}
