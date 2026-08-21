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
	var (
		gatewayStrings  []string
		hardwareStrings []string
		wifiSSID        string
	)
	if defaultInterface != nil {
		gateways := defaultInterface.Gateways
		if len(gateways) == 0 {
			gateways = systemGateways(defaultInterface.Interface.Index)
		}
		gateways = common.Uniq(gateways)
		slices.SortFunc(gateways, netip.Addr.Compare)
		gatewayStrings = common.Map(gateways, netip.Addr.String)
		wifiState := r.WIFIState()
		if wifiState.SSID != "" {
			wifiSSID = wifiState.SSID
		} else if len(gateways) > 0 {
			hardwareAddresses := systemNeighborHardwareAddresses(defaultInterface.Interface.Index, gateways)
			for _, gateway := range gateways {
				hardwareAddress := hardwareAddresses[gateway]
				if len(hardwareAddress) > 0 {
					hardwareStrings = append(hardwareStrings, hardwareAddress.String())
				}
			}
		}
	}
	var options []string
	if len(gatewayStrings) > 0 {
		options = append(options, "gateway "+formatEnvironmentValues(gatewayStrings))
	}
	if wifiSSID != "" {
		options = append(options, "ssid "+wifiSSID)
	}
	if len(hardwareStrings) > 0 {
		options = append(options, "gateway_mac "+formatEnvironmentValues(hardwareStrings))
	}
	var environmentHash uint64
	if len(options) > 0 {
		digest := fnv.New64a()
		for _, option := range options {
			digest.Write([]byte(option))
			digest.Write([]byte{0})
		}
		environmentHash = digest.Sum64()
	}
	r.stateAccess.Lock()
	changed := environmentHash != r.networkEnvironment
	r.networkEnvironment = environmentHash
	r.stateAccess.Unlock()
	if !changed || len(options) == 0 {
		return
	}
	r.logger.Info("updated network environment: ", strings.Join(options, ", "))
}

func formatEnvironmentValues(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	return "[" + strings.Join(values, " ") + "]"
}
