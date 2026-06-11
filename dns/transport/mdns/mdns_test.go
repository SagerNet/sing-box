package mdns

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
}
