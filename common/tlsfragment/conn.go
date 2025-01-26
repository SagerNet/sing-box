package tf

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"time"

	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/publicsuffix"
)

type Conn struct {
	net.Conn
	tcpConn            *net.TCPConn
	ctx                context.Context
	firstPacketWritten bool
	fallbackDelay      time.Duration
}

func NewConn(conn net.Conn, ctx context.Context, fallbackDelay time.Duration) (*Conn, error) {
	tcpConn, _ := N.UnwrapReader(conn).(*net.TCPConn)
	return &Conn{
		Conn:          conn,
		tcpConn:       tcpConn,
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
			if c.tcpConn != nil {
				err = c.tcpConn.SetNoDelay(true)
				if err != nil {
					return
				}
			}
			splits := strings.Split(serverName.ServerName, ".")
			currentIndex := serverName.Index
			if publicSuffix := publicsuffix.List.PublicSuffix(serverName.ServerName); publicSuffix != "" {
				splits = splits[:len(splits)-strings.Count(serverName.ServerName, ".")]
			}
			if len(splits) > 1 && splits[0] == "..." {
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
				var payload []byte
				if i == 0 {
					payload = b[:splitIndexes[i]]
				} else if i == len(splitIndexes) {
					payload = b[splitIndexes[i-1]:]
				} else {
					payload = b[splitIndexes[i-1]:splitIndexes[i]]
				}
				if c.tcpConn != nil && i != len(splitIndexes) {
					err = writeAndWaitAck(c.ctx, c.tcpConn, payload, c.fallbackDelay)
					if err != nil {
						return
					}
				} else {
					_, err = c.Conn.Write(payload)
					if err != nil {
						return
					}
				}
			}
			if c.tcpConn != nil {
				err = c.tcpConn.SetNoDelay(false)
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
