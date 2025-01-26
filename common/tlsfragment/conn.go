package tf

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/sagernet/sing/common"
	N "github.com/sagernet/sing/common/network"
)

type Conn struct {
	net.Conn
	syscallConn        syscall.Conn
	ctx                context.Context
	firstPacketWritten bool
	fallbackDelay      time.Duration
}

func NewConn(conn net.Conn, ctx context.Context, fallbackDelay time.Duration) (*Conn, error) {
	syscallConn, _ := N.UnwrapReader(conn).(syscall.Conn)
	return &Conn{
		Conn:          conn,
		syscallConn:   syscallConn,
		ctx:           ctx,
		fallbackDelay: fallbackDelay,
	}, nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if !c.firstPacketWritten {
		defer func() {
			c.firstPacketWritten = true
		}()
		serverName := indexTLSServerName(b)
		if serverName != nil {
			tcpConn, isTCPConn := c.syscallConn.(interface {
				SetNoDelay(bool) error
			})
			if isTCPConn {
				err = tcpConn.SetNoDelay(true)
				if err != nil {
					return
				}
			}
			splits := strings.Split(string(b[serverName.Index:serverName.Index+serverName.Length]), ".")
			currentIndex := serverName.Index
			var striped bool
			if len(splits) > 3 {
				suffix := splits[len(splits)-3] + "." + splits[len(splits)-2] + "." + splits[len(splits)-1]
				if publicSuffixMatcher().Match(suffix) {
					splits = splits[:len(splits)-3]
				}
				striped = true
			}
			if !striped && len(splits) > 2 {
				suffix := splits[len(splits)-2] + "." + splits[len(splits)-1]
				if publicSuffixMatcher().Match(suffix) {
					splits = splits[:len(splits)-2]
				}
				striped = true
			}
			if !striped && len(splits) > 1 {
				suffix := splits[len(splits)-1]
				if publicSuffixMatcher().Match(suffix) {
					splits = splits[:len(splits)-1]
				}
			}
			if len(splits) > 1 && common.Contains(publicPrefix, splits[0]) {
				currentIndex += len(splits[0]) + 1
				splits = splits[1:]
			}
			var splitIndexes []int
			for i, split := range splits {
				splitAt := rand.Intn(len(split))
				splitIndexes = append(splitIndexes, currentIndex+splitAt)
				currentIndex += len(split)
				if i != len(splits)-1 {
					currentIndex++
				}
			}
			for i := 0; i <= len(splitIndexes); i++ {
				if i == 0 {
					_, err = c.Conn.Write(b[:splitIndexes[i]])
				} else if i == len(splitIndexes) {
					_, err = c.Conn.Write(b[splitIndexes[i-1]:])
				} else {
					_, err = c.Conn.Write(b[splitIndexes[i-1]:splitIndexes[i]])
				}
				if err != nil {
					return
				}
				if c.syscallConn != nil && i != len(splitIndexes) {
					err = waitAck(c.ctx, c.syscallConn, c.fallbackDelay)
					if err != nil {
						return
					}
				}
			}
			if isTCPConn {
				err = tcpConn.SetNoDelay(false)
				if err != nil {
					return
				}
			}
			return len(b), nil
		}
	}
	return c.Conn.Write(b)
}

func (c *Conn) ReaderReplaceable() bool {
	return true
}

func (c *Conn) WriterReplaceable() bool {
	return c.firstPacketWritten
}

func (c *Conn) Upstream() any {
	return c.Conn
}
