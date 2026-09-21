package http

import (
	std_bufio "bufio"
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/transport/v2rayhttp"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

const (
	connectUDPProtocol   = "connect-udp"
	connectUDPPathPrefix = "/.well-known/masque/udp/"
)

func parseConnectUDPTarget(path string) (M.Socksaddr, bool) {
	rest, found := strings.CutPrefix(path, connectUDPPathPrefix)
	if !found {
		return M.Socksaddr{}, false
	}
	segments := strings.Split(strings.TrimSuffix(rest, "/"), "/")
	if len(segments) != 2 {
		return M.Socksaddr{}, false
	}
	host, err := url.PathUnescape(segments[0])
	if err != nil || host == "" {
		return M.Socksaddr{}, false
	}
	port, err := strconv.ParseUint(segments[1], 10, 16)
	if err != nil || port == 0 {
		return M.Socksaddr{}, false
	}
	destination := M.ParseSocksaddrHostPort(host, uint16(port)).Unwrap()
	if !destination.IsValid() {
		return M.Socksaddr{}, false
	}
	return destination, true
}

func connectUDPURL(authority string, destination M.Socksaddr) *url.URL {
	host := destination.AddrString()
	port := "/" + strconv.Itoa(int(destination.Port)) + "/"
	requestURL := &url.URL{
		Path:    connectUDPPathPrefix + host + port,
		RawPath: connectUDPPathPrefix + strings.ReplaceAll(url.PathEscape(host), ":", "%3A") + port,
	}
	if authority != "" {
		requestURL.Scheme = "https"
		requestURL.Host = authority
	}
	return requestURL
}

func requestIsConnectUDP(request *http.Request) bool {
	return request.Method == http.MethodGet && requestIsUpgrade(request) && strings.EqualFold(request.Header.Get("Upgrade"), connectUDPProtocol)
}

func (c *serverConn) serveConnectUDP(ctx context.Context, request *http.Request, source M.Socksaddr) (requestResult, error) {
	destination, valid := parseConnectUDPTarget(request.URL.EscapedPath())
	if !valid || !request.ProtoAtLeast(1, 1) {
		return c.reject(request, requestKeepAlive(request), http.StatusBadRequest, E.New("invalid connect-udp request: ", request.URL.Path))
	}
	_, err := c.conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: connect-udp\r\nCapsule-Protocol: ?1\r\n\r\n"))
	if err != nil {
		return requestClose, E.Cause(err, "write response")
	}
	c.handler.NewPacketConnectionEx(ctx, newCapsuleConn(c.reader.Reader, c.conn, destination), source, destination, c.onClose)
	return requestHandedOff, nil
}

func (h *httpHandler) serveConnectUDP(ctx context.Context, writer http.ResponseWriter, request *http.Request, source M.Socksaddr) {
	destination, valid := parseConnectUDPTarget(request.URL.EscapedPath())
	if !valid {
		h.server.logger.ErrorContext(ctx, "process connection from ", source, ": invalid connect-udp target: ", request.URL.Path)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	writer.Header().Set("Capsule-Protocol", "?1")
	writer.WriteHeader(http.StatusOK)
	writer.(http.Flusher).Flush()
	if request.ProtoMajor == 3 && HTTP3StreamFunc != nil {
		stream, isDatagramStream := HTTP3StreamFunc(request.Context(), writer)
		if isDatagramStream {
			localAddr, _ := request.Context().Value(http.LocalAddrContextKey).(net.Addr)
			conn := newHTTP3PacketConn(stream, destination, localAddr)
			h.handler.NewPacketConnectionEx(ctx, conn, source, destination, nil)
			conn.wait(request.Context())
			return
		}
	}
	conn := v2rayhttp.NewHTTP2Wrapper(&v2rayhttp.ServerHTTPConn{
		HTTP2Conn: v2rayhttp.NewHTTPConn(request.Body, writer),
		Flusher:   writer.(http.Flusher),
	})
	done := make(chan struct{})
	h.handler.NewPacketConnectionEx(ctx, newCapsuleConn(std_bufio.NewReader(conn), conn, destination), source, destination, N.OnceClose(func(it error) {
		close(done)
	}))
	<-done
	conn.CloseWrapper()
}

func (c *Client) connectUDPHTTP1(ctx context.Context, conn net.Conn, destination M.Socksaddr) (N.PacketConn, error) {
	stop := context.AfterFunc(ctx, func() {
		conn.Close()
	})
	defer stop()
	request := &http.Request{
		Method: http.MethodGet,
		URL:    connectUDPURL("", destination),
		Host:   c.authority(),
		Header: c.headers.Clone(),
	}
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", connectUDPProtocol)
	request.Header.Set("Capsule-Protocol", "?1")
	if _, loaded := request.Header["User-Agent"]; !loaded {
		request.Header["User-Agent"] = nil
	}
	if c.authorization != "" {
		request.Header.Set("Proxy-Authorization", c.authorization)
	}
	err := request.Write(conn)
	if err != nil {
		return nil, E.Cause(err, "write request")
	}
	reader := std_bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, request)
	if err != nil {
		return nil, E.Cause(err, "read response")
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		return nil, statusError(response)
	}
	if !strings.EqualFold(response.Header.Get("Upgrade"), connectUDPProtocol) {
		return nil, E.New("unexpected upgrade protocol: ", response.Header.Get("Upgrade"))
	}
	if !stop() {
		return nil, ctx.Err()
	}
	return newCapsuleConn(reader, conn, destination), nil
}

func (c *Client) connectUDPHTTP2(ctx context.Context, clientConn *http2ClientConn, destination M.Socksaddr) (N.PacketConn, error) {
	request := &http.Request{
		Method: http.MethodConnect,
		URL:    connectUDPURL(c.authority(), destination),
		Host:   c.authority(),
		Header: c.headers.Clone(),
	}
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	request.Header.Set(":protocol", connectUDPProtocol)
	request.Header.Set("Capsule-Protocol", "?1")
	stream, err := c.roundTripHTTP2(ctx, clientConn, request, destination)
	if err != nil {
		return nil, err
	}
	return newCapsuleConn(std_bufio.NewReader(stream), stream, destination), nil
}

func (c *Client) authority() string {
	if c.host != "" {
		return c.host
	}
	return c.server.String()
}
