package api

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/coder/websocket"
	"golang.org/x/net/http/httpguts"
	"golang.org/x/net/http2"
)

const (
	webSocketSubprotocol  = "grpc-websockets"
	webSocketReadLimit    = 1 << 22
	webSocketPingInterval = 30 * time.Second
)

func isWebSocketGRPCRequest(request *http.Request) bool {
	return httpguts.HeaderValuesContainsToken(request.Header.Values("Upgrade"), "websocket") &&
		httpguts.HeaderValuesContainsToken(request.Header.Values("Sec-Websocket-Protocol"), webSocketSubprotocol)
}

// serveWebSocket carries a single gRPC stream over a WebSocket connection:
// the first client message contains the request metadata, each subsequent
// binary message is prefixed with 0 for body data or is a single 1 byte for
// the half-close signal, and the server sends gRPC-Web frames back.
func (b *webBridge) serveWebSocket(writer http.ResponseWriter, request *http.Request) {
	conn, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		Subprotocols:       []string{webSocketSubprotocol},
		InsecureSkipVerify: true,
	})
	if err != nil {
		b.logger.Error("upgrade websocket request: ", err)
		return
	}
	conn.SetReadLimit(webSocketReadLimit)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	messageType, firstMessage, err := conn.Read(ctx)
	if err != nil {
		conn.CloseNow()
		return
	}
	if messageType != websocket.MessageBinary {
		conn.CloseNow()
		return
	}
	header, err := parseWebSocketHeader(firstMessage)
	if err != nil {
		b.logger.Error("parse websocket request metadata: ", err)
		conn.CloseNow()
		return
	}
	contentType := header.Get("Content-Type")
	if contentType == "" {
		header.Set("Content-Type", contentTypeGRPC)
	} else {
		header.Set("Content-Type", strings.Replace(contentType, contentTypeGRPCWeb, contentTypeGRPC, 1))
	}
	header.Del("Content-Length")
	response := newWebSocketResponseWriter(ctx, conn)
	grpcRequest := request.WithContext(ctx)
	grpcRequest.Method = http.MethodPost
	grpcRequest.ProtoMajor = 2
	grpcRequest.ProtoMinor = 0
	grpcRequest.Header = header
	grpcRequest.Body = &webSocketBodyReader{
		ctx:      ctx,
		cancel:   cancel,
		conn:     conn,
		response: response,
	}
	go keepWebSocketAlive(ctx, conn)
	b.grpcServer.ServeHTTP(response, grpcRequest)
	response.writeTrailerFrame()
	conn.Close(websocket.StatusNormalClosure, "")
}

func parseWebSocketHeader(content []byte) (http.Header, error) {
	reader := textproto.NewReader(bufio.NewReader(io.MultiReader(bytes.NewReader(content), strings.NewReader("\r\n"))))
	mimeHeader, err := reader.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	return http.Header(mimeHeader), nil
}

func keepWebSocketAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(webSocketPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := conn.Ping(ctx)
			if err != nil {
				return
			}
		}
	}
}

type webSocketResponseWriter struct {
	ctx           context.Context
	conn          *websocket.Conn
	header        http.Header
	flushedHeader http.Header
	wroteHeaders  bool
	wroteTrailers bool
}

func newWebSocketResponseWriter(ctx context.Context, conn *websocket.Conn) *webSocketResponseWriter {
	return &webSocketResponseWriter{
		ctx:           ctx,
		conn:          conn,
		header:        make(http.Header),
		flushedHeader: make(http.Header),
	}
}

func (w *webSocketResponseWriter) Header() http.Header {
	return w.header
}

func (w *webSocketResponseWriter) Write(content []byte) (int, error) {
	if !w.wroteHeaders {
		w.WriteHeader(http.StatusOK)
	}
	err := w.conn.Write(w.ctx, websocket.MessageBinary, content)
	if err != nil {
		return 0, err
	}
	return len(content), nil
}

func (w *webSocketResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeaders {
		return
	}
	w.wroteHeaders = true
	headerFrame := make(http.Header)
	for key, values := range w.header {
		canonicalKey := http.CanonicalHeaderKey(key)
		if canonicalKey == "Trailer" {
			continue
		}
		w.flushedHeader[canonicalKey] = values
		headerFrame[canonicalKey] = values
	}
	w.writeHeaderFrame(headerFrame)
}

func (w *webSocketResponseWriter) Flush() {
}

func (w *webSocketResponseWriter) writeHeaderFrame(header http.Header) {
	var headerBuffer bytes.Buffer
	header.Write(&headerBuffer)
	frame := make([]byte, 0, 5+headerBuffer.Len())
	frame = append(frame, webMetadataFrameHeader(headerBuffer.Len())...)
	frame = append(frame, headerBuffer.Bytes()...)
	w.conn.Write(w.ctx, websocket.MessageBinary, frame)
}

func (w *webSocketResponseWriter) writeTrailerFrame() {
	if w.wroteTrailers {
		return
	}
	w.wroteTrailers = true
	trailerHeader := make(http.Header)
	for key, values := range w.header {
		lowerKey := strings.ToLower(strings.TrimPrefix(key, http2.TrailerPrefix))
		if lowerKey == "trailer" {
			continue
		}
		_, flushed := w.flushedHeader[http.CanonicalHeaderKey(lowerKey)]
		if flushed {
			continue
		}
		trailerHeader[lowerKey] = values
	}
	w.writeHeaderFrame(trailerHeader)
}

type webSocketBodyReader struct {
	ctx       context.Context
	cancel    context.CancelFunc
	conn      *websocket.Conn
	response  *webSocketResponseWriter
	remaining []byte
}

func (r *webSocketBodyReader) Read(buffer []byte) (int, error) {
	if len(r.remaining) > 0 {
		n := copy(buffer, r.remaining)
		r.remaining = r.remaining[n:]
		return n, nil
	}
	for {
		messageType, payload, err := r.conn.Read(r.ctx)
		if err != nil {
			r.cancel()
			return 0, io.EOF
		}
		if messageType != websocket.MessageBinary {
			return 0, E.New("unexpected non-binary websocket message")
		}
		if len(payload) == 0 {
			continue
		}
		if payload[0] == 1 {
			go r.waitForClose()
			return 0, io.EOF
		}
		content := payload[1:]
		if len(content) == 0 {
			continue
		}
		n := copy(buffer, content)
		r.remaining = content[n:]
		return n, nil
	}
}

func (r *webSocketBodyReader) waitForClose() {
	for {
		_, _, err := r.conn.Read(r.ctx)
		if err != nil {
			r.cancel()
			return
		}
	}
}

// Close is called by the gRPC handler transport after the stream status has
// been written; the trailer frame must be sent before the connection closes.
func (r *webSocketBodyReader) Close() error {
	r.response.writeTrailerFrame()
	return r.conn.Close(websocket.StatusNormalClosure, "")
}
