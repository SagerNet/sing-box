package dns

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json/badoption"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

// flakyRetryTransport drives behaviour through a list of per-attempt callbacks
// and counts both Exchange and Reset invocations. Used by TestExchangeRetryIntervals.
type flakyRetryTransport struct {
	fakeDNSTransport
	mu          sync.Mutex
	attempts    int32
	resetCalled int32
	behaviors   []func(ctx context.Context) (*mDNS.Msg, error)
}

func (t *flakyRetryTransport) Exchange(ctx context.Context, _ *mDNS.Msg) (*mDNS.Msg, error) {
	idx := atomic.AddInt32(&t.attempts, 1) - 1
	t.mu.Lock()
	if int(idx) >= len(t.behaviors) {
		t.mu.Unlock()
		return nil, E.New("flakyRetryTransport: unexpected attempt #", idx+1)
	}
	cb := t.behaviors[idx]
	t.mu.Unlock()
	return cb(ctx)
}

func (t *flakyRetryTransport) Reset() {
	atomic.AddInt32(&t.resetCalled, 1)
}

func (t *flakyRetryTransport) Attempts() int32     { return atomic.LoadInt32(&t.attempts) }
func (t *flakyRetryTransport) ResetCount() int32   { return atomic.LoadInt32(&t.resetCalled) }

// blocksUntilDeadline returns a behavior that blocks until the per-attempt
// context's deadline fires, then returns context.DeadlineExceeded.
func blocksUntilDeadline() func(ctx context.Context) (*mDNS.Msg, error) {
	return func(ctx context.Context) (*mDNS.Msg, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

// returnsResponse synthesises a minimal NOERROR response.
func returnsResponse() func(ctx context.Context) (*mDNS.Msg, error) {
	return func(ctx context.Context) (*mDNS.Msg, error) {
		resp := &mDNS.Msg{}
		resp.SetRcode(&mDNS.Msg{}, mDNS.RcodeSuccess)
		return resp, nil
	}
}

func newRetryClient(timeout time.Duration, intervals []time.Duration) *Client {
	return NewClient(ClientOptions{
		Context:        context.Background(),
		Timeout:        timeout,
		RetryIntervals: intervals,
	})
}

func newQuestion() *mDNS.Msg {
	q := &mDNS.Msg{}
	q.SetQuestion("example.com.", mDNS.TypeA)
	return q
}

func TestExchangeRetryIntervals(t *testing.T) {
	t.Run("SucceedsOnSecondAttempt", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
				returnsResponse(),
			},
		}
		c := newRetryClient(5*time.Second, []time.Duration{100 * time.Millisecond, 1 * time.Second})
		resp, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.EqualValues(t, 2, tp.Attempts())
		require.EqualValues(t, 1, tp.ResetCount())
	})

	t.Run("AllAttemptsTimeout", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
				blocksUntilDeadline(),
				blocksUntilDeadline(),
			},
		}
		c := newRetryClient(5*time.Second, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond})
		resp, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		require.Nil(t, resp)
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded), "expected wrapped DeadlineExceeded, got: %v", err)
		require.EqualValues(t, 3, tp.Attempts())
		require.EqualValues(t, 2, tp.ResetCount())
	})

	t.Run("StopsOnRcodeError", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				func(context.Context) (*mDNS.Msg, error) {
					return nil, RcodeError(mDNS.RcodeServerFailure)
				},
			},
		}
		c := newRetryClient(5*time.Second, []time.Duration{1 * time.Second, 1 * time.Second, 1 * time.Second})
		resp, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, mDNS.RcodeServerFailure, resp.Rcode)
		require.EqualValues(t, 1, tp.Attempts())
		require.EqualValues(t, 0, tp.ResetCount())
	})

	t.Run("StopsOnNonRetriableError", func(t *testing.T) {
		nonRetriable := errors.New("malformed message")
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				func(context.Context) (*mDNS.Msg, error) { return nil, nonRetriable },
			},
		}
		c := newRetryClient(5*time.Second, []time.Duration{1 * time.Second, 1 * time.Second})
		_, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		require.ErrorIs(t, err, nonRetriable)
		require.EqualValues(t, 1, tp.Attempts())
		require.EqualValues(t, 0, tp.ResetCount())
	})

	t.Run("RespectsParentContextCancel", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
				blocksUntilDeadline(),
				blocksUntilDeadline(),
			},
		}
		c := newRetryClient(30*time.Second, []time.Duration{10 * time.Second, 10 * time.Second, 10 * time.Second})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		_, err := c.exchangeToTransportRetry(ctx, tp, newQuestion())
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "expected Canceled, got: %v", err)
		require.EqualValues(t, 1, tp.Attempts())
	})

	t.Run("EmptyScheduleUsesLegacyPath", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
			},
		}
		c := newRetryClient(100*time.Millisecond, nil)
		_, err := c.exchangeToTransport(context.Background(), tp, newQuestion())
		require.Error(t, err)
		// Legacy path returns the raw transport error without E.Cause wrap.
		require.True(t, errors.Is(err, context.DeadlineExceeded), "expected raw DeadlineExceeded, got: %v", err)
		// And the error string must NOT contain the retry wrap prefix.
		require.NotContains(t, err.Error(), "attempts failed")
		require.EqualValues(t, 1, tp.Attempts())
		require.EqualValues(t, 0, tp.ResetCount())
	})

	t.Run("SingleEntryUsesThatTimeout", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
			},
		}
		start := time.Now()
		c := newRetryClient(10*time.Second, []time.Duration{50 * time.Millisecond})
		_, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		elapsed := time.Since(start)
		require.Error(t, err)
		require.EqualValues(t, 1, tp.Attempts())
		require.Less(t, elapsed, 500*time.Millisecond, "single-attempt should respect 50ms cap; elapsed=%v", elapsed)
	})

	t.Run("OuterCapTruncatesSchedule", func(t *testing.T) {
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
				blocksUntilDeadline(),
				blocksUntilDeadline(),
			},
		}
		start := time.Now()
		c := newRetryClient(200*time.Millisecond, []time.Duration{5 * time.Second, 5 * time.Second, 5 * time.Second})
		_, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		elapsed := time.Since(start)
		require.Error(t, err)
		// Per plan v3 #8: outer cap fires before per-attempt; exactly 1 attempt;
		// Reset must be skipped before exit (N6 fix).
		require.EqualValues(t, 1, tp.Attempts())
		require.EqualValues(t, 0, tp.ResetCount())
		require.Less(t, elapsed, 1*time.Second, "outer cap should fire ~200ms; elapsed=%v", elapsed)
	})

	t.Run("ZombieSocketRecovery", func(t *testing.T) {
		// Custom transport: first Exchange blocks until Reset is called, then succeeds.
		// Models the Beeline-NAT zombie socket scenario the feature exists to fix.
		var resetCh = make(chan struct{}, 1)
		var callCount int32
		tp := &fakeRetryControlled{
			resetCh:   resetCh,
			callCount: &callCount,
		}
		c := newRetryClient(5*time.Second, []time.Duration{100 * time.Millisecond, 1 * time.Second})
		resp, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.EqualValues(t, 2, atomic.LoadInt32(&callCount))
	})

	t.Run("BackgroundRefreshDoesNotReset", func(t *testing.T) {
		// Background refresh path uses exchangeToTransport directly (no retry,
		// no Reset). Verifies fg/bg structural separation (Round 1 C3 fix).
		tp := &flakyRetryTransport{
			behaviors: []func(context.Context) (*mDNS.Msg, error){
				blocksUntilDeadline(),
			},
		}
		c := newRetryClient(50*time.Millisecond, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond})
		_, err := c.exchangeToTransport(context.Background(), tp, newQuestion())
		require.Error(t, err)
		require.EqualValues(t, 1, tp.Attempts())
		require.EqualValues(t, 0, tp.ResetCount())
	})

	t.Run("ConcurrentForegroundRetriesAreSafe", func(t *testing.T) {
		// Two goroutines retry on independent transports concurrently.
		// Verifies no panic, no deadlock under -race.
		c := newRetryClient(5*time.Second, []time.Duration{50 * time.Millisecond, 1 * time.Second})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				tp := &flakyRetryTransport{
					behaviors: []func(context.Context) (*mDNS.Msg, error){
						blocksUntilDeadline(),
						returnsResponse(),
					},
				}
				_, err := c.exchangeToTransportRetry(context.Background(), tp, newQuestion())
				errs[i] = err
			}()
		}
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
	})
}

// fakeRetryControlled blocks first Exchange until Reset is called.
type fakeRetryControlled struct {
	fakeDNSTransport
	resetCh   chan struct{}
	callCount *int32
}

func (t *fakeRetryControlled) Exchange(ctx context.Context, _ *mDNS.Msg) (*mDNS.Msg, error) {
	n := atomic.AddInt32(t.callCount, 1)
	if n == 1 {
		// Block until either Reset() is called or the per-attempt context dies.
		select {
		case <-t.resetCh:
			return nil, context.DeadlineExceeded
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	resp := &mDNS.Msg{}
	resp.SetRcode(&mDNS.Msg{}, mDNS.RcodeSuccess)
	return resp, nil
}

func (t *fakeRetryControlled) Reset() {
	select {
	case t.resetCh <- struct{}{}:
	default:
	}
}

var _ adapter.DNSTransport = (*fakeRetryControlled)(nil)
var _ adapter.DNSTransport = (*flakyRetryTransport)(nil)

func TestNewRouterRejectsNonPositiveRetryIntervals(t *testing.T) {
	cases := []struct {
		name      string
		intervals badoption.Listable[badoption.Duration]
		want      string
	}{
		{
			name: "zero",
			intervals: badoption.Listable[badoption.Duration]{
				badoption.Duration(time.Second),
				badoption.Duration(0),
			},
			want: "retry_intervals[1]: must be positive",
		},
		{
			name: "negative",
			intervals: badoption.Listable[badoption.Duration]{
				badoption.Duration(-time.Second),
			},
			want: "retry_intervals[0]: must be positive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRouter(context.Background(), log.NewNOPFactory(), option.DNSOptions{
				RawDNSOptions: option.RawDNSOptions{
					DNSClientOptions: option.DNSClientOptions{
						RetryIntervals: tc.intervals,
					},
				},
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}
