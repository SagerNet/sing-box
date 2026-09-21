package badhttp

import (
	std_bufio "bufio"
	"net/http"
	"net/url"
	"strings"
	_ "unsafe"

	M "github.com/sagernet/sing/common/metadata"
)

//go:linkname ReadRequest net/http.readRequest
func ReadRequest(b *std_bufio.Reader) (req *http.Request, err error)

//go:linkname URLSetPath net/url.(*URL).setPath
func URLSetPath(u *url.URL, p string) error

//go:linkname ParseBasicAuth net/http.parseBasicAuth
func ParseBasicAuth(auth string) (username, password string, valid bool)

func SourceAddress(request *http.Request) M.Socksaddr {
	return ForwardedSource(request, M.ParseSocksaddr(request.RemoteAddr).Unwrap())
}

func ForwardedSource(request *http.Request, source M.Socksaddr) M.Socksaddr {
	for _, value := range request.Header.Values("X-Forwarded-For") {
		for from := range strings.SplitSeq(value, ",") {
			address := M.ParseAddr(strings.TrimSpace(from))
			if address.IsValid() {
				return M.Socksaddr{Addr: address, Port: source.Port}.Unwrap()
			}
		}
	}
	return source
}
