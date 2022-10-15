package link

import (
	"encoding/json"
	"net/url"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "V2RayNG",
		Scheme: []string{"vmess"},
		Parse: func(u *url.URL) (Link, error) {
			link := &VMessV2RayNG{}
			return link, link.Parse(u)
		},
	}))
}

// VMessV2RayNG is the vmess link of V2RayNG
type VMessV2RayNG struct {
	vmess
}

// Parse implements Link
func (l *VMessV2RayNG) Parse(u *url.URL) error {
	if u.Scheme != "vmess" {
		return E.New("not a vmess link")
	}

	b64 := u.Host
	b, err := base64Decode(b64)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, l); err != nil {
		return err
	}

	return nil
}
