package outbound

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/protocol/socks"

	"github.com/cretz/bine/control"
	"github.com/cretz/bine/tor"
)

var _ adapter.Outbound = (*Tor)(nil)

type Tor struct {
	myOutboundAdapter
	ctx         context.Context
	proxy       *ProxyListener
	startConf   *tor.StartConf
	options     map[string]string
	events      chan control.Event
	instance    *tor.Tor
	socksClient *socks.Client
}

func NewTor(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, options option.TorOutboundOptions) (*Tor, error) {
	startConf := newConfig()
	startConf.DataDir = os.ExpandEnv(options.DataDirectory)
	startConf.TempDataDirBase = os.TempDir()
	startConf.ExtraArgs = options.ExtraArgs
	if options.DataDirectory != "" {
		dataDirAbs, _ := filepath.Abs(startConf.DataDir)
		if geoIPPath := filepath.Join(dataDirAbs, "geoip"); rw.FileExists(geoIPPath) && !common.Contains(options.ExtraArgs, "--GeoIPFile") {
			options.ExtraArgs = append(options.ExtraArgs, "--GeoIPFile", geoIPPath)
		}
		if geoIP6Path := filepath.Join(dataDirAbs, "geoip6"); rw.FileExists(geoIP6Path) && !common.Contains(options.ExtraArgs, "--GeoIPv6File") {
			options.ExtraArgs = append(options.ExtraArgs, "--GeoIPv6File", geoIP6Path)
		}
	}
	if options.ExecutablePath != "" {
		startConf.ExePath = options.ExecutablePath
		startConf.ProcessCreator = nil
		startConf.UseEmbeddedControlConn = false
	}
	if startConf.DataDir != "" {
		torrcFile := filepath.Join(startConf.DataDir, "torrc")
		if !rw.FileExists(torrcFile) {
			err := rw.WriteFile(torrcFile, []byte(""))
			if err != nil {
				return nil, err
			}
		}
		startConf.TorrcFile = torrcFile
	}
	outboundDialer, err := dialer.New(router, options.DialerOptions)
	if err != nil {
		return nil, err
	}
	return &Tor{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeTor,
			network:      []string{N.NetworkTCP},
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: withDialerDependency(options.DialerOptions),
		},
		ctx:       ctx,
		proxy:     NewProxyListener(ctx, logger, outboundDialer),
		startConf: &startConf,
		options:   options.Options,
	}, nil
}

func (t *Tor) Start() error {
	err := t.start()
	if err != nil {
		t.Close()
	}
	return err
}

var torLogEvents = []control.EventCode{
	control.EventCodeLogDebug,
	control.EventCodeLogErr,
	control.EventCodeLogInfo,
	control.EventCodeLogNotice,
	control.EventCodeLogWarn,
}

func (t *Tor) start() error {
	torInstance, err := tor.Start(t.ctx, t.startConf)
	if err != nil {
		return E.New(strings.ToLower(err.Error()))
	}
	t.instance = torInstance
	t.events = make(chan control.Event, 8)
	err = torInstance.Control.AddEventListener(t.events, torLogEvents...)
	if err != nil {
		return err
	}
	go t.recvLoop()
	err = t.proxy.Start()
	if err != nil {
		return err
	}
	proxyPort := "127.0.0.1:" + F.ToString(t.proxy.Port())
	proxyUsername := t.proxy.Username()
	proxyPassword := t.proxy.Password()
	t.logger.Trace("created upstream proxy at ", proxyPort)
	t.logger.Trace("upstream proxy username ", proxyUsername)
	t.logger.Trace("upstream proxy password ", proxyPassword)
	confOptions := []*control.KeyVal{
		control.NewKeyVal("Socks5Proxy", proxyPort),
		control.NewKeyVal("Socks5ProxyUsername", proxyUsername),
		control.NewKeyVal("Socks5ProxyPassword", proxyPassword),
	}
	err = torInstance.Control.ResetConf(confOptions...)
	if err != nil {
		return err
	}
	if len(t.options) > 0 {
		for key, value := range t.options {
			switch key {
			case "Socks5Proxy",
				"Socks5ProxyUsername",
				"Socks5ProxyPassword":
				continue
			}
			err = torInstance.Control.SetConf(control.NewKeyVal(key, value))
			if err != nil {
				return E.Cause(err, "set ", key, "=", value)
			}
		}
	}
	err = torInstance.EnableNetwork(t.ctx, true)
	if err != nil {
		return err
	}
	info, err := torInstance.Control.GetInfo("net/listeners/socks")
	if err != nil {
		return err
	}
	if len(info) != 1 || info[0].Key != "net/listeners/socks" {
		return E.New("get socks proxy address")
	}
	t.logger.Trace("obtained tor socks5 address ", info[0].Val)
	// TODO: set password for tor socks5 server if supported
	t.socksClient = socks.NewClient(N.SystemDialer, M.ParseSocksaddr(info[0].Val), socks.Version5, "", "")
	return nil
}

func (t *Tor) recvLoop() {
	for rawEvent := range t.events {
		switch event := rawEvent.(type) {
		case *control.LogEvent:
			event.Raw = strings.ToLower(event.Raw)
			switch event.Severity {
			case control.EventCodeLogDebug, control.EventCodeLogInfo:
				t.logger.Trace(event.Raw)
			case control.EventCodeLogNotice:
				if strings.Contains(event.Raw, "disablenetwork") || strings.Contains(event.Raw, "socks listener") {
					t.logger.Trace(event.Raw)
					continue
				}
				t.logger.Info(event.Raw)
			case control.EventCodeLogWarn:
				t.logger.Warn(event.Raw)
			case control.EventCodeLogErr:
				t.logger.Error(event.Raw)
			}
		}
	}
}

func (t *Tor) Close() error {
	err := common.Close(
		common.PtrOrNil(t.proxy),
		common.PtrOrNil(t.instance),
	)
	if t.events != nil {
		close(t.events)
		t.events = nil
	}
	return err
}

func (t *Tor) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	t.logger.InfoContext(ctx, "outbound connection to ", destination)
	return t.socksClient.DialContext(ctx, network, destination)
}

func (t *Tor) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

func (t *Tor) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return NewConnection(ctx, t, conn, metadata)
}

func (t *Tor) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return os.ErrInvalid
}
