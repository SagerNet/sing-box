package settings

import (
	"context"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"

	"github.com/godbus/dbus/v5"
)

type networkManagerMonitor struct {
	conn       *dbus.Conn
	callback   func(adapter.WIFIState)
	cancel     context.CancelFunc
	signalChan chan *dbus.Signal
}

func newNetworkManagerMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	nmObj := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	var state uint32
	err = nmObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager", "State").Store(&state)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &networkManagerMonitor{conn: conn, callback: callback}, nil
}

func (m *networkManagerMonitor) ReadWIFIState() adapter.WIFIState {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	nmObj := m.conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")

	var activeConnectionPaths []dbus.ObjectPath
	err := nmObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager", "ActiveConnections").Store(&activeConnectionPaths)
	if err != nil || len(activeConnectionPaths) == 0 {
		return adapter.WIFIState{}
	}

	for _, connectionPath := range activeConnectionPaths {
		connObj := m.conn.Object("org.freedesktop.NetworkManager", connectionPath)

		var devicePaths []dbus.ObjectPath
		err = connObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.Connection.Active", "Devices").Store(&devicePaths)
		if err != nil || len(devicePaths) == 0 {
			continue
		}

		for _, devicePath := range devicePaths {
			deviceObj := m.conn.Object("org.freedesktop.NetworkManager", devicePath)

			var deviceType uint32
			err = deviceObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.Device", "DeviceType").Store(&deviceType)
			if err != nil || deviceType != 2 {
				continue
			}

			var accessPointPath dbus.ObjectPath
			err = deviceObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.Device.Wireless", "ActiveAccessPoint").Store(&accessPointPath)
			if err != nil || accessPointPath == "/" {
				continue
			}

			apObj := m.conn.Object("org.freedesktop.NetworkManager", accessPointPath)

			var ssidBytes []byte
			err = apObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.AccessPoint", "Ssid").Store(&ssidBytes)
			if err != nil {
				continue
			}

			var hwAddress string
			err = apObj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.AccessPoint", "HwAddress").Store(&hwAddress)
			if err != nil {
				continue
			}

			ssid := strings.TrimSpace(string(ssidBytes))
			if ssid == "" {
				continue
			}

			return adapter.WIFIState{
				SSID:  ssid,
				BSSID: strings.ToUpper(strings.ReplaceAll(hwAddress, ":", "")),
			}
		}
	}

	return adapter.WIFIState{}
}

func (m *networkManagerMonitor) Start() error {
	if m.callback == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.signalChan = make(chan *dbus.Signal, 10)
	m.conn.Signal(m.signalChan)

	err := m.conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.NetworkManager"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
	)
	if err != nil {
		return err
	}

	state := m.ReadWIFIState()
	go m.monitorSignals(ctx, m.signalChan, state)
	m.callback(state)

	return nil
}

func (m *networkManagerMonitor) monitorSignals(ctx context.Context, signalChan chan *dbus.Signal, lastState adapter.WIFIState) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-signalChan:
			if !ok {
				return
			}
			if signal.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
				state := m.ReadWIFIState()
				if state != lastState {
					lastState = state
					m.callback(state)
				}
			}
		}
	}
}

func (m *networkManagerMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.signalChan != nil {
		m.conn.RemoveSignal(m.signalChan)
		close(m.signalChan)
	}
	if m.conn != nil {
		m.conn.RemoveMatchSignal(
			dbus.WithMatchSender("org.freedesktop.NetworkManager"),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		)
		return m.conn.Close()
	}
	return nil
}
