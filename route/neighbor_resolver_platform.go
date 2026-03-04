package route

import (
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/logger"
)

type platformNeighborResolver struct {
	logger        logger.ContextLogger
	platform      adapter.PlatformInterface
	access        sync.RWMutex
	ipToMAC       map[netip.Addr]net.HardwareAddr
	ipToHostname  map[netip.Addr]string
	macToHostname map[string]string
}

func newPlatformNeighborResolver(resolverLogger logger.ContextLogger, platform adapter.PlatformInterface) adapter.NeighborResolver {
	return &platformNeighborResolver{
		logger:        resolverLogger,
		platform:      platform,
		ipToMAC:       make(map[netip.Addr]net.HardwareAddr),
		ipToHostname:  make(map[netip.Addr]string),
		macToHostname: make(map[string]string),
	}
}

func (r *platformNeighborResolver) Start() error {
	return r.platform.StartNeighborMonitor(r)
}

func (r *platformNeighborResolver) Close() error {
	return r.platform.CloseNeighborMonitor(r)
}

func (r *platformNeighborResolver) LookupMAC(address netip.Addr) (net.HardwareAddr, bool) {
	r.access.RLock()
	defer r.access.RUnlock()
	mac, found := r.ipToMAC[address]
	if found {
		return mac, true
	}
	return extractMACFromEUI64(address)
}

func (r *platformNeighborResolver) LookupHostname(address netip.Addr) (string, bool) {
	r.access.RLock()
	defer r.access.RUnlock()
	hostname, found := r.ipToHostname[address]
	if found {
		return hostname, true
	}
	mac, found := r.ipToMAC[address]
	if !found {
		mac, found = extractMACFromEUI64(address)
	}
	if !found {
		return "", false
	}
	hostname, found = r.macToHostname[mac.String()]
	return hostname, found
}

func (r *platformNeighborResolver) UpdateNeighborTable(entries []adapter.NeighborEntry) {
	ipToMAC := make(map[netip.Addr]net.HardwareAddr)
	ipToHostname := make(map[netip.Addr]string)
	macToHostname := make(map[string]string)
	for _, entry := range entries {
		ipToMAC[entry.Address] = entry.MACAddress
		if entry.Hostname != "" {
			ipToHostname[entry.Address] = entry.Hostname
			macToHostname[entry.MACAddress.String()] = entry.Hostname
		}
	}
	r.access.Lock()
	r.ipToMAC = ipToMAC
	r.ipToHostname = ipToHostname
	r.macToHostname = macToHostname
	r.access.Unlock()
	r.logger.Info("updated neighbor table: ", len(entries), " entries")
}
