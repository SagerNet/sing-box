package derp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/listener"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	boxScale "github.com/sagernet/sing-box/protocol/tailscale"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
	"github.com/sagernet/tailscale/client/local"
	"github.com/sagernet/tailscale/derp"
	"github.com/sagernet/tailscale/derp/derphttp"
	"github.com/sagernet/tailscale/net/netmon"
	"github.com/sagernet/tailscale/net/stun"
	"github.com/sagernet/tailscale/net/wsconn"
	"github.com/sagernet/tailscale/tsweb"
	"github.com/sagernet/tailscale/types/key"

	"github.com/coder/websocket"
	"github.com/go-chi/render"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func Register(registry *boxService.Registry) {
	boxService.Register[option.DERPServiceOptions](registry, C.TypeDERP, NewService)
}

type Service struct {
	boxService.Adapter
	ctx                  context.Context
	logger               logger.ContextLogger
	listener             *listener.Listener
	stunListener         *listener.Listener
	tlsConfig            tls.ServerConfig
	server               *derp.Server
	configPath           string
	verifyClientEndpoint []string
	verifyClientURL      []*option.DERPVerifyClientURLOptions
	home                 string
	meshKey              string
	meshKeyPath          string
	meshWith             []*option.DERPMeshOptions
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.DERPServiceOptions) (adapter.Service, error) {
	if options.TLS == nil || !options.TLS.Enabled {
		return nil, E.New("TLS is required for DERP server")
	}
	tlsConfig, err := tls.NewServer(ctx, logger, common.PtrValueOrDefault(options.TLS))
	if err != nil {
		return nil, err
	}

	var configPath string
	if options.ConfigPath != "" {
		configPath = filemanager.BasePath(ctx, os.ExpandEnv(options.ConfigPath))
	} else {
		return nil, E.New("missing config_path")
	}

	if options.MeshPSK != "" {
		err = checkMeshKey(options.MeshPSK)
		if err != nil {
			return nil, E.Cause(err, "invalid mesh_psk")
		}
	}

	var stunListener *listener.Listener
	if options.STUN != nil && options.STUN.Enabled {
		if options.STUN.Listen == nil {
			options.STUN.Listen = (*badoption.Addr)(common.Ptr(netip.IPv6Unspecified()))
		}
		if options.STUN.ListenPort == 0 {
			options.STUN.ListenPort = 3478
		}
		stunListener = listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkUDP},
			Listen:  options.STUN.ListenOptions,
		})
	}

	return &Service{
		Adapter: boxService.NewAdapter(C.TypeDERP, tag),
		ctx:     ctx,
		logger:  logger,
		listener: listener.New(listener.Options{
			Context: ctx,
			Logger:  logger,
			Network: []string{N.NetworkTCP},
			Listen:  options.ListenOptions,
		}),
		stunListener:         stunListener,
		tlsConfig:            tlsConfig,
		configPath:           configPath,
		verifyClientEndpoint: options.VerifyClientEndpoint,
		verifyClientURL:      options.VerifyClientURL,
		home:                 options.Home,
		meshKey:              options.MeshPSK,
		meshKeyPath:          options.MeshPSKFile,
		meshWith:             options.MeshWith,
	}, nil
}

func (d *Service) Start(stage adapter.StartStage) error {
	switch stage {
	case adapter.StartStateStart:
		config, err := readDERPConfig(filemanager.BasePath(d.ctx, d.configPath))
		if err != nil {
			return err
		}

		server := derp.NewServer(config.PrivateKey, func(format string, args ...any) {
			d.logger.Debug(fmt.Sprintf(format, args...))
		})

		if len(d.verifyClientURL) > 0 {
			var httpClients []*http.Client
			var urls []string
			for index, options := range d.verifyClientURL {
				verifyDialer, createErr := dialer.NewWithOptions(dialer.Options{
					Context:        d.ctx,
					Options:        options.DialerOptions,
					RemoteIsDomain: options.ServerIsDomain(),
					NewDialer:      true,
				})
				if createErr != nil {
					return E.Cause(createErr, "verify_client_url[", index, "]")
				}
				httpClients = append(httpClients, &http.Client{
					Transport: &http.Transport{
						ForceAttemptHTTP2: true,
						DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
							return verifyDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
						},
					},
				})
				urls = append(urls, options.URL)
			}
			server.SetVerifyClientHTTPClient(httpClients)
			server.SetVerifyClientURL(urls)
		}

		if d.meshKey != "" {
			server.SetMeshKey(d.meshKey)
		} else if d.meshKeyPath != "" {
			var meshKeyContent []byte
			meshKeyContent, err = os.ReadFile(d.meshKeyPath)
			if err != nil {
				return err
			}
			err = checkMeshKey(string(meshKeyContent))
			if err != nil {
				return E.Cause(err, "invalid mesh_psk_path file")
			}
			server.SetMeshKey(string(meshKeyContent))
		}
		d.server = server

		derpMux := http.NewServeMux()
		derpHandler := derphttp.Handler(server)
		derpHandler = addWebSocketSupport(server, derpHandler)
		derpMux.Handle("/derp", derpHandler)

		homeHandler, ok := getHomeHandler(d.home)
		if !ok {
			return E.New("invalid home value: ", d.home)
		}

		derpMux.HandleFunc("/derp/probe", derphttp.ProbeHandler)
		derpMux.HandleFunc("/derp/latency-check", derphttp.ProbeHandler)
		derpMux.HandleFunc("/bootstrap-dns", tsweb.BrowserHeaderHandlerFunc(handleBootstrapDNS(d.ctx)))
		derpMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tsweb.AddBrowserHeaders(w)
			homeHandler.ServeHTTP(w, r)
		}))
		derpMux.Handle("/robots.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tsweb.AddBrowserHeaders(w)
			io.WriteString(w, "User-agent: *\nDisallow: /\n")
		}))
		derpMux.Handle("/generate_204", http.HandlerFunc(derphttp.ServeNoContent))

		err = d.tlsConfig.Start()
		if err != nil {
			return err
		}

		tcpListener, err := d.listener.ListenTCP()
		if err != nil {
			return err
		}
		if len(d.tlsConfig.NextProtos()) == 0 {
			d.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		} else if !common.Contains(d.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			d.tlsConfig.SetNextProtos(append([]string{http2.NextProtoTLS}, d.tlsConfig.NextProtos()...))
		}
		tcpListener = aTLS.NewListener(tcpListener, d.tlsConfig)
		httpServer := &http.Server{
			Handler: h2c.NewHandler(derpMux, &http2.Server{}),
		}
		go httpServer.Serve(tcpListener)

		if d.stunListener != nil {
			stunConn, err := d.stunListener.ListenUDP()
			if err != nil {
				return err
			}
			go d.loopSTUNPacket(stunConn.(*net.UDPConn))
		}
	case adapter.StartStatePostStart:
		if len(d.verifyClientEndpoint) > 0 {
			var endpoints []*local.Client
			endpointManager := service.FromContext[adapter.EndpointManager](d.ctx)
			for _, endpointTag := range d.verifyClientEndpoint {
				endpoint, loaded := endpointManager.Get(endpointTag)
				if !loaded {
					return E.New("verify_client_endpoint: endpoint not found: ", endpointTag)
				}
				tsEndpoint, isTailscale := endpoint.(*boxScale.Endpoint)
				if !isTailscale {
					return E.New("verify_client_endpoint: endpoint is not Tailscale: ", endpointTag)
				}
				localClient, err := tsEndpoint.Server().LocalClient()
				if err != nil {
					return err
				}
				endpoints = append(endpoints, localClient)
			}
			d.server.SetVerifyClientLocalClient(endpoints)
		}
		if len(d.meshWith) > 0 {
			if !d.server.HasMeshKey() {
				return E.New("missing mesh psk")
			}
			for _, options := range d.meshWith {
				err := d.startMeshWithHost(d.server, options)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkMeshKey(meshKey string) error {
	checkRegex, err := regexp.Compile(`^[0-9a-f]{64}$`)
	if err != nil {
		return err
	}
	if !checkRegex.MatchString(meshKey) {
		return E.New("key must contain exactly 64 hex digits")
	}
	return nil
}

func (d *Service) startMeshWithHost(derpServer *derp.Server, server *option.DERPMeshOptions) error {
	meshDialer, err := dialer.NewWithOptions(dialer.Options{
		Context:        d.ctx,
		Options:        server.DialerOptions,
		RemoteIsDomain: server.ServerIsDomain(),
		NewDialer:      true,
	})
	if err != nil {
		return err
	}
	var hostname string
	if server.Host != "" {
		hostname = server.Host
	} else {
		hostname = server.Server
	}
	var stdConfig *tls.STDConfig
	if server.TLS != nil && server.TLS.Enabled {
		tlsConfig, err := tls.NewClient(d.ctx, d.logger, hostname, common.PtrValueOrDefault(server.TLS))
		if err != nil {
			return err
		}
		stdConfig, err = tlsConfig.STDConfig()
		if err != nil {
			return err
		}
	}
	logf := func(format string, args ...any) {
		d.logger.Debug(F.ToString("mesh(", hostname, "): ", fmt.Sprintf(format, args...)))
	}
	var meshHost string
	if server.ServerPort == 0 || server.ServerPort == 443 {
		meshHost = hostname
	} else {
		meshHost = M.ParseSocksaddrHostPort(hostname, server.ServerPort).String()
	}
	var serverURL string
	if stdConfig != nil {
		serverURL = "https://" + meshHost + "/derp"
	} else {
		serverURL = "http://" + meshHost + "/derp"
	}
	meshClient, err := derphttp.NewClient(derpServer.PrivateKey(), serverURL, logf, netmon.NewStatic())
	if err != nil {
		return err
	}
	meshClient.TLSConfig = stdConfig
	meshClient.MeshKey = derpServer.MeshKey()
	meshClient.WatchConnectionChanges = true
	meshClient.SetURLDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return meshDialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
	})
	add := func(m derp.PeerPresentMessage) { derpServer.AddPacketForwarder(m.Key, meshClient) }
	remove := func(m derp.PeerGoneMessage) { derpServer.RemovePacketForwarder(m.Peer, meshClient) }
	notifyError := func(err error) { d.logger.Error(err) }
	go meshClient.RunWatchConnectionLoop(context.Background(), derpServer.PublicKey(), logf, add, remove, notifyError)
	return nil
}

func (d *Service) Close() error {
	return common.Close(
		common.PtrOrNil(d.listener),
		d.tlsConfig,
	)
}

var homePage = `
<h1>DERP</h1>
<p>
  This is a <a href="https://tailscale.com/">Tailscale</a> DERP server.
</p>

<p>
  It provides STUN, interactive connectivity establishment, and relaying of end-to-end encrypted traffic
  for Tailscale clients.
</p>

<p>
  Documentation:
</p>

<ul>

<li><a href="https://tailscale.com/kb/1232/derp-servers">About DERP</a></li>
<li><a href="https://pkg.go.dev/tailscale.com/derp">Protocol & Go docs</a></li>
<li><a href="https://github.com/tailscale/tailscale/tree/main/cmd/derper#derp">How to run a DERP server</a></li>

</body>
</html>
`

func getHomeHandler(val string) (_ http.Handler, ok bool) {
	if val == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(200)
			w.Write([]byte(homePage))
		}), true
	}
	if val == "blank" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(200)
		}), true
	}
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") {
		return http.RedirectHandler(val, http.StatusFound), true
	}
	return nil, false
}

func addWebSocketSupport(s *derp.Server, base http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := strings.ToLower(r.Header.Get("Upgrade"))

		// Very early versions of Tailscale set "Upgrade: WebSocket" but didn't actually
		// speak WebSockets (they still assumed DERP's binary framing). So to distinguish
		// clients that actually want WebSockets, look for an explicit "derp" subprotocol.
		if up != "websocket" || !strings.Contains(r.Header.Get("Sec-Websocket-Protocol"), "derp") {
			base.ServeHTTP(w, r)
			return
		}

		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols:   []string{"derp"},
			OriginPatterns: []string{"*"},
			// Disable compression because we transmit WireGuard messages that
			// are not compressible.
			// Additionally, Safari has a broken implementation of compression
			// (see https://github.com/nhooyr/websocket/issues/218) that makes
			// enabling it actively harmful.
			CompressionMode: websocket.CompressionDisabled,
		})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusInternalError, "closing")
		if c.Subprotocol() != "derp" {
			c.Close(websocket.StatusPolicyViolation, "client must speak the derp subprotocol")
			return
		}
		wc := wsconn.NetConn(r.Context(), c, websocket.MessageBinary, r.RemoteAddr)
		brw := bufio.NewReadWriter(bufio.NewReader(wc), bufio.NewWriter(wc))
		s.Accept(r.Context(), wc, brw, r.RemoteAddr)
	})
}

func handleBootstrapDNS(ctx context.Context) http.HandlerFunc {
	dnsRouter := service.FromContext[adapter.DNSRouter](ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "close")
		if queryDomain := r.URL.Query().Get("q"); queryDomain != "" {
			addresses, err := dnsRouter.Lookup(ctx, queryDomain, adapter.DNSQueryOptions{})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			render.JSON(w, r, render.M{
				queryDomain: addresses,
			})
			return
		}
		w.Write([]byte("{}"))
	}
}

type derpConfig struct {
	PrivateKey key.NodePrivate
}

func readDERPConfig(path string) (*derpConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeNewDERPConfig(path)
		}
		return nil, err
	}
	var config derpConfig
	err = json.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func writeNewDERPConfig(path string) (*derpConfig, error) {
	newKey := key.NewNode()
	err := os.MkdirAll(filepath.Dir(path), 0o777)
	if err != nil {
		return nil, err
	}
	config := derpConfig{
		PrivateKey: newKey,
	}
	content, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(path, content, 0o644)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (d *Service) loopSTUNPacket(packetConn *net.UDPConn) {
	buffer := make([]byte, 65535)
	oob := make([]byte, 1024)
	var (
		n        int
		oobN     int
		addrPort netip.AddrPort
		err      error
	)
	for {
		n, oobN, _, addrPort, err = packetConn.ReadMsgUDPAddrPort(buffer, oob)
		if err != nil {
			if E.IsClosedOrCanceled(err) {
				return
			}
			time.Sleep(time.Second)
			continue
		}
		if !stun.Is(buffer[:n]) {
			continue
		}
		txid, err := stun.ParseBindingRequest(buffer[:n])
		if err != nil {
			continue
		}
		packetConn.WriteMsgUDPAddrPort(stun.Response(txid, addrPort), oob[:oobN], addrPort)
	}
}
