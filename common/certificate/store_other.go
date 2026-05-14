//go:build !(darwin && cgo)

package certificate

//nolint:unused // referenced by Store.platform; populated only in store_darwin.go.
type storePlatform struct{}

func (s *Store) updatePlatformLocked(_ []byte) error {
	return nil
}

func (s *Store) closePlatform() error {
	return nil
}
