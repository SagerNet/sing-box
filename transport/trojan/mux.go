package trojan

import (
	std_bufio "bufio"
	"context"
	"net"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/smux"
)

func HandleMuxConnection(ctx context.Context, conn net.Conn, metadata M.Metadata, handler Handler) error {
	session, err := smux.Server(conn, smuxConfig())
	if err != nil {
		return err
	}
	var group task.Group
	group.Append0(func(_ context.Context) error {
		var stream net.Conn
		for {
			stream, err = session.AcceptStream()
			if err != nil {
				return err
			}
			go newMuxConnection(ctx, stream, metadata, handler)
		}
	})
	group.Cleanup(func() {
		session.Close()
	})
	return group.Run(ctx)
}

func newMuxConnection(ctx context.Context, conn net.Conn, metadata M.Metadata, handler Handler) {
	err := newMuxConnection0(ctx, conn, metadata, handler)
	if err != nil {
		handler.NewError(ctx, E.Cause(err, "process trojan-go multiplex connection"))
	}
}

func newMuxConnection0(ctx context.Context, conn net.Conn, metadata M.Metadata, handler Handler) error {
	reader := std_bufio.NewReader(conn)
	command, err := reader.ReadByte()
	if err != nil {
		return E.Cause(err, "read command")
	}
	metadata.Destination, err = M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return E.Cause(err, "read destination")
	}
	if reader.Buffered() > 0 {
		buffer := buf.NewSize(reader.Buffered())
		_, err = buffer.ReadFullFrom(reader, buffer.Len())
		if err != nil {
			return err
		}
		conn = bufio.NewCachedConn(conn, buffer)
	}
	switch command {
	case CommandTCP:
		return handler.NewConnection(ctx, conn, metadata)
	case CommandUDP:
		return handler.NewPacketConnection(ctx, &PacketConn{Conn: conn}, metadata)
	default:
		return E.New("unknown command ", command)
	}
}

func smuxConfig() *smux.Config {
	config := smux.DefaultConfig()
	config.KeepAliveDisabled = true
	return config
}
