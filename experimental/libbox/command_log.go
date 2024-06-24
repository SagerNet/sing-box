package libbox

import (
	"bufio"
	"context"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common/binary"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

func (s *CommandServer) ResetLog() {
	s.access.Lock()
	defer s.access.Unlock()
	s.savedLines.Init()
	select {
	case s.logReset <- struct{}{}:
	default:
	}
}

func (s *CommandServer) WriteMessage(message string) {
	s.subscriber.Emit(message)
	s.access.Lock()
	s.savedLines.PushBack(message)
	if s.savedLines.Len() > s.maxLines {
		s.savedLines.Remove(s.savedLines.Front())
	}
	s.access.Unlock()
}

func (s *CommandServer) handleLogConn(conn net.Conn) error {
	var (
		interval int64
		timer    *time.Timer
	)
	err := binary.Read(conn, binary.BigEndian, &interval)
	if err != nil {
		return E.Cause(err, "read interval")
	}
	timer = time.NewTimer(time.Duration(interval))
	if !timer.Stop() {
		<-timer.C
	}
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
	writer := bufio.NewWriter(conn)
	select {
	case <-s.logReset:
		err = writer.WriteByte(1)
		if err != nil {
			return err
		}
		err = writer.Flush()
		if err != nil {
			return err
		}
	default:
	}
	if len(savedLines) > 0 {
		err = writer.WriteByte(0)
		if err != nil {
			return err
		}
		err = varbin.Write(writer, binary.BigEndian, savedLines)
		if err != nil {
			return err
		}
	}
	ctx := connKeepAlive(conn)
	var logLines []string
	for {
		err = writer.Flush()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.logReset:
			err = writer.WriteByte(1)
			if err != nil {
				return err
			}
		case <-done:
			return nil
		case logLine := <-subscription:
			logLines = logLines[:0]
			logLines = append(logLines, logLine)
			timer.Reset(time.Duration(interval))
		loopLogs:
			for {
				select {
				case logLine = <-subscription:
					logLines = append(logLines, logLine)
				case <-timer.C:
					break loopLogs
				}
			}
			err = writer.WriteByte(0)
			if err != nil {
				return err
			}
			err = varbin.Write(writer, binary.BigEndian, logLines)
			if err != nil {
				return err
			}
		}
	}
}

func (c *CommandClient) handleLogConn(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		messageType, err := reader.ReadByte()
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		var messages []string
		switch messageType {
		case 0:
			err = varbin.Read(reader, binary.BigEndian, &messages)
			if err != nil {
				c.handler.Disconnected(err.Error())
				return
			}
			c.handler.WriteLogs(newIterator(messages))
		case 1:
			c.handler.ClearLogs()
		}
	}
}

func connKeepAlive(reader io.Reader) context.Context {
	ctx, cancel := context.WithCancelCause(context.Background())
	go func() {
		for {
			_, err := reader.Read(make([]byte, 1))
			if err != nil {
				cancel(err)
				return
			}
		}
	}()
	return ctx
}
