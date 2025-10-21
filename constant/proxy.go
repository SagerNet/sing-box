package constant

const (
	TypeTun          = "tun"
	TypeRedirect     = "redirect"
	TypeTProxy       = "tproxy"
	TypeDirect       = "direct"
	TypeBlock        = "block"
	TypeDNS          = "dns"
	TypeSOCKS        = "socks"
	TypeHTTP         = "http"
	TypeMixed        = "mixed"
	TypeShadowsocks  = "shadowsocks"
	TypeVMess        = "vmess"
	TypeTrojan       = "trojan"
	TypeNaive        = "naive"
	TypeWireGuard    = "wireguard"
	TypeHysteria     = "hysteria"
	TypeTor          = "tor"
	TypeSSH          = "ssh"
	TypeShadowTLS    = "shadowtls"
	TypeAnyTLS       = "anytls"
	TypeShadowsocksR = "shadowsocksr"
	TypeVLESS        = "vless"
	TypeTUIC         = "tuic"
	TypeHysteria2    = "hysteria2"
	TypeTailscale    = "tailscale"
	TypeDERP         = "derp"
	TypeResolved     = "resolved"
	TypeSSMAPI       = "ssm-api"
	TypeCCM          = "ccm"
)

const (
	TypeSelector = "selector"
	TypeURLTest  = "urltest"
)

func ProxyDisplayName(proxyType string) string {
	switch proxyType {
	case TypeTun:
		return "TUN"
	case TypeRedirect:
		return "Redirect"
	case TypeTProxy:
		return "TProxy"
	case TypeDirect:
		return "Direct"
	case TypeBlock:
		return "Block"
	case TypeDNS:
		return "DNS"
	case TypeSOCKS:
		return "SOCKS"
	case TypeHTTP:
		return "HTTP"
	case TypeMixed:
		return "Mixed"
	case TypeShadowsocks:
		return "Shadowsocks"
	case TypeVMess:
		return "VMess"
	case TypeTrojan:
		return "Trojan"
	case TypeNaive:
		return "Naive"
	case TypeWireGuard:
		return "WireGuard"
	case TypeHysteria:
		return "Hysteria"
	case TypeTor:
		return "Tor"
	case TypeSSH:
		return "SSH"
	case TypeShadowTLS:
		return "ShadowTLS"
	case TypeShadowsocksR:
		return "ShadowsocksR"
	case TypeVLESS:
		return "VLESS"
	case TypeTUIC:
		return "TUIC"
	case TypeHysteria2:
		return "Hysteria2"
	case TypeAnyTLS:
		return "AnyTLS"
	case TypeSelector:
		return "Selector"
	case TypeURLTest:
		return "URLTest"
	default:
		return "Unknown"
	}
}
