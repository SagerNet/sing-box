//go:build darwin

package route

import (
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/sagernet/fswatch"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

var defaultLeaseFiles = []string{
	"/var/db/dhcpd_leases",
	"/tmp/dhcp.leases",
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
	entries, err := ReadNeighborEntries()
	if err != nil {
		return err
	}
	r.access.Lock()
	defer r.access.Unlock()
	for _, entry := range entries {
		r.neighborIPToMAC[entry.Address] = entry.MACAddress
	}
	return nil
}

func (r *neighborResolver) subscribeNeighborUpdates() {
	routeSocket, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		r.logger.Warn(E.Cause(err, "subscribe neighbor updates"))
		return
	}
	err = unix.SetNonblock(routeSocket, true)
	if err != nil {
		unix.Close(routeSocket)
		r.logger.Warn(E.Cause(err, "set route socket nonblock"))
		return
	}
	routeSocketFile := os.NewFile(uintptr(routeSocket), "route")
	defer routeSocketFile.Close()
	buffer := buf.NewPacket()
	defer buffer.Release()
	for {
		select {
		case <-r.done:
			return
		default:
		}
		err = setReadDeadline(routeSocketFile, 3*time.Second)
		if err != nil {
			r.logger.Warn(E.Cause(err, "set route socket read deadline"))
			return
		}
		n, err := routeSocketFile.Read(buffer.FreeBytes())
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
		messages, err := route.ParseRIB(route.RIBTypeRoute, buffer.FreeBytes()[:n])
		if err != nil {
			continue
		}
		for _, message := range messages {
			routeMessage, isRouteMessage := message.(*route.RouteMessage)
			if !isRouteMessage {
				continue
			}
			if routeMessage.Flags&unix.RTF_LLINFO == 0 {
				continue
			}
			address, mac, isDelete, ok := ParseRouteNeighborMessage(routeMessage)
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

func setReadDeadline(file *os.File, timeout time.Duration) error {
	rawConn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var controlErr error
	err = rawConn.Control(func(fd uintptr) {
		tv := unix.NsecToTimeval(int64(timeout))
		controlErr = unix.SetsockoptTimeval(int(fd), unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
	})
	if err != nil {
		return err
	}
	return controlErr
}
