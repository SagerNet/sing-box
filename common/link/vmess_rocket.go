package link

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
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
	Vmess

	Ver string
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
	l.ServerPort = uint16(port)
	idadd := strings.SplitN(mhp[1], "@", 2)
	if len(idadd) != 2 {
		return fmt.Errorf("vmess unreconized: id@addr -- %v", idadd)
	}
	l.UUID = idadd[0]
	l.Server = idadd[1]
	l.AlterID = 0
	l.Security = "auto"

	for key, values := range u.Query() {
		switch key {
		case "remarks":
			l.Tag = firstValueOf(values)
		case "path":
			l.TransportPath = firstValueOf(values)
		case "tls":
			l.TLS = firstValueOf(values) == "tls"
		case "obfs":
			v := firstValueOf(values)
			switch v {
			case "ws", "websocket":
				l.Transport = C.V2RayTransportTypeWebsocket
			case "http":
				l.Transport = C.V2RayTransportTypeHTTP
			}
		case "obfsParam":
			l.Host = firstValueOf(values)
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
