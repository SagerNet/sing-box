//go:build !go1.27

package v2rayhttp

import (
	"sync"
	"unsafe"

	"golang.org/x/net/http2"
)

type clientConnPool struct {
	t     *http2.Transport
	mu    sync.Mutex
	conns map[string][]*http2.ClientConn // key is host:port
}

func closeHTTP2Connections(transport *http2.Transport) {
	connPool := transportConnPool(transport)
	p := (*clientConnPool)((*efaceWords)(unsafe.Pointer(&connPool)).data)
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, vv := range p.conns {
		for _, cc := range vv {
			cc.Close()
		}
	}
}

//go:linkname transportConnPool golang.org/x/net/http2.(*Transport).connPool
func transportConnPool(t *http2.Transport) http2.ClientConnPool
