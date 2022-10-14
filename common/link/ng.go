package link

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sagernet/sing/common"
)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "V2RayNG",
		Scheme: []string{"vmess"},
		Parse: func(input string) (Link, error) {
			return ParseVMessV2RayNG(input)
		},
	}))
}

// VMessV2RayNG is the vmess link of V2RayNG
type VMessV2RayNG struct {
	vmess
}

// String implements Link
func (v VMessV2RayNG) String() string {
	b, _ := json.Marshal(v)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

// ParseVMessV2RayNG parses V2RayN vemss link
func ParseVMessV2RayNG(vmess string) (*VMessV2RayNG, error) {
	if !strings.HasPrefix(vmess, "vmess://") {
		return nil, fmt.Errorf("vmess unreconized: %s", vmess)
	}

	b64 := vmess[8:]
	b, err := base64Decode(b64)
	if err != nil {
		return nil, err
	}

	v := &VMessV2RayNG{}
	if err := json.Unmarshal(b, v); err != nil {
		return nil, err
	}
	v.OrigLink = vmess

	return v, nil
}
