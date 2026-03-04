//go:build !linux

package libbox

import "os"

type NeighborEntry struct {
	Address    string
	MACAddress string
	Hostname   string
}

type NeighborEntryIterator interface {
	Next() *NeighborEntry
	HasNext() bool
}

type NeighborSubscription struct{}

func SubscribeNeighborTable(listener NeighborUpdateListener) (*NeighborSubscription, error) {
	return nil, os.ErrInvalid
}

func (s *NeighborSubscription) Close() {}
