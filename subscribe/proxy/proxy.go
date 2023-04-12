package proxy

import (
	"fmt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"strings"
)

type Proxy interface {
	GetTag() string                        // 获取节点名称
	GetType() string                       // 获取节点类型
	SetDialer(dialer option.DialerOptions) // 设置Dialer
	ParseLink(link string) error           // 解析链接
	// ParseClash(config string) error                     // 解析Clash配置
	GenerateOutboundOptions() (option.Outbound, error) // 获取配置
}

func CheckLink(link string) string {
	switch {
	case strings.Index(link, "http://") == 0:
		return C.TypeHTTP
	case strings.Index(link, "https://") == 0:
		return C.TypeHTTP
	case strings.Index(link, "socks") == 0:
		return C.TypeSocks
	case strings.Index(link, "socks4") == 0:
		return C.TypeSocks
	case strings.Index(link, "socks4a") == 0:
		return C.TypeSocks
	case strings.Index(link, "socks5") == 0:
		return C.TypeSocks
	case strings.Index(link, "socks5h") == 0:
		return C.TypeSocks
	case strings.Index(link, "vmess://") == 0:
		return C.TypeVMess
	case strings.Index(link, "ss://") == 0:
		return C.TypeShadowsocks
	case strings.Index(link, "ssr://") == 0:
		return C.TypeShadowsocksR
	case strings.Index(link, "trojan://") == 0:
		return C.TypeTrojan
	case strings.Index(link, "hysteria://") == 0:
		return C.TypeHysteria
	default:
		return ""
	}
}

func ParsePeers(content string) ([]Proxy, error) {
	raw, err := Base64Decode(content)
	if err != nil {
		return nil, fmt.Errorf("parse peers failed: %s", err)
	}

	peerLinks := strings.Split(string(raw), "\r\n")

	peers := make([]Proxy, 0)

	for _, link := range peerLinks {
		proxyType := CheckLink(link)
		var proxy Proxy
		switch proxyType {
		case C.TypeHTTP:
			proxy = &ProxyHTTP{}
		case C.TypeSocks:
			proxy = &ProxySocks{}
		case C.TypeVMess:
			proxy = &ProxyVMess{}
		case C.TypeShadowsocks:
			proxy = &ProxyShadowsocks{}
		case C.TypeShadowsocksR:
			proxy = &ProxyShadowsocksR{}
		case C.TypeTrojan:
			proxy = &ProxyTrojan{}
		case C.TypeHysteria:
			proxy = &ProxyHysteria{}
		default:
			continue
		}
		err := proxy.ParseLink(link)
		if err != nil {
			continue
		}
		peers = append(peers, proxy)
	}

	if len(peers) == 0 {
		return nil, fmt.Errorf("no valid peers")
	}

	return peers, nil
}
