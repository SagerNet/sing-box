package bridge

import (
	"net/netip"
	"os"
	"slices"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"

	"golang.org/x/sys/unix"
)

type ServiceOptions struct {
	MTU       int
	Inet4Port netip.Addr
	Inet6Port netip.Addr
	Interface string
	Logger    logger.ContextLogger
}

type Service struct {
	serviceBase

	boundInterface string
	inet4Local     netip.Addr
	inet6Local     netip.Addr
	anchorName     string
	pfDevice       *pfDevice
	pfToken        uint64
	currentRules   []pfAnchorRule
}

func NewService(options ServiceOptions) (*Service, error) {
	if !options.Inet4Port.IsValid() {
		return nil, E.New("missing bridge IPv4 port address")
	}
	serviceLogger := options.Logger
	if serviceLogger == nil {
		serviceLogger = logger.NOP()
	}
	instance := &Service{
		serviceBase: serviceBase{
			logger:            serviceLogger,
			mtu:               options.MTU,
			inet4Port:         options.Inet4Port,
			inet6Port:         options.Inet6Port,
			tunFileDescriptor: -1,
		},
		boundInterface: options.Interface,
	}
	instance.applyEgress = instance.syncEgressLocked
	index, err := bridgeIndexOf(options.Inet4Port)
	if err != nil {
		return nil, err
	}
	instance.inet4Local = addressAt(bridgeInet4LocalBase, index)
	instance.inet6Local = addressAt(bridgeInet6LocalBase, index)
	err = instance.start()
	if err != nil {
		instance.Close()
		return nil, err
	}
	return instance, nil
}

func bridgeIndexOf(inet4Port netip.Addr) (uint32, error) {
	for index := range uint32(bridgeMaxInstances) {
		if addressAt(bridgeInet4Base, index) == inet4Port {
			return index, nil
		}
	}
	return 0, E.New("unexpected bridge IPv4 port address: ", inet4Port)
}

func (s *Service) start() error {
	tunFileDescriptor, tunName, err := createBridgeTun(s.mtu)
	if err != nil {
		return E.Cause(err, "create bridge tun")
	}
	s.tunFileDescriptor = tunFileDescriptor
	s.tunName = tunName
	s.anchorName = bridgeAnchor(tunName)
	s.forwardingRestore = enableDarwinForwarding(s.logger, s.inet4Port.IsValid(), s.inet6Port.IsValid())
	err = assignBridgePortAddress(tunName, s.inet4Local, s.inet4Port)
	if err != nil {
		return E.Cause(err, "add bridge route")
	}
	err = assignBridgePortAddress(tunName, s.inet6Local, s.inet6Port)
	if err != nil {
		s.logger.Debug(E.Cause(err, "IPv6 bridge routing unavailable, disabling IPv6 forwarding"))
		s.inet6Port = netip.Addr{}
	}
	device, err := openPfDevice()
	if err != nil {
		return E.Cause(err, "enable pf")
	}
	s.pfDevice = device
	token, err := device.StartReference()
	if err != nil {
		return E.Cause(err, "enable pf")
	}
	s.pfToken = token
	dropRules := bridgeDropRules(s.tunName, s.inet4Port, s.inet6Port)
	err = s.pfDevice.LoadAnchor(s.anchorName, dropRules)
	if err != nil {
		return E.Cause(err, "initialize bridge pf rules")
	}
	s.currentRules = dropRules
	s.startNetworkMonitor()
	return nil
}

func (s *Service) syncEgressLocked() error {
	rules := bridgeDropRules(s.tunName, s.inet4Port, s.inet6Port)
	var buildErr error
	if s.egressName != "" {
		rules, buildErr = buildBridgeAnchorRules(s.tunName, s.egressName, s.boundInterface, s.inet4Port, s.inet6Port)
	}
	if slices.Equal(rules, s.currentRules) {
		return buildErr
	}
	err := s.pfDevice.LoadAnchor(s.anchorName, rules)
	if err != nil {
		return E.Cause(err, "apply bridge egress ", s.egressName)
	}
	s.currentRules = rules
	if buildErr != nil || s.egressName == "" {
		s.logger.Debug("bridge egress unavailable, dropping forwarded traffic")
	} else {
		s.logger.Debug("bridge egress ", s.egressName)
	}
	return buildErr
}

func (s *Service) Close() error {
	if !s.beginClose() {
		return nil
	}
	s.access.Lock()
	defer s.access.Unlock()
	if s.pfDevice != nil {
		// anchorName is set before pfDevice is opened, so a non-nil pfDevice means
		// it holds the intended target (the sub-anchor on macOS, "" on iOS).
		_ = s.pfDevice.LoadAnchor(s.anchorName, nil)
		if s.pfToken != 0 {
			_ = s.pfDevice.StopReference(s.pfToken)
		}
		_ = s.pfDevice.Close()
		s.pfDevice = nil
	}
	restoreDarwinForwarding(s.forwardingRestore)
	s.forwardingRestore = nil
	if s.tunFileDescriptor >= 0 {
		_ = unix.Close(s.tunFileDescriptor)
		s.tunFileDescriptor = -1
	}
	return nil
}

// The stock macOS /etc/pf.conf ends its main ruleset with wildcard
// nat/rdr/scrub/anchor references to "com.apple/*", so rules loaded into a
// sub-anchor below it are evaluated without editing the main ruleset. iOS ships
// no /etc/pf.conf and no such references, leaving the main ruleset empty and
// pf-unused; there an anchor is never traversed, so we own the main ruleset
// directly (anchor "") instead.
func bridgeAnchor(tunName string) string {
	_, err := os.Stat("/etc/pf.conf")
	if err != nil {
		return ""
	}
	return "com.apple/sing-box-" + tunName
}

func createBridgeTun(mtu int) (int, string, error) {
	tunFd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2)
	if err != nil {
		return -1, "", os.NewSyscallError("socket", err)
	}
	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], "com.apple.net.utun_control")
	err = unix.IoctlCtlInfo(tunFd, ctlInfo)
	if err != nil {
		unix.Close(tunFd)
		return -1, "", os.NewSyscallError("IoctlCtlInfo", err)
	}
	err = unix.Connect(tunFd, &unix.SockaddrCtl{ID: ctlInfo.Id, Unit: 0})
	if err != nil {
		unix.Close(tunFd)
		return -1, "", os.NewSyscallError("Connect", err)
	}
	name, err := unix.GetsockoptString(
		tunFd,
		2, /* #define SYSPROTO_CONTROL 2 */
		2, /* #define UTUN_OPT_IFNAME 2 */
	)
	if err != nil {
		unix.Close(tunFd)
		return -1, "", os.NewSyscallError("GetsockoptString", err)
	}
	socketFd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		unix.Close(tunFd)
		return -1, "", os.NewSyscallError("socket", err)
	}
	ifr := unix.IfreqMTU{MTU: int32(mtu)}
	copy(ifr.Name[:], name)
	err = unix.IoctlSetIfreqMTU(socketFd, &ifr)
	unix.Close(socketFd)
	if err != nil {
		unix.Close(tunFd)
		return -1, "", os.NewSyscallError("IoctlSetIfreqMTU", err)
	}
	return tunFd, name, nil
}
