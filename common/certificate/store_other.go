//go:build !(darwin && cgo)

package certificate

type storePlatform struct{}

func (s *Store) updatePlatformLocked(_ []byte) error {
	return nil
}

func (s *Store) closePlatform() error {
	return nil
}
