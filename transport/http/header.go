package http

import (
	"net/http"
	"strings"

	M "github.com/sagernet/sing/common/metadata"

	"golang.org/x/net/http/httpguts"
)

func headerHasToken(header http.Header, name string, token string) bool {
	for _, value := range header.Values(name) {
		if httpguts.HeaderValuesContainsToken([]string{value}, token) {
			return true
		}
	}
	return false
}

func removeHopByHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for token := range strings.SplitSeq(value, ",") {
			token = strings.TrimSpace(token)
			if token != "" {
				header.Del(token)
			}
		}
	}
	header.Del("Connection")
	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("Keep-Alive")
	header.Del("TE")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")
}

func requestKeepAlive(request *http.Request) bool {
	if request.ProtoMajor == 1 && request.ProtoMinor == 0 {
		return headerHasToken(request.Header, "Connection", "keep-alive") ||
			headerHasToken(request.Header, "Proxy-Connection", "keep-alive")
	}
	return !request.Close && !headerHasToken(request.Header, "Proxy-Connection", "close")
}

func requestIsUpgrade(request *http.Request) bool {
	return request.Header.Get("Upgrade") != "" && headerHasToken(request.Header, "Connection", "upgrade")
}

func parseAuthority(authority string, defaultPort uint16) M.Socksaddr {
	destination := M.ParseSocksaddr(authority)
	if destination.Port == 0 {
		destination.Port = defaultPort
	}
	return destination.Unwrap()
}

func connectDestination(request *http.Request) M.Socksaddr {
	authority := request.URL.Host
	if authority == "" {
		authority = request.Host
	}
	return parseAuthority(authority, 443)
}

func forwardDestination(request *http.Request) (M.Socksaddr, bool) {
	switch request.URL.Scheme {
	case "http", "ws":
	default:
		return M.Socksaddr{}, false
	}
	if request.URL.Host == "" {
		return M.Socksaddr{}, false
	}
	destination := parseAuthority(request.URL.Host, 80)
	if !destination.IsValid() {
		return M.Socksaddr{}, false
	}
	return destination, true
}
