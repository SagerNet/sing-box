package httpclient

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type innerTransport interface {
	http.RoundTripper
	CloseIdleConnections()
	Close() error
}

var _ adapter.HTTPTransport = (*ManagedTransport)(nil)

type ManagedTransport struct {
	epoch         atomic.Pointer[transportEpoch]
	rebuildAccess sync.Mutex
	factory       func() (innerTransport, error)
	cheapRebuild  bool

	dialer  N.Dialer
	headers http.Header
	host    string
	tag     string
}

type transportEpoch struct {
	transport innerTransport
	active    atomic.Int64
	marked    atomic.Bool
	closeOnce sync.Once
}

type managedResponseBody struct {
	body    io.ReadCloser
	release func()
	once    sync.Once
}

func (e *transportEpoch) tryClose() {
	e.closeOnce.Do(func() {
		e.transport.Close()
	})
}

func (b *managedResponseBody) Read(p []byte) (int, error) {
	return b.body.Read(p)
}

func (b *managedResponseBody) Close() error {
	err := b.body.Close()
	b.once.Do(b.release)
	return err
}

func (t *ManagedTransport) getEpoch() (*transportEpoch, error) {
	epoch := t.epoch.Load()
	if epoch != nil {
		return epoch, nil
	}
	t.rebuildAccess.Lock()
	defer t.rebuildAccess.Unlock()
	epoch = t.epoch.Load()
	if epoch != nil {
		return epoch, nil
	}
	inner, err := t.factory()
	if err != nil {
		return nil, err
	}
	epoch = &transportEpoch{transport: inner}
	t.epoch.Store(epoch)
	return epoch, nil
}

func (t *ManagedTransport) acquireEpoch() (*transportEpoch, error) {
	for {
		epoch, err := t.getEpoch()
		if err != nil {
			return nil, err
		}
		epoch.active.Add(1)
		if epoch == t.epoch.Load() {
			return epoch, nil
		}
		t.releaseEpoch(epoch)
	}
}

func (t *ManagedTransport) releaseEpoch(epoch *transportEpoch) {
	if epoch.active.Add(-1) == 0 && epoch.marked.Load() {
		epoch.tryClose()
	}
}

func (t *ManagedTransport) retireEpoch(epoch *transportEpoch) {
	if epoch == nil {
		return
	}
	epoch.marked.Store(true)
	if epoch.active.Load() == 0 {
		epoch.tryClose()
	}
}

func (t *ManagedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	epoch, err := t.acquireEpoch()
	if err != nil {
		return nil, E.Cause(err, "rebuild http transport")
	}
	if t.tag != "" {
		if transportTag, loaded := transportTagFromContext(request.Context()); loaded && transportTag == t.tag {
			t.releaseEpoch(epoch)
			return nil, E.New("HTTP request loopback in transport[", t.tag, "]")
		}
		request = request.Clone(contextWithTransportTag(request.Context(), t.tag))
	} else if len(t.headers) > 0 || t.host != "" {
		request = request.Clone(request.Context())
	}
	applyHeaders(request, t.headers, t.host)
	response, roundTripErr := epoch.transport.RoundTrip(request)
	if roundTripErr != nil || response == nil || response.Body == nil {
		t.releaseEpoch(epoch)
		return response, roundTripErr
	}
	response.Body = &managedResponseBody{
		body:    response.Body,
		release: func() { t.releaseEpoch(epoch) },
	}
	return response, roundTripErr
}

func (t *ManagedTransport) CloseIdleConnections() {
	oldEpoch := t.epoch.Swap(nil)
	if oldEpoch == nil {
		return
	}
	oldEpoch.transport.CloseIdleConnections()
	t.retireEpoch(oldEpoch)
}

func (t *ManagedTransport) Reset() {
	oldEpoch := t.epoch.Swap(nil)
	if t.cheapRebuild {
		t.rebuildAccess.Lock()
		if t.epoch.Load() == nil {
			inner, err := t.factory()
			if err == nil {
				t.epoch.Store(&transportEpoch{transport: inner})
			}
		}
		t.rebuildAccess.Unlock()
	}
	t.retireEpoch(oldEpoch)
}

func (t *ManagedTransport) close() error {
	epoch := t.epoch.Swap(nil)
	if epoch != nil {
		return epoch.transport.Close()
	}
	return nil
}

var _ adapter.HTTPTransport = (*sharedRef)(nil)

type sharedRef struct {
	managed *ManagedTransport
	shared  *sharedState
	idle    atomic.Bool
}

type sharedState struct {
	activeRefs atomic.Int32
}

func newSharedRef(managed *ManagedTransport, shared *sharedState) *sharedRef {
	shared.activeRefs.Add(1)
	return &sharedRef{
		managed: managed,
		shared:  shared,
	}
}

func (r *sharedRef) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.idle.CompareAndSwap(true, false) {
		r.shared.activeRefs.Add(1)
	}
	return r.managed.RoundTrip(request)
}

func (r *sharedRef) CloseIdleConnections() {
	if r.idle.CompareAndSwap(false, true) {
		if r.shared.activeRefs.Add(-1) == 0 {
			r.managed.CloseIdleConnections()
		}
	}
}

func (r *sharedRef) Reset() {
	r.managed.Reset()
}
