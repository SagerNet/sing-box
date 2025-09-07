package wsc

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	N "github.com/sagernet/sing/common/network"
)

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

type Service struct {
	logger        logger.ContextLogger
	router        adapter.Router
	handler       Handler
	authenticator Authenticator
	userManager   *wscUserManager
}

type ServiceConfig struct {
	Logger                     logger.ContextLogger
	Router                     adapter.Router
	Handler                    Handler
	Authenticator              Authenticator
	MaxConnectionPerUser       int
	UsageReportTrafficInterval int64
	UsageReportTimeInterval    time.Duration
}

type meteredConn struct {
	net.Conn
	user *wscUser
}

func NewService(config ServiceConfig) (*Service, error) {
	if config.Handler == nil {
		return nil, E.New("Handler required")
	}
	if config.Authenticator == nil {
		return nil, E.New("Authenticator required")
	}
	return &Service{
		logger:        config.Logger,
		router:        config.Router,
		handler:       config.Handler,
		authenticator: config.Authenticator,
		userManager: &wscUserManager{
			users:                      map[int64]*wscUser{},
			authenticator:              config.Authenticator,
			maxConnPerUser:             config.MaxConnectionPerUser,
			usageReportTrafficInterval: config.UsageReportTrafficInterval,
			usageReportTimeInterval:    config.UsageReportTimeInterval,
		},
	}, nil
}

func (service *Service) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	params, err := service.readQueryParams(conn)
	if err != nil {
		return err
	}

	auth := params.Get("auth")
	if auth == "" {
		return E.New("authentication required")
	}

	account, err := service.authenticator.Authenticate(ctx, AuthenticateParams{
		Auth:    auth,
		MaxConn: service.userManager.maxConnPerUser,
	})
	if err != nil {
		if account.ID != 0 {
			if err := service.userManager.cleanupUser(ctx, account.ID, false); err != nil {
				return err
			}
			return E.Cause(err, "authentication failed")
		}
	}

	// user cleanup
	//{}

	user := service.userManager.findOrCreateUser(ctx, account.ID, account.Rate, account.MaxConn)

	netw := params.Get("net")
	if netw == "" {
		netw = network.NetworkTCP
	}

	endpoint := params.Get("ep")
	addr, err := service.resolveDestination(ctx, M.ParseSocksaddr(endpoint))
	if err != nil {
		return E.Cause(err, "failed to parse and resolve endpoint")
	}

	service.log("New request (Client: ", metadata.Source, ", Auth: ", auth, ", User-ID: ", account.ID, ", ", netw+"-Addr: ", addr.String(), ")")

	metadata.Protocol = C.TypeWSC
	metadata.Destination = addr

	if popedConn, err := user.addConn(conn); err != nil {
		return err
	} else {
		if popedConn != nil {
			popedConn.Close()
		}
	}

	switch N.NetworkName(netw) {
	case N.NetworkTCP:
		err = service.handler.NewConnection(ctx, &meteredConn{Conn: conn, user: user}, metadata)
	case N.NetworkUDP:
		return service.handler.NewPacketConnection(ctx, &servicePacketConn{
			Conn: &meteredConn{
				Conn: conn,
				user: user,
			},
		}, metadata)
	default:
		return E.New("not supported protocol ", netw)
	}

	if cErr := service.userManager.cleanupUserConn(ctx, user, conn); cErr != nil && err == nil {
		err = cErr
	}

	return err
}

func (service *Service) readQueryParams(conn net.Conn) (url.Values, error) {
	var queryParamsRaw [500]byte
	n, err := conn.Read(queryParamsRaw[:])
	if err != nil {
		return nil, err
	}
	pURL := url.URL{
		RawQuery: string(queryParamsRaw[:n]),
	}
	return pURL.Query(), nil
}

func (service *Service) resolveDestination(ctx context.Context, dest M.Socksaddr) (M.Socksaddr, error) {
	if dest.IsFqdn() {
		addrs, err := service.router.LookupDefault(ctx, dest.Fqdn)
		if err != nil {
			return M.Socksaddr{}, err
		}
		if len(addrs) == 0 {
			return M.Socksaddr{}, E.New("no address found for endpoint domain: ", dest.Fqdn)
		}
		return M.Socksaddr{
			Addr: addrs[0],
			Port: dest.Port,
		}, nil
	}
	return dest, nil
}

func (service *Service) log(args ...any) {
	if service.logger != nil {
		service.logger.Debug(args...)
	}
}

func (conn *meteredConn) Read(p []byte) (int, error) {
	reader, err := conn.user.connReader(conn.Conn)
	if err != nil {
		return 0, err
	}
	n, err := reader.Read(p)
	if err != nil {
		return 0, err
	}
	conn.user.usedTrafficBytes.Add(int64(n))
	return n, nil
}

func (conn *meteredConn) Write(p []byte) (int, error) {
	writer, err := conn.user.connWriter(conn.Conn)
	if err != nil {
		return 0, err
	}
	n, err := writer.Write(p)
	if err != nil {
		return 0, err
	}
	conn.user.usedTrafficBytes.Add(int64(n))
	return n, nil
}
