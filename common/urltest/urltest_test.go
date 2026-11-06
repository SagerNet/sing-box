package urltest

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

type directDialer struct{}

func (directDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, destination.String())
}

func (directDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func TestBandwidthResultThroughput(t *testing.T) {
	t.Parallel()
	require.Equal(t, uint32(1024), (&BandwidthResult{Bytes: 1024, Duration: time.Second}).Throughput())
	require.Equal(t, uint32(2048), (&BandwidthResult{Bytes: 1024, Duration: 500 * time.Millisecond}).Throughput())
	// A transfer faster than the clock can resolve reports the floored rate, not zero:
	// zero would exclude the fastest path from throughput ranking altogether.
	require.Equal(t, uint32(1024000), (&BandwidthResult{Bytes: 1024}).Throughput())
	require.Equal(t, uint32(1024000), (&BandwidthResult{Bytes: 1024, Duration: time.Microsecond}).Throughput())
	// No bytes means no sample, whatever the clock says.
	require.Zero(t, (&BandwidthResult{Duration: time.Second}).Throughput())
}

// TestBandwidthTestCap is the property the whole design rests on: the probe reads the
// cap and stops, however much the server is willing to send.
func TestBandwidthTestCap(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 32*1024)
		// Far more than the cap, and more than the test would tolerate draining.
		for range 512 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	const maxBytes = 64 * 1024
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := BandwidthTest(ctx, server.URL, directDialer{}, maxBytes, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, uint32(maxBytes), result.Bytes)
	// Duration itself may legitimately be zero on a coarse clock, so assert on the
	// derived rate, which is what selection actually consumes.
	require.Positive(t, result.Throughput())
}

func TestBandwidthTestShortBody(t *testing.T) {
	t.Parallel()
	const bodySize = 1000
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, bodySize))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := BandwidthTest(ctx, server.URL, directDialer{}, 64*1024, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, uint32(bodySize), result.Bytes)
}

// TestBandwidthTestRejectsErrorStatus guards the failure mode where a rate-limited
// endpoint returns a small error page fast and scores as an excellent path.
func TestBandwidthTestRejectsErrorStatus(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("slow down"))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := BandwidthTest(ctx, server.URL, directDialer{}, 64*1024, 10*time.Second)
	require.Error(t, err)
}

func TestBandwidthTestEmptyBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := BandwidthTest(ctx, server.URL, directDialer{}, 64*1024, 10*time.Second)
	require.Error(t, err)
}

func TestStoreURLTestPreservesOtherMetric(t *testing.T) {
	t.Parallel()
	storage := NewHistoryStorage()
	defer storage.Close()

	// A bandwidth sample cannot create an entry on its own: without a latency sample
	// the outbound is not selectable, and a zero delay would corrupt latency ranking.
	storage.StoreURLTestBandwidth("proxy", 4096, 65536)
	require.Nil(t, storage.LoadURLTestHistory("proxy"))

	storage.StoreURLTestDelay("proxy", 100)
	storage.StoreURLTestBandwidth("proxy", 4096, 65536)
	history := storage.LoadURLTestHistory("proxy")
	require.NotNil(t, history)
	require.Equal(t, uint16(100), history.Delay)
	require.Equal(t, uint32(4096), history.Throughput)

	// A later latency probe must not discard the throughput sample.
	storage.StoreURLTestDelay("proxy", 120)
	history = storage.LoadURLTestHistory("proxy")
	require.Equal(t, uint16(120), history.Delay)
	require.Equal(t, uint32(4096), history.Throughput)
	require.Equal(t, uint32(65536), history.Bytes)
}
