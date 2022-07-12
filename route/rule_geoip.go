package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
)

var _ RuleItem = (*GeoIPItem)(nil)

type GeoIPItem struct {
	router   adapter.Router
	logger   log.ContextLogger
	isSource bool
	codes    []string
	codeMap  map[string]bool
}

func NewGeoIPItem(router adapter.Router, logger log.ContextLogger, isSource bool, codes []string) *GeoIPItem {
	codeMap := make(map[string]bool)
	for _, code := range codes {
		codeMap[code] = true
	}
	return &GeoIPItem{
		router:   router,
		logger:   logger,
		codes:    codes,
		isSource: isSource,
		codeMap:  codeMap,
	}
}

func (r *GeoIPItem) Match(metadata *adapter.InboundContext) bool {
	geoReader := r.router.GeoIPReader()
	if geoReader == nil {
		return false
	}
	if r.isSource {
		if metadata.SourceGeoIPCode == "" {
			metadata.SourceGeoIPCode = geoReader.Lookup(metadata.Source.Addr)
		}
		return r.codeMap[metadata.SourceGeoIPCode]
	} else {
		if metadata.Destination.IsIP() {
			if metadata.GeoIPCode == "" {
				metadata.GeoIPCode = geoReader.Lookup(metadata.Destination.Addr)
			}
			return r.codeMap[metadata.GeoIPCode]
		}
		for _, address := range metadata.DestinationAddresses {
			if r.codeMap[geoReader.Lookup(address)] {
				return true
			}
		}
		return false
	}
}

func (r *GeoIPItem) String() string {
	var description string
	if r.isSource {
		description = "source_geoip="
	} else {
		description = "geoip="
	}
	cLen := len(r.codes)
	if cLen == 1 {
		description += r.codes[0]
	} else if cLen > 3 {
		description += "[" + strings.Join(r.codes[:3], " ") + "...]"
	} else {
		description += "[" + strings.Join(r.codes, " ") + "]"
	}
	return description
}
