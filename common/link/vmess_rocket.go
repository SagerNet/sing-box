package link

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Link = (*VMessRocket)(nil)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "VMess ShadowRocket",
		Scheme: []string{"vmess"},
		Parse: func(input string) (Link, error) {
			return ParseVMessRocket(input)
		},
	}))
}

// VMessRocket is the vmess link of ShadowRocket
type VMessRocket struct {
	vmess
}

// String implements Link
func (v VMessRocket) String() string {
	mhp := fmt.Sprintf("%s:%s@%s:%d", v.Type, v.ID, v.Add, v.Port)
	qs := url.Values{}
	qs.Add("remarks", v.Ps)
	if v.Net == "ws" {
		qs.Add("obfs", "websocket")
	}
	if v.Host != "" {
		qs.Add("obfsParam", v.Host)
	}
	if v.Path != "" {
		qs.Add("path", v.Host)
	}
	if v.TLS == "tls" {
		qs.Add("tls", "1")
	}

	url := url.URL{
		Scheme:   "vmess",
		Host:     base64.URLEncoding.EncodeToString([]byte(mhp)),
		RawQuery: qs.Encode(),
	}

	return url.String()
}

// ParseVMessRocket parses ShadowRocket vemss link string to VMessRocket
func ParseVMessRocket(vmess string) (*VMessRocket, error) {
	url, err := url.Parse(vmess)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "vmess" {
		return nil, E.New("not a vmess:// link")
	}
	link := &VMessRocket{}
	link.Ver = "2"
	link.OrigLink = vmess

	b64 := url.Host
	b, err := base64Decode(b64)
	if err != nil {
		return nil, err
	}

	mhp := strings.SplitN(string(b), ":", 3)
	if len(mhp) != 3 {
		return nil, fmt.Errorf("vmess unreconized: method:host:port -- %v", mhp)
	}
	port, err := strconv.ParseUint(mhp[2], 10, 16)
	if err != nil {
		return nil, E.Cause(err, "invalid port")
	}
	// mhp[0] is the encryption method
	link.Port = uint16(port)
	idadd := strings.SplitN(mhp[1], "@", 2)
	if len(idadd) != 2 {
		return nil, fmt.Errorf("vmess unreconized: id@addr -- %v", idadd)
	}
	link.ID = idadd[0]
	link.Add = idadd[1]
	link.Aid = 0

	for key, values := range url.Query() {
		switch key {
		case "remarks":
			link.Ps = firstValueOf(values)
		case "path":
			link.Path = firstValueOf(values)
		case "tls":
			link.TLS = firstValueOf(values)
		case "obfs":
			v := firstValueOf(values)
			switch v {
			case "websocket":
				link.Net = "ws"
			case "none":
				link.Net = ""
			}
		case "obfsParam":
			link.Host = firstValueOf(values)
		default:
			return nil, fmt.Errorf("unsupported shadowrocket vmess parameter: %s=%v", key, values)
		}
	}
	return link, nil
}

func firstValueOf(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
