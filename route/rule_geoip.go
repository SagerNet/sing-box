package route

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	N "github.com/sagernet/sing/common/network"
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
	var geoipCode string
	if r.isSource && metadata.SourceGeoIPCode != "" {
		geoipCode = metadata.SourceGeoIPCode
	} else if !r.isSource && metadata.GeoIPCode != "" {
		geoipCode = metadata.GeoIPCode
	}
	if geoipCode != "" {
		return r.codeMap[geoipCode]
	}
	var destination netip.Addr
	if r.isSource {
		destination = metadata.Source.Addr
	} else {
		destination = metadata.Destination.Addr
	}
	if destination.IsValid() {
		return r.match(metadata, destination)
	}
	for _, destinationAddress := range metadata.DestinationAddresses {
		if r.match(metadata, destinationAddress) {
			return true
		}
	}
	return false
}

func (r *GeoIPItem) match(metadata *adapter.InboundContext, destination netip.Addr) bool {
	var geoipCode string
	geoReader := r.router.GeoIPReader()
	if !N.IsPublicAddr(destination) {
		geoipCode = "private"
	} else if geoReader != nil {
		geoipCode = geoReader.Lookup(destination)
	}
	if geoipCode == "" {
		return false
	}
	if r.isSource {
		metadata.SourceGeoIPCode = geoipCode
	} else {
		metadata.GeoIPCode = geoipCode
	}
	return r.codeMap[geoipCode]
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
