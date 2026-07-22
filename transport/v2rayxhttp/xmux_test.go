package v2rayxhttp

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/stretchr/testify/require"
)

type xmuxTestTransport struct{}

func (xmuxTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not used")
}

func newTestXMux(config xmuxConfig) *xmuxManager {
	return newXMuxManager(config, func() http.RoundTripper { return xmuxTestTransport{} })
}

func TestXMuxMaxConnections(t *testing.T) {
	manager := newTestXMux(xmuxConfig{maxConnections: byteRange{from: 4, to: 4}})
	clients := map[*xmuxClient]struct{}{}
	for range 32 {
		client := manager.acquireConnection()
		clients[client] = struct{}{}
		manager.doneConnection(client)
	}
	require.Len(t, clients, 4)
}

func TestXMuxReuseAndRequestBudgets(t *testing.T) {
	t.Run("connection reuse", func(t *testing.T) {
		manager := newTestXMux(xmuxConfig{maxReuse: byteRange{from: 2, to: 2}})
		clients := map[*xmuxClient]struct{}{}
		for range 64 {
			client := manager.acquireConnection()
			clients[client] = struct{}{}
			manager.doneConnection(client)
		}
		require.Len(t, clients, 32)
	})
	t.Run("request budget", func(t *testing.T) {
		manager := newTestXMux(xmuxConfig{maxRequests: byteRange{from: 2, to: 2}})
		clients := map[*xmuxClient]struct{}{}
		for range 64 {
			client := manager.acquireConnection()
			clients[client] = struct{}{}
			manager.doneConnection(client)
		}
		require.Len(t, clients, 32)
	})
	t.Run("packet request rotates after budget", func(t *testing.T) {
		manager := newTestXMux(xmuxConfig{maxRequests: byteRange{from: 2, to: 2}})
		first := manager.acquireConnection() // consumes the stream-down request
		require.True(t, manager.consumePacketRequest(first))
		require.False(t, manager.consumePacketRequest(first))
		second := manager.acquireRequest()
		require.NotSame(t, first, second)
		manager.doneConnection(first)
	})
}

func TestXMuxMaxConcurrency(t *testing.T) {
	manager := newTestXMux(xmuxConfig{maxConcurrency: byteRange{from: 2, to: 2}})
	clients := map[*xmuxClient]struct{}{}
	for range 64 {
		client := manager.acquireConnection()
		clients[client] = struct{}{}
	}
	require.Len(t, clients, 32)
}

func TestXMuxConfigValidation(t *testing.T) {
	_, err := newXMuxConfig(option.V2RayXHTTPXmuxOptions{
		MaxConcurrency: option.V2RayXHTTPRange{From: 1, To: 1},
		MaxConnections: option.V2RayXHTTPRange{From: 1, To: 1},
	})
	require.Error(t, err)
	_, err = NewClient(context.Background(), nil, option.ServerOptions{}.Build(), option.V2RayXHTTPOptions{
		XMUX: option.V2RayXHTTPXmuxOptions{HKeepAlivePeriod: -1},
	}, nil)
	require.Error(t, err)
}
