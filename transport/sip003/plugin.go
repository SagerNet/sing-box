package sip003

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type PluginConstructor func(pluginArgs Args, router adapter.Router, dialer N.Dialer, serverAddr M.Socksaddr) (Plugin, error)

type Plugin interface {
	DialContext(ctx context.Context) (net.Conn, error)
}

var plugins map[string]PluginConstructor

func RegisterPlugin(name string, constructor PluginConstructor) {
	plugins[name] = constructor
}

func CreatePlugin(name string, pluginArgs string, router adapter.Router, dialer N.Dialer, serverAddr M.Socksaddr) (Plugin, error) {
	pluginOptions, err := ParsePluginOptions(pluginArgs)
	if err != nil {
		return nil, E.Cause(err, "parse plugin_opts")
	}
	constructor, loaded := plugins[name]
	if !loaded {
		return nil, E.New("plugin not found: ", name)
	}
	return constructor(pluginOptions, router, dialer, serverAddr)
}
