package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

func TestUDPNatClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connCtx, connCancel := common.ContextWithCancelCause(context.Background())
	defer connCancel(net.ErrClosed)
	service := udpnat.New[int](1, &testUDPNatCloseHandler{connCancel})
	service.NewPacket(ctx, 0, buf.As([]byte("Hello")), M.Metadata{}, func(natConn N.PacketConn) N.PacketWriter {
		return &testPacketWriter{}
	})
	select {
	case <-connCtx.Done():
		if E.IsClosed(connCtx.Err()) {
			t.Fatal(E.New("conn closed unexpectedly: ", connCtx.Err()))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("conn not closed")
	}
}

type testUDPNatCloseHandler struct {
	done common.ContextCancelCauseFunc
}

func (h *testUDPNatCloseHandler) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	for {
		buffer := buf.NewPacket()
		_, err := conn.ReadPacket(buffer)
		buffer.Release()
		if err != nil {
			h.done(err)
			return err
		}
	}
}

func (h *testUDPNatCloseHandler) NewError(ctx context.Context, err error) {
}

type testPacketWriter struct{}

func (t *testPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	return nil
}
