package http

import (
	std_bufio "bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
)

type upstreamConn struct {
	net.Conn
	reader      *std_bufio.Reader
	limiter     *readLimiter
	user        string
	source      M.Socksaddr
	destination M.Socksaddr
	done        chan struct{}
	err         error
}

func (c *upstreamConn) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *upstreamConn) readResponse(request *http.Request) (*http.Response, error) {
	c.limiter.remaining = maxHeaderBytes
	response, err := http.ReadResponse(c.reader, request)
	c.limiter.remaining = -1
	return response, err
}

func (c *upstreamConn) closeErr() error {
	select {
	case <-c.done:
		return c.err
	default:
		return nil
	}
}

func (c *serverConn) openUpstream(ctx context.Context, source M.Socksaddr, destination M.Socksaddr) *upstreamConn {
	user, _ := auth.UserFromContext[string](ctx)
	if c.upstream != nil {
		if c.upstream.user == user && c.upstream.source == source && c.upstream.destination == destination && !c.upstream.closed() {
			return c.upstream
		}
		c.closeUpstream()
	}
	c.upstream = newUpstreamConn(ctx, c.handler, source, destination)
	c.upstream.user = user
	return c.upstream
}

func (c *serverConn) serveForward(ctx context.Context, request *http.Request, source M.Socksaddr, upgrade bool) (requestResult, error) {
	destination, valid := forwardDestination(request)
	if !valid {
		return c.reject(request, requestKeepAlive(request), http.StatusBadRequest, E.New("invalid forward target: ", request.URL.String()))
	}
	keepAlive := requestKeepAlive(request)
	upgradeProtocol := request.Header.Get("Upgrade")
	removeHopByHopHeaders(request.Header)
	if upgrade {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", upgradeProtocol)
	}
	if _, loaded := request.Header["User-Agent"]; !loaded {
		request.Header["User-Agent"] = nil
	}
	request.Close = false

	upstream := c.openUpstream(ctx, source, destination)
	writeDone := make(chan error, 1)
	go func() {
		err := request.Write(upstream)
		if err != nil {
			upstream.Close()
		}
		writeDone <- err
	}()

	var response *http.Response
	for {
		var err error
		response, err = upstream.readResponse(request)
		if err != nil {
			c.closeUpstream()
			writeErr := c.finishRequestWrite(writeDone)
			err = E.Errors(upstream.closeErr(), E.Cause(err, "read upstream response"))
			if !keepAlive || (writeErr != nil && request.Body != http.NoBody) {
				return c.rejectAndClose(request, http.StatusBadGateway, err)
			}
			request.Body = http.NoBody
			return c.reject(request, true, http.StatusBadGateway, err)
		}
		if response.StatusCode >= 200 || response.StatusCode == http.StatusSwitchingProtocols {
			break
		}
		if request.ProtoMajor == 1 && request.ProtoMinor == 0 {
			continue
		}
		removeHopByHopHeaders(response.Header)
		err = writeInterimResponse(c.conn, request, response)
		if err != nil {
			c.closeUpstream()
			c.finishRequestWrite(writeDone)
			return requestClose, E.Cause(err, "write interim response")
		}
	}

	response.ProtoMajor = request.ProtoMajor
	response.ProtoMinor = request.ProtoMinor
	if response.StatusCode == http.StatusSwitchingProtocols {
		if !upgrade {
			c.closeUpstream()
			c.finishRequestWrite(writeDone)
			return requestClose, E.New("unexpected 101 response")
		}
		return c.relayUpgrade(ctx, response, writeDone)
	}

	upstreamClose := response.Close
	removeHopByHopHeaders(response.Header)
	if response.ContentLength == -1 && responseHasBody(request, response) {
		if request.ProtoMajor == 1 && request.ProtoMinor == 0 {
			keepAlive = false
		} else {
			response.TransferEncoding = []string{"chunked"}
		}
	}
	response.Close = !keepAlive
	if keepAlive && request.ProtoMajor == 1 && request.ProtoMinor == 0 {
		response.Header.Set("Connection", "keep-alive")
		response.Header.Set("Proxy-Connection", "keep-alive")
	}
	err := response.Write(c.conn)
	if err != nil {
		c.closeUpstream()
		response.Body.Close()
		c.finishRequestWrite(writeDone)
		return requestClose, E.Cause(err, "write response")
	}
	response.Body.Close()
	writeErr := c.finishRequestWrite(writeDone)
	if writeErr != nil {
		c.closeUpstream()
		if keepAlive && request.Body != http.NoBody && !c.discardBody(request.Body) {
			keepAlive = false
		}
	} else if upstreamClose {
		c.closeUpstream()
	}
	if !keepAlive {
		return requestClose, nil
	}
	return requestContinue, nil
}

func (c *serverConn) finishRequestWrite(writeDone chan error) error {
	select {
	case err := <-writeDone:
		return err
	default:
	}
	c.conn.SetReadDeadline(time.Now().Add(discardBodyTimeout))
	timer := time.NewTimer(discardBodyTimeout)
	defer timer.Stop()
	var err error
	select {
	case err = <-writeDone:
	case <-timer.C:
		c.closeUpstream()
		err = <-writeDone
		if err == nil {
			err = E.New("request body timeout")
		}
	}
	c.conn.SetReadDeadline(time.Time{})
	return err
}

func (c *serverConn) relayUpgrade(ctx context.Context, response *http.Response, writeDone chan error) (requestResult, error) {
	response.Close = false
	err := response.Write(c.conn)
	if err != nil {
		c.closeUpstream()
		c.finishRequestWrite(writeDone)
		return requestClose, E.Cause(err, "write response")
	}
	err = c.finishRequestWrite(writeDone)
	if err != nil {
		c.closeUpstream()
		return requestClose, E.Cause(err, "write upgrade request")
	}
	upstream := c.upstream
	c.upstream = nil
	var upstreamConn net.Conn = upstream
	if upstream.reader.Buffered() > 0 {
		buffer := buf.NewSize(upstream.reader.Buffered())
		_, err = buffer.ReadFullFrom(upstream.reader, buffer.FreeLen())
		if err != nil {
			buffer.Release()
			upstream.Close()
			return requestClose, err
		}
		upstreamConn = bufio.NewCachedConn(upstream.Conn, buffer)
	}
	err = bufio.CopyConn(ctx, c.reader.cachedConn(c.conn), upstreamConn)
	return requestClose, err
}

func responseHasBody(request *http.Request, response *http.Response) bool {
	if request.Method == http.MethodHead {
		return false
	}
	if response.StatusCode < 200 || response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusNotModified {
		return false
	}
	return true
}

func writeInterimResponse(writer io.Writer, request *http.Request, response *http.Response) error {
	var buffer bytes.Buffer
	buffer.WriteString(F.ToString("HTTP/", request.ProtoMajor, ".", request.ProtoMinor, " ", response.StatusCode, " ", http.StatusText(response.StatusCode), "\r\n"))
	err := response.Header.Write(&buffer)
	if err != nil {
		return err
	}
	buffer.WriteString("\r\n")
	_, err = writer.Write(buffer.Bytes())
	return err
}
