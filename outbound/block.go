package outbound

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Outbound = (*Block)(nil)

type Block struct {
	myOutboundAdapter
}

func NewBlock(logger log.ContextLogger, tag string) *Block {
	return &Block{
		myOutboundAdapter{
			protocol: C.TypeBlock,
			network:  []string{N.NetworkTCP, N.NetworkUDP},
			logger:   logger,
			tag:      tag,
		},
	}
}

func (h *Block) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	h.logger.InfoContext(ctx, "blocked connection to ", destination)
	return nil, io.EOF
}

func (h *Block) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	h.logger.InfoContext(ctx, "blocked packet connection to ", destination)
	return nil, io.EOF
}

func (h *Block) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	conn.Close()
	h.logger.InfoContext(ctx, "blocked connection to ", metadata.Destination)
	return nil
}

func (h *Block) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	h.logger.InfoContext(ctx, "blocked packet connection to ", metadata.Destination)
	writer := &discardPacketWriter{
		timer: time.AfterFunc(C.UDPTimeout, func() {
			_ = conn.Close()
		}),
	}
	_, _ = bufio.CopyPacket(writer, conn)
	return nil
}

type discardPacketWriter struct {
	timer *time.Timer
}

func (w *discardPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if w.timer.Stop() {
		w.timer.Reset(C.UDPTimeout)
	}
	buffer.Release()
	return nil
}
