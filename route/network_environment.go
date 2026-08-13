package route

import (
	"hash/fnv"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
)

func (r *NetworkManager) NetworkEnvironment() uint64 {
	r.stateAccess.RLock()
	defer r.stateAccess.RUnlock()
	return r.networkEnvironment
}

func (r *NetworkManager) postUpdateNetworkEnvironment() {
	r.environmentUpdateAccess.Lock()
	defer r.environmentUpdateAccess.Unlock()
	if r.environmentUpdateTimer == nil {
		r.environmentUpdateTimer = time.AfterFunc(time.Second, r.updateNetworkEnvironment)
	} else {
		r.environmentUpdateTimer.Reset(time.Second)
	}
}

func (r *NetworkManager) updateNetworkEnvironment() {
	r.environmentUpdateAccess.Lock()
	defer r.environmentUpdateAccess.Unlock()
	if r.environmentUpdateTimer != nil {
		r.environmentUpdateTimer.Stop()
	}
	var defaultInterface *adapter.NetworkInterface
	if r.interfaceMonitor != nil {
		defaultInterface = r.DefaultNetworkInterface()
	}
	var environment []string
	if defaultInterface != nil {
		gateways := defaultInterface.Gateways
		if len(gateways) == 0 {
			gateways = systemGateways(defaultInterface.Interface.Index)
		}
		gateways = common.Uniq(gateways)
		slices.SortFunc(gateways, netip.Addr.Compare)
		for _, gateway := range gateways {
			environment = append(environment, "gateway:"+gateway.String())
		}
		wifiState := r.WIFIState()
		if wifiState.SSID != "" {
			environment = append(environment, "ssid:"+wifiState.SSID)
		} else if len(gateways) > 0 {
			hardwareAddresses := systemNeighborHardwareAddresses(defaultInterface.Interface.Index, gateways)
			for _, gateway := range gateways {
				hardwareAddress := hardwareAddresses[gateway]
				if len(hardwareAddress) > 0 {
					environment = append(environment, "gateway_mac:"+hardwareAddress.String())
				}
			}
		}
	}
	var environmentHash uint64
	if len(environment) > 0 {
		digest := fnv.New64a()
		for _, entry := range environment {
			digest.Write([]byte(entry))
			digest.Write([]byte{0})
		}
		environmentHash = digest.Sum64()
	}
	r.stateAccess.Lock()
	changed := environmentHash != r.networkEnvironment
	r.networkEnvironment = environmentHash
	r.stateAccess.Unlock()
	if changed {
		if len(environment) > 0 {
			r.logger.Info("updated network environment: ", strings.Join(environment, ", "))
		} else {
			r.logger.Info("updated network environment: empty")
		}
	}
}
