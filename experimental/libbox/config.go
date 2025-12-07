package libbox

import (
	"bytes"
	"context"
	"os"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	tun "github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/logger"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
)

func BaseContext(platformInterface PlatformInterface) context.Context {
	dnsRegistry := include.DNSTransportRegistry()
	if platformInterface != nil {
		if localTransport := platformInterface.LocalDNSTransport(); localTransport != nil {
			dns.RegisterTransport[option.LocalDNSServerOptions](dnsRegistry, C.DNSTypeLocal, func(ctx context.Context, logger log.ContextLogger, tag string, options option.LocalDNSServerOptions) (adapter.DNSTransport, error) {
				return newPlatformTransport(localTransport, tag, options), nil
			})
		}
	}
	ctx := context.Background()
	ctx = filemanager.WithDefault(ctx, sWorkingPath, sTempPath, sUserID, sGroupID)
	return box.Context(ctx, include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), dnsRegistry, include.ServiceRegistry())
}

func parseConfig(ctx context.Context, configContent string) (option.Options, error) {
	options, err := json.UnmarshalExtendedContext[option.Options](ctx, []byte(configContent))
	if err != nil {
		return option.Options{}, E.Cause(err, "decode config")
	}
	return options, nil
}

func CheckConfig(configContent string) error {
	ctx := BaseContext(nil)
	options, err := parseConfig(ctx, configContent)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = service.ContextWith[adapter.PlatformInterface](ctx, (*platformInterfaceStub)(nil))
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
	})
	if err == nil {
		instance.Close()
	}
	return err
}

type platformInterfaceStub struct{}

func (s *platformInterfaceStub) Initialize(networkManager adapter.NetworkManager) error {
	return nil
}

func (s *platformInterfaceStub) UsePlatformAutoDetectInterfaceControl() bool {
	return true
}

func (s *platformInterfaceStub) AutoDetectInterfaceControl(fd int) error {
	return nil
}

func (s *platformInterfaceStub) UsePlatformInterface() bool {
	return false
}

func (s *platformInterfaceStub) OpenInterface(options *tun.Options, platformOptions option.TunPlatformOptions) (tun.Tun, error) {
	return nil, os.ErrInvalid
}

func (s *platformInterfaceStub) UsePlatformDefaultInterfaceMonitor() bool {
	return true
}

func (s *platformInterfaceStub) CreateDefaultInterfaceMonitor(logger logger.Logger) tun.DefaultInterfaceMonitor {
	return (*interfaceMonitorStub)(nil)
}

func (s *platformInterfaceStub) UsePlatformNetworkInterfaces() bool {
	return false
}

func (s *platformInterfaceStub) NetworkInterfaces() ([]adapter.NetworkInterface, error) {
	return nil, os.ErrInvalid
}

func (s *platformInterfaceStub) UnderNetworkExtension() bool {
	return false
}

func (s *platformInterfaceStub) NetworkExtensionIncludeAllNetworks() bool {
	return false
}

func (s *platformInterfaceStub) ClearDNSCache() {
}

func (s *platformInterfaceStub) RequestPermissionForWIFIState() error {
	return nil
}

func (s *platformInterfaceStub) UsePlatformWIFIMonitor() bool {
	return false
}

func (s *platformInterfaceStub) ReadWIFIState() adapter.WIFIState {
	return adapter.WIFIState{}
}

func (s *platformInterfaceStub) SystemCertificates() []string {
	return nil
}

func (s *platformInterfaceStub) UsePlatformConnectionOwnerFinder() bool {
	return false
}

func (s *platformInterfaceStub) FindConnectionOwner(request *adapter.FindConnectionOwnerRequest) (*adapter.ConnectionOwner, error) {
	return nil, os.ErrInvalid
}

func (s *platformInterfaceStub) UsePlatformNotification() bool {
	return false
}

func (s *platformInterfaceStub) SendNotification(notification *adapter.Notification) error {
	return nil
}

func (s *platformInterfaceStub) UsePlatformLocalDNSTransport() bool {
	return false
}

func (s *platformInterfaceStub) LocalDNSTransport() dns.TransportConstructorFunc[option.LocalDNSServerOptions] {
	return nil
}

type interfaceMonitorStub struct{}

func (s *interfaceMonitorStub) Start() error {
	return os.ErrInvalid
}

func (s *interfaceMonitorStub) Close() error {
	return os.ErrInvalid
}

func (s *interfaceMonitorStub) DefaultInterface() *control.Interface {
	return nil
}

func (s *interfaceMonitorStub) OverrideAndroidVPN() bool {
	return false
}

func (s *interfaceMonitorStub) AndroidVPNEnabled() bool {
	return false
}

func (s *interfaceMonitorStub) RegisterCallback(callback tun.DefaultInterfaceUpdateCallback) *list.Element[tun.DefaultInterfaceUpdateCallback] {
	return nil
}

func (s *interfaceMonitorStub) UnregisterCallback(element *list.Element[tun.DefaultInterfaceUpdateCallback]) {
}

func (s *interfaceMonitorStub) RegisterMyInterface(interfaceName string) {
}

func (s *interfaceMonitorStub) MyInterface() string {
	return ""
}

func FormatConfig(configContent string) (*StringBox, error) {
	options, err := parseConfig(BaseContext(nil), configContent)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(options)
	if err != nil {
		return nil, err
	}
	return wrapString(buffer.String()), nil
}
