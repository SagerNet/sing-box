package transport

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testConnectorConnection struct{}

func TestConnectorRecursiveGetFailsFast(t *testing.T) {
	t.Parallel()

	var (
		dialCount  atomic.Int32
		closeCount atomic.Int32
		connector  *Connector[*testConnectorConnection]
	)

	dial := func(ctx context.Context) (*testConnectorConnection, error) {
		dialCount.Add(1)
		_, err := connector.Get(ctx)
		if err != nil {
			return nil, err
		}
		return &testConnectorConnection{}, nil
	}

	connector = NewConnector(context.Background(), dial, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {
			closeCount.Add(1)
		},
		Reset: func(connection *testConnectorConnection) {
			closeCount.Add(1)
		},
	})

	_, err := connector.Get(context.Background())
	require.ErrorIs(t, err, errRecursiveConnectorDial)
	require.EqualValues(t, 1, dialCount.Load())
	require.EqualValues(t, 0, closeCount.Load())
}

func TestConnectorRecursiveGetAcrossConnectorsAllowed(t *testing.T) {
	t.Parallel()

	var (
		outerDialCount atomic.Int32
		innerDialCount atomic.Int32
		outerConnector *Connector[*testConnectorConnection]
		innerConnector *Connector[*testConnectorConnection]
	)

	innerConnector = NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		innerDialCount.Add(1)
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	outerConnector = NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		outerDialCount.Add(1)
		_, err := innerConnector.Get(ctx)
		if err != nil {
			return nil, err
		}
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	_, err := outerConnector.Get(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, outerDialCount.Load())
	require.EqualValues(t, 1, innerDialCount.Load())
}

func TestConnectorDialContextPreservesValueAndDeadline(t *testing.T) {
	t.Parallel()

	type contextKey struct{}

	var (
		dialValue       any
		dialDeadline    time.Time
		dialHasDeadline bool
	)

	connector := NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		dialValue = ctx.Value(contextKey{})
		dialDeadline, dialHasDeadline = ctx.Deadline()
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	deadline := time.Now().Add(time.Minute)
	requestContext, cancel := context.WithDeadline(context.WithValue(context.Background(), contextKey{}, "test-value"), deadline)
	defer cancel()

	_, err := connector.Get(requestContext)
	require.NoError(t, err)
	require.Equal(t, "test-value", dialValue)
	require.True(t, dialHasDeadline)
	require.WithinDuration(t, deadline, dialDeadline, time.Second)
}

func TestConnectorDialSkipsCanceledRequest(t *testing.T) {
	t.Parallel()

	var dialCount atomic.Int32
	connector := NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		dialCount.Add(1)
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	requestContext, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := connector.Get(requestContext)
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 0, dialCount.Load())
}

func TestConnectorCanceledRequestDoesNotCacheConnection(t *testing.T) {
	t.Parallel()

	var (
		dialCount  atomic.Int32
		closeCount atomic.Int32
	)
	dialStarted := make(chan struct{}, 1)
	releaseDial := make(chan struct{})

	connector := NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		dialCount.Add(1)
		select {
		case dialStarted <- struct{}{}:
		default:
		}
		<-releaseDial
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {
			closeCount.Add(1)
		},
		Reset: func(connection *testConnectorConnection) {},
	})

	requestContext, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := connector.Get(requestContext)
		result <- err
	}()

	<-dialStarted
	cancel()
	close(releaseDial)

	err := <-result
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 1, dialCount.Load())
	require.EqualValues(t, 1, closeCount.Load())

	_, err = connector.Get(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, dialCount.Load())
}

func TestConnectorDialContextNotCanceledByRequestContextAfterDial(t *testing.T) {
	t.Parallel()

	var dialContext context.Context
	connector := NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		dialContext = ctx
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	requestContext, cancel := context.WithCancel(context.Background())
	_, err := connector.Get(requestContext)
	require.NoError(t, err)
	require.NotNil(t, dialContext)

	cancel()

	select {
	case <-dialContext.Done():
		t.Fatal("dial context canceled by request context after successful dial")
	case <-time.After(100 * time.Millisecond):
	}

	err = connector.Close()
	require.NoError(t, err)
}

func TestConnectorDialContextCanceledOnClose(t *testing.T) {
	t.Parallel()

	var dialContext context.Context
	connector := NewConnector(context.Background(), func(ctx context.Context) (*testConnectorConnection, error) {
		dialContext = ctx
		return &testConnectorConnection{}, nil
	}, ConnectorCallbacks[*testConnectorConnection]{
		IsClosed: func(connection *testConnectorConnection) bool {
			return false
		},
		Close: func(connection *testConnectorConnection) {},
		Reset: func(connection *testConnectorConnection) {},
	})

	_, err := connector.Get(context.Background())
	require.NoError(t, err)
	require.NotNil(t, dialContext)

	select {
	case <-dialContext.Done():
		t.Fatal("dial context canceled before connector close")
	default:
	}

	err = connector.Close()
	require.NoError(t, err)

	select {
	case <-dialContext.Done():
	case <-time.After(time.Second):
		t.Fatal("dial context not canceled after connector close")
	}
}
