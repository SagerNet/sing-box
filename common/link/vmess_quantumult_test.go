package link_test

import (
	"net/url"
	"testing"

	"github.com/sagernet/sing-box/common/link"
	C "github.com/sagernet/sing-box/constant"
)

func TestVMessQuantumult(t *testing.T) {
	tests := []struct {
		link string
		want link.Vmess
	}{
		{
			link: "vmess://cHMgPSB2bWVzcywxOTIuMTY4LjEwMC4xLDQ0MyxhZXMtMTI4LWdjbSwidXVpZCIsb3Zlci10bHM9dHJ1ZSxjZXJ0aWZpY2F0ZT0wLG9iZnM9d3Msb2Jmcy1wYXRoPSIvcGF0aCIsb2Jmcy1oZWFkZXI9Ikhvc3Q6aG9zdFtScl1bTm5dd2hhdGV2ZXI=",
			want: link.Vmess{
				Tag:              "ps",
				Server:           "192.168.100.1",
				ServerPort:       443,
				UUID:             "uuid",
				AlterID:          0,
				Security:         "aes-128-gcm",
				Host:             "host",
				Transport:        C.V2RayTransportTypeWebsocket,
				TransportPath:    "/path",
				TLS:              true,
				TLSAllowInsecure: true,
			},
		},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.link)
		if err != nil {
			t.Fatal(err)
		}
		link := link.VMessQuantumult{}
		err = link.Parse(u)
		if err != nil {
			t.Error(err)
			return
		}
		if link.Vmess != tt.want {
			t.Errorf("want %#v, got %#v", tt.want, link.Vmess)
		}
	}
}
