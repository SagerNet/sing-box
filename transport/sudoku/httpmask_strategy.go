package sudoku

import (
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/transport/sudoku/obfs/httpmask"
)

var (
	httpMaskUserAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	httpMaskAccepts = []string{
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"application/json, text/plain, */*",
		"application/octet-stream",
		"*/*",
	}
	httpMaskAcceptLanguages = []string{
		"en-US,en;q=0.9",
		"zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	}
	httpMaskAcceptEncodings = []string{
		"gzip, deflate, br",
		"gzip, deflate",
	}
	httpMaskPaths = []string{
		"/api/v1/upload",
		"/data/sync",
		"/v1/telemetry",
		"/session",
		"/ws",
	}
	httpMaskContentTypes = []string{
		"application/octet-stream",
		"application/json",
	}
)

var (
	httpMaskRngPool = sync.Pool{
		New: func() any { return rand.New(rand.NewSource(time.Now().UnixNano())) },
	}
	httpMaskBufPool = sync.Pool{
		New: func() any {
			b := make([]byte, 0, 1024)
			return &b
		},
	}
)

func trimPortForHost(host string) string {
	if host == "" {
		return host
	}
	h, _, err := net.SplitHostPort(host)
	if err == nil && h != "" {
		return h
	}
	return host
}

func appendCommonHeaders(buf []byte, host string, r *rand.Rand) []byte {
	ua := httpMaskUserAgents[r.Intn(len(httpMaskUserAgents))]
	accept := httpMaskAccepts[r.Intn(len(httpMaskAccepts))]
	lang := httpMaskAcceptLanguages[r.Intn(len(httpMaskAcceptLanguages))]
	enc := httpMaskAcceptEncodings[r.Intn(len(httpMaskAcceptEncodings))]

	buf = append(buf, "Host: "...)
	buf = append(buf, host...)
	buf = append(buf, "\r\nUser-Agent: "...)
	buf = append(buf, ua...)
	buf = append(buf, "\r\nAccept: "...)
	buf = append(buf, accept...)
	buf = append(buf, "\r\nAccept-Language: "...)
	buf = append(buf, lang...)
	buf = append(buf, "\r\nAccept-Encoding: "...)
	buf = append(buf, enc...)
	buf = append(buf, "\r\nConnection: keep-alive\r\n"...)
	buf = append(buf, "Cache-Control: no-cache\r\nPragma: no-cache\r\n"...)
	return buf
}

// WriteHTTPMaskHeader writes an HTTP/1.x request header as a mask, according to strategy.
// Supported strategies: ""/"random", "post", "websocket".
func WriteHTTPMaskHeader(w io.Writer, host string, strategy string) error {
	switch normalizeHTTPMaskStrategy(strategy) {
	case "random":
		return httpmask.WriteRandomRequestHeader(w, host)
	case "post":
		return writeHTTPMaskPOST(w, host)
	case "websocket":
		return writeHTTPMaskWebSocket(w, host)
	default:
		return fmt.Errorf("unsupported http_mask_strategy: %s", strategy)
	}
}

func writeHTTPMaskPOST(w io.Writer, host string) error {
	r := httpMaskRngPool.Get().(*rand.Rand)
	defer httpMaskRngPool.Put(r)

	path := httpMaskPaths[r.Intn(len(httpMaskPaths))]
	ctype := httpMaskContentTypes[r.Intn(len(httpMaskContentTypes))]

	bufPtr := httpMaskBufPool.Get().(*[]byte)
	buf := *bufPtr
	buf = buf[:0]
	defer func() {
		if cap(buf) <= 4096 {
			*bufPtr = buf
			httpMaskBufPool.Put(bufPtr)
		}
	}()

	const minCL = int64(4 * 1024)
	const maxCL = int64(10 * 1024 * 1024)
	contentLength := minCL + r.Int63n(maxCL-minCL+1)

	buf = append(buf, "POST "...)
	buf = append(buf, path...)
	buf = append(buf, " HTTP/1.1\r\n"...)
	buf = appendCommonHeaders(buf, host, r)
	buf = append(buf, "Content-Type: "...)
	buf = append(buf, ctype...)
	buf = append(buf, "\r\nContent-Length: "...)
	buf = strconv.AppendInt(buf, contentLength, 10)
	buf = append(buf, "\r\n\r\n"...)

	_, err := w.Write(buf)
	return err
}

func writeHTTPMaskWebSocket(w io.Writer, host string) error {
	r := httpMaskRngPool.Get().(*rand.Rand)
	defer httpMaskRngPool.Put(r)

	path := httpMaskPaths[r.Intn(len(httpMaskPaths))]

	bufPtr := httpMaskBufPool.Get().(*[]byte)
	buf := *bufPtr
	buf = buf[:0]
	defer func() {
		if cap(buf) <= 4096 {
			*bufPtr = buf
			httpMaskBufPool.Put(bufPtr)
		}
	}()

	hostNoPort := trimPortForHost(host)
	var keyBytes [16]byte
	for i := 0; i < len(keyBytes); i++ {
		keyBytes[i] = byte(r.Intn(256))
	}
	var wsKey [24]byte
	base64.StdEncoding.Encode(wsKey[:], keyBytes[:])

	buf = append(buf, "GET "...)
	buf = append(buf, path...)
	buf = append(buf, " HTTP/1.1\r\n"...)
	buf = appendCommonHeaders(buf, host, r)
	buf = append(buf, "Upgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: "...)
	buf = append(buf, wsKey[:]...)
	buf = append(buf, "\r\nOrigin: https://"...)
	buf = append(buf, hostNoPort...)
	buf = append(buf, "\r\n\r\n"...)

	_, err := w.Write(buf)
	return err
}

func normalizeHTTPMaskStrategy(strategy string) string {
	s := strings.TrimSpace(strings.ToLower(strategy))
	switch s {
	case "", "random":
		return "random"
	case "ws":
		return "websocket"
	default:
		return s
	}
}

