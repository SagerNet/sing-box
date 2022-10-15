package link

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

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
	vmess
}

// Parse implements Link
func (l *VMessQuantumult) Parse(u *url.URL) error {
	if u.Scheme != "vmess" {
		return E.New("not a vmess link")
	}
	b, err := base64Decode(u.Host)
	if err != nil {
		return err
	}

	info := string(b)
	l.Ver = "2"

	psn := strings.SplitN(info, " = ", 2)
	if len(psn) != 2 {
		return fmt.Errorf("part error: %s", info)
	}

	l.Ps = psn[0]
	params := strings.Split(psn[1], ",")
	port, err := strconv.ParseUint(params[2], 10, 16)
	if err != nil {
		return E.Cause(err, "invalid port")
	}
	l.Add = params[1]
	l.Port = uint16(port)
	l.ID = strings.Trim(params[4], "\"")
	l.Aid = 0
	l.Net = "tcp"
	l.Type = "none"

	if len(params) > 4 {
		for _, pkv := range params[5:] {
			kvp := strings.SplitN(pkv, "=", 2)
			switch kvp[0] {
			case "over-tls":
				if kvp[1] == "true" {
					l.TLS = "tls"
				}
			case "obfs":
				switch kvp[1] {
				case "ws", "http":
					l.Net = kvp[1]
				default:
					return fmt.Errorf("unsupported quantumult vmess obfs parameter: %s", kvp[1])
				}
			case "obfs-path":
				l.Path = strings.Trim(kvp[1], "\"")
			case "obfs-header":
				hd := strings.Trim(kvp[1], "\"")
				for _, hl := range strings.Split(hd, "[Rr][Nn]") {
					if strings.HasPrefix(hl, "Host:") {
						host := hl[5:]
						if host != l.Add {
							l.Host = host
						}
						break
					}
				}
			default:
				return fmt.Errorf("unsupported quantumult vmess parameter: %s", pkv)
			}
		}
	}
	return nil
}
