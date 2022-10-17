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

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "Quantumult",
		Scheme: []string{"vmess"},
		Parse: func(u *url.URL) (Link, error) {
			link := &VMessQuantumult{}
			return link, link.Parse(u)
		},
	}))
}

// VMessQuantumult is the vmess link of Quantumult
type VMessQuantumult struct {
	Vmess
}

// Parse implements Link
func (l *VMessQuantumult) Parse(u *url.URL) error {
	if u.Scheme != "vmess" {
		return E.New("not a vmess link")
	}
	b, err := base64Decode(u.Host + u.Path)
	if err != nil {
		return err
	}

	info := string(b)

	psn := strings.SplitN(info, " = ", 2)
	if len(psn) != 2 {
		return fmt.Errorf("part error: %s", info)
	}

	l.Tag = psn[0]
	params := strings.Split(psn[1], ",")
	port, err := strconv.ParseUint(params[2], 10, 16)
	if err != nil {
		return E.Cause(err, "invalid port")
	}
	l.Server = params[1]
	l.ServerPort = uint16(port)
	l.Security = params[3]
	l.UUID = strings.Trim(params[4], "\"")
	l.AlterID = 0
	l.Transport = ""

	if len(params) > 4 {
		for _, pkv := range params[5:] {
			kvp := strings.SplitN(pkv, "=", 2)
			switch kvp[0] {
			case "over-tls":
				if kvp[1] == "true" {
					l.TLS = true
				}
			case "obfs":
				switch kvp[1] {
				case "ws":
					l.Transport = C.V2RayTransportTypeWebsocket
				case "http":
					l.Transport = C.V2RayTransportTypeHTTP
				default:
					return fmt.Errorf("unsupported quantumult vmess obfs parameter: %s", kvp[1])
				}
			case "obfs-path":
				l.TransportPath = strings.Trim(kvp[1], "\"")
			case "obfs-header":
				hd := strings.Trim(kvp[1], "\"")
				for _, hl := range strings.Split(hd, "[Rr][Nn]") {
					if strings.HasPrefix(hl, "Host:") {
						l.Host = hl[5:]
						break
					}
				}
			case "certificate":
				switch kvp[1] {
				case "0":
					l.TLSAllowInsecure = true
				default:
					l.TLSAllowInsecure = false
				}
				// default:
				// 	return fmt.Errorf("unsupported quantumult vmess parameter: %s", pkv)
			}
		}
	}
	return nil
}
