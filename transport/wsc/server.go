package wsc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
)

var _ adapter.WSCServerTransport = &Server{}
var _ http.Handler = &Server{}

type Server struct {
	ctx           context.Context
	handler       adapter.WSCServerTransportHandler
	httpServer    *http.Server
	logger        logger.ContextLogger
	authenticator Authenticator
	userManager   *wscUserManager
	router        adapter.Router
}

type ServerConfig struct {
	Ctx                        context.Context
	Logger                     logger.ContextLogger
	Handler                    adapter.WSCServerTransportHandler
	Authenticator              Authenticator
	Router                     adapter.Router
	MaxConnectionPerUser       int
	UsageReportTrafficInterval int64
	UsageReportTimeInterval    time.Duration
}

/*TODO: Support TLS servers*/
/*TODO: Pipe conn and packetConn*/
func NewServer(config ServerConfig) (*Server, error) {
	if config.Authenticator == nil {
		return nil, errors.New("authenticator required")
	}
	server := &Server{
		ctx:           config.Ctx,
		handler:       config.Handler,
		logger:        config.Logger,
		authenticator: config.Authenticator,
		router:        config.Router,
		userManager: &wscUserManager{
			users:                      map[int64]*wscUser{},
			authenticator:              config.Authenticator,
			maxConnPerUser:             config.MaxConnectionPerUser,
			usageReportTrafficInterval: config.UsageReportTrafficInterval,
			usageReportTimeInterval:    config.UsageReportTimeInterval,
		},
	}
	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: constant.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(l net.Listener) context.Context {
			return config.Ctx
		},
	}
	return server, nil
}

func (server *Server) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	auth := req.URL.Query().Get("auth")
	if auth == "" {
		server.failRequest(res, req, "Authentication required", http.StatusBadRequest, 0, "", metadata.Socksaddr{})
		return
	}

	account, err := server.authenticator.Authenticate(ctx, AuthenticateParams{
		Auth: auth,
	})
	if err != nil {
		if account.ID != 0 {
			if err := server.userManager.cleanupUser(ctx, account.ID, false); err != nil {
				server.logger.Debug("Request failed. Couldn't cleanup user: ", err.Error(), " (Client: ", req.RemoteAddr, ", User-ID: ", account.ID, ")")
			}
		}
		server.failRequest(res, req, "Authentication failed: "+err.Error(), http.StatusBadRequest, account.ID, "", metadata.Socksaddr{})
		return
	}

	if req.Method == http.MethodPost && req.URL.Path == "/cleanup" {
		if err := server.userManager.cleanupUser(ctx, account.ID, true); err != nil {
			server.failRequest(res, req, "Failed to cleanup user: "+err.Error(), http.StatusInternalServerError, account.ID, "", metadata.Socksaddr{})
			return
		}
		res.WriteHeader(http.StatusOK)
		return
	}

	user := server.userManager.findOrCreateUser(ctx, account.ID, account.Rate)

	netW := req.URL.Query().Get("net")
	if netW == "" {
		netW = network.NetworkTCP
	}

	endpoint := req.URL.Query().Get("ep")
	addr, err := server.resolveDestination(ctx, metadata.ParseSocksaddr(endpoint))
	if err != nil {
		server.failRequest(res, req, "Failed to parse and resolve endpoint: "+err.Error(), http.StatusBadRequest, account.ID, netW, addr)
		return
	}

	server.logger.Debug("New request (Client: ", req.RemoteAddr, ", Auth: ", auth, ", User-ID: ", account.ID, ", ", netW+"-Addr: ", addr.String(), ")")

	conn, _, _, err := ws.UpgradeHTTP(req, res)
	if err != nil {
		server.failRequest(res, req, "Websocket upgrade failed: "+err.Error(), http.StatusBadRequest, account.ID, netW, addr)
		return
	}

	defer func() {
		if err := server.userManager.cleanupUserConn(ctx, user, conn); err != nil {
			server.logger.Error("Failed to cleanup user connection: "+err.Error(), "(Client: ", req.RemoteAddr, ", User-ID: ", account.ID, ")")
		}
		if err := conn.Close(); err != nil {
			server.logger.Debug("Failed to close connection: "+err.Error(), "(Client: ", req.RemoteAddr, ", User-ID: ", account.ID, ")")
		}
	}()

	server.logger.Info("serve http called: ", req.URL.String(), " | ", req.RemoteAddr, " | ", endpoint, " | ", addr)
	res.Write([]byte("endpoint is : " + endpoint))
	res.WriteHeader(http.StatusOK)
}

func (server *Server) Close() error {
	return common.Close(common.PtrOrNil(server.httpServer))
}

func (server *Server) Network() []string {
	return []string{network.NetworkTCP}
}

func (server *Server) Serve(listener net.Listener) error {
	return server.httpServer.Serve(listener)
}

func (server *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (server *Server) resolveDestination(ctx context.Context, dest metadata.Socksaddr) (metadata.Socksaddr, error) {
	if dest.IsFqdn() {
		addrs, err := server.router.LookupDefault(ctx, dest.Fqdn)
		if err != nil {
			return metadata.Socksaddr{}, err
		}
		if len(addrs) == 0 {
			return metadata.Socksaddr{}, exceptions.New("no addresses found for endpoint domina: ", dest.Fqdn)
		}
		return metadata.Socksaddr{
			Addr: addrs[0],
			Port: dest.Port,
		}, nil
	}
	return dest, nil
}

func (server *Server) failRequest(res http.ResponseWriter, request *http.Request, msg string, code int, uid int64, network string, addr metadata.Socksaddr) {
	http.Error(res, msg, code)

	info := "(Client: " + request.RemoteAddr
	info += ", User-ID: " + strconv.Itoa(int(uid))
	info += ", Network: " + network
	info += ", " + network + "-Address: " + addr.String()
	info += ")"

	server.logger.Debug(msg, " ", info)
}
