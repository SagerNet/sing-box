package mdns

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mDNS "github.com/miekg/dns"
)

func TestQueryDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()

	t.Run("no context deadline", func(t *testing.T) {
		require.Equal(t, now.Add(mdnsTimeout), queryDeadline(context.Background(), now))
	})

	t.Run("loose context deadline keeps window", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), now.Add(10*time.Second))
		defer cancel()
		require.Equal(t, now.Add(mdnsTimeout), queryDeadline(ctx, now))
	})

	t.Run("tight context deadline is clamped with headroom", func(t *testing.T) {
		ctxDeadline := now.Add(800 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), ctxDeadline)
		defer cancel()
		require.Equal(t, ctxDeadline.Add(-mdnsHeadroom), queryDeadline(ctx, now))
	})

	// Below mdnsHeadroom the clamp alone would land in the past, sending the
	// query and then abandoning it before any responder could answer.
	t.Run("deadline under headroom keeps the minimum window", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), now.Add(300*time.Millisecond))
		defer cancel()
		require.Equal(t, now.Add(mdnsMinWindow), queryDeadline(ctx, now))
	})

	t.Run("already expired deadline keeps the minimum window", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), now.Add(-time.Second))
		defer cancel()
		require.Equal(t, now.Add(mdnsMinWindow), queryDeadline(ctx, now))
	})

	t.Run("deadline never lands in the past", func(t *testing.T) {
		for _, timeout := range []time.Duration{
			-time.Second, 0, time.Millisecond, 100 * time.Millisecond,
			mdnsMinWindow, mdnsHeadroom, time.Second, 10 * time.Second,
		} {
			ctx, cancel := context.WithDeadline(context.Background(), now.Add(timeout))
			require.False(t, queryDeadline(ctx, now).Before(now.Add(mdnsMinWindow)),
				"timeout %s produced a window shorter than mdnsMinWindow", timeout)
			cancel()
		}
	})
}

func TestLocalZoneOf(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name     string
		expected string
	}{
		{"printer.local.", "local."},
		{"PRINTER.LOCAL", "local."},
		{"local.", "local."},
		{"1.0.254.169.in-addr.arpa.", "254.169.in-addr.arpa."},
		{"1.0.0.0.8.e.f.ip6.arpa.", "8.e.f.ip6.arpa."},
		{"example.com.", "example.com."},
	} {
		require.Equal(t, testCase.expected, localZoneOf(testCase.name), testCase.name)
	}
}

// A negative answer without an SOA makes dns.Client compute a zero TTL and
// skip caching entirely, so every repeated lookup re-floods the link.
func TestAppendNegativeAuthority(t *testing.T) {
	t.Parallel()

	question := mDNS.Question{Name: "absent.local.", Qtype: mDNS.TypeA, Qclass: mDNS.ClassINET}
	response := newResponseFromQuestion(question)
	appendNegativeAuthority(response, question)

	require.Empty(t, response.Answer)
	require.Len(t, response.Ns, 1)

	soa, isSOA := response.Ns[0].(*mDNS.SOA)
	require.True(t, isSOA)
	require.Equal(t, "local.", soa.Hdr.Name)
	require.EqualValues(t, mdnsNegativeTTL, soa.Hdr.Ttl)
	require.EqualValues(t, mdnsNegativeTTL, soa.Minttl)

	// The synthesized record has to survive a pack/unpack round trip, since it
	// goes out on the wire to the querying client.
	packed, err := response.Pack()
	require.NoError(t, err)
	var unpacked mDNS.Msg
	require.NoError(t, unpacked.Unpack(packed))
	require.Len(t, unpacked.Ns, 1)
}
