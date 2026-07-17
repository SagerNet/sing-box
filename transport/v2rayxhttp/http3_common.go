package v2rayxhttp

import "github.com/sagernet/sing-box/common/tls"

func isHTTP3(config tls.Config) bool {
	if config == nil {
		return false
	}
	nextProtos := config.NextProtos()
	return len(nextProtos) == 1 && nextProtos[0] == "h3"
}
