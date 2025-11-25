package option

import (
	"net/http"
	"net/url"
	"strings"

	Xbadoption "github.com/sagernet/sing-box/common/xray/json/badoption"
	C "github.com/sagernet/sing-box/constant"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"
	"github.com/sagernet/sing/common/json/badoption"
)

type _V2RayTransportOptions struct {
	Type               string                  `json:"type"`
	HTTPOptions        V2RayHTTPOptions        `json:"-"`
	WebsocketOptions   V2RayWebsocketOptions   `json:"-"`
	QUICOptions        V2RayQUICOptions        `json:"-"`
	GRPCOptions        V2RayGRPCOptions        `json:"-"`
	HTTPUpgradeOptions V2RayHTTPUpgradeOptions `json:"-"`
	XHTTPOptions       V2RayXHTTPOptions       `json:"-"`
}

type V2RayTransportOptions _V2RayTransportOptions

func (o V2RayTransportOptions) MarshalJSON() ([]byte, error) {
	var v any
	switch o.Type {
	case C.V2RayTransportTypeHTTP:
		v = o.HTTPOptions
	case C.V2RayTransportTypeWebsocket:
		v = o.WebsocketOptions
	case C.V2RayTransportTypeQUIC:
		v = o.QUICOptions
	case C.V2RayTransportTypeGRPC:
		v = o.GRPCOptions
	case C.V2RayTransportTypeHTTPUpgrade:
		v = o.HTTPUpgradeOptions
	case C.V2RayTransportTypeXHTTP:
		v = o.XHTTPOptions
	case "":
		return nil, E.New("missing transport type")
	default:
		return nil, E.New("unknown transport type: " + o.Type)
	}
	return badjson.MarshallObjects((_V2RayTransportOptions)(o), v)
}

func (o *V2RayTransportOptions) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*_V2RayTransportOptions)(o))
	if err != nil {
		return err
	}
	var v any
	switch o.Type {
	case C.V2RayTransportTypeHTTP:
		v = &o.HTTPOptions
	case C.V2RayTransportTypeWebsocket:
		v = &o.WebsocketOptions
	case C.V2RayTransportTypeQUIC:
		v = &o.QUICOptions
	case C.V2RayTransportTypeGRPC:
		v = &o.GRPCOptions
	case C.V2RayTransportTypeHTTPUpgrade:
		v = &o.HTTPUpgradeOptions
	case C.V2RayTransportTypeXHTTP:
		v = &o.XHTTPOptions
	default:
		return E.New("unknown transport type: " + o.Type)
	}
	err = badjson.UnmarshallExcluded(bytes, (*_V2RayTransportOptions)(o), v)
	if err != nil {
		return err
	}
	return nil
}

type V2RayHTTPOptions struct {
	Host        badoption.Listable[string] `json:"host,omitempty"`
	Path        string                     `json:"path,omitempty"`
	Method      string                     `json:"method,omitempty"`
	Headers     badoption.HTTPHeader       `json:"headers,omitempty"`
	IdleTimeout badoption.Duration         `json:"idle_timeout,omitempty"`
	PingTimeout badoption.Duration         `json:"ping_timeout,omitempty"`
}

type V2RayWebsocketOptions struct {
	Path                string               `json:"path,omitempty"`
	Headers             badoption.HTTPHeader `json:"headers,omitempty"`
	MaxEarlyData        uint32               `json:"max_early_data,omitempty"`
	EarlyDataHeaderName string               `json:"early_data_header_name,omitempty"`
}

type V2RayQUICOptions struct{}

type V2RayGRPCOptions struct {
	ServiceName         string             `json:"service_name,omitempty"`
	IdleTimeout         badoption.Duration `json:"idle_timeout,omitempty"`
	PingTimeout         badoption.Duration `json:"ping_timeout,omitempty"`
	PermitWithoutStream bool               `json:"permit_without_stream,omitempty"`
	ForceLite           bool               `json:"-"` // for test
}

type V2RayHTTPUpgradeOptions struct {
	Host    string               `json:"host,omitempty"`
	Path    string               `json:"path,omitempty"`
	Headers badoption.HTTPHeader `json:"headers,omitempty"`
}

type V2RayXHTTPBaseOptions struct {
	Mode                 string                 `json:"mode"`
	Host                 string                 `json:"host,omitempty"`
	Path                 string                 `json:"path,omitempty"`
	Headers              map[string]string      `json:"headers,omitempty"`
	DomainStrategy       DomainStrategy         `json:"domain_strategy,omitempty"`
	XPaddingBytes        Xbadoption.Range       `json:"x_padding_bytes"`
	NoGRPCHeader         bool                   `json:"no_grpc_header,omitempty"`
	NoSSEHeader          bool                   `json:"no_sse_header,omitempty"`
	ScMaxEachPostBytes   Xbadoption.Range       `json:"sc_max_each_post_bytes"`
	ScMinPostsIntervalMs Xbadoption.Range       `json:"sc_min_posts_interval_ms"`
	ScMaxBufferedPosts   int64                  `json:"sc_max_buffered_posts,omitempty"`
	ScStreamUpServerSecs Xbadoption.Range       `json:"sc_stream_up_server_secs"`
	Xmux                 *V2RayXHTTPXmuxOptions `json:"xmux"`
}

type V2RayXHTTPOptions struct {
	V2RayXHTTPBaseOptions
	Download *V2RayXHTTPDownloadOptions `json:"download"`
}

type V2RayXHTTPDownloadOptions struct {
	V2RayXHTTPBaseOptions
	ServerOptions
	OutboundTLSOptionsContainer
	Detour string `json:"detour,omitempty"`
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedPath() string {
	pathAndQuery := strings.SplitN(c.Path, "?", 2)
	path := pathAndQuery[0]
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	if path[len(path)-1] != '/' {
		path = path + "/"
	}
	return path
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedQuery() string {
	pathAndQuery := strings.SplitN(c.Path, "?", 2)
	query := ""
	if len(pathAndQuery) > 1 {
		query = pathAndQuery[1]
	}
	return query
}

func (c *V2RayXHTTPBaseOptions) GetRequestHeader(rawURL string) http.Header {
	header := http.Header{}
	for k, v := range c.Headers {
		header.Add(k, v)
	}
	u, _ := url.Parse(rawURL)
	// https://www.rfc-editor.org/rfc/rfc7541.html#appendix-B
	// h2's HPACK Header Compression feature employs a huffman encoding using a static table.
	// 'X' is assigned an 8 bit code, so HPACK compression won't change actual padding length on the wire.
	// https://www.rfc-editor.org/rfc/rfc9204.html#section-4.1.2-2
	// h3's similar QPACK feature uses the same huffman table.
	u.RawQuery = "x_padding=" + strings.Repeat("X", int(c.GetNormalizedXPaddingBytes().Rand()))
	header.Set("Referer", u.String())
	return header
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedXPaddingBytes() Xbadoption.Range {
	if c.XPaddingBytes.To == 0 {
		return Xbadoption.Range{
			From: 100,
			To:   1000,
		}
	}
	return c.XPaddingBytes
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedScMaxEachPostBytes() Xbadoption.Range {
	if c.ScMaxEachPostBytes.To == 0 {
		return Xbadoption.Range{
			From: 1000000,
			To:   1000000,
		}
	}
	return c.ScMaxEachPostBytes
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedScMinPostsIntervalMs() Xbadoption.Range {
	if c.ScMinPostsIntervalMs.To == 0 {
		return Xbadoption.Range{
			From: 30,
			To:   30,
		}
	}
	return c.ScMinPostsIntervalMs
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedScMaxBufferedPosts() int {
	if c.ScMaxBufferedPosts == 0 {
		return 30
	}

	return int(c.ScMaxBufferedPosts)
}

func (c *V2RayXHTTPBaseOptions) GetNormalizedScStreamUpServerSecs() Xbadoption.Range {
	if c.ScStreamUpServerSecs.To == 0 {
		return Xbadoption.Range{
			From: 20,
			To:   80,
		}
	}
	return c.ScStreamUpServerSecs
}

type V2RayXHTTPXmuxOptions struct {
	MaxConcurrency   Xbadoption.Range `json:"max_concurrency"`
	MaxConnections   Xbadoption.Range `json:"max_connections"`
	CMaxReuseTimes   Xbadoption.Range `json:"c_max_reuse_times"`
	HMaxRequestTimes Xbadoption.Range `json:"h_max_request_times"`
	HMaxReusableSecs Xbadoption.Range `json:"h_max_reusable_secs"`
	HKeepAlivePeriod int64            `json:"h_keep_alive_period"`
}

func (m *V2RayXHTTPXmuxOptions) GetNormalizedMaxConcurrency() Xbadoption.Range {
	return m.MaxConcurrency
}

func (m *V2RayXHTTPXmuxOptions) GetNormalizedMaxConnections() Xbadoption.Range {
	return m.MaxConnections
}

func (m *V2RayXHTTPXmuxOptions) GetNormalizedCMaxReuseTimes() Xbadoption.Range {
	return m.CMaxReuseTimes
}

func (m *V2RayXHTTPXmuxOptions) GetNormalizedHMaxRequestTimes() Xbadoption.Range {
	return m.HMaxRequestTimes
}

func (m *V2RayXHTTPXmuxOptions) GetNormalizedHMaxReusableSecs() Xbadoption.Range {
	return m.HMaxReusableSecs
}
