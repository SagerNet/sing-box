package trojan

import (
	"context"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/task"
	"github.com/sagernet/smux"
)

func HandleMuxConnection(ctx context.Context, conn net.Conn, metadata M.Metadata, handler Handler) error {
	session, err := smux.Server(conn, smuxConfig())
	if err != nil {
		return err
	}
	var group task.Group
	group.Append0(func(ctx context.Context) error {
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

func newMuxConnection(ctx context.Context, stream net.Conn, metadata M.Metadata, handler Handler) {
	err := newMuxConnection0(ctx, stream, metadata, handler)
	if err != nil {
		handler.NewError(ctx, E.Cause(err, "process trojan-go multiplex connection"))
	}
}

func newMuxConnection0(ctx context.Context, stream net.Conn, metadata M.Metadata, handler Handler) error {
	command, err := rw.ReadByte(stream)
	if err != nil {
		return E.Cause(err, "read command")
	}
	metadata.Destination, err = M.SocksaddrSerializer.ReadAddrPort(stream)
	if err != nil {
		return E.Cause(err, "read destination")
	}
	switch command {
	case CommandTCP:
		return handler.NewConnection(ctx, stream, metadata)
	case CommandUDP:
		return handler.NewPacketConnection(ctx, &PacketConn{stream}, metadata)
	default:
		return E.New("unknown command ", command)
	}
}

func smuxConfig() *smux.Config {
	config := smux.DefaultConfig()
	config.KeepAliveDisabled = true
	return config
}
