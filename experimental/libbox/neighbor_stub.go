//go:build !linux && !darwin

package libbox

import "os"

func SubscribeNeighborTable(_ NeighborUpdateListener) (*NeighborSubscription, error) {
	return nil, os.ErrInvalid
}
