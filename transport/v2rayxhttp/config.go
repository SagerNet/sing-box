// Package v2rayxhttp implements the XHTTP V2Ray transport.
//
// The implementation is independent from Xray-core. Its public behaviour is
// verified against the XHTTP wire protocol and sing-box integration tests.
package v2rayxhttp

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	placementPath          = "path"
	placementQuery         = "query"
	placementHeader        = "header"
	placementCookie        = "cookie"
	placementBody          = "body"
	placementAuto          = "auto"
	placementQueryInHeader = "queryInHeader"
)

type byteRange struct{ from, to int32 }

func makeRange(value option.V2RayXHTTPRange, defaultFrom, defaultTo int32) byteRange {
	if value.To == 0 {
		return byteRange{defaultFrom, defaultTo}
	}
	return byteRange{value.From, value.To}
}

func (r byteRange) valid() bool { return r.from > 0 && r.to >= r.from }

func (r byteRange) random() int {
	if r.to <= r.from {
		return int(r.from)
	}
	span := uint32(r.to-r.from) + 1
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	value := uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
	return int(r.from + int32(value%span))
}

type config struct {
	host, path, query                                          string
	mode                                                       string
	headers                                                    http.Header
	padding                                                    byteRange
	paddingObfs                                                bool
	paddingKey, paddingHeader, paddingPlacement, paddingMethod string
	uplinkMethod, sessionPlacement, sessionKey                 string
	seqPlacement, seqKey, dataPlacement, dataKey               string
	scMaxPost, scMinInterval, scStreamUp                       byteRange
	uplinkChunk                                                byteRange
	maxBufferedPosts, maxHeaderBytes                           int
	quic                                                       option.QUICOptions
	noGRPCHeader, noSSEHeader                                  bool
	sessionTable                                               string
	sessionLength                                              byteRange
	xmux                                                       xmuxConfig
}

func newConfig(options option.V2RayXHTTPOptions) (*config, error) {
	xmux, err := newXMuxConfig(options.XMUX)
	if err != nil {
		return nil, err
	}
	c := &config{
		host:             options.Host,
		mode:             options.Mode,
		headers:          options.Headers.Build(),
		padding:          makeRange(options.XPaddingBytes, 100, 1000),
		paddingObfs:      options.XPaddingObfsMode,
		paddingKey:       options.XPaddingKey,
		paddingHeader:    options.XPaddingHeader,
		paddingPlacement: options.XPaddingPlacement,
		paddingMethod:    options.XPaddingMethod,
		uplinkMethod:     strings.ToUpper(options.UplinkHTTPMethod),
		sessionPlacement: options.SessionIDPlacement,
		sessionKey:       options.SessionIDKey,
		seqPlacement:     options.SeqPlacement,
		seqKey:           options.SeqKey,
		dataPlacement:    options.UplinkDataPlacement,
		dataKey:          options.UplinkDataKey,
		scMaxPost:        makeRange(options.SCMaxEachPostBytes, 1000000, 1000000),
		scMinInterval:    makeRange(options.SCMinPostsIntervalMS, 30, 30),
		scStreamUp:       makeRange(options.SCStreamUpServerSecs, 20, 80),
		maxBufferedPosts: options.SCMaxBufferedPosts,
		maxHeaderBytes:   options.ServerMaxHeaderBytes,
		quic:             options.QUIC,
		noGRPCHeader:     options.NoGRPCHeader,
		noSSEHeader:      options.NoSSEHeader,
		sessionTable:     predefinedTable(options.SessionIDTable),
		sessionLength:    makeRange(options.SessionIDLength, 0, 0),
		xmux:             xmux,
	}
	if c.headers.Get("Host") != "" {
		return nil, E.New("xhttp headers must not contain Host")
	}
	pathAndQuery := strings.SplitN(options.Path, "?", 2)
	c.path = pathAndQuery[0]
	if c.path == "" {
		c.path = "/"
	}
	if !strings.HasPrefix(c.path, "/") {
		c.path = "/" + c.path
	}
	if len(pathAndQuery) == 2 {
		c.query = pathAndQuery[1]
	}
	if c.mode == "" {
		c.mode = "auto"
	}
	if c.mode != "auto" && c.mode != "packet-up" && c.mode != "stream-up" && c.mode != "stream-one" {
		return nil, E.New("unsupported xhttp mode: ", c.mode)
	}
	if !c.padding.valid() {
		return nil, E.New("invalid xhttp x_padding_bytes")
	}
	if c.paddingKey == "" {
		c.paddingKey = "x_padding"
	}
	if c.paddingHeader == "" {
		c.paddingHeader = "X-Padding"
	}
	if c.paddingPlacement == "" {
		c.paddingPlacement = placementQueryInHeader
	}
	if c.paddingPlacement != placementCookie && c.paddingPlacement != placementHeader && c.paddingPlacement != placementQuery && c.paddingPlacement != placementQueryInHeader {
		return nil, E.New("unsupported xhttp padding placement: ", c.paddingPlacement)
	}
	if c.paddingMethod == "" {
		c.paddingMethod = "repeat-x"
	}
	if c.paddingMethod != "repeat-x" && c.paddingMethod != "tokenish" {
		return nil, E.New("unsupported xhttp padding method: ", c.paddingMethod)
	}
	if c.uplinkMethod == "" {
		c.uplinkMethod = http.MethodPost
	}
	if c.uplinkMethod == http.MethodGet && c.mode != "packet-up" && c.mode != "auto" {
		return nil, E.New("xhttp uplink_http_method GET requires packet-up mode")
	}
	if c.sessionPlacement == "" {
		c.sessionPlacement = placementPath
	}
	if c.seqPlacement == "" {
		c.seqPlacement = placementPath
	}
	if !validMetaPlacement(c.sessionPlacement) || !validMetaPlacement(c.seqPlacement) {
		return nil, E.New("unsupported xhttp session or sequence placement")
	}
	if c.sessionKey == "" {
		c.sessionKey = defaultMetaKey(c.sessionPlacement, "session")
	}
	if c.seqKey == "" {
		c.seqKey = defaultMetaKey(c.seqPlacement, "seq")
	}
	if c.dataPlacement == "" {
		c.dataPlacement = placementBody
	}
	if c.dataPlacement != placementAuto && c.dataPlacement != placementBody && c.dataPlacement != placementHeader && c.dataPlacement != placementCookie {
		return nil, E.New("unsupported xhttp uplink data placement: ", c.dataPlacement)
	}
	if c.dataKey == "" && c.dataPlacement != placementBody {
		if c.dataPlacement == placementCookie {
			c.dataKey = "x_data"
		} else {
			c.dataKey = "X-Data"
		}
	}
	c.uplinkChunk = normalizedUplinkChunk(options.UplinkChunkSize, c.dataPlacement, c.scMaxPost)
	if !c.scMaxPost.valid() || !c.scMinInterval.valid() || !c.scStreamUp.valid() {
		return nil, E.New("invalid xhttp upload range")
	}
	if c.maxBufferedPosts == 0 {
		c.maxBufferedPosts = 30
	}
	if c.maxBufferedPosts < 1 || c.maxHeaderBytes < 0 {
		return nil, E.New("invalid xhttp server limit")
	}
	if c.maxHeaderBytes == 0 {
		c.maxHeaderBytes = 8192
	}
	return c, nil
}

func normalizedUplinkChunk(value option.V2RayXHTTPRange, placement string, postLimit byteRange) byteRange {
	if value.To != 0 {
		from := value.From
		if from < 64 {
			from = 64
		}
		to := value.To
		if to < from {
			to = from
		}
		return byteRange{from: from, to: to}
	}
	switch placement {
	case placementCookie:
		return byteRange{from: 2 * 1024, to: 3 * 1024}
	case placementHeader:
		return byteRange{from: 3 * 1000, to: 4 * 1000}
	default:
		return postLimit
	}
}

func validMetaPlacement(value string) bool {
	return value == placementPath || value == placementQuery || value == placementHeader || value == placementCookie
}

func defaultMetaKey(placement, name string) string {
	if placement == placementHeader {
		if name == "session" {
			return "X-Session"
		}
		return "X-Seq"
	}
	if placement == placementCookie || placement == placementQuery {
		return "x_" + name
	}
	return ""
}

func predefinedTable(table string) string {
	switch table {
	case "ALPHABET":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "Alphabet":
		return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	case "BASE36":
		return "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	case "Base62":
		return "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	case "HEX":
		return "0123456789ABCDEF"
	case "alphabet":
		return "abcdefghijklmnopqrstuvwxyz"
	case "base36":
		return "0123456789abcdefghijklmnopqrstuvwxyz"
	case "hex":
		return "0123456789abcdef"
	case "number":
		return "0123456789"
	default:
		return table
	}
}

func (c *config) newSessionID() string {
	if c.sessionTable != "" && c.sessionLength.valid() {
		length := c.sessionLength.random()
		id := make([]byte, length)
		for i := range id {
			id[i] = c.sessionTable[c.randomTableIndex()]
		}
		return string(id)
	}
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}

func (c *config) randomTableIndex() int {
	var raw [1]byte
	_, _ = rand.Read(raw[:])
	return int(raw[0]) % len(c.sessionTable)
}

func (c *config) requestURL(scheme, host string) url.URL {
	return url.URL{Scheme: scheme, Host: host, Path: c.path, RawQuery: c.query}
}
