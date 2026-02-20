//go:build !linux

package nftables

import (
	"net/netip"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

type stubManager struct{}

func newManager(options Options) (Manager, error) {
	return &stubManager{}, nil
}

func (m *stubManager) Start() error {
	return E.New("nftables is only supported on Linux")
}

func (m *stubManager) AddAddress(setName string, address netip.Addr, ttl time.Duration, reason string) error {
	return E.New("nftables is only supported on Linux")
}

func (m *stubManager) Close() error {
	return nil
}

func (m *stubManager) Flush() error {
	return E.New("nftables is only supported on Linux")
}
