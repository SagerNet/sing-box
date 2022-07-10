package iffmonitor

import (
	"os"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/vishvananda/netlink"
)

var _ InterfaceMonitor = (*monitor)(nil)

type monitor struct {
	logger                log.Logger
	defaultInterfaceName  string
	defaultInterfaceIndex int
	update                chan netlink.RouteUpdate
	close                 chan struct{}
}

func New(logger log.Logger) (InterfaceMonitor, error) {
	return &monitor{
		logger: logger,
		update: make(chan netlink.RouteUpdate, 2),
		close:  make(chan struct{}),
	}, nil
}

func (m *monitor) Start() error {
	err := netlink.RouteSubscribe(m.update, m.close)
	if err != nil {
		return err
	}
	err = m.checkUpdate()
	if err != nil {
		return err
	}
	go m.loopUpdate()
	return nil
}

func (m *monitor) loopUpdate() {
	for {
		select {
		case <-m.close:
			return
		case <-m.update:
			err := m.checkUpdate()
			if err != nil {
				m.logger.Error(E.Cause(err, "check default interface"))
			}
		}
	}
}

func (m *monitor) checkUpdate() error {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.Dst != nil {
			continue
		}
		var link netlink.Link
		link, err = netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			return err
		}

		if link.Type() == "tuntap" {
			continue
		}

		oldInterface := m.defaultInterfaceName
		oldIndex := m.defaultInterfaceIndex

		m.defaultInterfaceName = link.Attrs().Name
		m.defaultInterfaceIndex = link.Attrs().Index

		if oldInterface == m.defaultInterfaceName && oldIndex == m.defaultInterfaceIndex {
			return nil
		}

		m.logger.Info("updated default interface ", m.defaultInterfaceName, ", index ", m.defaultInterfaceIndex)
		return nil
	}
	return E.New("no route to internet")
}

func (m *monitor) Close() error {
	select {
	case <-m.close:
		return os.ErrClosed
	default:
	}
	close(m.close)
	return nil
}

func (m *monitor) DefaultInterfaceName() string {
	return m.defaultInterfaceName
}

func (m *monitor) DefaultInterfaceIndex() int {
	return m.defaultInterfaceIndex
}
