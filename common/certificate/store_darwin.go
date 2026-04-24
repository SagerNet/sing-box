//go:build darwin && cgo

package certificate

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework Security

#include <stdlib.h>
#include "anchors_darwin.h"
*/
import "C"

import (
	"crypto/sha256"
	"encoding/pem"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	_ adapter.AppleCertificateStore = (*Store)(nil)
	_ adapter.AppleAnchors          = (*appleAnchors)(nil)
)

type storePlatform struct {
	anchors *appleAnchors
	hash    [sha256.Size]byte
}

type appleAnchors struct {
	cfArray unsafe.Pointer
	refs    atomic.Int32
}

func newAppleAnchors(pemBytes []byte) (*appleAnchors, error) {
	anchors := &appleAnchors{}
	anchors.refs.Store(1)
	if len(pemBytes) == 0 {
		return anchors, nil
	}
	derBlocks := decodeCertificatePEM(pemBytes)
	if len(derBlocks) == 0 {
		return nil, E.New("parse certificate PEM")
	}
	pointerSize := C.size_t(unsafe.Sizeof((*C.uint8_t)(nil)))
	lenSize := C.size_t(unsafe.Sizeof(C.size_t(0)))
	pointersC := (**C.uint8_t)(C.malloc(pointerSize * C.size_t(len(derBlocks))))
	defer C.free(unsafe.Pointer(pointersC))
	lensC := (*C.size_t)(C.malloc(lenSize * C.size_t(len(derBlocks))))
	defer C.free(unsafe.Pointer(lensC))
	pointersSlice := unsafe.Slice(pointersC, len(derBlocks))
	lensSlice := unsafe.Slice(lensC, len(derBlocks))
	var pinner runtime.Pinner
	defer pinner.Unpin()
	for index, der := range derBlocks {
		pinner.Pin(&der[0])
		pointersSlice[index] = (*C.uint8_t)(unsafe.Pointer(&der[0]))
		lensSlice[index] = C.size_t(len(der))
	}
	cfArray := C.box_certificate_anchors_from_der(pointersC, lensC, C.size_t(len(derBlocks)))
	if cfArray == nil {
		return nil, E.New("parse certificate PEM")
	}
	anchors.cfArray = cfArray
	return anchors, nil
}

// NewAppleAnchors parses the given PEM and returns a ref-counted handle
// wrapping a CFArray of SecCertificateRef. The caller owns the returned
// reference and must call Release when finished. Returns an error when
// pemBytes is non-empty but contains no usable CERTIFICATE blocks.
func NewAppleAnchors(pemBytes []byte) (adapter.AppleAnchors, error) {
	return newAppleAnchors(pemBytes)
}

// AcquireAnchors returns a retained AppleAnchors handle, preferring the
// per-config userAnchors over the process-wide certificate store. Returns
// nil when neither source is available. Callers must Release the handle.
func AcquireAnchors(userAnchors adapter.AppleAnchors, store adapter.CertificateStore) adapter.AppleAnchors {
	if userAnchors != nil {
		return userAnchors.Retain()
	}
	if store == nil {
		return nil
	}
	apple, loaded := store.(adapter.AppleCertificateStore)
	if !loaded {
		return nil
	}
	return apple.AppleAnchors()
}

func (a *appleAnchors) Retain() adapter.AppleAnchors {
	a.refs.Add(1)
	return a
}

func (a *appleAnchors) Release() {
	if a.refs.Add(-1) != 0 {
		return
	}
	if a.cfArray != nil {
		C.box_certificate_release_anchors(a.cfArray)
	}
}

func (a *appleAnchors) Ref() unsafe.Pointer {
	return a.cfArray
}

func (s *Store) AppleAnchors() adapter.AppleAnchors {
	s.access.RLock()
	defer s.access.RUnlock()
	if s.platform.anchors == nil {
		return nil
	}
	return s.platform.anchors.Retain()
}

func (s *Store) updatePlatformLocked(pemBytes []byte) error {
	hash := sha256.Sum256(pemBytes)
	if s.platform.anchors != nil && s.platform.hash == hash {
		return nil
	}
	newAnchors, err := newAppleAnchors(pemBytes)
	if err != nil {
		return err
	}
	old := s.platform.anchors
	s.platform.anchors = newAnchors
	s.platform.hash = hash
	if old != nil {
		old.Release()
	}
	return nil
}

func (s *Store) closePlatform() error {
	s.access.Lock()
	defer s.access.Unlock()
	if s.platform.anchors != nil {
		s.platform.anchors.Release()
		s.platform.anchors = nil
	}
	return nil
}

func decodeCertificatePEM(pemBytes []byte) [][]byte {
	var blocks [][]byte
	rest := pemBytes
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" && len(block.Bytes) > 0 {
			blocks = append(blocks, block.Bytes)
		}
		rest = next
	}
	return blocks
}
