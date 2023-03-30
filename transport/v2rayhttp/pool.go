package v2rayhttp

import "net/http"

type ConnectionPool interface {
	CloseIdleConnections()
}

func CloseIdleConnections(transport http.RoundTripper) {
	if connectionPool, ok := transport.(ConnectionPool); ok {
		connectionPool.CloseIdleConnections()
	}
}
