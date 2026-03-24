package option

import (
	"context"
	"testing"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
	"github.com/stretchr/testify/require"
)

type stubDNSTransportOptionsRegistry struct{}

func (stubDNSTransportOptionsRegistry) CreateOptions(string) (any, bool) {
	return nil, false
}

func TestDNSOptionsRejectsEvaluateLegacyRcodeServer(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})
	var options DNSOptions
	err := json.UnmarshalContext(ctx, []byte(`{
		"servers": [
			{"tag": "legacy-rcode", "address": "rcode://success"},
			{"tag": "default", "address": "1.1.1.1"}
		],
		"rules": [
			{"domain": ["example.com"], "action": "evaluate", "server": "legacy-rcode"}
		]
	}`), &options)
	require.ErrorContains(t, err, "evaluate action cannot reference legacy rcode server: legacy-rcode")
}
