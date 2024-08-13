package provider

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

var (
	ssParser        = regexp.MustCompile(`^(.*?)@(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))$`)
	ssrParser       = regexp.MustCompile(`^(.*?):(.*?):(.*?):(.*?):(.*?):(.*?)(?:(?:\/|\?|\/\?)(.*?))?$`)
	tuicParser      = regexp.MustCompile(`^(.*?):(.*?)@(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))$`)
	vmessParser     = regexp.MustCompile(`^(.*?)@(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))$`)
	vlessParser     = regexp.MustCompile(`^(.*?)@(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))$`)
	trojanParser    = regexp.MustCompile(`^(.*?)@(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))$`)
	hysteriaParser  = regexp.MustCompile(`^(.*?):(\d+)(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))?$`)
	hysteria2Parser = regexp.MustCompile(`^(.+?)@(.+?)(?::(\d+))?(?:(?:\/|\?|\/\?)(.*?))?(?:#(.*?))?$`)
)

func newNativeURIParser(content string) ([]option.Outbound, error) {
	var outbounds []option.Outbound
	for _, proxyRaw := range strings.Split(content, "\n") {
		proxyRaw = strings.TrimSpace(proxyRaw)
		if !strings.Contains(proxyRaw, "://") {
			continue
		}
		splitedArr := strings.Split(proxyRaw, "://")
		var (
			outbound option.Outbound
			err      error
		)
		protocol := strings.ToLower(strings.TrimSpace(splitedArr[0]))
		parsedProxy := strings.TrimSpace(decodeBase64Safe(strings.TrimSpace(splitedArr[1])))
		switch protocol {
		case "ss":
			outbound, err = newSSNativeParser(parsedProxy)
		case "ssr":
			outbound, err = newSSRNativeParser(parsedProxy)
		case "tuic":
			outbound, err = newTuicNativeParser(parsedProxy)
		case "vmess":
			outbound, err = newVMessNativeParser(parsedProxy)
		case "vless":
			outbound, err = newVLESSNativeParser(parsedProxy)
		case "trojan":
			outbound, err = newTrojanNativeParser(parsedProxy)
		case "hysteria":
			outbound, err = newHysteriaNativeParser(parsedProxy)
		case "hy2", "hysteria2":
			outbound, err = newHysteria2NativeParser(parsedProxy)
		default:
			continue
		}
		if err == nil {
			outbounds = append(outbounds, outbound)
		}
	}
	return outbounds, nil
}

func stringToUint16(content string) uint16 {
	intNum, _ := strconv.Atoi(content)
	return uint16(intNum)
}

func stringToInt64(content string) int64 {
	intNum, _ := strconv.Atoi(content)
	return int64(intNum)
}

func stringToUint32(content string) uint32 {
	intNum, _ := strconv.Atoi(content)
	return uint32(intNum)
}

func decodeURIComponent(content string) string {
	result, _ := url.QueryUnescape(content)
	return result
}

func splitKeyValueWithEqual(content string) (string, string) {
	if !strings.Contains(content, "=") {
		return trimBlank(content), "1"
	}
	arr := strings.Split(content, "=")
	return trimBlank(arr[0]), trimBlank(arr[1])
}

func splitKeyValueWithColon(content string) (string, string) {
	if !strings.Contains(content, ":") {
		return trimBlank(content), "1"
	}
	arr := strings.Split(content, ":")
	return trimBlank(arr[0]), trimBlank(arr[1])
}

func newSSNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeShadowsocks,
	}
	result := ssParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid ss uri")
	}
	outbound.Tag = decodeURIComponent(result[5])
	options := option.ShadowsocksOutboundOptions{}
	options.Server = result[2]
	options.ServerPort = stringToUint16(result[3])
	cryptoArr := strings.Split(decodeBase64Safe(result[1]), ":")
	options.Method, options.Password = cryptoArr[0], cryptoArr[1]
	plugin := ""
	pluginArr := []string{}
	for _, addon := range strings.Split(decodeURIComponent(result[4]), "&") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "plugin":
			if strings.Contains(value, "obfs") {
				plugin = "obfs-local"
			} else if strings.Contains(value, "obfs") {
				plugin = "obfs-local"
			}
		default:
			pluginArr = append(pluginArr, addon)
		}
	}
	if plugin != "" {
		options.Plugin = plugin
		options.PluginOptions = strings.Join(pluginArr, ";")
	}
	outbound.ShadowsocksOptions = options
	return outbound, nil
}

func newSSRNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeShadowsocksR,
	}
	result := ssrParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid ssr uri")
	}
	options := option.ShadowsocksROutboundOptions{}
	options.Server = result[1]
	options.ServerPort = stringToUint16(result[2])
	options.Protocol = result[3]
	options.Method = result[4]
	options.Obfs = result[5]
	options.Password = decodeBase64Safe(result[6])
	for _, addon := range strings.Split(decodeURIComponent(result[7]), "&") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "remarks":
			outbound.Tag = decodeBase64Safe(value)
		case "obfsparam":
			options.ObfsParam = decodeBase64Safe(value)
		case "protoparam":
			options.ProtocolParam = decodeBase64Safe(value)
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.DialerOptions.TCPFastOpen = true
			}
		}
	}
	outbound.ShadowsocksROptions = options
	return outbound, nil
}

func newTuicNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeTUIC,
	}
	result := tuicParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid tuic uri")
	}
	outbound.Tag = decodeURIComponent(result[6])
	options := option.TUICOutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		Enabled: true,
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	options.UUID = result[1]
	options.Password = result[2]
	options.Server = result[3]
	TLSOptions.ServerName = result[3]
	options.ServerPort = stringToUint16(result[4])
	for _, addon := range strings.Split(result[5], "&") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "congestion_control":
			if value != "cubic" {
				options.CongestionControl = value
			}
		case "udp_relay_mode":
			options.UDPRelayMode = value
		case "udp_over_stream":
			if value == "true" || value == "1" {
				options.UDPOverStream = true
			}
		case "zero_rtt_handshake", "reduce_rtt":
			if value == "true" || value == "1" {
				options.ZeroRTTHandshake = true
			}
		case "heartbeat_interval":
			options.Heartbeat = option.Duration(stringToInt64(value))
		case "sni":
			TLSOptions.ServerName = value
		case "insecure", "skip-cert-verify", "allow_insecure":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		case "disable_sni":
			if value == "1" || value == "true" {
				TLSOptions.DisableSNI = true
			}
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.TCPFastOpen = true
			}
		case "alpn":
			TLSOptions.ALPN = strings.Split(value, ",")
		}
	}
	if options.UDPOverStream {
		options.UDPRelayMode = ""
	}
	options.TLS = &TLSOptions
	outbound.TUICOptions = options
	return outbound, nil
}

func newVMessNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeVMess,
	}
	var proxy map[string]string
	err := json.Unmarshal([]byte(content), &proxy)
	if err != nil {
		proxy = make(map[string]string)
		result := vmessParser.FindStringSubmatch(content)
		if len(result) == 0 {
			return outbound, E.New("invalid vmess uri")
		}
		proxy["id"] = decodeURIComponent(result[1])
		proxy["add"] = decodeURIComponent(result[2])
		proxy["port"] = decodeURIComponent(result[3])
		proxy["ps"] = decodeURIComponent(result[5])
		for _, addon := range strings.Split(result[4], "&") {
			key, value := splitKeyValueWithEqual(addon)
			switch key {
			case "type":
				if value == "http" {
					proxy["net"] = "tcp"
					proxy["type"] = "http"
				}
			case "encryption":
				proxy["scy"] = value
			case "alterId":
				proxy["aid"] = value
			case "key", "alpn", "seed", "path", "host":
				proxy[key] = decodeURIComponent(value)
			default:
				proxy[key] = value
			}
		}
	}
	outbound.Type = C.TypeVMess
	options := option.VMessOutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	for key, value := range proxy {
		switch key {
		case "ps":
			outbound.Tag = value
		case "add":
			options.Server = value
			TLSOptions.ServerName = value
		case "port":
			options.ServerPort = stringToUint16(value)
		case "id":
			options.UUID = value
		case "scy":
			options.Security = value
		case "aid":
			options.AlterId, _ = strconv.Atoi(value)
		case "packet_encoding":
			options.PacketEncoding = value
		case "xudp":
			if value == "1" || value == "true" {
				options.PacketEncoding = "xudp"
			}
		case "tls":
			if value == "1" || value == "true" || value == "tls" {
				TLSOptions.Enabled = true
			}
		case "insecure", "skip-cert-verify":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		case "fp":
			TLSOptions.UTLS.Enabled = true
			TLSOptions.UTLS.Fingerprint = value
		case "net":
			Transport := option.V2RayTransportOptions{
				Type: "",
				WebsocketOptions: option.V2RayWebsocketOptions{
					Headers: map[string]option.Listable[string]{},
				},
				HTTPOptions: option.V2RayHTTPOptions{
					Host:    option.Listable[string]{},
					Headers: map[string]option.Listable[string]{},
				},
				GRPCOptions: option.V2RayGRPCOptions{},
			}
			switch value {
			case "ws":
				Transport.Type = C.V2RayTransportTypeWebsocket
				if host, exists := proxy["host"]; exists && host != "" {
					for _, headerStr := range strings.Split(fmt.Sprint("Host:", host), "\n") {
						key, valueRaw := splitKeyValueWithColon(headerStr)
						value := []string{}
						for _, item := range strings.Split(valueRaw, ",") {
							value = append(value, trimBlank(item))
						}
						Transport.WebsocketOptions.Headers[key] = value
					}
				}
				if path, exists := proxy["path"]; exists && path != "" {
					reg := regexp.MustCompile(`^(.*?)(?:\?ed=(\d*))?$`)
					result := reg.FindStringSubmatch(path)
					Transport.WebsocketOptions.Path = result[1]
					if result[2] != "" {
						Transport.WebsocketOptions.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
						Transport.WebsocketOptions.MaxEarlyData = stringToUint32(result[2])
					}
				}
			case "h2":
				Transport.Type = C.V2RayTransportTypeHTTP
				TLSOptions.Enabled = true
				if host, exists := proxy["host"]; exists && host != "" {
					Transport.HTTPOptions.Host = []string{host}
				}
				if path, exists := proxy["path"]; exists && path != "" {
					Transport.HTTPOptions.Path = path
				}
			case "tcp":
				if tType, exists := proxy["type"]; exists {
					if tType == "http" {
						Transport.Type = C.V2RayTransportTypeHTTP
						if method, exists := proxy["method"]; exists {
							Transport.HTTPOptions.Method = method
						}
						if host, exists := proxy["host"]; exists && host != "" {
							Transport.HTTPOptions.Host = []string{host}
						}
						if path, exists := proxy["path"]; exists && path != "" {
							Transport.HTTPOptions.Path = path
						}
						if headers, exists := proxy["headers"]; exists {
							for _, header := range strings.Split(headers, "\n") {
								reg := regexp.MustCompile(`^[ \t]*?(\S+?):[ \t]*?(\S+?)[ \t]*?$`)
								result := reg.FindStringSubmatch(header)
								key := result[1]
								value := []string{}
								for _, item := range strings.Split(result[2], ",") {
									value = append(value, trimBlank(item))
								}
								Transport.HTTPOptions.Headers[key] = value
							}
						}
					}
				}
			case "grpc":
				Transport.Type = C.V2RayTransportTypeGRPC
				if host, exists := proxy["host"]; exists && host != "" {
					Transport.GRPCOptions.ServiceName = host
				}
			}
			options.Transport = &Transport
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.TCPFastOpen = true
			}
		}
	}
	options.TLS = &TLSOptions
	outbound.VMessOptions = options
	return outbound, nil
}

func newVLESSNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeVLESS,
	}
	result := vlessParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid vless uri")
	}
	outbound.Tag = decodeURIComponent(result[5])
	options := option.VLESSOutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	options.UUID = decodeURIComponent(result[1])
	options.Server = result[2]
	TLSOptions.ServerName = result[2]
	options.ServerPort = stringToUint16(result[3])
	proxy := map[string]string{}
	for _, addon := range strings.Split(result[4], "&") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "key", "alpn", "seed", "path", "host":
			proxy[key] = decodeURIComponent(value)
		default:
			proxy[key] = value
		}
	}
	for key, value := range proxy {
		switch key {
		case "type":
			Transport := option.V2RayTransportOptions{
				Type: "",
				WebsocketOptions: option.V2RayWebsocketOptions{
					Headers: map[string]option.Listable[string]{},
				},
				HTTPOptions: option.V2RayHTTPOptions{
					Host:    option.Listable[string]{},
					Headers: map[string]option.Listable[string]{},
				},
				GRPCOptions: option.V2RayGRPCOptions{},
			}
			switch value {
			case "kcp":
				return outbound, E.New("unsupported transport type: kcp")
			case "ws":
				Transport.Type = C.V2RayTransportTypeWebsocket
				if host, exists := proxy["host"]; exists && host != "" {
					for _, header := range strings.Split(fmt.Sprint("Host:", host), "\n") {
						reg := regexp.MustCompile(`^[ \t]*?(\S+?):[ \t]*?(\S+?)[ \t]*?$`)
						result := reg.FindStringSubmatch(header)
						key := result[1]
						value := []string{}
						for _, item := range strings.Split(result[2], ",") {
							value = append(value, trimBlank(item))
						}
						Transport.WebsocketOptions.Headers[key] = value
					}
				}
				if path, exists := proxy["path"]; exists && path != "" {
					reg := regexp.MustCompile(`^(.*?)(?:\?ed=(\d*))?$`)
					result := reg.FindStringSubmatch(path)
					Transport.WebsocketOptions.Path = result[1]
					if result[2] != "" {
						Transport.WebsocketOptions.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
						Transport.WebsocketOptions.MaxEarlyData = stringToUint32(result[2])
					}
				}
			case "http":
				Transport.Type = C.V2RayTransportTypeHTTP
				if host, exists := proxy["host"]; exists && host != "" {
					Transport.HTTPOptions.Host = strings.Split(host, ",")
				}
				if path, exists := proxy["path"]; exists && path != "" {
					Transport.HTTPOptions.Path = path
				}
			case "grpc":
				Transport.Type = C.V2RayTransportTypeGRPC
				if serviceName, exists := proxy["serviceName"]; exists && serviceName != "" {
					Transport.GRPCOptions.ServiceName = serviceName
				}
			}
			options.Transport = &Transport
		case "security":
			if value == "tls" {
				TLSOptions.Enabled = true
			} else if value == "reality" {
				TLSOptions.Enabled = true
				TLSOptions.Reality.Enabled = true
			}
		case "insecure", "skip-cert-verify":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		case "serviceName", "sni", "peer":
			TLSOptions.ServerName = value
		case "alpn":
			TLSOptions.ALPN = strings.Split(value, ",")
		case "fp":
			TLSOptions.UTLS.Enabled = true
			TLSOptions.UTLS.Fingerprint = value
		case "flow":
			if value == "xtls-rprx-vision" {
				options.Flow = "xtls-rprx-vision"
			}
		case "pbk":
			TLSOptions.Reality.PublicKey = value
		case "sid":
			TLSOptions.Reality.ShortID = value
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.TCPFastOpen = true
			}
		}
	}
	options.TLS = &TLSOptions
	outbound.VLESSOptions = options
	return outbound, nil
}

func newTrojanNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeTrojan,
	}
	result := trojanParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid trojan uri")
	}
	outbound.Tag = decodeURIComponent(result[5])
	options := option.TrojanOutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		Enabled: true,
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	options.Server = result[2]
	TLSOptions.ServerName = result[2]
	options.ServerPort = stringToUint16(result[3])
	options.Password = decodeURIComponent(result[1])
	proxy := map[string]string{}
	for _, addon := range strings.Split(result[4], "&") {
		key, value := splitKeyValueWithEqual(addon)
		proxy[key] = decodeURIComponent(value)
	}
	for key, value := range proxy {
		switch key {
		case "insecure", "allowInsecure", "skip-cert-verify":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		case "serviceName", "sni", "peer":
			TLSOptions.ServerName = value
		case "alpn":
			TLSOptions.ALPN = strings.Split(value, ",")
		case "fp":
			TLSOptions.UTLS.Enabled = true
			TLSOptions.UTLS.Fingerprint = value
		case "type":
			Transport := option.V2RayTransportOptions{
				Type: "",
				WebsocketOptions: option.V2RayWebsocketOptions{
					Headers: map[string]option.Listable[string]{},
				},
				HTTPOptions: option.V2RayHTTPOptions{
					Host:    option.Listable[string]{},
					Headers: map[string]option.Listable[string]{},
				},
				GRPCOptions: option.V2RayGRPCOptions{},
			}
			switch value {
			case "ws":
				Transport.Type = C.V2RayTransportTypeWebsocket
				if host, exists := proxy["host"]; exists && host != "" {
					for _, header := range strings.Split(fmt.Sprint("Host:", host), "\n") {
						reg := regexp.MustCompile(`^[ \t]*?(\S+?):[ \t]*?(\S+?)[ \t]*?$`)
						result := reg.FindStringSubmatch(header)
						key := result[1]
						value := []string{}
						for _, item := range strings.Split(result[2], ",") {
							value = append(value, trimBlank(item))
						}
						Transport.WebsocketOptions.Headers[key] = value
					}
				}
				if path, exists := proxy["path"]; exists && path != "" {
					reg := regexp.MustCompile(`^(.*?)(?:\?ed=(\d*))?$`)
					result := reg.FindStringSubmatch(path)
					Transport.WebsocketOptions.Path = result[1]
					if result[2] != "" {
						Transport.WebsocketOptions.EarlyDataHeaderName = "Sec-WebSocket-Protocol"
						Transport.WebsocketOptions.MaxEarlyData = stringToUint32(result[2])
					}
				}
			case "grpc":
				Transport.Type = C.V2RayTransportTypeGRPC
				if serviceName, exists := proxy["grpc-service-name"]; exists && serviceName != "" {
					Transport.GRPCOptions.ServiceName = serviceName
				}
			}
			options.Transport = &Transport
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.TCPFastOpen = true
			}
		}
	}
	options.TLS = &TLSOptions
	outbound.TrojanOptions = options
	return outbound, nil
}

func newHysteriaNativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeHysteria,
	}
	result := hysteriaParser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid hysteria uri")
	}
	outbound.Tag = result[4]
	options := option.HysteriaOutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		Enabled: true,
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	options.Server = result[1]
	TLSOptions.ServerName = result[1]
	options.ServerPort = stringToUint16(result[2])
	for _, addon := range strings.Split(result[3], "&") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "auth":
			options.AuthString = value
		case "peer", "sni":
			TLSOptions.ServerName = value
		case "alpn":
			TLSOptions.ALPN = strings.Split(value, ",")
		case "ca":
			TLSOptions.CertificatePath = value
		case "ca_str":
			TLSOptions.Certificate = strings.Split(value, "\n")
		case "up":
			options.Up = value
		case "up_mbps":
			options.UpMbps, _ = strconv.Atoi(value)
		case "down":
			options.Down = value
		case "down_mbps":
			options.DownMbps, _ = strconv.Atoi(value)
		case "obfs", "obfsParam":
			options.Obfs = value
		case "insecure", "skip-cert-verify":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		case "tfo", "tcp-fast-open", "tcp_fast_open":
			if value == "1" || value == "true" {
				options.TCPFastOpen = true
			}
		}
	}
	options.TLS = &TLSOptions
	outbound.HysteriaOptions = options
	return outbound, nil
}

func newHysteria2NativeParser(content string) (option.Outbound, error) {
	outbound := option.Outbound{
		Type: C.TypeHysteria2,
	}
	result := hysteria2Parser.FindStringSubmatch(content)
	if len(result) == 0 {
		return outbound, E.New("invalid hysteria2 uri")
	}
	outbound.Tag = result[5]
	options := option.Hysteria2OutboundOptions{}
	TLSOptions := option.OutboundTLSOptions{
		Enabled: true,
		ECH:     &option.OutboundECHOptions{},
		UTLS:    &option.OutboundUTLSOptions{},
		Reality: &option.OutboundRealityOptions{},
	}
	options.ServerPort = uint16(443)
	options.Server = result[2]
	TLSOptions.ServerName = result[2]
	options.Password = result[1]
	if strings.Contains(result[1], ":") {
		options.Password = strings.Split(result[1], ":")[1]
	}
	if result[3] != "" {
		options.ServerPort = stringToUint16(result[3])
	}
	for _, addon := range strings.Split(result[4], "\n") {
		key, value := splitKeyValueWithEqual(addon)
		switch key {
		case "up":
			options.UpMbps, _ = strconv.Atoi(value)
		case "down":
			options.DownMbps, _ = strconv.Atoi(value)
		case "obfs":
			if value == "salamander" {
				options.Obfs.Type = "salamander"
			}
		case "obfs-password":
			options.Obfs.Password = value
		case "insecure", "skip-cert-verify":
			if value == "1" || value == "true" {
				TLSOptions.Insecure = true
			}
		}
	}
	options.TLS = &TLSOptions
	outbound.Hysteria2Options = options
	return outbound, nil
}
