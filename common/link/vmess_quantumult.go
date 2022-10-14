package link

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "Quantumult",
		Scheme: []string{"vmess"},
		Parse: func(input string) (Link, error) {
			return ParseVMessQuantumult(input)
		},
	}))
}

// VMessQuantumult is the vmess link of Quantumult
type VMessQuantumult struct {
	vmess
}

// String implements Link
func (v VMessQuantumult) String() string {
	/*
	   let obfs = `,obfs=${jsonConf.net === 'ws' ? 'ws' : 'http'},obfs-path="${jsonConf.path || '/'}",obfs-header="Host:${jsonConf.host || jsonConf.add}[Rr][Nn]User-Agent:${ua}"`
	   let quanVmess  = `${jsonConf.ps} = vmess,${jsonConf.add},${jsonConf.port},${method},"${jsonConf.id}",over-tls=${jsonConf.tls === 'tls' ? 'true' : 'false'},certificate=1${jsonConf.type === 'none' && jsonConf.net !== 'ws' ? '' : obfs},group=${group}`
	*/

	method := "aes-128-gcm"
	vbase := fmt.Sprintf("%s = vmess,%s,%d,%s,\"%s\",over-tls=%v,certificate=1", v.Ps, v.Add, v.Port, method, v.ID, v.TLS == "tls")

	var obfs string
	if (v.Net == "ws" || v.Net == "http") && (v.Type == "none" || v.Type == "") {
		if v.Path == "" {
			v.Path = "/"
		}
		if v.Host == "" {
			v.Host = v.Add
		}
		obfs = fmt.Sprintf(`,obfs=ws,obfs-path="%s",obfs-header="Host:%s[Rr][Nn]User-Agent:Mozilla/5.0 (iPhone; CPU iPhone OS 12_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/16A5366a"`, v.Path, v.Host)
	}

	vbase += obfs
	vbase += ",group=Fndroid"
	return "vmess://" + base64.URLEncoding.EncodeToString([]byte(vbase))
}

// ParseVMessQuantumult parses Quantumult vemss link
func ParseVMessQuantumult(vmess string) (*VMessQuantumult, error) {
	if !strings.HasPrefix(vmess, "vmess://") {
		return nil, fmt.Errorf("vmess unreconized: %s", vmess)
	}
	b64 := vmess[8:]
	b, err := base64Decode(b64)
	if err != nil {
		return nil, err
	}

	info := string(b)
	v := &VMessQuantumult{}
	v.OrigLink = vmess
	v.Ver = "2"

	psn := strings.SplitN(info, " = ", 2)
	if len(psn) != 2 {
		return nil, fmt.Errorf("part error: %s", info)
	}

	v.Ps = psn[0]
	params := strings.Split(psn[1], ",")
	port, err := strconv.ParseUint(params[2], 10, 16)
	if err != nil {
		return nil, E.Cause(err, "invalid port")
	}
	v.Add = params[1]
	v.Port = uint16(port)
	v.ID = strings.Trim(params[4], "\"")
	v.Aid = 0
	v.Net = "tcp"
	v.Type = "none"

	if len(params) > 4 {
		for _, pkv := range params[5:] {
			kvp := strings.SplitN(pkv, "=", 2)
			switch kvp[0] {
			case "over-tls":
				if kvp[1] == "true" {
					v.TLS = "tls"
				}
			case "obfs":
				switch kvp[1] {
				case "ws", "http":
					v.Net = kvp[1]
				default:
					return nil, fmt.Errorf("unsupported quantumult vmess obfs parameter: %s", kvp[1])
				}
			case "obfs-path":
				v.Path = strings.Trim(kvp[1], "\"")
			case "obfs-header":
				hd := strings.Trim(kvp[1], "\"")
				for _, hl := range strings.Split(hd, "[Rr][Nn]") {
					if strings.HasPrefix(hl, "Host:") {
						host := hl[5:]
						if host != v.Add {
							v.Host = host
						}
						break
					}
				}
			default:
				return nil, fmt.Errorf("unsupported quantumult vmess parameter: %s", pkv)
			}
		}
	}
	return v, nil
}
