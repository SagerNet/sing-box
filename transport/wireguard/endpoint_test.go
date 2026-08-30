package wireguard

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/stretchr/testify/require"
)

func TestWireGuardHandshakeRetryLog(t *testing.T) {
	require.True(t, isWireGuardHandshakeRetry(wireGuardHandshakeRetryLog))
	require.False(t, isWireGuardHandshakeRetry("%s - Sending handshake initiation"))
	require.False(t, isWireGuardHandshakeRetry("%s - Handshake did not complete after %d attempts, giving up"))
}

func TestRefreshPeerEndpoint(t *testing.T) {
	oldAddress := netip.MustParseAddr("192.0.2.1")
	newAddress := netip.MustParseAddr("192.0.2.2")
	var disableCache bool
	var ipcConfig string
	endpoint := newRefreshTestEndpoint(oldAddress, func(ctx context.Context, domain string, noCache bool) ([]netip.Addr, error) {
		require.Equal(t, "peer.example", domain)
		disableCache = noCache
		return []netip.Addr{oldAddress, newAddress}, nil
	})
	endpoint.ipcSet = func(config string) error {
		ipcConfig = config
		return nil
	}

	endpoint.refreshPeerEndpoints(context.Background())

	require.True(t, disableCache)
	require.Equal(t, netip.AddrPortFrom(newAddress, 51820), endpoint.peers[0].endpoint)
	require.Equal(t, "public_key="+strings.Repeat("ab", 32)+"\nupdate_only=true\nendpoint=192.0.2.2:51820", ipcConfig)
}

func TestRefreshPeerEndpointUnchanged(t *testing.T) {
	oldAddress := netip.MustParseAddr("192.0.2.1")
	endpoint := newRefreshTestEndpoint(oldAddress, func(ctx context.Context, domain string, noCache bool) ([]netip.Addr, error) {
		return []netip.Addr{oldAddress}, nil
	})
	endpoint.ipcSet = func(config string) error {
		t.Fatal("unexpected WireGuard IPC update")
		return nil
	}

	endpoint.refreshPeerEndpoints(context.Background())

	require.Equal(t, netip.AddrPortFrom(oldAddress, 51820), endpoint.peers[0].endpoint)
}

func TestRefreshPeerEndpointKeepsOldAddressOnFailure(t *testing.T) {
	oldAddress := netip.MustParseAddr("192.0.2.1")
	newAddress := netip.MustParseAddr("192.0.2.2")
	endpoint := newRefreshTestEndpoint(oldAddress, func(ctx context.Context, domain string, noCache bool) ([]netip.Addr, error) {
		return []netip.Addr{newAddress}, nil
	})
	endpoint.ipcSet = func(config string) error {
		return errors.New("update failed")
	}

	endpoint.refreshPeerEndpoints(context.Background())

	require.Equal(t, netip.AddrPortFrom(oldAddress, 51820), endpoint.peers[0].endpoint)
}

func TestDNSRefreshEventsAreCoalescedAndThrottled(t *testing.T) {
	oldAddress := netip.MustParseAddr("192.0.2.1")
	var lookupCount atomic.Int32
	lookupDone := make(chan struct{}, 1)
	endpoint := newRefreshTestEndpoint(oldAddress, func(ctx context.Context, domain string, noCache bool) ([]netip.Addr, error) {
		lookupCount.Add(1)
		lookupDone <- struct{}{}
		return []netip.Addr{oldAddress}, nil
	})
	endpoint.ipcSet = func(config string) error { return nil }
	endpoint.startDNSRefresh()
	t.Cleanup(endpoint.stopDNSRefresh)

	endpoint.triggerDNSRefresh()
	endpoint.triggerDNSRefresh()
	select {
	case <-lookupDone:
	case <-time.After(time.Second):
		t.Fatal("DNS refresh did not run")
	}
	endpoint.triggerDNSRefresh()
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, int32(1), lookupCount.Load())
}

func newRefreshTestEndpoint(oldAddress netip.Addr, resolve func(context.Context, string, bool) ([]netip.Addr, error)) *Endpoint {
	return &Endpoint{
		options: EndpointOptions{
			Context:     context.Background(),
			Logger:      logger.NOP(),
			ResolvePeer: resolve,
		},
		peers: []peerConfig{
			{
				destination:  M.ParseSocksaddrHostPort("peer.example", 51820),
				endpoint:     netip.AddrPortFrom(oldAddress, 51820),
				publicKeyHex: strings.Repeat("ab", 32),
			},
		},
	}
}
