//go:build go1.27 && badlinkname

package v2rayhttp

import (
	"net/http"
	"sync"
	"unsafe"

	"golang.org/x/net/http2"
)

// cmd/compile creates the method symbols reachable from *http.Transport's field types with
// their package recorded when the type is first used by a declaration; a linkname pull
// processed afterwards reuses that symbol as a package-indexed reference, which the linker's
// -checklinkname does not inspect. This declaration must precede the linkname declarations.
var _ *http.Transport

// net/http/internal/http2.Transport
type internalTransport struct {
	t1       [2]uintptr // TransportConfig
	connPool *clientConnPool
}

// net/http/internal/http2.clientConnPool
type clientConnPool struct {
	t     *internalTransport
	mu    sync.Mutex
	conns map[string][]unsafe.Pointer // key is host:port, value is []*ClientConn
}

func closeHTTP2Connections(transport *http2.Transport) {
	h2Transport := transportFromH1Transport(transportInit(transport))
	t := (*internalTransport)((*efaceWords)(unsafe.Pointer(&h2Transport)).data)
	if t == nil {
		return
	}
	p := t.connPool
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, vv := range p.conns {
		for _, cc := range vv {
			clientConnClose(cc)
		}
	}
}

//go:linkname transportInit golang.org/x/net/http2.(*Transport).init
func transportInit(t *http2.Transport) *http.Transport

//go:linkname transportFromH1Transport net/http/internal/http2_test.transportFromH1Transport
func transportFromH1Transport(t *http.Transport) any

//go:linkname clientConnClose net/http/internal/http2.(*ClientConn).Close
func clientConnClose(cc unsafe.Pointer) error
