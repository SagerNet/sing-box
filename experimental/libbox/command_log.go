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
	if messageLength == 0 {
		return nil, nil
	}
	data := make([]byte, messageLength)
	_, err = io.ReadFull(reader, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func writeLog(writer io.Writer, message []byte) error {
	err := binary.Write(writer, binary.BigEndian, uint8(0))
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, uint16(len(message)))
	if err != nil {
		return err
	}
	if len(message) > 0 {
		_, err = writer.Write(message)
	}
	return err
}

func writeClearLog(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, uint8(1))
}

func (s *CommandServer) handleLogConn(conn net.Conn) error {
	var savedLines []string
	s.access.Lock()
	savedLines = make([]string, 0, s.savedLines.Len())
	for element := s.savedLines.Front(); element != nil; element = element.Next() {
		savedLines = append(savedLines, element.Value)
	}
	subscription, done, err := s.observer.Subscribe()
	if err != nil {
		return err
	}
	s.access.Unlock()
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
		case <-s.logReset:
			err = writeClearLog(conn)
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
		var messageType uint8
		err := binary.Read(conn, binary.BigEndian, &messageType)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		var message []byte
		switch messageType {
		case 0:
			message, err = readLog(conn)
			if err != nil {
				c.handler.Disconnected(err.Error())
				return
			}
			c.handler.WriteLog(string(message))
		case 1:
			c.handler.ClearLog()
		}
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
