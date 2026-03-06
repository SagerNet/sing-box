package route

import (
	"bufio"
	"encoding/hex"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

func parseLeaseFile(path string, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if strings.HasSuffix(path, "dhcpd_leases") {
		parseBootpdLeases(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	if strings.HasSuffix(path, "kea-leases4.csv") {
		parseKeaCSV4(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	if strings.HasSuffix(path, "kea-leases6.csv") {
		parseKeaCSV6(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	if strings.HasSuffix(path, "dhcpd.leases") {
		parseISCDhcpd(file, ipToMAC, ipToHostname, macToHostname)
		return
	}
	parseDnsmasqOdhcpd(file, ipToMAC, ipToHostname, macToHostname)
}

func ReloadLeaseFiles(leaseFiles []string) (leaseIPToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	leaseIPToMAC = make(map[netip.Addr]net.HardwareAddr)
	ipToHostname = make(map[netip.Addr]string)
	macToHostname = make(map[string]string)
	for _, path := range leaseFiles {
		parseLeaseFile(path, leaseIPToMAC, ipToHostname, macToHostname)
	}
	return
}

func parseDnsmasqOdhcpd(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	now := time.Now().Unix()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "duid ") {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			parseOdhcpdLine(line[2:], ipToMAC, ipToHostname, macToHostname)
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

func parseOdhcpdLine(line string, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
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

func parseISCDhcpd(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
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

func parseKeaCSV4(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
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

func parseKeaCSV6(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
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

func parseBootpdLeases(file *os.File, ipToMAC map[netip.Addr]net.HardwareAddr, ipToHostname map[netip.Addr]string, macToHostname map[string]string) {
	now := time.Now().Unix()
	scanner := bufio.NewScanner(file)
	var currentName string
	var currentIP netip.Addr
	var currentMAC net.HardwareAddr
	var currentLease int64
	var inBlock bool
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "{" {
			inBlock = true
			currentName = ""
			currentIP = netip.Addr{}
			currentMAC = nil
			currentLease = 0
			continue
		}
		if line == "}" && inBlock {
			if currentMAC != nil && currentIP.IsValid() {
				if currentLease == 0 || currentLease >= now {
					ipToMAC[currentIP] = currentMAC
					if currentName != "" {
						ipToHostname[currentIP] = currentName
						macToHostname[currentMAC.String()] = currentName
					}
				}
			}
			inBlock = false
			continue
		}
		if !inBlock {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "name":
			currentName = value
		case "ip_address":
			parsed, addrOK := netip.AddrFromSlice(net.ParseIP(value))
			if addrOK {
				currentIP = parsed.Unmap()
			}
		case "hw_address":
			typeAndMAC, hasSep := strings.CutPrefix(value, "1,")
			if hasSep {
				mac, macErr := net.ParseMAC(typeAndMAC)
				if macErr == nil {
					currentMAC = mac
				}
			}
		case "lease":
			leaseHex := strings.TrimPrefix(value, "0x")
			parsed, parseErr := strconv.ParseInt(leaseHex, 16, 64)
			if parseErr == nil {
				currentLease = parsed
			}
		}
	}
}
