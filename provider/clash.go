package provider

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"regexp"
	"runtime"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"

	"gopkg.in/yaml.v3"
)

type ClashConfig struct {
	Proxies []map[string]any `yaml:"proxies"`
}

func convertTLSOptions(proxy map[string]any) *option.OutboundTLSOptions {
	options := option.OutboundTLSOptions{
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	if tls, exists := proxy["tls"].(bool); exists && tls {
		options.Enabled = true
	}
	if insecure, exists := proxy["skip-cert-verify"].(bool); exists {
		options.Enabled = true
		options.Insecure = insecure
	}
	if sni, exists := proxy["sni"].(string); exists {
		options.ServerName = sni
	}
	if peer, exists := proxy["peer"].(string); exists {
		options.ServerName = peer
	}
	if servername, exists := proxy["servername"].(string); exists {
		options.ServerName = servername
	}
	if disableSNI, exists := proxy["disable-sni"].(bool); exists {
		options.DisableSNI = disableSNI
	}
	if alpn, exists := proxy["alpn"].([]any); exists {
		alpnArr := []string{}
		for _, item := range alpn {
			alpnArr = append(alpnArr, fmt.Sprint(item))
		}
		options.ALPN = alpnArr
	}
	if fingerprint, exists := proxy["client-fingerprint"].(string); exists {
		options.Enabled = true
		options.UTLS.Enabled = true
		options.UTLS.Fingerprint = fingerprint
	}
	if reality, exists := proxy["reality-opts"].(map[string]any); exists {
		options.Enabled = true
		options.Reality.Enabled = true
		if pbk, exists := reality["public-key"].(string); exists {
			options.Reality.PublicKey = pbk
		}
		if sid, exists := reality["short-id"].(string); exists {
			options.Reality.ShortID = sid
		}
	}
	return &options
}

func convertSMuxOptions(proxy map[string]any) *option.OutboundMultiplexOptions {
	options := option.OutboundMultiplexOptions{
		Enabled: false,
	}
	smux, exists := proxy["smux"].(map[string]any)
	if !exists {
		return &options
	}
	if enabled, exists := smux["enabled"].(bool); exists {
		options.Enabled = enabled
	}
	if protocol, exists := smux["protocol"].(string); exists {
		options.Protocol = protocol
	}
	if maxConnections, exists := smux["max-connections"].(int); exists {
		options.MaxConnections = maxConnections
	}
	if maxStreams, exists := smux["max-streams"].(int); exists {
		options.MaxStreams = maxStreams
	}
	if minStreams, exists := smux["min-streams"].(int); exists {
		options.MinStreams = minStreams
	}
	return &options
}

func convertWSTransport(proxy map[string]any) option.V2RayWebsocketOptions {
	options := option.V2RayWebsocketOptions{
		Headers: map[string]option.Listable[string]{},
	}
	if wsOpts, exists := proxy["ws-opts"].(map[string]any); exists {
		if path, exists := wsOpts["path"].(string); exists {
			reg := regexp.MustCompile(`^(.*?)(?:\?ed=(\d+))?$`)
			result := reg.FindStringSubmatch(path)
			options.Path = result[1]
			if result[2] != "" {
				options.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
				options.MaxEarlyData = stringToUint32(result[2])
			}
		}
		if headers, exists := wsOpts["headers"].(map[string]any); exists {
			for key, valueRaw := range headers {
				valueArr := []string{}
				switch value := valueRaw.(type) {
				case []any:
					for _, item := range value {
						valueArr = append(valueArr, fmt.Sprint(item))
					}
				default:
					valueArr = append(valueArr, fmt.Sprint(value))
				}
				options.Headers[key] = valueArr
			}
		}
		if maxEarlyData, exists := wsOpts["max-early-data"].(int); exists {
			options.MaxEarlyData = uint32(maxEarlyData)
		}
		if earlyDataHeaderName, exists := wsOpts["early-data-header-name"].(string); exists {
			options.EarlyDataHeaderName = earlyDataHeaderName
		}
	}
	if path, exists := proxy["ws-path"].(string); exists {
		reg := regexp.MustCompile(`^(.*?)(?:\?ed=(\d+))?$`)
		result := reg.FindStringSubmatch(path)
		options.Path = result[1]
		if result[2] != "" {
			options.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
			options.MaxEarlyData = stringToUint32(result[2])
		}
	}
	if headers, exists := proxy["ws-headers"].(map[string]any); exists {
		for key, valueRaw := range headers {
			valueArr := []string{}
			switch value := valueRaw.(type) {
			case []any:
				for _, item := range value {
					valueArr = append(valueArr, fmt.Sprint(item))
				}
			default:
				valueArr = append(valueArr, fmt.Sprint(value))
			}
			options.Headers[key] = valueArr
		}
	}
	return options
}

func convertHTTPUpgradeTransport(proxy map[string]any) option.V2RayHTTPUpgradeOptions {
	options := option.V2RayHTTPUpgradeOptions{
		Headers: map[string]option.Listable[string]{},
	}
	wsOpts := proxy["ws-opts"].(map[string]any)
	if path, exists := wsOpts["path"].(string); exists {
		options.Path = path
	}
	if headers, exists := wsOpts["headers"].(map[string]any); exists {
		for key, valueRaw := range headers {
			valueArr := []string{}
			switch value := valueRaw.(type) {
			case []any:
				for _, item := range value {
					valueArr = append(valueArr, fmt.Sprint(item))
				}
			default:
				valueArr = append(valueArr, fmt.Sprint(value))
			}
			if strings.ToLower(key) == "host" {
				options.Host = valueArr[0]
				continue
			}
			options.Headers[key] = valueArr
		}
	}
	if path, exists := proxy["ws-path"].(string); exists {
		options.Path = path
	}
	if headers, exists := proxy["ws-headers"].(map[string]any); exists {
		for key, valueRaw := range headers {
			valueArr := []string{}
			switch value := valueRaw.(type) {
			case []any:
				for _, item := range value {
					valueArr = append(valueArr, fmt.Sprint(item))
				}
			default:
				valueArr = append(valueArr, fmt.Sprint(value))
			}
			if strings.ToLower(key) == "host" {
				options.Host = valueArr[0]
				continue
			}
			options.Headers[key] = valueArr
		}
	}
	return options
}

func convertHTTPTransport(proxy map[string]any) option.V2RayHTTPOptions {
	options := option.V2RayHTTPOptions{
		Host:    option.Listable[string]{},
		Headers: map[string]option.Listable[string]{},
	}
	if httpOpts, exists := proxy["http-opts"].(map[string]any); exists {
		if method, exists := httpOpts["method"].(string); exists {
			options.Method = method
		}
		if pathRaw, exists := httpOpts["path"]; exists {
			switch path := pathRaw.(type) {
			case []string:
				options.Path = path[0]
			case string:
				options.Path = path
			}
		}
		if hostsRaw, exists := httpOpts["host"]; exists {
			switch hosts := hostsRaw.(type) {
			case []string:
				options.Host = hosts
			case string:
				options.Host = []string{hosts}
			}
		}
		if headers, exists := httpOpts["headers"].(map[string]any); exists {
			for key, valueRaw := range headers {
				valueArr := []string{}
				switch value := valueRaw.(type) {
				case []any:
					for _, item := range value {
						valueArr = append(valueArr, fmt.Sprint(item))
					}
				default:
					valueArr = append(valueArr, fmt.Sprint(value))
				}
				options.Headers[key] = valueArr
			}
		}
	}
	return options
}

func convertH2Transport(proxy map[string]any) option.V2RayHTTPOptions {
	options := option.V2RayHTTPOptions{
		Host:    option.Listable[string]{},
		Headers: map[string]option.Listable[string]{},
	}
	if h2Opts, exists := proxy["h2-opts"].(map[string]any); exists {
		if hostsRaw, exists := h2Opts["host"]; exists {
			switch hosts := hostsRaw.(type) {
			case []string:
				options.Host = hosts
			case string:
				options.Host = []string{hosts}
			}
		}
	}
	return options
}

func convertGRPCTransport(proxy map[string]any) option.V2RayGRPCOptions {
	options := option.V2RayGRPCOptions{}
	if grpcOpts, exists := proxy["grpc-opts"].(map[string]any); exists {
		if servername, exists := grpcOpts["grpc-service-name"].(string); exists {
			options.ServiceName = servername
		}
	}
	return options
}

func convertDialerOption(proxy map[string]any) option.DialerOptions {
	options := option.DialerOptions{}
	if tfo, exists := proxy["tcp-fast-open"].(bool); exists && tfo {
		options.TCPFastOpen = true
	}
	if tfo, exists := proxy["tfo"].(bool); exists && tfo {
		options.TCPFastOpen = true
	}
	if name, exists := proxy["interface-name"].(string); exists {
		options.BindInterface = name
	}
	if mark, exists := proxy["routing-mark"].(uint32); exists {
		if runtime.GOOS == "android" || runtime.GOOS == "linux" {
			options.RoutingMark = mark
		}
	}
	if detour, exists := proxy["dialer-proxy"].(string); exists {
		options.Detour = detour
	}
	if version, exists := proxy["ip-version"].(string); exists {
		var strategy option.DomainStrategy
		switch version {
		case "dual":
			strategy.UnmarshalJSON([]byte(""))
		case "ipv4":
			strategy.UnmarshalJSON([]byte("ipv4_only"))
		case "ipv6":
			strategy.UnmarshalJSON([]byte("ipv6_only"))
		case "ipv4-prefer":
			strategy.UnmarshalJSON([]byte("prefer_ipv4"))
		case "ipv6-prefer":
			strategy.UnmarshalJSON([]byte("prefer_ipv6"))
		}
		options.DomainStrategy = strategy
	}
	return options
}

func newClashParser(content string) ([]option.Outbound, error) {
	var outbounds []option.Outbound
	clashConfig := &ClashConfig{
		Proxies: []map[string]any{},
	}
	err := yaml.Unmarshal([]byte(content), clashConfig)
	if err != nil {
		return outbounds, err
	}
	for _, proxy := range clashConfig.Proxies {
		protocol, exists := proxy["type"]
		if !exists {
			continue
		}
		var (
			outbound option.Outbound
			stlsPart option.Outbound
			err      error
		)
		stlsPart = option.Outbound{}
		switch protocol {
		case "ss":
			if plugin, exists := proxy["plugin"]; exists {
				switch plugin {
				case "shadow-tls":
					outbound, stlsPart, err = newSTLSClashParser(proxy)
				case "obfs", "v2ray-plugin":
					outbound, err = newSSClashParser(proxy)
				default:
					continue
				}
			} else {
				outbound, err = newSSClashParser(proxy)
			}
		case "ssr":
			outbound, err = newSSRClashParser(proxy)
		case "http":
			outbound, err = newHTTPClashParser(proxy)
		case "tuic":
			if _, exists := proxy["token"]; exists {
				continue
			}
			outbound, err = newTUICClashParser(proxy)
		case "vmess":
			outbound, err = newVMessClashParser(proxy)
		case "vless":
			if flow, exists := proxy["flow"].(string); exists && flow != "xtls-rprx-vision" && flow != "" {
				continue
			}
			outbound, err = newVLESSClashParser(proxy)
		case "socks5":
			outbound, err = newSOCKS5ClashParser(proxy)
		case "trojan":
			if _, exists := proxy["flow"].(string); exists {
				continue
			}
			outbound, err = newTrojanClashParser(proxy)
		case "hysteria":
			outbound, err = newHysteriaClashParser(proxy)
		case "hysteria2":
			outbound, err = newHysteria2ClashParser(proxy)
		case "wireguard":
			outbound, err = newWireGuardClashParser(proxy)
		default:
			continue
		}
		if err == nil {
			outbounds = append(outbounds, outbound)
			if stlsPart.Type != "" {
				outbounds = append(outbounds, stlsPart)
			}
		}
	}
	return outbounds, nil
}

func newSSClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeShadowsocks,
	}
	options := option.ShadowsocksOutboundOptions{
		Multiplex: &option.OutboundMultiplexOptions{},
	}
	UoTOptions := option.UDPOverTCPOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if method, exists := proxy["cipher"].(string); exists {
		options.Method = method
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if plugin, exists := proxy["plugin"].(string); exists {
		var optArr []string
		switch plugin {
		case "obfs":
			options.Plugin = "obfs-local"
			if opts, exists := proxy["plugin-opts"].(map[string]any); exists {
				for key, value := range opts {
					switch key {
					case "mode":
						optArr = append(optArr, fmt.Sprint("obfs=", value))
					case "host":
						optArr = append(optArr, fmt.Sprint("obfs-host=", value))
					default:
						optArr = append(optArr, fmt.Sprint(key, "=", value))
					}
				}
			}
		case "v2ray-plugin":
			options.Plugin = "v2ray-plugin"
			if opts, exists := proxy["plugin-opts"].(map[string]any); exists {
				for key, value := range opts {
					switch key {
					case "mode":
						optArr = append(optArr, fmt.Sprint("obfs=", value))
					case "host":
						host := value
						if h, ok := proxy["ws-host"].(string); ok {
							host = h
						}
						optArr = append(optArr, fmt.Sprint("host=", host))
					case "path":
						path := value
						if p, ok := proxy["ws-path"].(string); ok {
							path = p
						}
						optArr = append(optArr, fmt.Sprint("path=", path))
					case "headers":
						headers, _ := value.(map[string]any)
						data, _ := json.Marshal(headers)
						optArr = append(optArr, fmt.Sprint("headers", "=", string(data)))
					case "mux":
						if mux, _ := value.(bool); mux {
							options.Multiplex.Enabled = true
						}
					default:
						optArr = append(optArr, fmt.Sprint(key, "=", value))
					}
				}
			}
		}
		options.PluginOptions = strings.Join(optArr, ";")
	}
	if uot, exists := proxy["uot"].(bool); exists && uot {
		UoTOptions.Enabled = true
	}
	if uot, exists := proxy["udp-over-tcp"].(bool); exists && uot {
		UoTOptions.Enabled = true
	}
	options.Multiplex = convertSMuxOptions(proxy)
	options.UDPOverTCP = &UoTOptions
	options.DialerOptions = convertDialerOption(proxy)
	outbound.ShadowsocksOptions = options
	return outbound, nil
}

func newSTLSClashParser(proxy map[string]any) (option.Outbound, option.Outbound, error) {
	ssPart := option.Outbound{
		Type: C.TypeShadowsocks,
	}
	stlsPart := option.Outbound{
		Type: C.TypeShadowTLS,
	}
	ssOptions := option.ShadowsocksOutboundOptions{}
	stOptions := option.ShadowTLSOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		ssPart.Tag = name
		stlsPart.Tag = name + "-st"
		ssOptions.Detour = name + "-st"
	}
	if server, exists := proxy["server"].(string); exists {
		stOptions.Server = server
	}
	if port, exists := proxy["port"]; exists {
		stOptions.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if method, exists := proxy["cipher"].(string); exists {
		ssOptions.Method = method
	}
	if password, exists := proxy["password"].(string); exists {
		ssOptions.Password = password
	}
	if uot, exists := proxy["uot"].(bool); exists {
		ssOptions.UDPOverTCP = &option.UDPOverTCPOptions{
			Enabled: uot,
		}
	}
	if uot, exists := proxy["udp-over-tcp"].(bool); exists {
		ssOptions.UDPOverTCP = &option.UDPOverTCPOptions{
			Enabled: uot,
		}
	}
	stOptions.TLS = convertTLSOptions(proxy)
	stOptions.TLS.Enabled = true
	stOptions.TLS.UTLS.Enabled = true
	opts, exists := proxy["plugin-opts"].(map[string]any)
	if !exists {
		return ssPart, stlsPart, E.New("missing ShadowTLS")
	}
	if version, exists := opts["version"].(int); exists {
		stOptions.Version = version
	}
	if password, exists := opts["password"].(string); exists {
		stOptions.Password = password
	}
	if host, exists := opts["host"].(string); exists {
		stOptions.TLS.ServerName = host
	}
	ssOptions.Multiplex = convertSMuxOptions(proxy)
	stOptions.DialerOptions = convertDialerOption(proxy)
	ssPart.ShadowsocksOptions = ssOptions
	stlsPart.ShadowTLSOptions = stOptions
	return ssPart, stlsPart, nil
}

func newSSRClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeShadowsocksR,
	}
	options := option.ShadowsocksROutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if method, exists := proxy["cipher"].(string); exists {
		options.Method = method
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if obfs, exists := proxy["obfs"].(string); exists {
		options.Obfs = obfs
	}
	if param, exists := proxy["obfs-param"].(string); exists {
		options.ObfsParam = param
	}
	if protocol, exists := proxy["protocol"].(string); exists {
		options.Protocol = protocol
	}
	if param, exists := proxy["protocol-param"].(string); exists {
		options.ProtocolParam = param
	}
	options.DialerOptions = convertDialerOption(proxy)
	outbound.ShadowsocksROptions = options
	return outbound, nil
}

func newHTTPClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeHTTP,
	}
	options := option.HTTPOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if username, exists := proxy["username"].(string); exists {
		options.Username = username
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if headers, exists := proxy["headers"].(map[string]any); exists {
		options.Headers = map[string]option.Listable[string]{}
		for key, value := range headers {
			valueArr := option.Listable[string]{}
			if arr, exists := value.([]any); exists {
				for _, item := range arr {
					valueArr = append(valueArr, fmt.Sprint(item))
				}
			} else {
				valueArr = append(valueArr, fmt.Sprint(value))
			}
			options.Headers[key] = valueArr
		}
	}
	options.TLS = convertTLSOptions(proxy)
	options.DialerOptions = convertDialerOption(proxy)
	outbound.HTTPOptions = options
	return outbound, nil
}

func newTUICClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeTUIC,
	}
	options := option.TUICOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if UUID, exists := proxy["uuid"].(string); exists {
		options.UUID = UUID
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if cc, exists := proxy["congestion-controller"].(string); exists && cc == "cubic" {
		options.CongestionControl = cc
	}
	if urm, exists := proxy["udp-relay-mode"].(string); exists && urm == "quic" {
		options.UDPRelayMode = "quic"
	}
	if zrtt, exists := proxy["reduce-rtt"].(bool); exists && zrtt {
		options.ZeroRTTHandshake = zrtt
	}
	if uos, exists := proxy["udp-over-stream"].(bool); exists && uos {
		options.UDPOverStream = uos
	}
	if heartbeat, exists := proxy["heartbeat-interval"].(int); exists {
		duration, _ := time.ParseDuration(fmt.Sprint(heartbeat, "ms"))
		options.Heartbeat = option.Duration(duration)
	}
	options.TLS = convertTLSOptions(proxy)
	options.TLS.UTLS.Enabled = false
	options.DialerOptions = convertDialerOption(proxy)
	outbound.TUICOptions = options
	return outbound, nil
}

func newVMessClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeVMess,
	}
	options := option.VMessOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if uuid, exists := proxy["uuid"].(string); exists {
		options.UUID = uuid
	}
	if aid, exists := proxy["alterId"].(int); exists {
		options.AlterId = aid
	}
	if cipher, exists := proxy["cipher"].(string); exists {
		options.Security = cipher
	}
	options.TLS = convertTLSOptions(proxy)
	options.Multiplex = convertSMuxOptions(proxy)
	if network, exists := proxy["network"].(string); exists {
		Transport := option.V2RayTransportOptions{}
		switch network {
		case "ws":
			isHTTPUpgrade := false
			if wsOpts, exists := proxy["ws-opts"].(map[string]any); exists {
				if ok, exists := wsOpts["v2ray-http-upgrade"].(bool); exists && ok {
					isHTTPUpgrade = true
				}
			}
			if isHTTPUpgrade {
				Transport.Type = C.V2RayTransportTypeHTTPUpgrade
				Transport.HTTPUpgradeOptions = convertHTTPUpgradeTransport(proxy)
			} else {
				Transport.Type = C.V2RayTransportTypeWebsocket
				Transport.WebsocketOptions = convertWSTransport(proxy)
			}
		case "http":
			Transport.Type = C.V2RayTransportTypeHTTP
			Transport.HTTPOptions = convertHTTPTransport(proxy)
		case "h2":
			options.TLS.Enabled = true
			Transport.Type = C.V2RayTransportTypeHTTP
			Transport.HTTPOptions = convertH2Transport(proxy)
		case "grpc":
			Transport.Type = C.V2RayTransportTypeGRPC
			Transport.GRPCOptions = convertGRPCTransport(proxy)
		}
		options.Transport = &Transport
	}
	options.DialerOptions = convertDialerOption(proxy)
	outbound.VMessOptions = options
	return outbound, nil
}

func newVLESSClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeVLESS,
	}
	options := option.VLESSOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if uuid, exists := proxy["uuid"].(string); exists {
		options.UUID = uuid
	}
	if flow, exists := proxy["flow"].(string); exists && flow == "xtls-rprx-vision" {
		options.Flow = "xtls-rprx-vision"
	}
	if network, exists := proxy["network"].(string); exists {
		Transport := option.V2RayTransportOptions{}
		switch network {
		case "ws":
			Transport.Type = C.V2RayTransportTypeWebsocket
			Transport.WebsocketOptions = convertWSTransport(proxy)
		case "grpc":
			Transport.Type = C.V2RayTransportTypeGRPC
			Transport.GRPCOptions = convertGRPCTransport(proxy)
		}
		options.Transport = &Transport
	}
	options.TLS = convertTLSOptions(proxy)
	options.Multiplex = convertSMuxOptions(proxy)
	options.DialerOptions = convertDialerOption(proxy)
	outbound.VLESSOptions = options
	return outbound, nil
}

func newSOCKS5ClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeSOCKS,
	}
	options := option.SocksOutboundOptions{
		Version: "5",
	}
	UOTOptions := option.UDPOverTCPOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if username, exists := proxy["username"].(string); exists {
		options.Username = username
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if uot, exists := proxy["uot"].(bool); exists {
		UOTOptions.Enabled = uot
	}
	if uot, exists := proxy["udp-over-tcp"].(bool); exists {
		UOTOptions.Enabled = uot
	}
	options.UDPOverTCP = &UOTOptions
	options.DialerOptions = convertDialerOption(proxy)
	outbound.SocksOptions = options
	return outbound, nil
}

func newTrojanClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeTrojan,
	}
	options := option.TrojanOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if network, exists := proxy["network"].(string); exists {
		Transport := option.V2RayTransportOptions{}
		switch network {
		case "ws":
			Transport.Type = C.V2RayTransportTypeWebsocket
			Transport.WebsocketOptions = convertWSTransport(proxy)
		case "grpc":
			Transport.Type = C.V2RayTransportTypeGRPC
			Transport.GRPCOptions = convertGRPCTransport(proxy)
		}
		options.Transport = &Transport
	}
	options.TLS = convertTLSOptions(proxy)
	options.TLS.Enabled = true
	options.Multiplex = convertSMuxOptions(proxy)
	options.DialerOptions = convertDialerOption(proxy)
	outbound.TrojanOptions = options
	return outbound, nil
}

func newHysteriaClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeHysteria,
	}
	options := option.HysteriaOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	options.TLS = convertTLSOptions(proxy)
	options.TLS.Enabled = true
	options.TLS.UTLS.Enabled = false
	if upRaw, exists := proxy["up"]; exists {
		switch up := upRaw.(type) {
		case string:
			options.Up = up
		case int:
			options.UpMbps = up
		}
	}
	if downRaw, exists := proxy["down"]; exists {
		switch down := downRaw.(type) {
		case string:
			options.Down = down
		case int:
			options.DownMbps = down
		}
	}
	if authStr, exists := proxy["auth-str"]; exists {
		options.AuthString = fmt.Sprint(authStr)
	}
	if authStr, exists := proxy["auth_str"]; exists {
		options.AuthString = fmt.Sprint(authStr)
	}
	if obfs, exists := proxy["obfs"].(string); exists {
		options.Obfs = obfs
	}
	if recvWindowConn, exists := proxy["recv-window-conn"].(int); exists {
		options.ReceiveWindowConn = uint64(recvWindowConn)
	}
	if recvWindowConn, exists := proxy["recv_window_conn"].(int); exists {
		options.ReceiveWindowConn = uint64(recvWindowConn)
	}
	if recvWindow, exists := proxy["recv-window"].(int); exists {
		options.ReceiveWindow = uint64(recvWindow)
	}
	if recvWindow, exists := proxy["recv_window"].(int); exists {
		options.ReceiveWindow = uint64(recvWindow)
	}
	if disable, exists := proxy["disable_mtu_discovery"].(bool); exists && disable {
		options.DisableMTUDiscovery = true
	}
	if ca, exists := proxy["ca"]; exists {
		options.TLS.CertificatePath = fmt.Sprint(ca)
	}
	if caStr, exists := proxy["ca-str"].([]any); exists {
		var caStrArr []string
		for _, item := range caStr {
			caStrArr = append(caStrArr, fmt.Sprint(item))
		}
		options.TLS.Certificate = caStrArr
	}
	if caStr, exists := proxy["ca_str"].([]any); exists {
		caStrArr := []string{}
		for _, item := range caStr {
			caStrArr = append(caStrArr, fmt.Sprint(item))
		}
		options.TLS.Certificate = caStrArr
	}
	options.DialerOptions = convertDialerOption(proxy)
	outbound.HysteriaOptions = options
	return outbound, nil
}

func newHysteria2ClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeHysteria2,
	}
	options := option.Hysteria2OutboundOptions{}
	obfsOptions := option.Hysteria2Obfs{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	if password, exists := proxy["password"].(string); exists {
		options.Password = password
	}
	if up, exists := proxy["up"].(int); exists {
		options.UpMbps = up
	}
	if down, exists := proxy["down"].(int); exists {
		options.DownMbps = down
	}
	if obfs, exists := proxy["obfs"].(string); exists && obfs == "salamander" {
		obfsOptions.Type = obfs
	}
	if obfsPassword, exists := proxy["obfs-password"].(string); exists {
		obfsOptions.Password = obfsPassword
	}
	if obfsOptions.Type != "" {
		options.Obfs = &obfsOptions
	}
	options.TLS = convertTLSOptions(proxy)
	options.TLS.Enabled = true
	options.TLS.UTLS.Enabled = false
	if ca, exists := proxy["ca"]; exists {
		options.TLS.CertificatePath = fmt.Sprint(ca)
	}
	if caStr, exists := proxy["ca-str"].([]any); exists {
		caStrArr := []string{}
		for _, item := range caStr {
			caStrArr = append(caStrArr, fmt.Sprint(item))
		}
		options.TLS.Certificate = caStrArr
	}
	if caStr, exists := proxy["ca_str"].([]any); exists {
		caStrArr := []string{}
		for _, item := range caStr {
			caStrArr = append(caStrArr, fmt.Sprint(item))
		}
		options.TLS.Certificate = caStrArr
	}
	options.DialerOptions = convertDialerOption(proxy)
	outbound.Hysteria2Options = options
	return outbound, nil
}

func newWireGuardClashParser(proxy map[string]any) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeWireGuard,
	}
	options := option.WireGuardOutboundOptions{}
	if name, exists := proxy["name"].(string); exists {
		outbound.Tag = name
	}
	if server, exists := proxy["server"].(string); exists {
		options.Server = server
	}
	if port, exists := proxy["port"]; exists {
		options.ServerPort = stringToUint16(fmt.Sprint(port))
	}
	options.LocalAddress = []netip.Prefix{}
	if ip, exists := proxy["ip"].(string); exists {
		prefix, _ := netip.ParsePrefix(ip)
		options.LocalAddress = append(options.LocalAddress, prefix)
	}
	if ip, exists := proxy["ipv6"].(string); exists {
		prefix, _ := netip.ParsePrefix(ip)
		options.LocalAddress = append(options.LocalAddress, prefix)
	}
	if pk, exists := proxy["private-key"].(string); exists {
		options.PrivateKey = pk
	}
	if pbk, exists := proxy["public-key"].(string); exists {
		options.PeerPublicKey = pbk
	}
	if psk, exists := proxy["pre-shared-key"].(string); exists {
		options.PreSharedKey = psk
	}
	if reserved, exists := proxy["reserved"].([]any); exists {
		reservedArr := []uint8{}
		for _, reserve := range reserved {
			if r, ok := reserve.(int); ok {
				reservedArr = append(reservedArr, uint8(r))
			}
		}
		options.Reserved = reservedArr
	}
	if peerArr, exists := proxy["peers"].([]map[string]any); exists {
		var peers []option.WireGuardPeer
		for _, peerItem := range peerArr {
			peer := option.WireGuardPeer{}
			if server, exists := peerItem["server"].(string); exists {
				peer.Server = server
			}
			if port, exists := peerItem["port"].(int); exists {
				peer.ServerPort = uint16(port)
			}
			if pbk, exists := peerItem["public-key"].(string); exists {
				peer.PublicKey = pbk
			}
			if psk, exists := peerItem["pre-shared-key"].(string); exists {
				peer.PreSharedKey = psk
			}
			if aips, exists := peerItem["allowed_ips"].([]any); exists {
				aipArr := []string{}
				for _, item := range aips {
					aipArr = append(aipArr, fmt.Sprint(item))
				}
				peer.AllowedIPs = aipArr
			}
			if reserved, exists := peerItem["reserved"].([]any); exists {
				reservedArr := []uint8{}
				for _, reserve := range reserved {
					if r, ok := reserve.(int); ok {
						reservedArr = append(reservedArr, uint8(r))
					}
				}
				peer.Reserved = reservedArr
			}
			if peer.Server != "" {
				peers = append(peers, peer)
			}
		}
		if len(peers) > 0 {
			options.Peers = peers
		}
	}
	options.DialerOptions = convertDialerOption(proxy)
	outbound.WireGuardOptions = options
	return outbound, nil
}
