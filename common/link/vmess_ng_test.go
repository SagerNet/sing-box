package link_test

import (
	"net/url"
	"testing"

	"github.com/sagernet/sing-box/common/link"
	C "github.com/sagernet/sing-box/constant"
)

func TestVMessV2RayNG(t *testing.T) {
	tests := []struct {
		link string
		want link.Vmess
	}{
		{
			link: "vmess://ewoiYWRkIjogIjE5Mi4xNjguMTAwLjEiLAoidiI6ICIyIiwKInBzIjogInBzIiwKInBvcnQiOiA0NDMsCiJpZCI6ICJ1dWlkIiwKImFpZCI6ICI0IiwKIm5ldCI6ICJ3cyIsCiJ0eXBlIjogInR5cGUiLAoiaG9zdCI6ICJob3N0IiwKInBhdGgiOiAiL3BhdGgiLAoidGxzIjogInRscyIsCiJzbmkiOiAic25pIiwKImFscG4iOiJhbHBuIiwKInNlY3VyaXR5IjogImF1dG8iLAoic2tpcC1jZXJ0LXZlcmlmeSI6IGZhbHNlCn0=",
			want: link.Vmess{
				Tag:              "ps",
				Server:           "192.168.100.1",
				ServerPort:       443,
				UUID:             "uuid",
				AlterID:          4,
				Security:         "auto",
				Host:             "host",
				Transport:        C.V2RayTransportTypeWebsocket,
				TransportPath:    "/path",
				TLS:              true,
				TLSAllowInsecure: false,
			},
		},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.link)
		if err != nil {
			t.Fatal(err)
		}
		link := link.VMessV2RayNG{}
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
