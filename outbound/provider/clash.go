package provider

import (
	"context"
	"fmt"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"gopkg.in/yaml.v3"
)

var (
	clashProxyParsers = make(map[string]func(context.Context, adapter.Router,
		log.ContextLogger, map[string]interface{}) (adapter.Outbound, error))
)

type ClashProviderResolver struct {
}

func (r *ClashProviderResolver) GetOutbounds(rawData []byte, ctx context.Context,
	router adapter.Router, logger log.ContextLogger) []adapter.Outbound {
	data := make(map[string]interface{}, 0)
	yaml.Unmarshal(rawData, &data)
	outbounds := make([]adapter.Outbound, 0)
	proxies, ok := data["proxies"]
	if !ok {
		return outbounds
	}
	proxyList, ok := proxies.([]interface{})
	if !ok {
		return outbounds
	}
	for _, proxy := range proxyList {
		if proxyItem, ok := proxy.(map[string]interface{}); ok {
			newOutbound, err := parseClashProxy(ctx, router, logger, proxyItem)
			if err == nil {
				outbounds = append(outbounds, newOutbound)
			}
		}
	}
	return outbounds
}

func parseClashProxy(ctx context.Context, router adapter.Router,
	logger log.ContextLogger, proxyItem map[string]interface{}) (res adapter.Outbound, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("cannot parse proxy: ", r)
			res = nil
			err = fmt.Errorf("cannot parse proxy: %v", r)
		}
	}()
	if proxyType, ok := proxyItem["type"].(string); ok {
		if parser, ok := clashProxyParsers[proxyType]; ok {
			return parser(ctx, router, logger, proxyItem)
		}
	}
	return nil, fmt.Errorf("unkown proxy type")
}

func parseClashSsProxy(ctx context.Context, router adapter.Router,
	logger log.ContextLogger, proxyItem map[string]interface{}) (res adapter.Outbound, err error) {
	options := option.ShadowsocksOutboundOptions{
		DialerOptions: option.DialerOptions{},
		ServerOptions: option.ServerOptions{
			Server:     proxyItem["server"].(string),
			ServerPort: uint16(proxyItem["port"].(int))},
		Password: proxyItem["password"].(string),
		Method:   proxyItem["cipher"].(string),
	}
	if udpEnabled, ok := proxyItem["udp"].(bool); ok && !udpEnabled {
		options.Network = "tcp"
	}
	return outbound.NewShadowsocks(
		ctx, router, logger,
		proxyItem["name"].(string),
		options,
	)
}

func ParseClashTrojanProxy(ctx context.Context, router adapter.Router,
	logger log.ContextLogger, proxyItem map[string]interface{}) (res adapter.Outbound, err error) {
	options := option.TrojanOutboundOptions{
		DialerOptions: option.DialerOptions{},
		ServerOptions: option.ServerOptions{
			Server:     proxyItem["server"].(string),
			ServerPort: uint16(proxyItem["port"].(int))},
		Password:  proxyItem["password"].(string),
		TLS:       &option.OutboundTLSOptions{},
		Multiplex: &option.MultiplexOptions{},
		Transport: &option.V2RayTransportOptions{},
	}
	if udpEnabled, ok := proxyItem["udp"].(bool); ok && !udpEnabled {
		options.Network = "tcp"
	}
	if sni, ok := proxyItem["sni"].(string); ok {
		options.TLS.ServerName = sni
	}
	if skipCertVerity, ok := proxyItem["skip-cert-verify"].(bool); ok && skipCertVerity {
		options.TLS.Insecure = true
	}
	return outbound.NewTrojan(ctx, router, logger, proxyItem["name"].(string), options)
}

func init() {
	clashProxyParsers["ss"] = parseClashSsProxy
	clashProxyParsers["trojan"] = ParseClashTrojanProxy
	outbound.InjectClashProviderResolver("clash", &ClashProviderResolver{})
}
