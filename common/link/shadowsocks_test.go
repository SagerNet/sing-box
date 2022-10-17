package link_test

import (
	"net/url"
	"testing"

	"github.com/sagernet/sing-box/common/link"
)

func TestShadowSocks(t *testing.T) {
	tests := []struct {
		link string
		want link.ShadowSocks
	}{
		{
			link: "ss://YWVzLTEyOC1nY206dGVzdA@192.168.100.1:8888#Example1",
			want: link.ShadowSocks{
				Address:  "192.168.100.1",
				Port:     8888,
				Ps:       "Example1",
				Method:   "aes-128-gcm",
				Password: "test",
			},
		},
		{
			link: "ss://cmM0LW1kNTpwYXNzd2Q@192.168.100.1:8888/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host=abc.com#Example2",
			want: link.ShadowSocks{
				Address:    "192.168.100.1",
				Port:       8888,
				Ps:         "Example2",
				Method:     "rc4-md5",
				Password:   "passwd",
				Plugin:     "obfs-local",
				PluginOpts: "obfs=http;obfs-host=abc.com",
			},
		},
		{
			link: "ss://2022-blake3-aes-256-gcm:YctPZ6U7xPPcU%2Bgp3u%2B0tx%2FtRizJN9K8y%2BuKlW2qjlI%3D@192.168.100.1:8888#Example3",
			want: link.ShadowSocks{
				Address:  "192.168.100.1",
				Port:     8888,
				Ps:       "Example3",
				Method:   "2022-blake3-aes-256-gcm",
				Password: "YctPZ6U7xPPcU gp3u 0tx/tRizJN9K8y uKlW2qjlI=",
			},
		},
		{
			link: "ss://2022-blake3-aes-256-gcm:YctPZ6U7xPPcU%2Bgp3u%2B0tx%2FtRizJN9K8y%2BuKlW2qjlI%3D@192.168.100.1:8888/?plugin=v2ray-plugin%3Bserver&unsupported-arguments=should-be-ignored#Example3",
			want: link.ShadowSocks{
				Address:    "192.168.100.1",
				Port:       8888,
				Ps:         "Example3",
				Method:     "2022-blake3-aes-256-gcm",
				Password:   "YctPZ6U7xPPcU gp3u 0tx/tRizJN9K8y uKlW2qjlI=",
				Plugin:     "v2ray-plugin",
				PluginOpts: "server",
			},
		},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.link)
		if err != nil {
			t.Fatal(err)
		}
		link := link.ShadowSocks{}
		err = link.Parse(u)
		if err != nil {
			t.Error(err)
			return
		}
		if link != tt.want {
			t.Errorf("want %v, got %v", tt.want, link)
		}
	}
}
