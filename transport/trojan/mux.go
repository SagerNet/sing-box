package trojan

import (
	std_bufio "bufio"
	"context"
	"net"
	"os"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/smux"
)

func HandleMuxConnection(ctx context.Context, conn net.Conn, source M.Socksaddr, handler Handler, logger logger.ContextLogger, onClose N.CloseHandlerFunc) error {
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
			go newMuxConnection(ctx, stream, source, handler, logger)
		}
	})
	group.Cleanup(func() {
		session.Close()
		if onClose != nil {
			onClose(os.ErrClosed)
		}
	})
	return group.Run(ctx)
}

func newMuxConnection(ctx context.Context, conn net.Conn, source M.Socksaddr, handler Handler, logger logger.ContextLogger) {
	err := newMuxConnection0(ctx, conn, source, handler)
	if err != nil {
		logger.ErrorContext(ctx, E.Cause(err, "process trojan-go multiplex connection"))
	}
}

func newMuxConnection0(ctx context.Context, conn net.Conn, source M.Socksaddr, handler Handler) error {
	reader := std_bufio.NewReader(conn)
	command, err := reader.ReadByte()
	if err != nil {
		return E.Cause(err, "read command")
	}
	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
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
		handler.NewConnectionEx(ctx, conn, source, destination, nil)
	case CommandUDP:
		handler.NewPacketConnectionEx(ctx, &PacketConn{Conn: conn}, source, destination, nil)
	default:
		return E.New("unknown command ", command)
	}
	return nil
}

func smuxConfig() *smux.Config {
	config := smux.DefaultConfig()
	config.KeepAliveDisabled = true
	return config
}
