package route_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/route"

	"github.com/stretchr/testify/require"
)

func TestMonitor(t *testing.T) {
	t.Parallel()
	var closer myCloser
	closer.Add(1)
	monitor := route.NewConnectionMonitor()
	require.NoError(t, monitor.Start())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	monitor.Add(ctx, &closer)
	done := make(chan struct{})
	go func() {
		closer.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second + 100*time.Millisecond):
		t.Fatal("timeout")
	}
	cancel()
	require.NoError(t, monitor.Close())
}

type myCloser struct {
	sync.WaitGroup
}

func (c *myCloser) Close() error {
	c.Done()
	return nil
}
