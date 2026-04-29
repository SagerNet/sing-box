//go:build linux

package route

import (
	"net"
	"net/netip"
	"os"
	"slices"
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
	r.doReloadLeaseFiles()
	go r.subscribeNeighborUpdates()
	if len(r.leaseFiles) > 0 {
		watcher, err := fswatch.NewWatcher(fswatch.Options{
			Path:   r.leaseFiles,
			Logger: r.logger,
			Callback: func(_ string) {
				r.doReloadLeaseFiles()
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

func (r *neighborResolver) LookupAddresses(hostname string) []netip.Addr {
	r.access.RLock()
	defer r.access.RUnlock()
	return lookupAddressesByHostname(hostname, r.ipToHostname, r.macToHostname, r.neighborIPToMAC, r.leaseIPToMAC)
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
			address, mac, isDelete, ok := ParseNeighborMessage(message)
			if !ok {
				continue
			}
			r.access.Lock()
			if isDelete {
				delete(r.neighborIPToMAC, address)
			} else {
				r.neighborIPToMAC[address] = mac
			}
			r.access.Unlock()
		}
	}
}

func (r *neighborResolver) doReloadLeaseFiles() {
	leaseIPToMAC, ipToHostname, macToHostname := ReloadLeaseFiles(r.leaseFiles)
	r.access.Lock()
	r.leaseIPToMAC = leaseIPToMAC
	r.ipToHostname = ipToHostname
	r.macToHostname = macToHostname
	r.access.Unlock()
}
