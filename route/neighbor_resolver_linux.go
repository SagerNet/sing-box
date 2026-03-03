//go:build linux

package route

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"net"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

var defaultLeaseFiles = []string{
	"/tmp/dhcp.leases",
	"/var/lib/dhcp/dhcpd.leases",
	"/var/lib/dhcpd/dhcpd.leases",
	"/var/lib/kea/kea-leases4.csv",
	"/var/lib/kea/kea-leases6.csv",
}

type neighborResolver struct {
	logger          logger.ContextLogger
	leaseFiles      []string
	access          sync.RWMutex
	neighborIPToMAC map[netip.Addr]net.HardwareAddr
	leaseIPToMAC    map[netip.Addr]net.HardwareAddr
	ipToHostname    map[netip.Addr]string
	macToHostname   map[string]string
	watcher         *fswatch.Watcher
	done            chan struct{}
}

func newNeighborResolver(resolverLogger logger.ContextLogger, leaseFiles []string) (adapter.NeighborResolver, error) {
	if len(leaseFiles) == 0 {
		for _, path := range defaultLeaseFiles {
			info, err := os.Stat(path)
			if err == nil && info.Size() > 0 {
				leaseFiles = append(leaseFiles, path)
			}
		}
	}
	return &neighborResolver{
		logger:          resolverLogger,
		leaseFiles:      leaseFiles,
		neighborIPToMAC: make(map[netip.Addr]net.HardwareAddr),
		leaseIPToMAC:    make(map[netip.Addr]net.HardwareAddr),
		ipToHostname:    make(map[netip.Addr]string),
		macToHostname:   make(map[string]string),
		done:            make(chan struct{}),
	}, nil
}

func (r *neighborResolver) Start() error {
	err := r.loadNeighborTable()
	if err != nil {
		r.logger.Warn(E.Cause(err, "load neighbor table"))
	}
	r.reloadLeaseFiles()
	go r.subscribeNeighborUpdates()
	if len(r.leaseFiles) > 0 {
		watcher, err := fswatch.NewWatcher(fswatch.Options{
			Path:   r.leaseFiles,
			Logger: r.logger,
			Callback: func(_ string) {
				r.reloadLeaseFiles()
			},
		})
		if err != nil {
			r.logger.Warn(E.Cause(err, "create lease file watcher"))
		} else {
			r.watcher = watcher
			err = watcher.Start()
			if err != nil {
				r.logger.Warn(E.Cause(err, "start lease file watcher"))
			}
		}
	}
	return nil
}

func (r *neighborResolver) Close() error {
	close(r.done)
	if r.watcher != nil {
		return r.watcher.Close()
	}
	return nil
}

func (r *neighborResolver) LookupMAC(address netip.Addr) (net.HardwareAddr, bool) {
	r.access.RLock()
	defer r.access.RUnlock()
	mac, found := r.neighborIPToMAC[address]
	if found {
		return mac, true
	}
	mac, found = r.leaseIPToMAC[address]
	if found {
		return mac, true
	}
	mac, found = extractMACFromEUI64(address)
	if found {
		return mac, true
	}
	return nil, false
}

func (r *neighborResolver) LookupHostname(address netip.Addr) (string, bool) {
	r.access.RLock()
	defer r.access.RUnlock()
	hostname, found := r.ipToHostname[address]
	if found {
		return hostname, true
	}
	mac, macFound := r.neighborIPToMAC[address]
	if !macFound {
		mac, macFound = r.leaseIPToMAC[address]
	}
	if !macFound {
		mac, macFound = extractMACFromEUI64(address)
	}
	if macFound {
		hostname, found = r.macToHostname[mac.String()]
		if found {
			return hostname, true
		}
	}
	return "", false
}

func (r *neighborResolver) loadNeighborTable() error {
	connection, err := rtnetlink.Dial(nil)
	if err != nil {
		return E.Cause(err, "dial rtnetlink")
	}
	defer connection.Close()
	neighbors, err := connection.Neigh.List()
	if err != nil {
		return E.Cause(err, "list neighbors")
	}
	r.access.Lock()
	defer r.access.Unlock()
	for _, neigh := range neighbors {
		if neigh.Attributes == nil {
			continue
		}
		if neigh.Attributes.LLAddress == nil || len(neigh.Attributes.Address) == 0 {
			continue
		}
		address, ok := netip.AddrFromSlice(neigh.Attributes.Address)
		if !ok {
			continue
		}
		r.neighborIPToMAC[address] = slices.Clone(neigh.Attributes.LLAddress)
	}
	return nil
}

func (r *neighborResolver) subscribeNeighborUpdates() {
	connection, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{
		Groups: 1 << (unix.RTNLGRP_NEIGH - 1),
	})
	if err != nil {
		r.logger.Warn(E.Cause(err, "subscribe neighbor updates"))
		return
	}
	defer connection.Close()
	for {
		select {
		case <-r.done:
			return
		default:
		}
		err = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err != nil {
			r.logger.Warn(E.Cause(err, "set netlink read deadline"))
			return
		}
		messages, err := connection.Receive()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				continue
			}
			select {
			case <-r.done:
				return
			default:
			}
			r.logger.Warn(E.Cause(err, "receive neighbor update"))
			continue
		}
		for _, message := range messages {
			switch message.Header.Type {
			case unix.RTM_NEWNEIGH:
				var neighMessage rtnetlink.NeighMessage
				unmarshalErr := neighMessage.UnmarshalBinary(message.Data)
				if unmarshalErr != nil {
					continue
				}
				if neighMessage.Attributes == nil {
					continue
				}
				if neighMessage.Attributes.LLAddress == nil || len(neighMessage.Attributes.Address) == 0 {
					continue
				}
				address, ok := netip.AddrFromSlice(neighMessage.Attributes.Address)
				if !ok {
					continue
				}
				r.access.Lock()
				r.neighborIPToMAC[address] = slices.Clone(neighMessage.Attributes.LLAddress)
				r.access.Unlock()
			case unix.RTM_DELNEIGH:
				var neighMessage rtnetlink.NeighMessage
				unmarshalErr := neighMessage.UnmarshalBinary(message.Data)
				if unmarshalErr != nil {
					continue
				}
				if neighMessage.Attributes == nil || len(neighMessage.Attributes.Address) == 0 {
					continue
				}
				address, ok := netip.AddrFromSlice(neighMessage.Attributes.Address)
				if !ok {
					continue
				}
				r.access.Lock()
				delete(r.neighborIPToMAC, address)
				r.access.Unlock()
			}
		}
	}
}

func (r *neighborResolver) reloadLeaseFiles() {
	leaseIPToMAC := make(map[netip.Addr]net.HardwareAddr)
	ipToHostname := make(map[netip.Addr]string)
	macToHostname := make(map[string]string)
	for _, path := range r.leaseFiles {
		r.parseLeaseFile(path, leaseIPToMAC, ipToHostname, macToHostname)
	}
	r.access.Lock()
	r.leaseIPToMAC = leaseIPToMAC
	r.ipToHostname = ipToHostname
	r.macToHostname = macToHostname
	r.access.Unlock()
}

func (r *neighborResolver) parseLeaseFile(path string, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if strings.HasSuffix(path, "kea-leases4.csv") {
		r.parseKeaCSV4(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	if strings.HasSuffix(path, "kea-leases6.csv") {
		r.parseKeaCSV6(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	if strings.HasSuffix(path, "dhcpd.leases") {
		r.parseISCDhcpd(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	r.parseDnsmasqOdhcpd(file, ipToMAC, ipToHostname, macToHostname)
}

func (r *neighborResolver) parseDnsmasqOdhcpd(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	now := time.Now().Unix()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "duid ") {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			r.parseOdhcpdLine(line[2:], ipToMAC, ipToHostname, macToHostname)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		expiry, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		if expiry != 0 && expiry < now {
			continue
		}
		if strings.Contains(fields[1], ":") {
			mac, macErr := net.ParseMAC(fields[1])
			if macErr != nil {
				continue
			}
			address, addrOK := netip.AddrFromSlice(net.ParseIP(fields[2]))
			if !addrOK {
				continue
			}
			address = address.Unmap()
			ipToMAC[address] = mac
			hostname := fields[3]
			if hostname != "*" {
				ipToHostname[address] = hostname
				macToHostname[mac.String()] = hostname
			}
		} else {
			var mac net.HardwareAddr
			if len(fields) >= 5 {
				duid, duidErr := parseDUID(fields[4])
				if duidErr == nil {
					mac, _ = extractMACFromDUID(duid)
				}
			}
			address, addrOK := netip.AddrFromSlice(net.ParseIP(fields[2]))
			if !addrOK {
				continue
			}
			address = address.Unmap()
			if mac != nil {
				ipToMAC[address] = mac
			}
			hostname := fields[3]
			if hostname != "*" {
				ipToHostname[address] = hostname
				if mac != nil {
					macToHostname[mac.String()] = hostname
				}
			}
		}
	}
}

func (r *neighborResolver) parseOdhcpdLine(line string, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return
	}
	validTime, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return
	}
	if validTime == 0 {
		return
	}
	if validTime > 0 && validTime < time.Now().Unix() {
		return
	}
	hostname := fields[3]
	if hostname == "-" || strings.HasPrefix(hostname, `broken\x20`) {
		hostname = ""
	}
	if len(fields) >= 8 && fields[2] == "ipv4" {
		mac, macErr := net.ParseMAC(fields[1])
		if macErr != nil {
			return
		}
		addressField := fields[7]
		slashIndex := strings.IndexByte(addressField, '/')
		if slashIndex >= 0 {
			addressField = addressField[:slashIndex]
		}
		address, addrOK := netip.AddrFromSlice(net.ParseIP(addressField))
		if !addrOK {
			return
		}
		address = address.Unmap()
		ipToMAC[address] = mac
		if hostname != "" {
			ipToHostname[address] = hostname
			macToHostname[mac.String()] = hostname
		}
		return
	}
	var mac net.HardwareAddr
	duidHex := fields[1]
	duidBytes, hexErr := hex.DecodeString(duidHex)
	if hexErr == nil {
		mac, _ = extractMACFromDUID(duidBytes)
	}
	for i := 7; i < len(fields); i++ {
		addressField := fields[i]
		slashIndex := strings.IndexByte(addressField, '/')
		if slashIndex >= 0 {
			addressField = addressField[:slashIndex]
		}
		address, addrOK := netip.AddrFromSlice(net.ParseIP(addressField))
		if !addrOK {
			continue
		}
		address = address.Unmap()
		if mac != nil {
			ipToMAC[address] = mac
		}
		if hostname != "" {
			ipToHostname[address] = hostname
			if mac != nil {
				macToHostname[mac.String()] = hostname
			}
		}
	}
}

func (r *neighborResolver) parseISCDhcpd(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	scanner := bufio.NewScanner(file)
	var currentIP netip.Addr
	var currentMAC net.HardwareAddr
	var currentHostname string
	var currentActive bool
	var inLease bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "lease ") && strings.HasSuffix(line, "{") {
			ipString := strings.TrimSuffix(strings.TrimPrefix(line, "lease "), " {")
			parsed, addrOK := netip.AddrFromSlice(net.ParseIP(ipString))
			if addrOK {
				currentIP = parsed.Unmap()
				inLease = true
				currentMAC = nil
				currentHostname = ""
				currentActive = false
			}
			continue
		}
		if line == "}" && inLease {
			if currentActive && currentMAC != nil {
				ipToMAC[currentIP] = currentMAC
				if currentHostname != "" {
					ipToHostname[currentIP] = currentHostname
					macToHostname[currentMAC.String()] = currentHostname
				}
			} else {
				delete(ipToMAC, currentIP)
				delete(ipToHostname, currentIP)
			}
			inLease = false
			continue
		}
		if !inLease {
			continue
		}
		if strings.HasPrefix(line, "hardware ethernet ") {
			macString := strings.TrimSuffix(strings.TrimPrefix(line, "hardware ethernet "), ";")
			parsed, macErr := net.ParseMAC(macString)
			if macErr == nil {
				currentMAC = parsed
			}
		} else if strings.HasPrefix(line, "client-hostname ") {
			hostname := strings.TrimSuffix(strings.TrimPrefix(line, "client-hostname "), ";")
			hostname = strings.Trim(hostname, "\"")
			if hostname != "" {
				currentHostname = hostname
			}
		} else if strings.HasPrefix(line, "binding state ") {
			state := strings.TrimSuffix(strings.TrimPrefix(line, "binding state "), ";")
			currentActive = state == "active"
		}
	}
}

func (r *neighborResolver) parseKeaCSV4(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	scanner := bufio.NewScanner(file)
	firstLine := true
	for scanner.Scan() {
		if firstLine {
			firstLine = false
			continue
		}
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) < 10 {
			continue
		}
		if fields[9] != "0" {
			continue
		}
		address, addrOK := netip.AddrFromSlice(net.ParseIP(fields[0]))
		if !addrOK {
			continue
		}
		address = address.Unmap()
		mac, macErr := net.ParseMAC(fields[1])
		if macErr != nil {
			continue
		}
		ipToMAC[address] = mac
		hostname := ""
		if len(fields) > 8 {
			hostname = fields[8]
		}
		if hostname != "" {
			ipToHostname[address] = hostname
			macToHostname[mac.String()] = hostname
		}
	}
}

func (r *neighborResolver) parseKeaCSV6(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	scanner := bufio.NewScanner(file)
	firstLine := true
	for scanner.Scan() {
		if firstLine {
			firstLine = false
			continue
		}
		fields := strings.Split(scanner.Text(), ",")
		if len(fields) < 14 {
			continue
		}
		if fields[13] != "0" {
			continue
		}
		address, addrOK := netip.AddrFromSlice(net.ParseIP(fields[0]))
		if !addrOK {
			continue
		}
		address = address.Unmap()
		var mac net.HardwareAddr
		if fields[12] != "" {
			mac, _ = net.ParseMAC(fields[12])
		}
		if mac == nil {
			duid, duidErr := hex.DecodeString(strings.ReplaceAll(fields[1], ":", ""))
			if duidErr == nil {
				mac, _ = extractMACFromDUID(duid)
			}
		}
		hostname := ""
		if len(fields) > 11 {
			hostname = fields[11]
		}
		if mac != nil {
			ipToMAC[address] = mac
		}
		if hostname != "" {
			ipToHostname[address] = hostname
			if mac != nil {
				macToHostname[mac.String()] = hostname
			}
		}
	}
}

func extractMACFromDUID(duid []byte) (net.HardwareAddr, bool) {
	if len(duid) < 4 {
		return nil, false
	}
	duidType := binary.BigEndian.Uint16(duid[0:2])
	hwType := binary.BigEndian.Uint16(duid[2:4])
	if hwType != 1 {
		return nil, false
	}
	switch duidType {
	case 1:
		if len(duid) < 14 {
			return nil, false
		}
		return net.HardwareAddr(slices.Clone(duid[8:14])), true
	case 3:
		if len(duid) < 10 {
			return nil, false
		}
		return net.HardwareAddr(slices.Clone(duid[4:10])), true
	}
	return nil, false
}

func extractMACFromEUI64(address netip.Addr) (net.HardwareAddr, bool) {
	if !address.Is6() {
		return nil, false
	}
	b := address.As16()
	if b[11] != 0xff || b[12] != 0xfe {
		return nil, false
	}
	return net.HardwareAddr{b[8] ^ 0x02, b[9], b[10], b[13], b[14], b[15]}, true
}

func parseDUID(s string) ([]byte, error) {
	cleaned := strings.ReplaceAll(s, ":", "")
	return hex.DecodeString(cleaned)
}
