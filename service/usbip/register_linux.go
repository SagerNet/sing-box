//go:build linux

package usbip

import (
	boxService "github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.USBIPServerServiceOptions](registry, C.TypeUSBIPServer, NewServerService)
	boxService.Register[option.USBIPClientServiceOptions](registry, C.TypeUSBIPClient, NewClientService)
}
