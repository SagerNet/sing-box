package v2rayhttp

import (
	"net/http"
	"reflect"
	"unsafe"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/net/http2"
)

type efaceWords struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

func ResetTransport(rawTransport http.RoundTripper) http.RoundTripper {
	switch transport := rawTransport.(type) {
	case *http.Transport:
		transport.CloseIdleConnections()
		return transport.Clone()
	case *http2.Transport:
		closeHTTP2Connections(transport)
		return transport
	default:
		panic(E.New("unknown transport type: ", reflect.TypeOf(transport)))
	}
}
