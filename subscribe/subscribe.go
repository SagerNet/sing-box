//go:build with_subscribe

package subscribe

import (
	"bytes"
	"context"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/subscribe/proxy"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"time"
)

const requestTimeout = 20 * time.Second

func ParsePeer(ctx context.Context, tag string, options option.SubscribeOutboundOptions) ([]option.Outbound, error) {
	content, err := requestWithCache(ctx, options)
	if err != nil {
		return nil, err
	}

	outboundRawOptions, err := proxy.ParsePeers(string(content))
	if err != nil {
		return nil, err
	}

	if options.Filter != nil {
		newOutboundRawOptions := make([]proxy.Proxy, 0)
		for _, outboundRawOption := range outboundRawOptions {
			if filterMatch(options.Filter, outboundRawOption.GetTag()) {
				newOutboundRawOptions = append(newOutboundRawOptions, outboundRawOption)
			}
		}
		outboundRawOptions = newOutboundRawOptions
	}

	if options.DialerOptions != nil {
		for i := range outboundRawOptions {
			outboundRawOptions[i].SetDialer(*options.DialerOptions)
		}
	}

	outboundOptions := make([]option.Outbound, 0)
	for _, outboundRawOption := range outboundRawOptions {
		outboundOption, err := outboundRawOption.GenerateOutboundOptions()
		if err != nil {
			return nil, err
		}
		outboundOptions = append(outboundOptions, outboundOption)
	}

	globalOptions := make([]option.Outbound, 0)
	for _, o := range outboundOptions {
		globalOptions = append(globalOptions, o)
	}

	if options.CustomGroup != nil && len(options.CustomGroup) > 0 {
		groupOptions := make([]option.Outbound, 0)
		for _, c := range options.CustomGroup {
			if c.Tag == "" {
				return nil, E.New("group tag cannot be empty")
			}

			groupOutTags := make([]string, 0)
			groupOutTagMap := make(map[string]bool)
			for _, o := range outboundOptions {
				if filterMatch(c.Filter, o.Tag) {
					groupOutTags = append(groupOutTags, o.Tag)
					groupOutTagMap[o.Tag] = true
				}
			}

			groupOut := option.Outbound{}

			switch c.ProxyType {
			case C.TypeSelector:
				groupOut.Tag = c.Tag
				groupOut.Type = C.TypeSelector
				if c.SelectorOptions != nil {
					groupOut.SelectorOptions = *c.SelectorOptions
					if c.SelectorOptions.Default != "" {
						if _, ok := groupOutTagMap[c.SelectorOptions.Default]; !ok {
							return nil, E.New("default outbound '", c.SelectorOptions.Default, "' not found")
						}
					}
				} else {
					groupOut.SelectorOptions = option.SelectorOutboundOptions{}
				}

				groupOut.SelectorOptions.Outbounds = groupOutTags
			case C.TypeURLTest:
				groupOut.Tag = c.Tag
				groupOut.Type = C.TypeURLTest
				if c.URLTestOptions != nil {
					groupOut.URLTestOptions = *c.URLTestOptions
				} else {
					groupOut.URLTestOptions = option.URLTestOutboundOptions{}
				}

				groupOut.URLTestOptions.Outbounds = groupOutTags
			default:
				return nil, E.New("unsupported proxy type: ", c.ProxyType)
			}

			groupOptions = append(groupOptions, groupOut)
		}
		globalOptions = append(globalOptions, groupOptions...)
	}

	globalTags := make([]string, 0)
	globalTagMap := make(map[string]bool)

	for _, g := range globalOptions {
		globalTagMap[g.Tag] = true
		globalTags = append(globalTags, g.Tag)
	}

	globalOut := option.Outbound{}

	switch options.ProxyType {
	case C.TypeSelector:
		globalOut.Tag = tag
		globalOut.Type = C.TypeSelector
		if options.SelectorOptions != nil {
			globalOut.SelectorOptions = *options.SelectorOptions
			if options.SelectorOptions.Default != "" {
				if _, ok := globalTagMap[options.SelectorOptions.Default]; !ok {
					return nil, E.New("default outbound '", options.SelectorOptions.Default, "' not found")
				}
			}
		} else {
			globalOut.SelectorOptions = option.SelectorOutboundOptions{}
		}

		globalOut.SelectorOptions.Outbounds = globalTags
	case C.TypeURLTest:
		globalOut.Tag = tag
		globalOut.Type = C.TypeURLTest
		if options.URLTestOptions != nil {
			globalOut.URLTestOptions = *options.URLTestOptions
		} else {
			globalOut.URLTestOptions = option.URLTestOutboundOptions{}
		}

		globalOut.URLTestOptions.Outbounds = globalTags
	default:
		return nil, E.New("unsupported proxy type: ", options.ProxyType)
	}

	globalOptions = append(globalOptions, globalOut)

	return globalOptions, nil
}

func request(ctx context.Context, options option.SubscribeOutboundOptions) ([]byte, error) {
	u, err := url.Parse(options.Url)
	if err != nil {
		return nil, err
	}

	if options.RequestDialerOptions == nil {
		options.RequestDialerOptions = &option.RequestDialerOptions{}
	}

	dialer := NewDialer(*options.RequestDialerOptions)

	host := u.Hostname()
	ip, err := netip.ParseAddr(host)
	if err != nil {
		dns, err := NewDNS(ctx, options.DNS, dialer)
		if err != nil {
			return nil, err
		}

		ips, err := dns.QueryIP(host)
		if err != nil {
			return nil, err
		}

		ip, _ = netip.ParseAddr(ips[0])
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.RemoteAddr = net.JoinHostPort(ip.String(), u.Port())

	client := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	reader := bytes.NewBuffer(nil)

	_, err = reader.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return reader.Bytes(), nil
}

func requestWithCache(ctx context.Context, options option.SubscribeOutboundOptions) ([]byte, error) {
	var cache []byte

	if options.CacheFile != "" {
		f, err := os.OpenFile(options.CacheFile, os.O_RDONLY, 0666)
		if err == nil {
			fs, err := f.Stat()
			if err == nil {
				if fs.Size() > 0 {
					readBuf := bytes.NewBuffer(nil)
					_, err = readBuf.ReadFrom(f)
					if err == nil {
						cache = readBuf.Bytes()
						if time.Now().Before(fs.ModTime().Add(time.Duration(options.ForceUpdateDuration))) {
							f.Close()
							return cache, nil
						}
					}
				}
			}
		}

		f.Close()
	}

	content, err := request(ctx, options)
	if err != nil {
		if cache != nil {
			return cache, nil
		}
		return nil, err
	}

	if options.CacheFile != "" {
		err = os.WriteFile(options.CacheFile, content, 0666)
		if err != nil {
			return nil, err
		}
	}

	return content, nil
}

func RequestAndCache(ctx context.Context, options option.SubscribeOutboundOptions) error {
	content, err := request(ctx, options)
	if err != nil {
		return err
	}

	return os.WriteFile(options.CacheFile, content, 0666)
}

func filterMatch(f *option.Filter, tag string) bool {
	if f.Rule != nil && len(f.Rule) > 0 {
		match := false
		for _, r := range f.Rule {
			if r.MatchString(tag) {
				match = true
				break
			}
		}
		if f.WhiteMode {
			if !match {
				return false
			}
		} else {
			if match {
				return false
			}
		}
	}

	return true
}
