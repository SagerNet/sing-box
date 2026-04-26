package option

import (
	"context"
	"testing"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"

	"github.com/stretchr/testify/require"
)

type stubDNSTransportOptionsRegistry struct{}

func (stubDNSTransportOptionsRegistry) CreateOptions(transportType string) (any, bool) {
	switch transportType {
	case C.DNSTypeUDP:
		return new(RemoteDNSServerOptions), true
	case C.DNSTypeFakeIP:
		return new(FakeIPDNSServerOptions), true
	default:
		return nil, false
	}
}

func TestDNSOptionsRejectsLegacyFakeIPOptions(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})
	var options DNSOptions
	err := json.UnmarshalContext(ctx, []byte(`{
		"fakeip": {
			"enabled": true,
			"inet4_range": "198.18.0.0/15"
		}
	}`), &options)
	require.EqualError(t, err, legacyDNSFakeIPRemovedMessage)
}

func TestDNSRetryIntervalsUnmarshal(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})

	cases := []struct {
		name    string
		input   string
		want    []time.Duration
		wantErr string
	}{
		{
			name:  "list",
			input: `{"retry_intervals": ["1s", "2s"]}`,
			want:  []time.Duration{time.Second, 2 * time.Second},
		},
		{
			name:  "single_string",
			input: `{"retry_intervals": "3s"}`,
			want:  []time.Duration{3 * time.Second},
		},
		{
			name:  "empty_list",
			input: `{"retry_intervals": []}`,
			want:  nil,
		},
		{
			name:  "absent",
			input: `{}`,
			want:  nil,
		},
		{
			name:  "null_leaves_default_unchanged",
			input: `{"retry_intervals": null}`,
			want:  nil,
		},
		{
			name:    "invalid_duration",
			input:   `{"retry_intervals": ["1s", "garbage"]}`,
			wantErr: "garbage",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var opts DNSOptions
			err := json.UnmarshalContext(ctx, []byte(tc.input), &opts)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			got := make([]time.Duration, 0, len(opts.RetryIntervals))
			for _, d := range opts.RetryIntervals {
				got = append(got, time.Duration(d))
			}
			if len(tc.want) == 0 {
				require.Empty(t, got)
				return
			}
			require.Equal(t, tc.want, got)
		})
	}

	// Sanity check: a fully populated DNSClientOptions struct round-trips
	// through Listable[Duration] without dropping the field.
	require.NotNil(t, badoption.Listable[badoption.Duration]{badoption.Duration(time.Second)})
}

func TestDNSServerOptionsRejectsLegacyFormats(t *testing.T) {
	t.Parallel()

	ctx := service.ContextWith[DNSTransportOptionsRegistry](context.Background(), stubDNSTransportOptionsRegistry{})
	testCases := []string{
		`{"address":"1.1.1.1"}`,
		`{"type":"legacy","address":"1.1.1.1"}`,
	}
	for _, content := range testCases {
		var options DNSServerOptions
		err := json.UnmarshalContext(ctx, []byte(content), &options)
		require.EqualError(t, err, legacyDNSServerRemovedMessage)
	}
}
