//go:build linux

package resolved

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/process"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/godbus/dbus/v5"
	mDNS "github.com/miekg/dns"
)

type resolve1Manager Service

type Address struct {
	IfIndex int32
	Family  int32
	Address []byte
}

type Name struct {
	IfIndex  int32
	Hostname string
}

type ResourceRecord struct {
	IfIndex int32
	Type    uint16
	Class   uint16
	Data    []byte
}

type SRVRecord struct {
	Priority  uint16
	Weight    uint16
	Port      uint16
	Hostname  string
	Addresses []Address
	CNAME     string
}

type TXTRecord []byte

type LinkDNS struct {
	Family  int32
	Address []byte
}

type LinkDNSEx struct {
	Family  int32
	Address []byte
	Port    uint16
	Name    string
}

type LinkDomain struct {
	Domain      string
	RoutingOnly bool
}

func (t *resolve1Manager) getLink(ifIndex int32) (*TransportLink, *dbus.Error) {
	link, loaded := t.links[ifIndex]
	if !loaded {
		link = &TransportLink{}
		t.links[ifIndex] = link
		iif, err := t.network.InterfaceFinder().ByIndex(int(ifIndex))
		if err != nil {
			return nil, wrapError(err)
		}
		link.iif = iif
	}
	return link, nil
}

func (t *resolve1Manager) getSenderProcess(sender dbus.Sender) (int32, error) {
	var senderPid int32
	dbusObject := t.systemBus.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	if dbusObject == nil {
		return 0, E.New("missing dbus object")
	}
	err := dbusObject.Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, string(sender)).Store(&senderPid)
	if err != nil {
		return 0, E.Cause(err, "GetConnectionUnixProcessID")
	}
	return senderPid, nil
}

func (t *resolve1Manager) createMetadata(sender dbus.Sender) adapter.InboundContext {
	var metadata adapter.InboundContext
	metadata.Inbound = t.Tag()
	metadata.InboundType = C.TypeResolved
	senderPid, err := t.getSenderProcess(sender)
	if err != nil {
		return metadata
	}
	var processInfo process.Info
	metadata.ProcessInfo = &processInfo
	processInfo.ProcessID = uint32(senderPid)

	processPath, err := os.Readlink(F.ToString("/proc/", senderPid, "/exe"))
	if err == nil {
		processInfo.ProcessPath = processPath
	} else {
		processPath, err = os.Readlink(F.ToString("/proc/", senderPid, "/comm"))
		if err == nil {
			processInfo.ProcessPath = processPath
		}
	}

	var uidFound bool
	statusContent, err := os.ReadFile(F.ToString("/proc/", senderPid, "/status"))
	if err == nil {
		for _, line := range strings.Split(string(statusContent), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					uid, parseErr := strconv.ParseUint(fields[1], 10, 32)
					if parseErr != nil {
						break
					}
					processInfo.UserId = int32(uid)
					uidFound = true
					if osUser, _ := user.LookupId(F.ToString(uid)); osUser != nil {
						processInfo.User = osUser.Username
					}
					break
				}
			}
		}
	}
	if !uidFound {
		metadata.ProcessInfo.UserId = -1
	}
	return metadata
}

func (t *resolve1Manager) log(sender dbus.Sender, message ...any) {
	metadata := t.createMetadata(sender)
	if metadata.ProcessInfo != nil {
		var prefix string
		if metadata.ProcessInfo.ProcessPath != "" {
			prefix = filepath.Base(metadata.ProcessInfo.ProcessPath)
		} else if metadata.ProcessInfo.User != "" {
			prefix = F.ToString("user:", metadata.ProcessInfo.User)
		} else if metadata.ProcessInfo.UserId != 0 {
			prefix = F.ToString("uid:", metadata.ProcessInfo.UserId)
		}
		t.logger.Info("(", prefix, ") ", F.ToString(message...))
	} else {
		t.logger.Info(F.ToString(message...))
	}
}

func (t *resolve1Manager) logRequest(sender dbus.Sender, message ...any) context.Context {
	ctx := log.ContextWithNewID(t.ctx)
	metadata := t.createMetadata(sender)
	if metadata.ProcessInfo != nil {
		var prefix string
		if metadata.ProcessInfo.ProcessPath != "" {
			prefix = filepath.Base(metadata.ProcessInfo.ProcessPath)
		} else if metadata.ProcessInfo.User != "" {
			prefix = F.ToString("user:", metadata.ProcessInfo.User)
		} else if metadata.ProcessInfo.UserId != 0 {
			prefix = F.ToString("uid:", metadata.ProcessInfo.UserId)
		}
		t.logger.InfoContext(ctx, "(", prefix, ") ", strings.Join(F.MapToString(message), " "))
	} else {
		t.logger.InfoContext(ctx, strings.Join(F.MapToString(message), " "))
	}
	return adapter.WithContext(ctx, &metadata)
}

func familyToString(family int32) string {
	switch family {
	case syscall.AF_UNSPEC:
		return "AF_UNSPEC"
	case syscall.AF_INET:
		return "AF_INET"
	case syscall.AF_INET6:
		return "AF_INET6"
	default:
		return F.ToString(family)
	}
}

func (t *resolve1Manager) ResolveHostname(sender dbus.Sender, ifIndex int32, hostname string, family int32, flags uint64) (addresses []Address, canonical string, outflags uint64, err *dbus.Error) {
	t.linkAccess.Lock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return
	}
	t.linkAccess.Unlock()
	var strategy C.DomainStrategy
	switch family {
	case syscall.AF_UNSPEC:
		strategy = C.DomainStrategyAsIS
	case syscall.AF_INET:
		strategy = C.DomainStrategyIPv4Only
	case syscall.AF_INET6:
		strategy = C.DomainStrategyIPv6Only
	}
	ctx := t.logRequest(sender, "ResolveHostname ", link.iif.Name, " ", hostname, " ", familyToString(family), " ", flags)
	responseAddresses, lookupErr := t.dnsRouter.Lookup(ctx, hostname, adapter.DNSQueryOptions{
		LookupStrategy: strategy,
	})
	if lookupErr != nil {
		err = wrapError(err)
		return
	}
	addresses = common.Map(responseAddresses, func(it netip.Addr) Address {
		var addrFamily int32
		if it.Is4() {
			addrFamily = syscall.AF_INET
		} else {
			addrFamily = syscall.AF_INET6
		}
		return Address{
			IfIndex: ifIndex,
			Family:  addrFamily,
			Address: it.AsSlice(),
		}
	})
	canonical = mDNS.CanonicalName(hostname)
	return
}

func (t *resolve1Manager) ResolveAddress(sender dbus.Sender, ifIndex int32, family int32, address []byte, flags uint64) (names []Name, outflags uint64, err *dbus.Error) {
	t.linkAccess.Lock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return
	}
	t.linkAccess.Unlock()
	addr, ok := netip.AddrFromSlice(address)
	if !ok {
		err = wrapError(E.New("invalid address"))
		return
	}
	var nibbles []string
	for i := len(address) - 1; i >= 0; i-- {
		b := address[i]
		nibbles = append(nibbles, fmt.Sprintf("%x", b&0x0F))
		nibbles = append(nibbles, fmt.Sprintf("%x", b>>4))
	}
	var ptrDomain string
	if addr.Is4() {
		ptrDomain = strings.Join(nibbles, ".") + ".in-addr.arpa."
	} else {
		ptrDomain = strings.Join(nibbles, ".") + ".ip6.arpa."
	}
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{
			{
				Name:   mDNS.Fqdn(ptrDomain),
				Qtype:  mDNS.TypePTR,
				Qclass: mDNS.ClassINET,
			},
		},
	}
	ctx := t.logRequest(sender, "ResolveAddress ", link.iif.Name, familyToString(family), addr, flags)
	var metadata adapter.InboundContext
	metadata.InboundType = t.Type()
	metadata.Inbound = t.Tag()
	response, lookupErr := t.dnsRouter.Exchange(adapter.WithContext(ctx, &metadata), request, adapter.DNSQueryOptions{})
	if lookupErr != nil {
		err = wrapError(err)
		return
	}
	if response.Rcode != mDNS.RcodeSuccess {
		err = rcodeError(response.Rcode)
		return
	}
	for _, rawRR := range response.Answer {
		switch rr := rawRR.(type) {
		case *mDNS.PTR:
			names = append(names, Name{
				IfIndex:  ifIndex,
				Hostname: rr.Ptr,
			})
		}
	}
	return
}

func (t *resolve1Manager) ResolveRecord(sender dbus.Sender, ifIndex int32, hostname string, qClass uint16, qType uint16, flags uint64) (records []ResourceRecord, outflags uint64, err *dbus.Error) {
	t.linkAccess.Lock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return
	}
	t.linkAccess.Unlock()
	request := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{
			{
				Name:   mDNS.Fqdn(hostname),
				Qtype:  qType,
				Qclass: qClass,
			},
		},
	}
	ctx := t.logRequest(sender, "ResolveRecord", link.iif.Name, hostname, mDNS.Class(qClass), mDNS.Type(qType), flags)
	var metadata adapter.InboundContext
	metadata.InboundType = t.Type()
	metadata.Inbound = t.Tag()
	response, exchangeErr := t.dnsRouter.Exchange(adapter.WithContext(ctx, &metadata), request, adapter.DNSQueryOptions{})
	if exchangeErr != nil {
		err = wrapError(exchangeErr)
		return
	}
	if response.Rcode != mDNS.RcodeSuccess {
		err = rcodeError(response.Rcode)
		return
	}
	for _, rr := range response.Answer {
		var record ResourceRecord
		record.IfIndex = ifIndex
		record.Type = rr.Header().Rrtype
		record.Class = rr.Header().Class
		data := make([]byte, mDNS.Len(rr))
		_, unpackErr := mDNS.PackRR(rr, data, 0, nil, false)
		if unpackErr != nil {
			err = wrapError(unpackErr)
		}
		record.Data = data
		records = append(records, record)
	}
	return
}

func (t *resolve1Manager) ResolveService(sender dbus.Sender, ifIndex int32, hostname string, sType string, domain string, family int32, flags uint64) (srvData []SRVRecord, txtData []TXTRecord, canonicalName string, canonicalType string, canonicalDomain string, outflags uint64, err *dbus.Error) {
	t.linkAccess.Lock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return
	}
	t.linkAccess.Unlock()

	serviceName := hostname
	if hostname != "" && !strings.HasSuffix(hostname, ".") {
		serviceName += "."
	}
	serviceName += sType
	if !strings.HasSuffix(serviceName, ".") {
		serviceName += "."
	}
	serviceName += domain
	if !strings.HasSuffix(serviceName, ".") {
		serviceName += "."
	}

	ctx := t.logRequest(sender, "ResolveService ", link.iif.Name, " ", hostname, " ", sType, " ", domain, " ", familyToString(family), " ", flags)

	srvRequest := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{
			{
				Name:   serviceName,
				Qtype:  mDNS.TypeSRV,
				Qclass: mDNS.ClassINET,
			},
		},
	}
	var metadata adapter.InboundContext
	metadata.InboundType = t.Type()
	metadata.Inbound = t.Tag()
	srvResponse, exchangeErr := t.dnsRouter.Exchange(adapter.WithContext(ctx, &metadata), srvRequest, adapter.DNSQueryOptions{})
	if exchangeErr != nil {
		err = wrapError(exchangeErr)
		return
	}
	if srvResponse.Rcode != mDNS.RcodeSuccess {
		err = rcodeError(srvResponse.Rcode)
		return
	}

	txtRequest := &mDNS.Msg{
		MsgHdr: mDNS.MsgHdr{
			RecursionDesired: true,
		},
		Question: []mDNS.Question{
			{
				Name:   serviceName,
				Qtype:  mDNS.TypeTXT,
				Qclass: mDNS.ClassINET,
			},
		},
	}

	txtResponse, exchangeErr := t.dnsRouter.Exchange(ctx, txtRequest, adapter.DNSQueryOptions{})
	if exchangeErr != nil {
		err = wrapError(exchangeErr)
		return
	}

	for _, rawRR := range srvResponse.Answer {
		switch rr := rawRR.(type) {
		case *mDNS.SRV:
			var srvRecord SRVRecord
			srvRecord.Priority = rr.Priority
			srvRecord.Weight = rr.Weight
			srvRecord.Port = rr.Port
			srvRecord.Hostname = rr.Target

			var strategy C.DomainStrategy
			switch family {
			case syscall.AF_UNSPEC:
				strategy = C.DomainStrategyAsIS
			case syscall.AF_INET:
				strategy = C.DomainStrategyIPv4Only
			case syscall.AF_INET6:
				strategy = C.DomainStrategyIPv6Only
			}

			addrs, lookupErr := t.dnsRouter.Lookup(ctx, rr.Target, adapter.DNSQueryOptions{
				LookupStrategy: strategy,
			})
			if lookupErr == nil {
				srvRecord.Addresses = common.Map(addrs, func(it netip.Addr) Address {
					var addrFamily int32
					if it.Is4() {
						addrFamily = syscall.AF_INET
					} else {
						addrFamily = syscall.AF_INET6
					}
					return Address{
						IfIndex: ifIndex,
						Family:  addrFamily,
						Address: it.AsSlice(),
					}
				})
			}
			for _, a := range srvResponse.Answer {
				if cname, ok := a.(*mDNS.CNAME); ok && cname.Header().Name == rr.Target {
					srvRecord.CNAME = cname.Target
					break
				}
			}
			srvData = append(srvData, srvRecord)
		}
	}
	for _, rawRR := range txtResponse.Answer {
		switch rr := rawRR.(type) {
		case *mDNS.TXT:
			data := make([]byte, mDNS.Len(rr))
			_, packErr := mDNS.PackRR(rr, data, 0, nil, false)
			if packErr == nil {
				txtData = append(txtData, data)
			}
		}
	}
	canonicalName = mDNS.CanonicalName(hostname)
	canonicalType = mDNS.CanonicalName(sType)
	canonicalDomain = mDNS.CanonicalName(domain)
	return
}

func (t *resolve1Manager) SetLinkDNS(sender dbus.Sender, ifIndex int32, addresses []LinkDNS) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return wrapError(err)
	}
	link.address = addresses
	if len(addresses) > 0 {
		t.log(sender, "SetLinkDNS ", link.iif.Name, " ", strings.Join(common.Map(addresses, func(it LinkDNS) string {
			return M.AddrFromIP(it.Address).String()
		}), ", "))
	} else {
		t.log(sender, "SetLinkDNS ", link.iif.Name, " (empty)")
	}
	return t.postUpdate(link)
}

func (t *resolve1Manager) SetLinkDNSEx(sender dbus.Sender, ifIndex int32, addresses []LinkDNSEx) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return wrapError(err)
	}
	link.addressEx = addresses
	if len(addresses) > 0 {
		t.log(sender, "SetLinkDNSEx ", link.iif.Name, " ", strings.Join(common.Map(addresses, func(it LinkDNSEx) string {
			return M.SocksaddrFrom(M.AddrFromIP(it.Address), it.Port).String()
		}), ", "))
	} else {
		t.log(sender, "SetLinkDNSEx ", link.iif.Name, " (empty)")
	}
	return t.postUpdate(link)
}

func (t *resolve1Manager) SetLinkDomains(sender dbus.Sender, ifIndex int32, domains []LinkDomain) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return wrapError(err)
	}
	link.domain = domains
	if len(domains) > 0 {
		t.log(sender, "SetLinkDomains ", link.iif.Name, " ", strings.Join(common.Map(domains, func(domain LinkDomain) string {
			if !domain.RoutingOnly {
				return domain.Domain
			} else {
				return "~" + domain.Domain
			}
		}), ", "))
	} else {
		t.log(sender, "SetLinkDomains ", link.iif.Name, " (empty)")
	}
	return t.postUpdate(link)
}

func (t *resolve1Manager) SetLinkDefaultRoute(sender dbus.Sender, ifIndex int32, defaultRoute bool) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return err
	}
	link.defaultRoute = defaultRoute
	if defaultRoute {
		t.defaultRouteSequence = append(common.Filter(t.defaultRouteSequence, func(it int32) bool { return it != ifIndex }), ifIndex)
	} else {
		t.defaultRouteSequence = common.Filter(t.defaultRouteSequence, func(it int32) bool { return it != ifIndex })
	}
	var defaultRouteString string
	if defaultRoute {
		defaultRouteString = "yes"
	} else {
		defaultRouteString = "no"
	}
	t.log(sender, "SetLinkDefaultRoute ", link.iif.Name, " ", defaultRouteString)
	return t.postUpdate(link)
}

func (t *resolve1Manager) SetLinkLLMNR(ifIndex int32, llmnrMode string) *dbus.Error {
	return nil
}

func (t *resolve1Manager) SetLinkMulticastDNS(ifIndex int32, mdnsMode string) *dbus.Error {
	return nil
}

func (t *resolve1Manager) SetLinkDNSOverTLS(sender dbus.Sender, ifIndex int32, dotMode string) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return wrapError(err)
	}
	switch dotMode {
	case "yes":
		link.dnsOverTLS = true
	case "":
		dotMode = "no"
		fallthrough
	case "opportunistic", "no":
		link.dnsOverTLS = false
	}
	t.log(sender, "SetLinkDNSOverTLS ", link.iif.Name, " ", dotMode)
	return t.postUpdate(link)
}

func (t *resolve1Manager) SetLinkDNSSEC(ifIndex int32, dnssecMode string) *dbus.Error {
	return nil
}

func (t *resolve1Manager) SetLinkDNSSECNegativeTrustAnchors(ifIndex int32, domains []string) *dbus.Error {
	return nil
}

func (t *resolve1Manager) RevertLink(sender dbus.Sender, ifIndex int32) *dbus.Error {
	t.linkAccess.Lock()
	defer t.linkAccess.Unlock()
	link, err := t.getLink(ifIndex)
	if err != nil {
		return wrapError(err)
	}
	delete(t.links, ifIndex)
	t.log(sender, "RevertLink ", link.iif.Name)
	return t.postUpdate(link)
}

// TODO: implement RegisterService, UnregisterService

func (t *resolve1Manager) RegisterService(sender dbus.Sender, identifier string, nameTemplate string, serviceType string, port uint16, priority uint16, weight uint16, txtRecords []TXTRecord) (objectPath dbus.ObjectPath, dbusErr *dbus.Error) {
	return "", wrapError(E.New("not implemented"))
}

func (t *resolve1Manager) UnregisterService(sender dbus.Sender, servicePath dbus.ObjectPath) error {
	return wrapError(E.New("not implemented"))
}

func (t *resolve1Manager) ResetStatistics() *dbus.Error {
	return nil
}

func (t *resolve1Manager) FlushCaches(sender dbus.Sender) *dbus.Error {
	t.dnsRouter.ClearCache()
	t.log(sender, "FlushCaches")
	return nil
}

func (t *resolve1Manager) ResetServerFeatures() *dbus.Error {
	return nil
}

func (t *resolve1Manager) postUpdate(link *TransportLink) *dbus.Error {
	if t.updateCallback != nil {
		return wrapError(t.updateCallback(link))
	}
	return nil
}

func rcodeError(rcode int) *dbus.Error {
	return dbus.NewError("org.freedesktop.resolve1.DnsError."+mDNS.RcodeToString[rcode], []any{mDNS.RcodeToString[rcode]})
}

func wrapError(err error) *dbus.Error {
	if err == nil {
		return nil
	}
	var rcode dns.RcodeError
	if errors.As(err, &rcode) {
		return rcodeError(int(rcode))
	}
	return dbus.MakeFailedError(err)
}
