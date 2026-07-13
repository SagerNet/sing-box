package main

import (
	"net"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/tailscale/go-winio"
)

// libuv (Node net.connect) opens the client end with GENERIC_READ|GENERIC_WRITE and
// falls back to read-only/write-only opens on ERROR_ACCESS_DENIED (src/win/pipe.c,
// open_named_pipe), so the client principal needs the full GRGW grant.
const pipeSecurityDescriptor = `D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)`

// winio's PipeConfig defaults to zero-quota pipe instances, where NPFS pends every
// WriteFile until the peer posts a consuming ReadFile. Node's http2 client never
// posts one when it opens a pipe immediately after destroying a previous pipe
// socket (the renderer host's deadline-abort-then-retry pattern), which deadlocks
// the gRPC handshake on the server's initial SETTINGS write and surfaces in the
// app as a connect timeout. Nonzero quotas let the handshake complete into the
// pipe buffer.
const (
	pipeBufferSize = 65536
	daemonPipePath = `\\.\pipe\ProtectedPrefix\Administrators\sing-box`
)

func listenEndpoint() (net.Listener, error) {
	if socketPath != "" && !strings.EqualFold(socketPath, daemonPipePath) {
		return nil, E.New("custom Windows daemon pipe paths are not supported")
	}
	return winio.ListenPipe(daemonPipePath, &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
		InputBufferSize:    pipeBufferSize,
		OutputBufferSize:   pipeBufferSize,
	})
}
