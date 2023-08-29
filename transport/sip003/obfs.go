package sip003

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/transport/simple-obfs"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ Plugin = (*ObfsLocal)(nil)

func init() {
	RegisterPlugin("obfs-local", newObfsLocal)
}

func newObfsLocal(ctx context.Context, pluginOpts Args, router adapter.Router, dialer N.Dialer, serverAddr M.Socksaddr) (Plugin, error) {
	plugin := &ObfsLocal{
		dialer:     dialer,
		serverAddr: serverAddr,
	}
	mode := "http"
	if obfsMode, loaded := pluginOpts.Get("obfs"); loaded {
		mode = obfsMode
	}
	if obfsHost, loaded := pluginOpts.Get("obfs-host"); loaded {
		plugin.host = obfsHost
	}
	switch mode {
	case "http":
	case "tls":
		plugin.tls = true
	default:
		return nil, E.New("unknown obfs mode ", mode)
	}
	plugin.port = F.ToString(serverAddr.Port)
	return plugin, nil
}

type ObfsLocal struct {
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tls        bool
	host       string
	port       string
}

func (o *ObfsLocal) DialContext(ctx context.Context) (net.Conn, error) {
	conn, err := o.dialer.DialContext(ctx, N.NetworkTCP, o.serverAddr)
	if err != nil {
		return nil, err
	}
	if !o.tls {
		return obfs.NewHTTPObfs(conn, o.host, o.port), nil
	} else {
		return obfs.NewTLSObfs(conn, o.host), nil
	}
}
