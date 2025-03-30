package split

import (
	"net/netip"
	"strings"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/godbus/dbus/v5"
)

type resolve1Manager Transport

type resolve1LinkNameserver struct {
	Family  int32
	Address []byte
}

type resolve1LinkDomain struct {
	Domain      string
	RoutingOnly bool
}

func (t *resolve1Manager) getLink(ifIndex uint32) (*TransportLink, error) {
	link, loaded := t.links[ifIndex]
	if !loaded {
		link = &TransportLink{}
		t.links[ifIndex] = link
		iif, err := t.network.InterfaceFinder().ByIndex(int(ifIndex))
		if err != nil {
			return nil, dbus.MakeFailedError(err)
		}
		link.iif = iif
	}
	return link, nil
}

func (t *resolve1Manager) SetLinkDNS(ifIndex uint32, addresses []resolve1LinkNameserver) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	for _, ns := range link.nameservers {
		ns.Close()
	}
	link.nameservers = link.nameservers[:0]
	if len(addresses) > 0 {
		serverDialer := common.Must1(dialer.NewDefault(t.ctx, option.DialerOptions{
			BindInterface:      link.iif.Name,
			UDPFragmentDefault: true,
		}))
		var serverAddresses []netip.Addr
		for _, address := range addresses {
			serverAddr, ok := netip.AddrFromSlice(address.Address)
			if !ok {
				return dbus.MakeFailedError(E.New("invalid address"))
			}
			serverAddresses = append(serverAddresses, serverAddr)
		}
		for _, serverAddress := range serverAddresses {
			link.nameservers = append(link.nameservers, transport.NewUDPRaw(t.logger, t.TransportAdapter, serverDialer, M.SocksaddrFrom(serverAddress, 53)))
		}
		t.logger.Info("SetLinkDNS ", link.iif.Name, " ", strings.Join(common.Map(serverAddresses, netip.Addr.String), ", "))
	} else {
		t.logger.Info("SetLinkDNS ", link.iif.Name, " (empty)")
	}
	return nil
}

func (t *resolve1Manager) SetLinkDomains(ifIndex uint32, domains []resolve1LinkDomain) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	link.domains = domains
	if len(domains) > 0 {
		t.logger.Info("SetLinkDomains ", link.iif.Name, " ", strings.Join(common.Map(domains, func(domain resolve1LinkDomain) string {
			if !domain.RoutingOnly {
				return domain.Domain
			} else {
				return domain.Domain + " (routing)"
			}
		}), ", "))
	} else {
		t.logger.Info("SetLinkDomains ", link.iif.Name, " (empty)")
	}
	return nil
}

func (t *resolve1Manager) SetLinkDefaultRoute(ifIndex uint32, defaultRoute bool) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	link.defaultRoute = defaultRoute
	t.logger.Info("SetLinkDefaultRoute ", link.iif.Name, " ", defaultRoute)
	return nil
}

func (t *resolve1Manager) SetLinkLLMNR(ifIndex uint32, llmnrMode string) {
}

func (t *resolve1Manager) SetLinkMulticastDNS(ifIndex uint32, mdnsMode string) {
}

func (t *resolve1Manager) SetLinkDNSOverTLS(ifIndex uint32, dotMode string) {
}

func (t *resolve1Manager) SetLinkDNSSEC(ifIndex uint32, dnssecMode string) {
}

func (t *resolve1Manager) SetLinkDNSSECNegativeTrustAnchors(ifIndex uint32, domains []string) {
}

func (t *resolve1Manager) RevertLink(ifIndex uint32) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return dbus.MakeFailedError(err)
	}
	delete(t.links, ifIndex)
	t.logger.Info("RevertLink ", link.iif.Name)
	return nil
}

func (t *resolve1Manager) FlushCaches() {
	t.dnsRouter.ClearCache()
}
