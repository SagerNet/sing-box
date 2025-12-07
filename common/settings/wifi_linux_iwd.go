package settings

import (
	"context"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"

	"github.com/godbus/dbus/v5"
)

type iwdMonitor struct {
	conn       *dbus.Conn
	callback   func(adapter.WIFIState)
	cancel     context.CancelFunc
	signalChan chan *dbus.Signal
}

func newIWDMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	iwdObj := conn.Object("net.connman.iwd", "/")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	call := iwdObj.CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0)
	if call.Err != nil {
		conn.Close()
		return nil, call.Err
	}
	return &iwdMonitor{conn: conn, callback: callback}, nil
}

func (m *iwdMonitor) ReadWIFIState() adapter.WIFIState {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	iwdObj := m.conn.Object("net.connman.iwd", "/")
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := iwdObj.CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	if err != nil {
		return adapter.WIFIState{}
	}

	for _, interfaces := range objects {
		stationProps, hasStation := interfaces["net.connman.iwd.Station"]
		if !hasStation {
			continue
		}

		stateVariant, hasState := stationProps["State"]
		if !hasState {
			continue
		}
		state, ok := stateVariant.Value().(string)
		if !ok || state != "connected" {
			continue
		}

		connectedNetworkVariant, hasNetwork := stationProps["ConnectedNetwork"]
		if !hasNetwork {
			continue
		}
		networkPath, ok := connectedNetworkVariant.Value().(dbus.ObjectPath)
		if !ok || networkPath == "/" {
			continue
		}

		networkInterfaces, hasNetworkPath := objects[networkPath]
		if !hasNetworkPath {
			continue
		}

		networkProps, hasNetworkInterface := networkInterfaces["net.connman.iwd.Network"]
		if !hasNetworkInterface {
			continue
		}

		nameVariant, hasName := networkProps["Name"]
		if !hasName {
			continue
		}
		ssid, ok := nameVariant.Value().(string)
		if !ok {
			continue
		}

		connectedBSSVariant, hasBSS := stationProps["ConnectedAccessPoint"]
		if !hasBSS {
			return adapter.WIFIState{SSID: ssid}
		}
		bssPath, ok := connectedBSSVariant.Value().(dbus.ObjectPath)
		if !ok || bssPath == "/" {
			return adapter.WIFIState{SSID: ssid}
		}

		bssInterfaces, hasBSSPath := objects[bssPath]
		if !hasBSSPath {
			return adapter.WIFIState{SSID: ssid}
		}

		bssProps, hasBSSInterface := bssInterfaces["net.connman.iwd.BasicServiceSet"]
		if !hasBSSInterface {
			return adapter.WIFIState{SSID: ssid}
		}

		addressVariant, hasAddress := bssProps["Address"]
		if !hasAddress {
			return adapter.WIFIState{SSID: ssid}
		}
		bssid, ok := addressVariant.Value().(string)
		if !ok {
			return adapter.WIFIState{SSID: ssid}
		}

		return adapter.WIFIState{
			SSID:  ssid,
			BSSID: strings.ToUpper(strings.ReplaceAll(bssid, ":", "")),
		}
	}

	return adapter.WIFIState{}
}

func (m *iwdMonitor) Start() error {
	if m.callback == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.signalChan = make(chan *dbus.Signal, 10)
	m.conn.Signal(m.signalChan)

	err := m.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchSender("net.connman.iwd"),
	)
	if err != nil {
		return err
	}

	state := m.ReadWIFIState()
	go m.monitorSignals(ctx, m.signalChan, state)
	m.callback(state)

	return nil
}

func (m *iwdMonitor) monitorSignals(ctx context.Context, signalChan chan *dbus.Signal, lastState adapter.WIFIState) {
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

func (m *iwdMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.signalChan != nil {
		m.conn.RemoveSignal(m.signalChan)
		close(m.signalChan)
	}
	if m.conn != nil {
		m.conn.RemoveMatchSignal(
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			dbus.WithMatchSender("net.connman.iwd"),
		)
		return m.conn.Close()
	}
	return nil
}
