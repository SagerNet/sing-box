package libbox

import (
	"context"
	"encoding/binary"
	"io"
	"net"
)

func (s *CommandServer) WriteMessage(message string) {
	s.subscriber.Emit(message)
	s.access.Lock()
	s.savedLines.PushBack(message)
	if s.savedLines.Len() > s.maxLines {
		s.savedLines.Remove(s.savedLines.Front())
	}
	s.access.Unlock()
}

func readLog(reader io.Reader) ([]byte, error) {
	var messageLength uint16
	err := binary.Read(reader, binary.BigEndian, &messageLength)
	if err != nil {
		return nil, err
	}
	data := make([]byte, messageLength)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeLog(writer io.Writer, message []byte) error {
	err := binary.Write(writer, binary.BigEndian, uint16(len(message)))
	if err != nil {
		return err
	}
	_, err = writer.Write(message)
	return err
}

func (s *CommandServer) handleLogConn(conn net.Conn) error {
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
		err = writeLog(conn, []byte(line))
		if err != nil {
			return err
		}
	}
	ctx := connKeepAlive(conn)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message := <-subscription:
			err = writeLog(conn, []byte(message))
			if err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (c *CommandClient) handleLogConn(conn net.Conn) {
	for {
		message, err := readLog(conn)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.WriteLog(string(message))
	}
}

func connKeepAlive(reader io.Reader) context.Context {
	ctx, cancel := context.WithCancelCause(context.Background())
	go func() {
		for {
			_, err := readLog(reader)
			if err != nil {
				cancel(err)
				return
			}
		}
	}()
	return ctx
}
