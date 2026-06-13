package api

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"strings"

	"github.com/sagernet/cors"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

const (
	contentTypeGRPC        = "application/grpc"
	contentTypeGRPCWeb     = "application/grpc-web"
	contentTypeGRPCWebText = "application/grpc-web-text"
)

// newHTTPHandler additionally accepts gRPC-Web requests
// (https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md) and gRPC-Web
// streams over WebSocket, wire compatible with the improbable-eng/grpc-web
// client transports.
func newHTTPHandler(logger log.ContextLogger, grpcServer *grpc.Server, options option.APIServiceOptions, dashboard *dashboard) http.Handler {
	allowedOrigins := options.AccessControlAllowOrigin
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:      allowedOrigins,
		AllowedMethods:      []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:      []string{"Content-Type", "Authorization", "X-Grpc-Web", "X-User-Agent", "Grpc-Timeout"},
		ExposedHeaders:      []string{"Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin"},
		AllowPrivateNetwork: options.AccessControlAllowPrivateNetwork,
		MaxAge:              300,
	})
	return corsHandler.Handler(&webBridge{
		logger:     logger,
		grpcServer: grpcServer,
		dashboard:  dashboard,
	})
}

type webBridge struct {
	logger     log.ContextLogger
	grpcServer *grpc.Server
	dashboard  *dashboard
}

func (b *webBridge) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	contentType := request.Header.Get("Content-Type")
	switch {
	case isWebSocketGRPCRequest(request):
		b.serveWebSocket(writer, request)
	case request.Method == http.MethodPost && strings.HasPrefix(contentType, contentTypeGRPCWeb):
		b.serveWeb(writer, request)
	case request.ProtoMajor == 2 && strings.HasPrefix(contentType, contentTypeGRPC):
		b.grpcServer.ServeHTTP(writer, request)
	case b.dashboard != nil:
		b.dashboard.serveHTTP(writer, request)
	default:
		http.NotFound(writer, request)
	}
}

func (b *webBridge) serveWeb(writer http.ResponseWriter, request *http.Request) {
	isTextFormat := strings.HasPrefix(request.Header.Get("Content-Type"), contentTypeGRPCWebText)
	webContentType := contentTypeGRPCWeb
	grpcRequest := request.Clone(request.Context())
	if isTextFormat {
		webContentType = contentTypeGRPCWebText
		grpcRequest.Body = &bodyReadCloser{
			Reader: base64.NewDecoder(base64.StdEncoding, request.Body),
			Closer: request.Body,
		}
	}
	// The gRPC server handler transport only accepts requests it sees as
	// native gRPC over HTTP/2.
	grpcRequest.ProtoMajor = 2
	grpcRequest.ProtoMinor = 0
	grpcRequest.Header.Set("Content-Type", strings.Replace(request.Header.Get("Content-Type"), webContentType, contentTypeGRPC, 1))
	grpcRequest.Header.Del("Content-Length")
	response := newWebResponseWriter(writer, isTextFormat)
	b.grpcServer.ServeHTTP(response, grpcRequest)
	response.finish()
}

type bodyReadCloser struct {
	io.Reader
	io.Closer
}

// webResponseWriter translates a native gRPC response into a gRPC-Web
// response: headers set after the first write, including the gRPC status the
// handler transport sets via http2.TrailerPrefix keys, become a trailer
// frame at the end of the body instead of HTTP trailers.
type webResponseWriter struct {
	writer       http.ResponseWriter
	rawWriter    http.ResponseWriter
	header       http.Header
	contentType  string
	wroteHeaders bool
	wroteBody    bool
}

func newWebResponseWriter(writer http.ResponseWriter, isTextFormat bool) *webResponseWriter {
	response := &webResponseWriter{
		writer:      writer,
		rawWriter:   writer,
		header:      make(http.Header),
		contentType: contentTypeGRPCWeb,
	}
	if isTextFormat {
		response.writer = newBase64ResponseWriter(writer)
		response.contentType = contentTypeGRPCWebText
	}
	return response
}

func (w *webResponseWriter) Header() http.Header {
	return w.header
}

func (w *webResponseWriter) Write(content []byte) (int, error) {
	if !w.wroteHeaders {
		w.prepareHeaders()
		w.wroteHeaders = true
	}
	w.wroteBody = true
	return w.writer.Write(content)
}

func (w *webResponseWriter) WriteHeader(statusCode int) {
	if !w.wroteHeaders {
		w.prepareHeaders()
		w.wroteHeaders = true
	}
	w.writer.WriteHeader(statusCode)
}

func (w *webResponseWriter) Flush() {
	// Flushing before anything was written would commit a 200 response
	// even for requests that end up as trailers-only responses.
	if w.wroteHeaders || w.wroteBody {
		flushWriter(w.writer)
	}
}

func (w *webResponseWriter) prepareHeaders() {
	rawHeader := w.rawWriter.Header()
	for key, values := range w.header {
		canonicalKey := http.CanonicalHeaderKey(strings.TrimPrefix(key, http2.TrailerPrefix))
		if canonicalKey == "Trailer" {
			continue
		}
		if canonicalKey == "Content-Type" {
			newValues := make([]string, 0, len(values))
			for _, value := range values {
				newValues = append(newValues, strings.Replace(value, contentTypeGRPC, w.contentType, 1))
			}
			values = newValues
		}
		rawHeader[canonicalKey] = values
	}
}

func (w *webResponseWriter) finish() {
	if w.wroteHeaders || w.wroteBody {
		w.writeTrailerFrame()
	} else {
		w.WriteHeader(http.StatusOK)
		flushWriter(w.writer)
	}
}

func (w *webResponseWriter) writeTrailerFrame() {
	flushedKeys := make(map[string]bool)
	for key := range w.rawWriter.Header() {
		flushedKeys[strings.ToLower(key)] = true
	}
	trailerHeader := make(http.Header)
	for key, values := range w.header {
		lowerKey := strings.ToLower(strings.TrimPrefix(key, http2.TrailerPrefix))
		if lowerKey == "trailer" || flushedKeys[lowerKey] {
			continue
		}
		trailerHeader[lowerKey] = values
	}
	var trailerBuffer bytes.Buffer
	trailerHeader.Write(&trailerBuffer)
	w.writer.Write(webMetadataFrameHeader(trailerBuffer.Len()))
	w.writer.Write(trailerBuffer.Bytes())
	flushWriter(w.writer)
}

func webMetadataFrameHeader(payloadLength int) []byte {
	return binary.BigEndian.AppendUint32([]byte{1 << 7}, uint32(payloadLength))
}

func flushWriter(writer http.ResponseWriter) {
	flusher, isFlusher := writer.(http.Flusher)
	if isFlusher {
		flusher.Flush()
	}
}

type base64ResponseWriter struct {
	wrapped http.ResponseWriter
	encoder io.WriteCloser
}

func newBase64ResponseWriter(wrapped http.ResponseWriter) http.ResponseWriter {
	writer := &base64ResponseWriter{wrapped: wrapped}
	writer.encoder = base64.NewEncoder(base64.StdEncoding, wrapped)
	return writer
}

func (w *base64ResponseWriter) Header() http.Header {
	return w.wrapped.Header()
}

func (w *base64ResponseWriter) Write(content []byte) (int, error) {
	return w.encoder.Write(content)
}

func (w *base64ResponseWriter) WriteHeader(statusCode int) {
	w.wrapped.WriteHeader(statusCode)
}

func (w *base64ResponseWriter) Flush() {
	w.encoder.Close()
	w.encoder = base64.NewEncoder(base64.StdEncoding, w.wrapped)
	flushWriter(w.wrapped)
}
