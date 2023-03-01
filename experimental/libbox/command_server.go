//go:build darwin

package libbox

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
)

type CommandServer struct {
	sockPath string
	listener net.Listener

	access     sync.Mutex
	savedLines *list.List[string]
	subscriber *observable.Subscriber[string]
	observer   *observable.Observer[string]
}

func NewCommandServer(sharedDirectory string) *CommandServer {
	server := &CommandServer{
		sockPath:   filepath.Join(sharedDirectory, "command.sock"),
		savedLines: new(list.List[string]),
		subscriber: observable.NewSubscriber[string](128),
	}
	server.observer = observable.NewObserver[string](server.subscriber, 64)
	return server
}

func (s *CommandServer) Start() error {
	os.Remove(s.sockPath)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: s.sockPath,
		Net:  "unix",
	})
	if err != nil {
		return err
	}
	go s.loopConnection(listener)
	return nil
}

func (s *CommandServer) Close() error {
	return s.listener.Close()
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
				log.Warn("log-server: process connection: ", hErr)
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
	default:
		return E.New("unknown command: ", command)
	}
}
