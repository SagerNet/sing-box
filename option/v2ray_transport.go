package option

import (
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

// V2RayXHTTPRange is an inclusive random range. A zero value selects the
// corresponding XHTTP protocol default.
type V2RayXHTTPRange struct {
	From int32 `json:"from,omitempty"`
	To   int32 `json:"to,omitempty"`
}

// V2RayXHTTPXmuxOptions controls reuse of the HTTP client used by XHTTP.
// max_connections and max_concurrency are mutually exclusive.
type V2RayXHTTPXmuxOptions struct {
	MaxConcurrency   V2RayXHTTPRange `json:"max_concurrency,omitempty"`
	MaxConnections   V2RayXHTTPRange `json:"max_connections,omitempty"`
	CMaxReuseTimes   V2RayXHTTPRange `json:"c_max_reuse_times,omitempty"`
	HMaxRequestTimes V2RayXHTTPRange `json:"h_max_request_times,omitempty"`
	HMaxReusableSecs V2RayXHTTPRange `json:"h_max_reusable_secs,omitempty"`
	HKeepAlivePeriod int64           `json:"h_keep_alive_period,omitempty"`
}

// V2RayXHTTPDownloadSettings defines the independent XHTTP endpoint used for
// the download stream. Its embedded XHTTP options describe that endpoint's
// HTTP request shape; server and tls select how to reach it.
type V2RayXHTTPDownloadSettings struct {
	ServerOptions
	OutboundTLSOptionsContainer
	V2RayXHTTPOptions
}

// V2RayXHTTPOptions intentionally follows Xray's XHTTP wire configuration.
// Field names use sing-box's snake_case JSON convention.
type V2RayXHTTPOptions struct {
	Host                 string                      `json:"host,omitempty"`
	Path                 string                      `json:"path,omitempty"`
	Mode                 string                      `json:"mode,omitempty"`
	Headers              badoption.HTTPHeader        `json:"headers,omitempty"`
	XPaddingBytes        V2RayXHTTPRange             `json:"x_padding_bytes,omitempty"`
	XPaddingObfsMode     bool                        `json:"x_padding_obfs_mode,omitempty"`
	XPaddingKey          string                      `json:"x_padding_key,omitempty"`
	XPaddingHeader       string                      `json:"x_padding_header,omitempty"`
	XPaddingPlacement    string                      `json:"x_padding_placement,omitempty"`
	XPaddingMethod       string                      `json:"x_padding_method,omitempty"`
	UplinkHTTPMethod     string                      `json:"uplink_http_method,omitempty"`
	SessionIDPlacement   string                      `json:"session_id_placement,omitempty"`
	SessionIDKey         string                      `json:"session_id_key,omitempty"`
	SessionIDTable       string                      `json:"session_id_table,omitempty"`
	SessionIDLength      V2RayXHTTPRange             `json:"session_id_length,omitempty"`
	SeqPlacement         string                      `json:"seq_placement,omitempty"`
	SeqKey               string                      `json:"seq_key,omitempty"`
	UplinkDataPlacement  string                      `json:"uplink_data_placement,omitempty"`
	UplinkDataKey        string                      `json:"uplink_data_key,omitempty"`
	UplinkChunkSize      V2RayXHTTPRange             `json:"uplink_chunk_size,omitempty"`
	NoGRPCHeader         bool                        `json:"no_grpc_header,omitempty"`
	NoSSEHeader          bool                        `json:"no_sse_header,omitempty"`
	SCMaxEachPostBytes   V2RayXHTTPRange             `json:"sc_max_each_post_bytes,omitempty"`
	SCMinPostsIntervalMS V2RayXHTTPRange             `json:"sc_min_posts_interval_ms,omitempty"`
	SCMaxBufferedPosts   int                         `json:"sc_max_buffered_posts,omitempty"`
	SCStreamUpServerSecs V2RayXHTTPRange             `json:"sc_stream_up_server_secs,omitempty"`
	ServerMaxHeaderBytes int                         `json:"server_max_header_bytes,omitempty"`
	QUIC                 QUICOptions                 `json:"quic,omitempty"`
	XMUX                 V2RayXHTTPXmuxOptions       `json:"xmux,omitempty"`
	DownloadSettings     *V2RayXHTTPDownloadSettings `json:"download_settings,omitempty"`
}
