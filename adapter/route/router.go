package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.Router = (*Router)(nil)

type Router struct {
	ctx             context.Context
	logger          log.Logger
	defaultOutbound adapter.Outbound
	outboundByTag   map[string]adapter.Outbound
	rules           []adapter.Rule

	needGeoDatabase bool
	geoOptions      option.GeoIPOptions
	geoReader       *geoip2.Reader
}

func NewRouter(ctx context.Context, logger log.Logger, options option.RouteOptions) (*Router, error) {
	router := &Router{
		ctx:             ctx,
		logger:          logger.WithPrefix("router: "),
		outboundByTag:   make(map[string]adapter.Outbound),
		rules:           make([]adapter.Rule, 0, len(options.Rules)),
		needGeoDatabase: hasGeoRule(options.Rules),
		geoOptions:      common.PtrValueOrDefault(options.GeoIP),
	}
	for i, ruleOptions := range options.Rules {
		rule, err := NewRule(router, logger, ruleOptions)
		if err != nil {
			return nil, E.Cause(err, "parse rule[", i, "]")
		}
		router.rules = append(router.rules, rule)
	}
	return router, nil
}

func hasGeoRule(rules []option.Rule) bool {
	for _, rule := range rules {
		if rule.DefaultOptions != nil {
			if isGeoRule(common.PtrValueOrDefault(rule.DefaultOptions)) {
				return true
			}
		} else if rule.LogicalOptions != nil {
			for _, subRule := range rule.LogicalOptions.Rules {
				if isGeoRule(subRule) {
					return true
				}
			}
		}
	}
	return false
}

func isGeoRule(rule option.DefaultRule) bool {
	return len(rule.SourceGeoIP) > 0 || len(rule.GeoIP) > 0
}

func (r *Router) UpdateOutbounds(outbounds []adapter.Outbound) {
	var defaultOutbound adapter.Outbound
	outboundByTag := make(map[string]adapter.Outbound)
	if len(outbounds) > 0 {
		defaultOutbound = outbounds[0]
	}
	for _, outbound := range outbounds {
		outboundByTag[outbound.Tag()] = outbound
	}
	r.defaultOutbound = defaultOutbound
	r.outboundByTag = outboundByTag
}

func (r *Router) Start() error {
	if r.needGeoDatabase {
		go r.prepareGeoIPDatabase()
	}
	return nil
}

func (r *Router) Close() error {
	return common.Close(
		common.PtrOrNil(r.geoReader),
	)
}

func (r *Router) GeoIPReader() *geoip2.Reader {
	return r.geoReader
}

func (r *Router) prepareGeoIPDatabase() {
	var geoPath string
	if r.geoOptions.Path != "" {
		geoPath = r.geoOptions.Path
	} else {
		geoPath = "Country.mmdb"
	}
	geoPath, loaded := C.Find(geoPath)
	if !loaded {
		r.logger.Warn("geoip database not exists: ", geoPath)
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			err = r.downloadGeoIPDatabase(geoPath)
			if err == nil {
				break
			}
			r.logger.Error("download geoip database: ", err)
			os.Remove(geoPath)
			time.Sleep(10 * time.Second)
		}
		if err != nil {
			return
		}
	}
	geoReader, err := geoip2.Open(geoPath)
	if err == nil {
		r.logger.Info("loaded geoip database")
		r.geoReader = geoReader
	} else {
		r.logger.Error("open geoip database: ", err)
		return
	}
}

func (r *Router) downloadGeoIPDatabase(savePath string) error {
	var downloadURL string
	if r.geoOptions.DownloadURL != "" {
		downloadURL = r.geoOptions.DownloadURL
	} else {
		downloadURL = "https://cdn.jsdelivr.net/gh/Dreamacro/maxmind-geoip@release/Country.mmdb"
	}
	r.logger.Info("downloading geoip database")
	var detour adapter.Outbound
	if r.geoOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geoOptions.DownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", r.geoOptions.DownloadDetour)
		}
		detour = outbound
	} else {
		detour = r.defaultOutbound
	}

	if parentDir := filepath.Dir(savePath); parentDir != "" {
		os.MkdirAll(parentDir, 0o755)
	}

	saveFile, err := os.OpenFile(savePath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	defer saveFile.Close()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(saveFile, response.Body)
	return err
}

func (r *Router) DefaultOutbound() adapter.Outbound {
	if r.defaultOutbound == nil {
		panic("missing default outbound")
	}
	return r.defaultOutbound
}

func (r *Router) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := r.outboundByTag[tag]
	return outbound, loaded
}

func (r *Router) RouteConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	return r.match(ctx, metadata).NewConnection(ctx, conn, metadata.Destination)
}

func (r *Router) RoutePacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	return r.match(ctx, metadata).NewPacketConnection(ctx, conn, metadata.Destination)
}

func (r *Router) match(ctx context.Context, metadata adapter.InboundContext) adapter.Outbound {
	for i, rule := range r.rules {
		if rule.Match(&metadata) {
			detour := rule.Outbound()
			r.logger.WithContext(ctx).Info("match [", i, "]", rule.String(), " => ", detour)
			if outbound, loaded := r.Outbound(detour); loaded {
				return outbound
			}
			r.logger.WithContext(ctx).Error("outbound not found: ", detour)
		}
	}
	r.logger.WithContext(ctx).Info("no match")
	return r.defaultOutbound
}
