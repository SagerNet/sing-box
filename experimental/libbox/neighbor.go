package libbox

type NeighborEntry struct {
	Address    string
	MacAddress string
	Hostname   string
}

type NeighborEntryIterator interface {
	Next() *NeighborEntry
	HasNext() bool
}

type NeighborSubscription struct {
	done chan struct{}
}

func (s *NeighborSubscription) Close() {
	close(s.done)
}
