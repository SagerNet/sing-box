package wsc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
	"github.com/sagernet/ws/wsutil"
)

type serverUDPPiper struct {
	conn   net.Conn
	user   *wscUser
	addr   *metadata.Socksaddr
	dialer network.Dialer
}

func (piper *serverUDPPiper) pipe(ctx context.Context) error {
	remote, err := piper.prepare(ctx)
	if err != nil {
		return err
	}
	defer remote.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var gErr error = nil
	collectErr := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		gErr = errors.Join(gErr, err)
	}

	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()
		if err := piper.pipeInbound(ctx, remote); err != nil {
			collectErr(err)
		}
	}()

	if err := piper.pipeOutbount(ctx, remote); err != nil {
		collectErr(err)
	}
	cancel()

	wg.Wait()

	return gErr
}

func (piper *serverUDPPiper) pipeInbound(ctx context.Context, remote net.PacketConn) error {
	clientInReader, err := piper.user.connReader(piper.conn)
	if err != nil {
		return err
	}
	clientOut, err := piper.user.connWriter(piper.conn)
	if err != nil {
		return err
	}

	clientIn := wsutil.NewReader(clientInReader, ws.StateServerSide)
	buf := piper.user.inBuffer(piper.conn)
	payload := packetConnPayload{}

	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := piper.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 300)); err != nil {
			return err
		}

		header, err := clientIn.NextFrame()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if isTimeoutErr(err) {
				continue
			}
			return err
		}

		pass := false
		switch header.OpCode {
		case ws.OpPing:
			wsutil.WriteServerMessage(clientOut, ws.OpPong, nil)
			pass = true
		case ws.OpPong:
			pass = true
		case ws.OpClose:
			wsutil.WriteServerMessage(clientOut, ws.OpClose, nil)
			return nil
		}
		if pass {
			continue
		}

		for {
			n, err := clientIn.Read(buf)
			if n > 0 {
				if err := payload.UnmarshalBinaryUnsafe(buf[:n]); err != nil {
					return err
				}

				if _, wErr := remote.WriteTo(payload.payload, net.UDPAddrFromAddrPort(payload.addrPort)); wErr != nil {
					return wErr
				} else {
					piper.user.usedTrafficBytes.Add(int64(n))
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
		}
	}
}

func (piper *serverUDPPiper) pipeOutbount(ctx context.Context, remote net.PacketConn) error {
	clientOut, err := piper.user.connWriter(piper.conn)
	if err != nil {
		return err
	}

	buf := piper.user.outBuffer(piper.conn)
	payload := packetConnPayload{}

	for {
		if ctx.Err() != nil {
			return nil
		}
		if err := remote.SetReadDeadline(time.Now().Add(time.Millisecond * 300)); err != nil {
			return err
		}

		n, netAddr, err := remote.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if isTimeoutErr(err) {
				continue
			}
			return err
		}

		if ua, ok := netAddr.(*net.UDPAddr); ok {
			payload.addrPort = ua.AddrPort()
		} else {
			return errors.New("unexpected addr type")
		}
		payload.payload = buf[:n]
		payloadBytes, err := payload.MarshalBinary()
		if err != nil {
			return err
		}

		piper.user.usedTrafficBytes.Add(int64(n))

		if err := wsutil.WriteServerBinary(clientOut, payloadBytes); err != nil {
			return err
		}
	}
}

func (piper *serverUDPPiper) prepare(ctx context.Context) (net.PacketConn, error) {
	remote, err := piper.dialer.ListenPacket(ctx, *piper.addr)
	if err != nil {
		return nil, err
	}
	return remote, nil
}
