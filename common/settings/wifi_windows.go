//go:build windows

package settings

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"syscall"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/winwlanapi"

	"golang.org/x/sys/windows"
)

type windowsWIFIMonitor struct {
	handle    windows.Handle
	callback  func(adapter.WIFIState)
	cancel    context.CancelFunc
	lastState adapter.WIFIState
	mutex     sync.Mutex
}

func NewWIFIMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	handle, err := winwlanapi.OpenHandle()
	if err != nil {
		return nil, err
	}

	interfaces, err := winwlanapi.EnumInterfaces(handle)
	if err != nil {
		winwlanapi.CloseHandle(handle)
		return nil, err
	}
	if len(interfaces) == 0 {
		winwlanapi.CloseHandle(handle)
		return nil, fmt.Errorf("no wireless interfaces found")
	}

	return &windowsWIFIMonitor{
		handle:   handle,
		callback: callback,
	}, nil
}

func (m *windowsWIFIMonitor) ReadWIFIState() adapter.WIFIState {
	interfaces, err := winwlanapi.EnumInterfaces(m.handle)
	if err != nil || len(interfaces) == 0 {
		return adapter.WIFIState{}
	}

	for _, iface := range interfaces {
		if iface.InterfaceState != winwlanapi.InterfaceStateConnected {
			continue
		}

		guid := iface.InterfaceGUID
		attrs, err := winwlanapi.QueryCurrentConnection(m.handle, &guid)
		if err != nil {
			continue
		}

		ssidLength := attrs.AssociationAttributes.SSID.Length
		if ssidLength == 0 || ssidLength > winwlanapi.Dot11SSIDMaxLength {
			continue
		}

		ssid := string(attrs.AssociationAttributes.SSID.SSID[:ssidLength])
		bssid := formatBSSID(attrs.AssociationAttributes.BSSID)

		return adapter.WIFIState{
			SSID:  strings.TrimSpace(ssid),
			BSSID: bssid,
		}
	}

	return adapter.WIFIState{}
}

func formatBSSID(mac winwlanapi.Dot11MacAddress) string {
	return fmt.Sprintf("%02X%02X%02X%02X%02X%02X",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

func (m *windowsWIFIMonitor) Start() error {
	if m.callback == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.lastState = m.ReadWIFIState()

	callbackFunc := func(data *winwlanapi.NotificationData, callbackContext uintptr) uintptr {
		if data.NotificationSource != winwlanapi.NotificationSourceACM {
			return 0
		}
		switch data.NotificationCode {
		case winwlanapi.NotificationACMConnectionComplete,
			winwlanapi.NotificationACMDisconnected:
			m.checkAndNotify()
		}
		return 0
	}

	callbackPointer := syscall.NewCallback(callbackFunc)

	err := winwlanapi.RegisterNotification(m.handle, winwlanapi.NotificationSourceACM, callbackPointer, 0)
	if err != nil {
		cancel()
		return err
	}

	go func() {
		<-ctx.Done()
	}()

	m.callback(m.lastState)
	return nil
}

func (m *windowsWIFIMonitor) checkAndNotify() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	state := m.ReadWIFIState()
	if state != m.lastState {
		m.lastState = state
		if m.callback != nil {
			m.callback(state)
		}
	}
}

func (m *windowsWIFIMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	winwlanapi.UnregisterNotification(m.handle)
	return winwlanapi.CloseHandle(m.handle)
}
