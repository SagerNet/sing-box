package settings

import (
	"context"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"

	"github.com/godbus/dbus/v5"
)

type connmanMonitor struct {
	conn       *dbus.Conn
	callback   func(adapter.WIFIState)
	cancel     context.CancelFunc
	signalChan chan *dbus.Signal
}

func newConnManMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	cmObj := conn.Object("net.connman", "/")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	call := cmObj.CallWithContext(ctx, "net.connman.Manager.GetServices", 0)
	if call.Err != nil {
		conn.Close()
		return nil, call.Err
	}
	return &connmanMonitor{conn: conn, callback: callback}, nil
}

func (m *connmanMonitor) ReadWIFIState() adapter.WIFIState {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmObj := m.conn.Object("net.connman", "/")
	var services []interface{}
	err := cmObj.CallWithContext(ctx, "net.connman.Manager.GetServices", 0).Store(&services)
	if err != nil {
		return adapter.WIFIState{}
	}

	for _, service := range services {
		servicePair, ok := service.([]interface{})
		if !ok || len(servicePair) != 2 {
			continue
		}

		serviceProps, ok := servicePair[1].(map[string]dbus.Variant)
		if !ok {
			continue
		}

		typeVariant, hasType := serviceProps["Type"]
		if !hasType {
			continue
		}
		serviceType, ok := typeVariant.Value().(string)
		if !ok || serviceType != "wifi" {
			continue
		}

		stateVariant, hasState := serviceProps["State"]
		if !hasState {
			continue
		}
		state, ok := stateVariant.Value().(string)
		if !ok || (state != "online" && state != "ready") {
			continue
		}

		nameVariant, hasName := serviceProps["Name"]
		if !hasName {
			continue
		}
		ssid, ok := nameVariant.Value().(string)
		if !ok || ssid == "" {
			continue
		}

		bssidVariant, hasBSSID := serviceProps["BSSID"]
		if !hasBSSID {
			return adapter.WIFIState{SSID: ssid}
		}
		bssid, ok := bssidVariant.Value().(string)
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

func (m *connmanMonitor) Start() error {
	if m.callback == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.signalChan = make(chan *dbus.Signal, 10)
	m.conn.Signal(m.signalChan)

	err := m.conn.AddMatchSignal(
		dbus.WithMatchInterface("net.connman.Service"),
		dbus.WithMatchSender("net.connman"),
	)
	if err != nil {
		return err
	}

	state := m.ReadWIFIState()
	go m.monitorSignals(ctx, m.signalChan, state)
	m.callback(state)

	return nil
}

func (m *connmanMonitor) monitorSignals(ctx context.Context, signalChan chan *dbus.Signal, lastState adapter.WIFIState) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-signalChan:
			if !ok {
				return
			}
			// godbus Signal.Name uses "interface.member" format (e.g. "net.connman.Service.PropertyChanged"),
			// not just the member name. This differs from the D-Bus signal member in the match rule.
			if signal.Name == "net.connman.Service.PropertyChanged" {
				state := m.ReadWIFIState()
				if state != lastState {
					lastState = state
					m.callback(state)
				}
			}
		}
	}
}

func (m *connmanMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	if m.signalChan != nil {
		m.conn.RemoveSignal(m.signalChan)
		close(m.signalChan)
	}
	if m.conn != nil {
		m.conn.RemoveMatchSignal(
			dbus.WithMatchInterface("net.connman.Service"),
			dbus.WithMatchSender("net.connman"),
		)
		return m.conn.Close()
	}
	return nil
}
