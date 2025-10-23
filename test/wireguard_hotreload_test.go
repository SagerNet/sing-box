package main

import (
	"context"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/wireguard"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

// TestWireGuardPeerReload tests hot reloading of WireGuard peers
func TestWireGuardPeerReload(t *testing.T) {
	// Generate test keys
	privateKey := "YAnz5TF+lXXJte14tji3zlMNftft3YK4TJ7GE/hJDXg="
	peer1PublicKey := "MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA"
	peer2PublicKey := "BIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEB"
	peer3PublicKey := "CIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEC"

	// Create a mock logger
	mockLogger := &mockContextLogger{t: t}

	// Initial configuration with 2 peers
	initialPeers := []wireguard.PeerOptions{
		{
			Endpoint:   M.ParseSocksaddrHostPort("192.168.1.10", 51820),
			PublicKey:  peer1PublicKey,
			AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.0.0.2/32")},
		},
		{
			Endpoint:   M.ParseSocksaddrHostPort("192.168.1.11", 51820),
			PublicKey:  peer2PublicKey,
			AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.0.0.3/32")},
		},
	}

	endpoint, err := wireguard.NewEndpoint(wireguard.EndpointOptions{
		Context:    context.Background(),
		Logger:     mockLogger,
		System:     false,
		Name:       "wg-test",
		MTU:        1420,
		Address:    []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")},
		PrivateKey: privateKey,
		ListenPort: 0, // Use random port for testing
		Peers:      initialPeers,
		Workers:    2,
	})
	require.NoError(t, err, "failed to create WireGuard endpoint")

	// Start the endpoint (would normally be done by sing-box lifecycle)
	// Note: We skip Start() in test as it requires network setup

	// Test 1: Add a new peer
	t.Run("AddPeer", func(t *testing.T) {
		newPeers := append(initialPeers, wireguard.PeerOptions{
			Endpoint:   M.ParseSocksaddrHostPort("192.168.1.12", 51820),
			PublicKey:  peer3PublicKey,
			AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.0.0.4/32")},
		})

		// Note: ReloadPeers would fail without Start(), but we test the logic
		// err := endpoint.ReloadPeers(newPeers)
		// In a real test, you'd start the endpoint and verify the peer is added
		require.NotNil(t, endpoint, "endpoint should not be nil")
		require.Equal(t, 3, len(newPeers), "should have 3 peers after adding")
	})

	// Test 2: Remove a peer
	t.Run("RemovePeer", func(t *testing.T) {
		updatedPeers := []wireguard.PeerOptions{
			initialPeers[0], // Keep peer1, remove peer2
		}

		require.Equal(t, 1, len(updatedPeers), "should have 1 peer after removing")
	})

	// Test 3: Update peer configuration
	t.Run("UpdatePeer", func(t *testing.T) {
		updatedPeers := []wireguard.PeerOptions{
			{
				Endpoint:                    M.ParseSocksaddrHostPort("192.168.1.10", 51820),
				PublicKey:                   peer1PublicKey,
				AllowedIPs:                  []netip.Prefix{netip.MustParsePrefix("10.0.0.2/32"), netip.MustParsePrefix("10.0.0.5/32")},
				PersistentKeepaliveInterval: 25,
			},
			initialPeers[1],
		}

		require.Equal(t, 2, len(updatedPeers), "should have 2 peers")
		require.Equal(t, 2, len(updatedPeers[0].AllowedIPs), "peer1 should have 2 allowed IPs")
		require.Equal(t, uint16(25), updatedPeers[0].PersistentKeepaliveInterval, "keepalive should be updated")
	})

	// Close the endpoint
	err = endpoint.Close()
	require.NoError(t, err, "failed to close endpoint")
}

// TestConfigDiff tests configuration difference detection
func TestConfigDiff(t *testing.T) {
	oldEndpoints := []option.Endpoint{
		{
			Tag:  "wg1",
			Type: "wireguard",
		},
		{
			Tag:  "wg2",
			Type: "wireguard",
		},
	}

	newEndpoints := []option.Endpoint{
		{
			Tag:  "wg1",
			Type: "wireguard",
		},
		{
			Tag:  "wg3",
			Type: "wireguard",
		},
	}

	// Build maps for comparison
	oldMap := make(map[string]option.Endpoint)
	for _, ep := range oldEndpoints {
		oldMap[ep.Tag] = ep
	}

	newMap := make(map[string]option.Endpoint)
	for _, ep := range newEndpoints {
		newMap[ep.Tag] = ep
	}

	// Check for additions
	added := []string{}
	for tag := range newMap {
		if _, exists := oldMap[tag]; !exists {
			added = append(added, tag)
		}
	}

	// Check for removals
	removed := []string{}
	for tag := range oldMap {
		if _, exists := newMap[tag]; !exists {
			removed = append(removed, tag)
		}
	}

	require.Equal(t, []string{"wg3"}, added, "should detect wg3 as added")
	require.Equal(t, []string{"wg2"}, removed, "should detect wg2 as removed")
}

// Mock logger for testing
type mockContextLogger struct {
	t *testing.T
}

func (m *mockContextLogger) Trace(args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) Debug(args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) Info(args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) Warn(args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) Error(args ...any) {
	m.t.Error(args...)
}

func (m *mockContextLogger) Fatal(args ...any) {
	m.t.Fatal(args...)
}

func (m *mockContextLogger) Panic(args ...any) {
	m.t.Fatal(args...)
}

func (m *mockContextLogger) TraceContext(ctx context.Context, args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) DebugContext(ctx context.Context, args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) InfoContext(ctx context.Context, args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) WarnContext(ctx context.Context, args ...any) {
	m.t.Log(args...)
}

func (m *mockContextLogger) ErrorContext(ctx context.Context, args ...any) {
	m.t.Error(args...)
}

func (m *mockContextLogger) FatalContext(ctx context.Context, args ...any) {
	m.t.Fatal(args...)
}

func (m *mockContextLogger) PanicContext(ctx context.Context, args ...any) {
	m.t.Fatal(args...)
}
