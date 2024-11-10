package libbox

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/debug"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
)

type CommandServer struct {
	listener net.Listener
	handler  CommandServerHandler

	access     sync.Mutex
	savedLines list.List[string]
	maxLines   int
	subscriber *observable.Subscriber[string]
	observer   *observable.Observer[string]
	service    *BoxService

	// These channels only work with a single client. if multi-client support is needed, replace with Subscriber/Observer
	urlTestUpdate chan struct{}
	modeUpdate    chan struct{}
	logReset      chan struct{}

	closedConnections []Connection
}

type CommandServerHandler interface {
	ServiceReload() error
	PostServiceClose()
	GetSystemProxyStatus() *SystemProxyStatus
	SetSystemProxyEnabled(isEnabled bool) error
}

func NewCommandServer(handler CommandServerHandler, maxLines int32) *CommandServer {
	server := &CommandServer{
		handler:       handler,
		maxLines:      int(maxLines),
		subscriber:    observable.NewSubscriber[string](128),
		urlTestUpdate: make(chan struct{}, 1),
		modeUpdate:    make(chan struct{}, 1),
		logReset:      make(chan struct{}, 1),
	}
	server.observer = observable.NewObserver[string](server.subscriber, 64)
	return server
}

func (s *CommandServer) SetService(newService *BoxService) {
	if newService != nil {
		service.PtrFromContext[urltest.HistoryStorage](newService.ctx).SetHook(s.urlTestUpdate)
		newService.clashServer.(*clashapi.Server).SetModeUpdateHook(s.modeUpdate)
	}
	s.service = newService
	s.notifyURLTestUpdate()
}

func (s *CommandServer) notifyURLTestUpdate() {
	select {
	case s.urlTestUpdate <- struct{}{}:
	default:
	}
}

func (s *CommandServer) Start() error {
	if !sTVOS {
		return s.listenUNIX()
	} else {
		return s.listenTCP()
	}
}

func (s *CommandServer) listenUNIX() error {
	sockPath := filepath.Join(sBasePath, "command.sock")
	os.Remove(sockPath)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: sockPath,
		Net:  "unix",
	})
	if err != nil {
		return E.Cause(err, "listen ", sockPath)
	}
	err = os.Chown(sockPath, sUserID, sGroupID)
	if err != nil {
		listener.Close()
		os.Remove(sockPath)
		return E.Cause(err, "chown")
	}
	s.listener = listener
	go s.loopConnection(listener)
	return nil
}

func (s *CommandServer) listenTCP() error {
	listener, err := net.Listen("tcp", "127.0.0.1:8964")
	if err != nil {
		return E.Cause(err, "listen")
	}
	s.listener = listener
	go s.loopConnection(listener)
	return nil
}

func (s *CommandServer) Close() error {
	return common.Close(
		s.listener,
		s.observer,
	)
}

func (s *CommandServer) loopConnection(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			hErr := s.handleConnection(conn)
			if hErr != nil && !E.IsClosed(err) {
				if debug.Enabled {
					log.Warn("log-server: process connection: ", hErr)
				}
			}
		}()
	}
}

func (s *CommandServer) handleConnection(conn net.Conn) error {
	defer conn.Close()
	var command uint8
	err := binary.Read(conn, binary.BigEndian, &command)
	if err != nil {
		return E.Cause(err, "read command")
	}
	switch int32(command) {
	case CommandLog:
		return s.handleLogConn(conn)
	case CommandStatus:
		return s.handleStatusConn(conn)
	case CommandServiceReload:
		return s.handleServiceReload(conn)
	case CommandServiceClose:
		return s.handleServiceClose(conn)
	case CommandCloseConnections:
		return s.handleCloseConnections(conn)
	case CommandGroup:
		return s.handleGroupConn(conn)
	case CommandSelectOutbound:
		return s.handleSelectOutbound(conn)
	case CommandURLTest:
		return s.handleURLTest(conn)
	case CommandGroupExpand:
		return s.handleSetGroupExpand(conn)
	case CommandClashMode:
		return s.handleModeConn(conn)
	case CommandSetClashMode:
		return s.handleSetClashMode(conn)
	case CommandGetSystemProxyStatus:
		return s.handleGetSystemProxyStatus(conn)
	case CommandSetSystemProxyEnabled:
		return s.handleSetSystemProxyEnabled(conn)
	case CommandConnections:
		return s.handleConnectionsConn(conn)
	case CommandCloseConnection:
		return s.handleCloseConnection(conn)
	case CommandGetDeprecatedNotes:
		return s.handleGetDeprecatedNotes(conn)
	default:
		return E.New("unknown command: ", command)
	}
}
