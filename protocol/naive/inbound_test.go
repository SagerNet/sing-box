package naive

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	inboundAdapter "github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/common/listener"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/stretchr/testify/require"
)

type naiveTestRouter struct {
	adapter.ConnectionRouterEx
	connection chan net.Conn
	onClose    chan N.CloseHandlerFunc
}

func (r *naiveTestRouter) RouteConnectionEx(_ context.Context, conn net.Conn, _ adapter.InboundContext, onClose N.CloseHandlerFunc) {
	r.connection <- conn
	r.onClose <- onClose
}

type naiveTestBlockingWriteConn struct {
	net.Conn
	writeStarted chan struct{}
	writeRelease chan struct{}
}

func (c *naiveTestBlockingWriteConn) Write(p []byte) (n int, err error) {
	c.writeStarted <- struct{}{}
	<-c.writeRelease
	return len(p), nil
}

func TestNewConnectionRoutesHTTP2Wrapper(t *testing.T) {
	t.Parallel()
	router := &naiveTestRouter{
		connection: make(chan net.Conn),
		onClose:    make(chan N.CloseHandlerFunc),
	}
	rawConn, peerConn := net.Pipe()
	defer rawConn.Close()
	defer peerConn.Close()
	blockingConn := &naiveTestBlockingWriteConn{
		Conn:         rawConn,
		writeStarted: make(chan struct{}, 1),
		writeRelease: make(chan struct{}),
	}
	defer func() {
		select {
		case <-blockingConn.writeRelease:
		default:
			close(blockingConn.writeRelease)
		}
	}()
	inbound := &Inbound{
		Adapter:  inboundAdapter.NewAdapter(C.TypeNaive, "test"),
		router:   router,
		logger:   log.NewNOPFactory().NewLogger("naive"),
		listener: listener.New(listener.Options{}),
	}
	returned := make(chan struct{})
	go func() {
		inbound.newConnection(context.Background(), true, blockingConn, "", M.Socksaddr{}, M.Socksaddr{})
		close(returned)
	}()

	routedConn := <-router.connection
	onClose := <-router.onClose
	defer onClose(nil)
	require.IsType(t, &v2rayhttp.HTTP2ConnWrapper{}, routedConn)

	writeDone := make(chan error, 1)
	go func() {
		_, err := routedConn.Write([]byte("response"))
		writeDone <- err
	}()
	select {
	case <-blockingConn.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("routed write did not reach the underlying connection")
	}

	onClose(nil)
	select {
	case <-returned:
		t.Fatal("newConnection returned with a write in progress")
	case <-time.After(100 * time.Millisecond):
	}
	close(blockingConn.writeRelease)
	select {
	case err := <-writeDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("routed write did not finish")
	}
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("newConnection did not return after the route closed")
	}
	_, err := routedConn.Write([]byte("late response"))
	require.ErrorIs(t, err, net.ErrClosed)
}
