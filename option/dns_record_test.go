package option

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func mustRecordOptions(t *testing.T, record string) DNSRecordOptions {
	t.Helper()
	var value DNSRecordOptions
	require.NoError(t, value.UnmarshalJSON([]byte(`"`+record+`"`)))
	return value
}

func TestDNSRecordOptionsUnmarshalJSONAcceptsRelativeOwnerNames(t *testing.T) {
	t.Parallel()

	for _, record := range []string{
		"example.com A 1.1.1.1",
		"@ IN A 1.1.1.1",
		"www IN CNAME @",
	} {
		value := mustRecordOptions(t, record)
		require.NotNil(t, value.RR)
	}
}

func TestDNSRecordOptionsMatchIgnoresTTL(t *testing.T) {
	t.Parallel()

	expected := mustRecordOptions(t, "example.com. 600 IN A 1.1.1.1")
	record, err := dns.NewRR("example.com. 60 IN A 1.1.1.1")
	require.NoError(t, err)

	require.True(t, expected.Match(record))
}
