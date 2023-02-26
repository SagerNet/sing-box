//go:build darwin

package libbox

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
)

type LogServer struct {
	sockPath string
	listener net.Listener

	access     sync.Mutex
	savedLines *list.List[string]
	subscriber *observable.Subscriber[string]
	observer   *observable.Observer[string]
}

func NewLogServer(sharedDirectory string) *LogServer {
	server := &LogServer{
		sockPath:   filepath.Join(sharedDirectory, "log.sock"),
		savedLines: new(list.List[string]),
		subscriber: observable.NewSubscriber[string](128),
	}
	server.observer = observable.NewObserver[string](server.subscriber, 64)
	return server
}

func (s *LogServer) Start() error {
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

func (s *LogServer) Close() error {
	return s.listener.Close()
}

func (s *LogServer) WriteMessage(message string) {
	s.subscriber.Emit(message)
	s.access.Lock()
	s.savedLines.PushBack(message)
	if s.savedLines.Len() > 100 {
		s.savedLines.Remove(s.savedLines.Front())
	}
	s.access.Unlock()
}

func (s *LogServer) loopConnection(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			hErr := s.handleConnection(&messageConn{conn})
			if hErr != nil && !E.IsClosed(err) {
				log.Warn("log-server: process connection: ", hErr)
			}
		}()
	}
}

func (s *LogServer) handleConnection(conn *messageConn) error {
	var savedLines []string
	s.access.Lock()
	savedLines = make([]string, 0, s.savedLines.Len())
	for element := s.savedLines.Front(); element != nil; element = element.Next() {
		savedLines = append(savedLines, element.Value)
	}
	s.access.Unlock()
	subscription, done, err := s.observer.Subscribe()
	if err != nil {
		return err
	}
	defer s.observer.UnSubscribe(subscription)
	for _, line := range savedLines {
		err = conn.Write([]byte(line))
		if err != nil {
			return err
		}
	}
	for {
		select {
		case message := <-subscription:
			err = conn.Write([]byte(message))
			if err != nil {
				return err
			}
		case <-done:
			conn.Close()
			return nil
		}
	}
}

type messageConn struct {
	net.Conn
}

func (c *messageConn) Read() ([]byte, error) {
	var messageLength uint16
	err := binary.Read(c.Conn, binary.BigEndian, &messageLength)
	if err != nil {
		return nil, err
	}
	data := make([]byte, messageLength)
	_, err = io.ReadFull(c.Conn, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *messageConn) Write(message []byte) error {
	err := binary.Write(c.Conn, binary.BigEndian, uint16(len(message)))
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(message)
	return err
}
