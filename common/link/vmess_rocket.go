package link

import (
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
		Parse: func(u *url.URL) (Link, error) {
			link := &VMessRocket{}
			return link, link.Parse(u)
		},
	}))
}

// VMessRocket is the vmess link of ShadowRocket
type VMessRocket struct {
	vmess
}

// Parse implements Link
func (l *VMessRocket) Parse(u *url.URL) error {
	if u.Scheme != "vmess" {
		return E.New("not a vmess link")
	}

	l.Ver = "2"

	b, err := base64Decode(u.Host)
	if err != nil {
		return err
	}

	mhp := strings.SplitN(string(b), ":", 3)
	if len(mhp) != 3 {
		return fmt.Errorf("vmess unreconized: method:host:port -- %v", mhp)
	}
	port, err := strconv.ParseUint(mhp[2], 10, 16)
	if err != nil {
		return E.Cause(err, "invalid port")
	}
	// mhp[0] is the encryption method
	l.Port = uint16(port)
	idadd := strings.SplitN(mhp[1], "@", 2)
	if len(idadd) != 2 {
		return fmt.Errorf("vmess unreconized: id@addr -- %v", idadd)
	}
	l.ID = idadd[0]
	l.Add = idadd[1]
	l.Aid = 0

	for key, values := range u.Query() {
		switch key {
		case "remarks":
			l.Ps = firstValueOf(values)
		case "path":
			l.Path = firstValueOf(values)
		case "tls":
			l.TLS = firstValueOf(values)
		case "obfs":
			v := firstValueOf(values)
			switch v {
			case "websocket":
				l.Net = "ws"
			case "none":
				l.Net = ""
			}
		case "obfsParam":
			l.Host = firstValueOf(values)
		default:
			return fmt.Errorf("unsupported shadowrocket vmess parameter: %s=%v", key, values)
		}
	}
	return nil
}

func firstValueOf(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
