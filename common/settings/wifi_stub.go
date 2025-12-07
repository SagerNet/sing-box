//go:build !linux

package settings

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
)

type stubWIFIMonitor struct{}

func NewWIFIMonitor(callback func(adapter.WIFIState)) (WIFIMonitor, error) {
	return nil, os.ErrInvalid
}

func (m *stubWIFIMonitor) ReadWIFIState() adapter.WIFIState {
	return adapter.WIFIState{}
}

func (m *stubWIFIMonitor) Start() error {
	return nil
}

func (m *stubWIFIMonitor) Close() error {
	return nil
}
