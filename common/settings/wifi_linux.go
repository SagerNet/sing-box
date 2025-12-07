package settings

import (
	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

type LinuxWIFIMonitor struct {
	monitor WIFIMonitor
}

func NewWIFIMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	monitors := []func(func(adapter.WIFIState)) (WIFIMonitor, error){
		newNetworkManagerMonitor,
		newIWDMonitor,
		newWpaSupplicantMonitor,
		newConnManMonitor,
	}
	var errors []error
	for _, factory := range monitors {
		monitor, err := factory(callback)
		if err == nil {
			return &LinuxWIFIMonitor{monitor: monitor}, nil
		}
		errors = append(errors, err)
	}
	return nil, E.Cause(E.Errors(errors...), "no supported WIFI manager found")
}

func (m *LinuxWIFIMonitor) ReadWIFIState() adapter.WIFIState {
	return m.monitor.ReadWIFIState()
}

func (m *LinuxWIFIMonitor) Start() error {
	if m.monitor != nil {
		return m.monitor.Start()
	}
	return nil
}

func (m *LinuxWIFIMonitor) Close() error {
	if m.monitor != nil {
		return m.monitor.Close()
	}
	return nil
}
