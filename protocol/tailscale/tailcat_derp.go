//go:build with_tailscale

package tailscale

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/tailscale/tailcfg"
)

const (
	tailcatDefaultDERPMapURL = "https://tailcat.dev/derpmap.json"
	tailcatDERPMapMaxAge     = time.Hour
	tailcatDERPMapMaxSize    = 8 << 20
	tailcatDERPMapTimeout    = 10 * time.Second
)

type tailcatDERP struct {
	region      *tailcfg.DERPRegion
	regionID    int
	mapURL      string
	mode        string
	httpClient  option.HTTPClientOptions
	transport   adapter.HTTPTransport
	access      sync.Mutex
	cachedMap   []byte
	cachedETag  string
	cachedSince time.Time
}

func newTailcatDERP(options option.TailcatDERPOptions, mode string) (*tailcatDERP, error) {
	derp := &tailcatDERP{mode: mode}
	switch {
	case len(options.DERPServers) > 0 && options.DERPRegion != 0:
		return nil, E.New("`derp_region` conflicts with `derp_servers`")
	case len(options.DERPServers) > 0:
		if options.DERPMapURL != "" {
			return nil, E.New("`derp_map_url` conflicts with `derp_servers`")
		}
		if options.HTTPClient != nil {
			return nil, E.New("`http_client` conflicts with `derp_servers`")
		}
		region := &tailcfg.DERPRegion{RegionID: 1, RegionCode: "1"}
		for _, server := range options.DERPServers {
			if server.Host == "" {
				return nil, E.New("missing `host` in `derp_servers`")
			}
			region.Nodes = append(region.Nodes, &tailcfg.DERPNode{
				Name:     server.Host,
				RegionID: 1,
				HostName: server.Host,
				CertName: server.CertName,
				IPv4:     server.IPv4,
				IPv6:     server.IPv6,
				STUNPort: int(server.STUNPort),
				DERPPort: int(server.DERPPort),
			})
		}
		derp.region = region
	case options.DERPRegion > 0:
		derp.regionID = options.DERPRegion
		derp.mapURL = options.DERPMapURL
		if derp.mapURL == "" {
			derp.mapURL = tailcatDefaultDERPMapURL
		}
		derp.httpClient = common.PtrValueOrDefault(options.HTTPClient)
	default:
		return nil, E.New("one of `derp_region` or `derp_servers` is required")
	}
	return derp, nil
}

func (d *tailcatDERP) start(ctx context.Context, logger logger.ContextLogger) error {
	if d.region != nil {
		return nil
	}
	transport, err := service.FromContext[adapter.HTTPClientManager](ctx).ResolveTransport(ctx, logger, d.httpClient)
	if err != nil {
		return E.Cause(err, "create DERP map http client")
	}
	d.transport = transport
	return nil
}

func (d *tailcatDERP) resolve(ctx context.Context) (*tailcfg.DERPRegion, error) {
	if d.region != nil {
		return d.region, nil
	}
	derpMap, err := d.fetchMap(ctx)
	if err != nil {
		return nil, E.Cause(err, "fetch DERP map")
	}
	region, loaded := derpMap.Regions[d.regionID]
	if !loaded {
		return nil, E.New("DERP region ", d.regionID, " not found in ", d.mapURL)
	}
	return region, nil
}

// Mirrors tailcat's cache policy: a copy younger than an hour is used
// as is, an older one is revalidated with If-None-Match, and any copy
// serves as a fallback when the fetch fails. The Tailcat-Mode header
// lets tailcat.dev filter its map for servers.
func (d *tailcatDERP) fetchMap(ctx context.Context) (*tailcfg.DERPMap, error) {
	d.access.Lock()
	defer d.access.Unlock()
	if d.cachedMap != nil && time.Since(d.cachedSince) < tailcatDERPMapMaxAge {
		return decodeTailcatDERPMap(d.cachedMap)
	}
	ctx, cancel := context.WithTimeout(ctx, tailcatDERPMapTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.mapURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Tailcat-Mode", d.mode)
	if d.cachedETag != "" {
		request.Header.Set("If-None-Match", d.cachedETag)
	}
	response, err := (&http.Client{Transport: d.transport}).Do(request)
	if err != nil {
		return d.staleOr(err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified && d.cachedMap != nil {
		d.cachedSince = time.Now()
		return decodeTailcatDERPMap(d.cachedMap)
	}
	if response.StatusCode != http.StatusOK {
		return d.staleOr(E.New("unexpected status: ", response.Status))
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, tailcatDERPMapMaxSize))
	if err != nil {
		return d.staleOr(err)
	}
	derpMap, err := decodeTailcatDERPMap(content)
	if err != nil {
		return d.staleOr(err)
	}
	d.cachedMap = content
	d.cachedETag = response.Header.Get("Etag")
	d.cachedSince = time.Now()
	return derpMap, nil
}

func (d *tailcatDERP) staleOr(err error) (*tailcfg.DERPMap, error) {
	if d.cachedMap == nil {
		return nil, err
	}
	return decodeTailcatDERPMap(d.cachedMap)
}

func decodeTailcatDERPMap(content []byte) (*tailcfg.DERPMap, error) {
	var derpMap tailcfg.DERPMap
	err := json.Unmarshal(content, &derpMap)
	if err != nil {
		return nil, E.Cause(err, "decode DERP map")
	}
	return &derpMap, nil
}
