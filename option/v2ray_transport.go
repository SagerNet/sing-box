package option

import (
	"math/rand"

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

type V2RayXHTTPOptions struct {
	Host                   string               `json:"host,omitempty"`
	Path                   string               `json:"path,omitempty"`
	Headers                badoption.HTTPHeader `json:"headers,omitempty"`
	Mode                   string               `json:"mode,omitempty"`
	NoGRPCHeader           bool                 `json:"no_grpc_header,omitempty"`
	NoSSEHeader            bool                 `json:"no_sse_header,omitempty"`
	XPaddingBytes          *RangeConfig         `json:"x_padding_bytes,omitempty"`
	ScMaxEachPostBytes     *RangeConfig         `json:"sc_max_each_post_bytes,omitempty"`
	ScMinPostsIntervalMs   *RangeConfig         `json:"sc_min_posts_interval_ms,omitempty"`
	ScMaxBufferedPosts     int                  `json:"sc_max_buffered_posts,omitempty"`
	ScStreamUpServerSecs   *RangeConfig         `json:"sc_stream_up_server_secs,omitempty"`
	XmuxMaxConcurrency     *RangeConfig         `json:"xmux_max_concurrency,omitempty"`
	XmuxMaxConnections     *RangeConfig         `json:"xmux_max_connections,omitempty"`
	XmuxCMaxReuseTimes     *RangeConfig         `json:"xmux_c_max_reuse_times,omitempty"`
	XmuxHMaxRequestTimes   *RangeConfig         `json:"xmux_h_max_request_times,omitempty"`
	XmuxHMaxReusableSecs   *RangeConfig         `json:"xmux_h_max_reusable_secs,omitempty"`
	XmuxHKeepAlivePeriod   badoption.Duration   `json:"xmux_h_keep_alive_period,omitempty"`
}

type RangeConfig struct {
	From int32 `json:"from,omitempty"`
	To   int32 `json:"to,omitempty"`
}

func (r *RangeConfig) RandValue() int32 {
	if r == nil {
		return 0
	}
	if r.From == r.To {
		return r.From
	}
	if r.From > r.To {
		return r.From
	}
	return r.From + int32(rand.Int63n(int64(r.To-r.From+1)))
}

func (r *RangeConfig) GetFrom(defaultValue int32) int32 {
	if r == nil || r.From == 0 {
		return defaultValue
	}
	return r.From
}

func (r *RangeConfig) GetTo(defaultValue int32) int32 {
	if r == nil || r.To == 0 {
		return defaultValue
	}
	return r.To
}
