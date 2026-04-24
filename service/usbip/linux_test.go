//go:build linux

package usbip

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type testDialer struct{}

func (testDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, destination.String())
}

func (testDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

type failingDialer struct {
	err error
}

func (d failingDialer) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	return nil, d.err
}

func (d failingDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

type opaqueConn struct {
	net.Conn
}

type wrappingDialer struct{}

func (wrappingDialer) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, network, destination.String())
	if err != nil {
		return nil, err
	}
	return opaqueConn{Conn: conn}, nil
}

func (wrappingDialer) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, errors.New("unused")
}

type testDeviceStore struct {
	mu       sync.Mutex
	devices  map[string]sysfsDevice
	statuses map[string]int
	sockfds  map[string]int
}

func newTestDeviceStore(devices ...sysfsDevice) *testDeviceStore {
	store := &testDeviceStore{
		devices:  make(map[string]sysfsDevice),
		statuses: make(map[string]int),
		sockfds:  make(map[string]int),
	}
	store.setDevices(devices...)
	return store
}

func (s *testDeviceStore) setDevices(devices ...sysfsDevice) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.devices = make(map[string]sysfsDevice, len(devices))
	for _, device := range devices {
		s.devices[device.BusID] = device
	}
}

func (s *testDeviceStore) setStatus(busid string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[busid] = status
}

func (s *testDeviceStore) listUSBDevices() ([]sysfsDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]sysfsDevice, 0, len(s.devices))
	for _, device := range s.devices {
		out = append(out, device)
	}
	slices.SortFunc(out, func(left, right sysfsDevice) int {
		switch {
		case left.BusID < right.BusID:
			return -1
		case left.BusID > right.BusID:
			return 1
		default:
			return 0
		}
	})
	return out, nil
}

func (s *testDeviceStore) readSysfsDevice(busid, path string) (sysfsDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, ok := s.devices[busid]
	if !ok {
		return sysfsDevice{}, os.ErrNotExist
	}
	return device, nil
}

func (s *testDeviceStore) readUsbipStatus(busid string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.statuses[busid]
	if !ok {
		return 0, os.ErrNotExist
	}
	return status, nil
}

func (s *testDeviceStore) writeUsbipSockfd(busid string, fd int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockfds[busid] = fd
	return nil
}

func (s *testDeviceStore) lastSockfd(busid string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sockfds[busid]
}

func newTestUSBIPOps(t *testing.T) usbipOps {
	t.Helper()

	return usbipOps{
		ensureHostDriver: func() error {
			t.Fatalf("unexpected ensureHostDriver")
			return nil
		},
		ensureVHCI: func() error {
			t.Fatalf("unexpected ensureVHCI")
			return nil
		},
		listUSBDevices: func() ([]sysfsDevice, error) {
			t.Fatalf("unexpected listUSBDevices")
			return nil, nil
		},
		readSysfsDevice: func(string, string) (sysfsDevice, error) {
			t.Fatalf("unexpected readSysfsDevice")
			return sysfsDevice{}, nil
		},
		currentDriver: func(string) (string, error) {
			t.Fatalf("unexpected currentDriver")
			return "", nil
		},
		unbindFromDriver: func(string, string) error {
			t.Fatalf("unexpected unbindFromDriver")
			return nil
		},
		bindToDriver: func(string, string) error {
			t.Fatalf("unexpected bindToDriver")
			return nil
		},
		hostMatchBusID: func(string, bool) error {
			t.Fatalf("unexpected hostMatchBusID")
			return nil
		},
		hostBind: func(string) error {
			t.Fatalf("unexpected hostBind")
			return nil
		},
		hostUnbind: func(string) error {
			t.Fatalf("unexpected hostUnbind")
			return nil
		},
		readUsbipStatus: func(string) (int, error) {
			t.Fatalf("unexpected readUsbipStatus")
			return 0, nil
		},
		writeUsbipSockfd: func(string, int) error {
			t.Fatalf("unexpected writeUsbipSockfd")
			return nil
		},
		newUEventListener: func() (usbEventListener, error) {
			t.Fatalf("unexpected newUEventListener")
			return nil, nil
		},
		vhciPickFreePort: func(uint32) (int, error) {
			t.Fatalf("unexpected vhciPickFreePort")
			return 0, nil
		},
		vhciAttach: func(int, uintptr, uint32, uint32) error {
			t.Fatalf("unexpected vhciAttach")
			return nil
		},
		vhciDetach: func(int) error {
			t.Fatalf("unexpected vhciDetach")
			return nil
		},
		vhciPortUsed: func(int) (bool, error) {
			t.Fatalf("unexpected vhciPortUsed")
			return false, nil
		},
	}
}

func newTestLogger() log.ContextLogger {
	if os.Getenv("CODEX_USBIP_TEST_LOG") != "" {
		factory := log.NewDefaultFactory(
			context.Background(),
			log.Formatter{
				BaseTime:      time.Now(),
				DisableColors: true,
			},
			os.Stderr,
			"",
			nil,
			false,
		)
		factory.SetLevel(log.LevelTrace)
		return factory.NewLogger("usbip")
	}
	return log.NewNOPFactory().NewLogger("usbip")
}

func newTestDevice(busid string, vendorID, productID uint16, serial string, speed uint32) sysfsDevice {
	return sysfsDevice{
		BusID:         busid,
		Path:          sysBusDevicePath(busid),
		BusNum:        3,
		DevNum:        9,
		Speed:         speed,
		VendorID:      vendorID,
		ProductID:     productID,
		DeviceClass:   0,
		ConfigValue:   1,
		NumConfigs:    1,
		NumInterfaces: 1,
		Serial:        serial,
		Interfaces: []DeviceInterface{{
			BInterfaceClass: 0xff,
		}},
	}
}

func startDispatchServer(t *testing.T, server *ServerService) (M.Socksaddr, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go server.dispatchConn(conn)
		}
	}()

	return M.SocksaddrFromNet(listener.Addr()), func() {
		_ = listener.Close()
		<-done
	}
}

func duplicateConnFromFD(t *testing.T, fd uintptr, name string) net.Conn {
	t.Helper()

	conn, err := duplicateNetConnFromFD(fd, name)
	require.NoError(t, err)
	return conn
}

func duplicateNetConnFromFD(fd uintptr, name string) (net.Conn, error) {
	dupFD, err := unix.Dup(int(fd))
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(dupFD), name)
	conn, err := net.FileConn(file)
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return conn, nil
}

func duplicateHandoffKernelConn(t *testing.T, handoff *usbipConnHandoff) net.Conn {
	t.Helper()

	conn := duplicateConnFromFD(t, handoff.kernelFD(), "usbip-test-kernel")
	require.NoError(t, handoff.closeKernelFD())
	return conn
}

func requireConnRead(t *testing.T, conn net.Conn, expected []byte) {
	t.Helper()

	buffer := make([]byte, len(expected))
	_, err := io.ReadFull(conn, buffer)
	require.NoError(t, err)
	require.Equal(t, expected, buffer)
}

func requireConnEOF(t *testing.T, conn net.Conn) {
	t.Helper()

	buffer := make([]byte, 1)
	n, err := conn.Read(buffer)
	require.Zero(t, n)
	require.ErrorIs(t, err, io.EOF)
}

func setConnDeadline(t *testing.T, conn net.Conn) {
	t.Helper()

	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
}

func requireStreamSocketFD(t *testing.T, fd uintptr) {
	t.Helper()

	socketType, err := unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_TYPE)
	require.NoError(t, err)
	require.Equal(t, unix.SOCK_STREAM, socketType)
}

type testUSBGadget struct {
	path   string
	serial string
	busid  string
}

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("root required")
	}
}

func requireKernelModule(t *testing.T, module string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join("/sys/module", module)); err == nil {
		return
	}

	modprobePath, err := findModprobePath()
	if err != nil {
		t.Skipf("kernel module %s unavailable: %v", module, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, modprobePath, module)
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Skipf("modprobe %s timed out: %s", module, string(output))
	}
	if err != nil {
		t.Skipf("kernel module %s unavailable: %v: %s", module, err, string(output))
	}
}

func requireUSBIPHost(t *testing.T) {
	t.Helper()
	if err := ensureHostDriver(); err != nil {
		t.Skipf("usbip-host unavailable: %v", err)
	}
}

func requireVHCI(t *testing.T) {
	t.Helper()
	if err := ensureVHCI(); err != nil {
		t.Skipf("vhci_hcd unavailable: %v", err)
	}
}

func writeSysfsLine(path string, content string) error {
	return os.WriteFile(path, []byte(content+"\n"), 0)
}

func newTestUSBGadget(t *testing.T) *testUSBGadget {
	t.Helper()
	requireRoot(t)

	requireKernelModule(t, "configfs")
	requireKernelModule(t, "libcomposite")
	requireKernelModule(t, "dummy_hcd")

	udcs, err := os.ReadDir("/sys/class/udc")
	if err != nil {
		t.Skipf("USB device controllers unavailable: %v", err)
	}
	if len(udcs) == 0 {
		t.Skip("USB device controllers unavailable")
	}

	gadget := &testUSBGadget{
		path:   filepath.Join("/sys/kernel/config/usb_gadget", fmt.Sprintf("codex_usbip_%d", time.Now().UnixNano())),
		serial: fmt.Sprintf("codex-usbip-%d", time.Now().UnixNano()),
	}
	require.NoError(t, os.MkdirAll(filepath.Join(gadget.path, "strings/0x409"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(gadget.path, "configs/c.1/strings/0x409"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(gadget.path, "functions/acm.usb0"), 0o755))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "idVendor"), "0x1d6b"))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "idProduct"), "0x0104"))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/serialnumber"), gadget.serial))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/manufacturer"), "OpenAI"))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/product"), "Codex USBIP Test"))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "configs/c.1/strings/0x409/configuration"), "config-1"))
	require.NoError(t, os.Symlink(filepath.Join(gadget.path, "functions/acm.usb0"), filepath.Join(gadget.path, "configs/c.1/acm.usb0")))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "UDC"), udcs[0].Name()))

	require.Eventually(t, func() bool {
		devices, err := listUSBDevices()
		if err != nil {
			return false
		}
		for i := range devices {
			if devices[i].VendorID == 0x1d6b &&
				devices[i].ProductID == 0x0104 &&
				devices[i].Serial == gadget.serial {
				gadget.busid = devices[i].BusID
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond)

	t.Cleanup(func() {
		if gadget.busid != "" {
			if driver, err := currentDriver(gadget.busid); err == nil {
				switch driver {
				case "usbip-host":
					_ = hostUnbind(gadget.busid)
					_ = hostMatchBusID(gadget.busid, false)
					_ = bindToDriver(gadget.busid, "usb")
				case "usb":
				case "":
				default:
					_ = bindToDriver(gadget.busid, "usb")
				}
			}
		}

		_ = writeSysfsLine(filepath.Join(gadget.path, "UDC"), "")
		_ = os.Remove(filepath.Join(gadget.path, "configs/c.1/acm.usb0"))
		_ = os.Remove(filepath.Join(gadget.path, "functions/acm.usb0"))
		_ = os.Remove(filepath.Join(gadget.path, "configs/c.1/strings/0x409"))
		_ = os.Remove(filepath.Join(gadget.path, "configs/c.1"))
		_ = os.Remove(filepath.Join(gadget.path, "strings/0x409"))
		_ = os.Remove(gadget.path)
	})

	return gadget
}

func TestBuildTargetsDedupesFixedBusID(t *testing.T) {
	t.Parallel()

	client := &ClientService{
		matches: []option.USBIPDeviceMatch{
			{BusID: "1-1"},
			{VendorID: 0x1d6b, ProductID: 0x0002},
			{BusID: "1-1"},
			{BusID: "1-2"},
		},
	}

	require.Equal(t, []clientTarget{
		{fixedBusID: "1-1"},
		{match: option.USBIPDeviceMatch{VendorID: 0x1d6b, ProductID: 0x0002}},
		{fixedBusID: "1-2"},
	}, client.buildTargets())
}

func TestClientApplyRemoteExportsKeepsActiveBusIDWorker(t *testing.T) {
	t.Parallel()

	canceled := false
	client := &ClientService{
		ctx:          context.Background(),
		logger:       newTestLogger(),
		allWorkers:   map[string]*clientBusIDWorker{"1-1": {cancel: func() { canceled = true }}},
		activeBusIDs: map[string]struct{}{"1-1": {}},
		ops:          newTestUSBIPOps(t),
	}

	client.applyRemoteExports(nil)

	require.False(t, canceled)
	require.Contains(t, client.allWorkers, "1-1")

	client.setBusIDActive("1-1", false)
	client.applyRemoteExports(nil)

	require.True(t, canceled)
	require.NotContains(t, client.allWorkers, "1-1")
}

func TestClientShouldRetryBusIDRefreshesImportAllState(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         newTestUSBIPOps(t),
	}
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	canceled := false
	client := &ClientService{
		ctx:          context.Background(),
		logger:       newTestLogger(),
		dialer:       testDialer{},
		serverAddr:   serverAddr,
		allWorkers:   map[string]*clientBusIDWorker{"1-1": {cancel: func() { canceled = true }}},
		allDesired:   map[string]struct{}{"1-1": {}},
		activeBusIDs: make(map[string]struct{}),
		ops:          newTestUSBIPOps(t),
	}

	require.False(t, client.shouldRetryBusID(context.Background(), "1-1"))
	require.True(t, canceled)
	require.NotContains(t, client.allWorkers, "1-1")
	require.Empty(t, client.allDesired)
}

func TestClientShouldRetryBusIDKeepsRetryOnRefreshFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("devlist unavailable")
	canceled := false
	client := &ClientService{
		ctx:          context.Background(),
		logger:       newTestLogger(),
		dialer:       failingDialer{err: expectedErr},
		serverAddr:   M.ParseSocksaddrHostPort("127.0.0.1", 3240),
		allWorkers:   map[string]*clientBusIDWorker{"1-1": {cancel: func() { canceled = true }}},
		allDesired:   map[string]struct{}{"1-1": {}},
		activeBusIDs: make(map[string]struct{}),
		ops:          newTestUSBIPOps(t),
	}

	require.True(t, client.shouldRetryBusID(context.Background(), "1-1"))
	require.False(t, canceled)
	require.Contains(t, client.allWorkers, "1-1")
	require.Contains(t, client.allDesired, "1-1")
}

func TestAssignMatchedBusIDs(t *testing.T) {
	t.Parallel()

	match := option.USBIPDeviceMatch{VendorID: 0x1d6b, ProductID: 0x0002}
	fixed := newTestDevice("1-1", 0x1d6b, 0x0001, "fixed", SpeedHigh)
	first := newTestDevice("1-2", 0x1d6b, 0x0002, "first", SpeedHigh)
	second := newTestDevice("1-3", 0x1d6b, 0x0002, "second", SpeedHigh)
	entries := []DeviceEntry{
		{Info: fixed.toProtocol()},
		{Info: first.toProtocol()},
		{Info: second.toProtocol()},
	}

	require.Equal(t, []string{"1-1", "1-3", "1-2"}, assignMatchedBusIDs(
		[]clientTarget{
			{fixedBusID: "1-1"},
			{match: match},
			{match: match},
		},
		[]string{"1-1", "1-3", ""},
		entries,
	))
}

func TestLinuxHelpers(t *testing.T) {
	t.Parallel()

	require.Equal(t, []vhciStatusRecord{
		{hub: "hs", port: 0, state: 6},
		{hub: "ss", port: 3, state: 4},
	}, parseVHCIStatus("hub port sta spd dev sockfd local_busid\nhs 0 6 3 0 0 0\nignored line\nss 3 4 5 0 0 0\n"))

	require.Equal(t, SpeedLow, speedCodeFromString("1.5"))
	require.Equal(t, SpeedFull, speedCodeFromString("12"))
	require.Equal(t, SpeedHigh, speedCodeFromString("480"))
	require.Equal(t, SpeedSuper, speedCodeFromString("5000"))
	require.Equal(t, SpeedSuperPlus, speedCodeFromString("10000"))
	require.Equal(t, SpeedUnknown, speedCodeFromString("25"))

	require.Equal(t, "hs", vhciHubForSpeed(SpeedHigh))
	require.Equal(t, "ss", vhciHubForSpeed(SpeedSuper))
	require.True(t, isUSBUEvent([]byte("ACTION=add\x00SUBSYSTEM=usb\x00")))
	require.False(t, isUSBUEvent([]byte("ACTION=add\x00SUBSYSTEM=net\x00")))
}

func TestUSBIPConnHandoffDirectTCP(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	acceptedConn := <-accepted
	defer acceptedConn.Close()

	handoff, err := newUSBIPConnHandoff(conn)
	require.NoError(t, err)
	defer handoff.Close()

	require.False(t, handoff.relay())
	require.Equal(t, "direct", handoff.mode())
	requireStreamSocketFD(t, handoff.kernelFD())
}

func TestUSBIPConnHandoffRelaySocketpairCopies(t *testing.T) {
	t.Parallel()

	left, right := net.Pipe()
	defer right.Close()
	handoff, err := newUSBIPConnHandoff(opaqueConn{Conn: left})
	require.NoError(t, err)
	defer handoff.Close()
	require.True(t, handoff.relay())
	require.Equal(t, "relay", handoff.mode())
	requireStreamSocketFD(t, handoff.kernelFD())

	kernelConn := duplicateHandoffKernelConn(t, handoff)
	defer kernelConn.Close()
	setConnDeadline(t, right)
	setConnDeadline(t, kernelConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.True(t, handoff.startRelay(ctx, newTestLogger(), "test", "relay"))

	_, err = right.Write([]byte("ping"))
	require.NoError(t, err)
	requireConnRead(t, kernelConn, []byte("ping"))

	_, err = kernelConn.Write([]byte("pong"))
	require.NoError(t, err)
	requireConnRead(t, right, []byte("pong"))
}

func TestServerStartRequiresHostDriver(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("host driver unavailable")
	server := &ServerService{
		ctx:         context.Background(),
		logger:      newTestLogger(),
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops: usbipOps{
			ensureHostDriver: func() error { return expectedErr },
		},
	}

	err := server.Start(adapter.StartStateStart)
	require.ErrorIs(t, err, expectedErr)
}

func TestClientStartRequiresVHCI(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("vhci unavailable")
	client := &ClientService{
		ctx:    context.Background(),
		logger: newTestLogger(),
		ops: usbipOps{
			ensureVHCI: func() error { return expectedErr },
		},
	}

	err := client.Start(adapter.StartStateStart)
	require.ErrorIs(t, err, expectedErr)
}

func TestServerReconcileExportsBindsMatchesAndSkipsHub(t *testing.T) {
	t.Parallel()

	regular := newTestDevice("1-1", 0x1d6b, 0x0002, "regular", SpeedHigh)
	hub := newTestDevice("1-2", 0x1d6b, 0x0002, "hub", SpeedHigh)
	hub.DeviceClass = 0x09
	store := newTestDeviceStore(regular, hub)
	ops := newTestUSBIPOps(t)
	var actions []string
	ops.listUSBDevices = store.listUSBDevices
	ops.currentDriver = func(busid string) (string, error) {
		return map[string]string{
			"1-1": "usbhid",
			"1-2": "hubdrv",
		}[busid], nil
	}
	ops.unbindFromDriver = func(busid, driver string) error {
		actions = append(actions, "unbind "+busid+" "+driver)
		return nil
	}
	ops.hostMatchBusID = func(busid string, add bool) error {
		actions = append(actions, "match "+busid+" "+map[bool]string{true: "add", false: "del"}[add])
		return nil
	}
	ops.hostBind = func(busid string) error {
		actions = append(actions, "hostbind "+busid)
		return nil
	}
	ops.bindToDriver = func(busid, driver string) error {
		actions = append(actions, "bind "+busid+" "+driver)
		return nil
	}

	server := &ServerService{
		ctx:         context.Background(),
		logger:      newTestLogger(),
		matches:     []option.USBIPDeviceMatch{{VendorID: 0x1d6b, ProductID: 0x0002}},
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         ops,
	}

	changed, err := server.reconcileExports()
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{
		"unbind 1-1 usbhid",
		"match 1-1 add",
		"hostbind 1-1",
	}, actions)
	require.Equal(t, map[string]serverExport{
		"1-1": {
			busid:          "1-1",
			managed:        true,
			originalDriver: "usbhid",
		},
	}, server.snapshotExports())
}

func TestServerReconcileExportsSkipsVHCIDevices(t *testing.T) {
	t.Parallel()

	physical := newTestDevice("1-1", 0x1d6b, 0x0002, "physical", SpeedHigh)
	imported := newTestDevice("3-1", 0x1d6b, 0x0002, "imported", SpeedHigh)
	imported.Path = "/sys/devices/platform/vhci_hcd.0/usb3/3-1"

	store := newTestDeviceStore(physical, imported)
	ops := newTestUSBIPOps(t)
	var bound []string
	ops.listUSBDevices = store.listUSBDevices
	ops.currentDriver = func(busid string) (string, error) {
		return "usb", nil
	}
	ops.unbindFromDriver = func(busid, driver string) error {
		bound = append(bound, "unbind "+busid+" "+driver)
		return nil
	}
	ops.hostMatchBusID = func(busid string, add bool) error {
		bound = append(bound, "match "+busid)
		return nil
	}
	ops.hostBind = func(busid string) error {
		bound = append(bound, "bind "+busid)
		return nil
	}

	server := &ServerService{
		ctx:         context.Background(),
		logger:      newTestLogger(),
		matches:     []option.USBIPDeviceMatch{{VendorID: 0x1d6b, ProductID: 0x0002}},
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         ops,
	}

	changed, err := server.reconcileExports()
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{
		"unbind 1-1 usb",
		"match 1-1",
		"bind 1-1",
	}, bound)
	require.Equal(t, map[string]serverExport{
		"1-1": {
			busid:          "1-1",
			managed:        true,
			originalDriver: "usb",
		},
	}, server.snapshotExports())
}

func TestServerReconcileExportsReleasesRemovedExports(t *testing.T) {
	t.Parallel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "regular", SpeedHigh)
	store := newTestDeviceStore(device)
	ops := newTestUSBIPOps(t)
	var actions []string
	ops.listUSBDevices = store.listUSBDevices
	ops.writeUsbipSockfd = func(busid string, fd int) error {
		actions = append(actions, "sockfd "+busid)
		return nil
	}
	ops.hostUnbind = func(busid string) error {
		actions = append(actions, "hostunbind "+busid)
		return nil
	}
	ops.hostMatchBusID = func(busid string, add bool) error {
		actions = append(actions, "match "+busid+" "+map[bool]string{true: "add", false: "del"}[add])
		return nil
	}
	ops.bindToDriver = func(busid, driver string) error {
		actions = append(actions, "bind "+busid+" "+driver)
		return nil
	}
	ops.readSysfsDevice = store.readSysfsDevice

	server := &ServerService{
		ctx:     context.Background(),
		logger:  newTestLogger(),
		exports: map[string]serverExport{"1-1": {busid: "1-1", managed: true, originalDriver: "usbhid"}},
		ops:     ops,
	}

	changed, err := server.reconcileExports()
	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, server.snapshotExports())
	require.Equal(t, []string{
		"sockfd 1-1",
		"hostunbind 1-1",
		"match 1-1 del",
		"bind 1-1 usbhid",
	}, actions)
}

func TestServerReleaseExportLeavesCooptedSocketUntouched(t *testing.T) {
	t.Parallel()

	ops := newTestUSBIPOps(t)
	var calls []string
	ops.writeUsbipSockfd = func(busid string, fd int) error {
		calls = append(calls, fmt.Sprintf("%s=%d", busid, fd))
		return nil
	}

	server := &ServerService{
		logger:  newTestLogger(),
		exports: map[string]serverExport{"1-1": {busid: "1-1"}},
		ops:     ops,
	}

	err := server.releaseExport(serverExport{busid: "1-1"}, true)
	require.NoError(t, err)
	require.Empty(t, calls)
	require.Empty(t, server.snapshotExports())
}

func TestServerReleaseExportRetainsTrackingOnFailure(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("host unbind failed")
	export := serverExport{
		busid:          "1-1",
		managed:        true,
		originalDriver: "usbhid",
	}

	ops := newTestUSBIPOps(t)
	ops.writeUsbipSockfd = func(string, int) error {
		return nil
	}
	ops.hostUnbind = func(string) error {
		return expectedErr
	}
	ops.hostMatchBusID = func(string, bool) error {
		return nil
	}
	ops.bindToDriver = func(string, string) error {
		return nil
	}

	server := &ServerService{
		logger:  newTestLogger(),
		exports: map[string]serverExport{"1-1": export},
		ops:     ops,
	}

	err := server.releaseExport(export, true)
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, map[string]serverExport{"1-1": export}, server.snapshotExports())
}

func TestServerBuildDevListEntriesFiltersUnavailableAndRefreshFailures(t *testing.T) {
	t.Parallel()

	available := newTestDevice("1-1", 0x1d6b, 0x0002, "ok", SpeedHigh)
	store := newTestDeviceStore(available)
	store.setStatus("1-1", usbipStatusAvailable)
	store.setStatus("1-2", usbipStatusUsed)
	store.setStatus("1-3", usbipStatusAvailable)

	ops := newTestUSBIPOps(t)
	ops.readUsbipStatus = store.readUsbipStatus
	ops.readSysfsDevice = store.readSysfsDevice

	server := &ServerService{
		logger: newTestLogger(),
		exports: map[string]serverExport{
			"1-1": {busid: "1-1"},
			"1-2": {busid: "1-2"},
			"1-3": {busid: "1-3"},
		},
		ops: ops,
	}

	entries := server.buildDevListEntries()
	require.Len(t, entries, 1)
	require.Equal(t, "1-1", entries[0].Info.BusIDString())
	require.Equal(t, "ok", entries[0].Info.SerialString())
}

func TestServerHandleImportWithOpaqueConnRelay(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusAvailable)

	kernelConnCh := make(chan net.Conn, 1)
	kernelErrCh := make(chan error, 1)
	ops := newTestUSBIPOps(t)
	ops.readUsbipStatus = store.readUsbipStatus
	ops.readSysfsDevice = store.readSysfsDevice
	ops.writeUsbipSockfd = func(busid string, fd int) error {
		if fd < 0 {
			return nil
		}
		if busid != "1-1" {
			kernelErrCh <- fmt.Errorf("unexpected busid %s", busid)
			return nil
		}
		socketType, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TYPE)
		if err != nil {
			kernelErrCh <- err
			return nil
		}
		if socketType != unix.SOCK_STREAM {
			kernelErrCh <- fmt.Errorf("unexpected socket type %d", socketType)
			return nil
		}
		kernelConn, err := duplicateNetConnFromFD(uintptr(fd), "usbip-server-test-kernel")
		if err != nil {
			kernelErrCh <- err
			return nil
		}
		kernelConnCh <- kernelConn
		return nil
	}

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         ops,
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.dispatchConn(opaqueConn{Conn: serverConn})

	setConnDeadline(t, clientConn)
	require.NoError(t, WriteOpReqImport(clientConn, "1-1"))
	header, err := ReadOpHeader(clientConn)
	require.NoError(t, err)
	require.Equal(t, OpRepImport, header.Code)
	require.Equal(t, OpStatusOK, header.Status)
	_, err = ReadOpRepImportBody(clientConn)
	require.NoError(t, err)

	var kernelConn net.Conn
	select {
	case kernelConn = <-kernelConnCh:
	case err = <-kernelErrCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server relay kernel conn")
	}
	defer kernelConn.Close()
	setConnDeadline(t, kernelConn)

	_, err = clientConn.Write([]byte("server-in"))
	require.NoError(t, err)
	requireConnRead(t, kernelConn, []byte("server-in"))

	_, err = kernelConn.Write([]byte("server-out"))
	require.NoError(t, err)
	requireConnRead(t, clientConn, []byte("server-out"))
}

func TestServerHandleImportRelayClosesHandoffOnSockfdFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusAvailable)

	expectedErr := errors.New("sockfd handoff failed")
	kernelConnCh := make(chan net.Conn, 1)
	kernelErrCh := make(chan error, 1)
	ops := newTestUSBIPOps(t)
	ops.readUsbipStatus = store.readUsbipStatus
	ops.readSysfsDevice = store.readSysfsDevice
	ops.writeUsbipSockfd = func(busid string, fd int) error {
		if fd < 0 {
			return nil
		}
		if busid != "1-1" {
			kernelErrCh <- fmt.Errorf("unexpected busid %s", busid)
			return expectedErr
		}
		kernelConn, err := duplicateNetConnFromFD(uintptr(fd), "usbip-server-sockfd-failure-kernel")
		if err != nil {
			kernelErrCh <- err
		} else {
			kernelConnCh <- kernelConn
		}
		return expectedErr
	}

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         ops,
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.dispatchConn(opaqueConn{Conn: serverConn})

	setConnDeadline(t, clientConn)
	require.NoError(t, WriteOpReqImport(clientConn, "1-1"))
	header, err := ReadOpHeader(clientConn)
	require.NoError(t, err)
	require.Equal(t, OpRepImport, header.Code)
	require.Equal(t, OpStatusError, header.Status)

	var kernelConn net.Conn
	select {
	case kernelConn = <-kernelConnCh:
	case err = <-kernelErrCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for failed server relay kernel conn")
	}
	defer kernelConn.Close()
	setConnDeadline(t, kernelConn)
	requireConnEOF(t, kernelConn)
}

func TestServerHandleImportRelayClosesHandoffOnReplyFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusAvailable)

	kernelConnCh := make(chan net.Conn, 1)
	kernelErrCh := make(chan error, 1)
	rollbackCh := make(chan string, 1)
	allowReply := make(chan struct{})
	ops := newTestUSBIPOps(t)
	ops.readUsbipStatus = store.readUsbipStatus
	ops.readSysfsDevice = store.readSysfsDevice
	ops.writeUsbipSockfd = func(busid string, fd int) error {
		if fd < 0 {
			rollbackCh <- busid
			return nil
		}
		if busid != "1-1" {
			kernelErrCh <- fmt.Errorf("unexpected busid %s", busid)
			<-allowReply
			return nil
		}
		kernelConn, err := duplicateNetConnFromFD(uintptr(fd), "usbip-server-reply-failure-kernel")
		if err != nil {
			kernelErrCh <- err
		} else {
			kernelConnCh <- kernelConn
		}
		<-allowReply
		return nil
	}

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         ops,
	}

	serverConn, clientConn := net.Pipe()
	go server.dispatchConn(opaqueConn{Conn: serverConn})

	setConnDeadline(t, clientConn)
	require.NoError(t, WriteOpReqImport(clientConn, "1-1"))

	var kernelConn net.Conn
	select {
	case kernelConn = <-kernelConnCh:
	case err := <-kernelErrCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reply-failure relay kernel conn")
	}
	defer kernelConn.Close()
	require.NoError(t, clientConn.Close())
	close(allowReply)

	select {
	case busid := <-rollbackCh:
		require.Equal(t, "1-1", busid)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for import rollback")
	}
	setConnDeadline(t, kernelConn)
	requireConnEOF(t, kernelConn)
}

func TestServerDispatchConnHandlesControlPingAndChanged(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         newTestUSBIPOps(t),
	}
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	conn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, WriteControlPreface(conn))
	require.NoError(t, WriteControlHello(conn))

	ack, err := ReadControlFrame(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameAck, ack.Type)
	require.Equal(t, controlProtocolVersion, ack.Version)
	require.Equal(t, controlCapabilities, ack.Capabilities)
	require.Zero(t, ack.Sequence)

	snapshotMessage, err := readControlMessage(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceSnapshot, snapshotMessage.Frame.Type)
	var snapshot controlDeviceSnapshot
	require.NoError(t, unmarshalControlPayload(snapshotMessage.Payload, &snapshot))
	require.Empty(t, snapshot.Devices)

	require.NoError(t, WriteControlPing(conn))
	pong, err := ReadControlFrame(conn)
	require.NoError(t, err)
	require.Equal(t, controlFramePong, pong.Type)
	require.Equal(t, controlProtocolVersion, pong.Version)

	server.broadcastChanged()
	changed, err := readControlMessage(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceDelta, changed.Frame.Type)
	require.Equal(t, uint64(1), changed.Frame.Sequence)
	var delta controlDeviceDelta
	require.NoError(t, unmarshalControlPayload(changed.Payload, &delta))
	require.Equal(t, uint64(1), delta.Sequence)
}

func TestServerReconcileBroadcastsStatusOnlyDeviceDelta(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusUsed)

	serverOps := newTestUSBIPOps(t)
	serverOps.listUSBDevices = store.listUSBDevices
	serverOps.readUsbipStatus = store.readUsbipStatus
	serverOps.readSysfsDevice = store.readSysfsDevice

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		matches:     []option.USBIPDeviceMatch{{BusID: "1-1"}},
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         serverOps,
	}
	server.refreshControlState()
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	conn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer conn.Close()
	setConnDeadline(t, conn)

	require.NoError(t, WriteControlPreface(conn))
	require.NoError(t, WriteControlHello(conn))
	ack, err := ReadControlFrame(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameAck, ack.Type)

	snapshotMessage, err := readControlMessage(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceSnapshot, snapshotMessage.Frame.Type)
	var snapshot controlDeviceSnapshot
	require.NoError(t, unmarshalControlPayload(snapshotMessage.Payload, &snapshot))
	require.Len(t, snapshot.Devices, 1)
	require.Equal(t, "1-1", snapshot.Devices[0].BusID)
	require.Equal(t, deviceStateBusy, snapshot.Devices[0].State)
	require.Equal(t, usbipStatusUsed, snapshot.Devices[0].StatusCode)

	store.setStatus("1-1", usbipStatusAvailable)
	require.NoError(t, server.reconcileAndBroadcast(true))

	changed, err := readControlMessage(conn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceDelta, changed.Frame.Type)
	require.Equal(t, uint64(1), changed.Frame.Sequence)
	var delta controlDeviceDelta
	require.NoError(t, unmarshalControlPayload(changed.Payload, &delta))
	require.Equal(t, uint64(1), delta.Sequence)
	require.Empty(t, delta.Added)
	require.Empty(t, delta.Removed)
	require.Len(t, delta.Updated, 1)
	require.Equal(t, "1-1", delta.Updated[0].BusID)
	require.Equal(t, deviceStateAvailable, delta.Updated[0].State)
	require.Equal(t, usbipStatusAvailable, delta.Updated[0].StatusCode)

	sequence := server.currentControlSequence()
	require.NoError(t, server.reconcileAndBroadcast(true))
	require.Equal(t, sequence, server.currentControlSequence())

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	_, err = readControlMessage(conn)
	require.Error(t, err)
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	require.True(t, netErr.Timeout())
}

func TestServerControlSnapshotPreservesPendingDelta(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusUsed)

	serverOps := newTestUSBIPOps(t)
	serverOps.listUSBDevices = store.listUSBDevices
	serverOps.readUsbipStatus = store.readUsbipStatus
	serverOps.readSysfsDevice = store.readSysfsDevice

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		matches:     []option.USBIPDeviceMatch{{BusID: "1-1"}},
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         serverOps,
	}
	server.refreshControlState()
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	firstConn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer firstConn.Close()
	setConnDeadline(t, firstConn)
	require.NoError(t, WriteControlPreface(firstConn))
	require.NoError(t, WriteControlHello(firstConn))
	_, err = ReadControlFrame(firstConn)
	require.NoError(t, err)
	firstSnapshot, err := readControlMessage(firstConn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceSnapshot, firstSnapshot.Frame.Type)

	store.setStatus("1-1", usbipStatusAvailable)

	secondConn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer secondConn.Close()
	setConnDeadline(t, secondConn)
	require.NoError(t, WriteControlPreface(secondConn))
	require.NoError(t, WriteControlHello(secondConn))
	_, err = ReadControlFrame(secondConn)
	require.NoError(t, err)
	secondSnapshotMessage, err := readControlMessage(secondConn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceSnapshot, secondSnapshotMessage.Frame.Type)
	var secondSnapshot controlDeviceSnapshot
	require.NoError(t, unmarshalControlPayload(secondSnapshotMessage.Payload, &secondSnapshot))
	require.Len(t, secondSnapshot.Devices, 1)
	require.Equal(t, deviceStateAvailable, secondSnapshot.Devices[0].State)

	require.NoError(t, server.reconcileAndBroadcast(true))
	changed, err := readControlMessage(firstConn)
	require.NoError(t, err)
	require.Equal(t, controlFrameDeviceDelta, changed.Frame.Type)
	var delta controlDeviceDelta
	require.NoError(t, unmarshalControlPayload(changed.Payload, &delta))
	require.Len(t, delta.Updated, 1)
	require.Equal(t, "1-1", delta.Updated[0].BusID)
	require.Equal(t, deviceStateAvailable, delta.Updated[0].State)
}

func TestServerControlLeaseEnablesImportExt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusAvailable)

	serverOps := newTestUSBIPOps(t)
	serverOps.readUsbipStatus = store.readUsbipStatus
	serverOps.readSysfsDevice = store.readSysfsDevice
	serverOps.writeUsbipSockfd = store.writeUsbipSockfd

	server := &ServerService{
		ctx:          ctx,
		cancel:       cancel,
		logger:       newTestLogger(),
		exports:      map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs:  make(map[uint64]*serverControlConn),
		controlState: make(map[string]DeviceInfoV2),
		leases:       make(map[uint64]serverImportLease),
		leaseByBusID: make(map[string]uint64),
		ops:          serverOps,
	}
	server.refreshControlState()
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	controlConn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer controlConn.Close()
	require.NoError(t, WriteControlPreface(controlConn))
	require.NoError(t, WriteControlHello(controlConn))
	ack, err := ReadControlFrame(controlConn)
	require.NoError(t, err)
	require.Equal(t, controlCapabilities, ack.Capabilities)
	_, err = readControlMessage(controlConn)
	require.NoError(t, err)

	require.NoError(t, writeControlMessage(controlConn, controlFrame{
		Type:    controlFrameLeaseRequest,
		Version: controlProtocolVersion,
	}, controlLeaseRequest{BusID: "1-1", ClientNonce: 42}))
	leaseMessage, err := readControlMessage(controlConn)
	require.NoError(t, err)
	require.Equal(t, controlFrameLeaseResponse, leaseMessage.Frame.Type)
	var lease controlLeaseResponse
	require.NoError(t, unmarshalControlPayload(leaseMessage.Payload, &lease))
	require.Empty(t, lease.ErrorCode)
	require.Equal(t, uint64(42), lease.ClientNonce)
	require.NotZero(t, lease.LeaseID)

	require.NoError(t, writeControlMessage(controlConn, controlFrame{
		Type:    controlFrameLeaseRequest,
		Version: controlProtocolVersion,
	}, controlLeaseRequest{BusID: "1-1", ClientNonce: 43}))
	busyMessage, err := readControlMessage(controlConn)
	require.NoError(t, err)
	var busy controlLeaseResponse
	require.NoError(t, unmarshalControlPayload(busyMessage.Payload, &busy))
	require.Equal(t, "busy", busy.ErrorCode)

	importConn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	require.NoError(t, WriteOpReqImportExt(importConn, ImportExtRequest{
		BusID:       "1-1",
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	}))
	header, err := ReadOpHeader(importConn)
	require.NoError(t, err)
	require.Equal(t, OpRepImportExt, header.Code)
	require.Equal(t, OpStatusOK, header.Status)
	info, err := ReadOpRepImportBody(importConn)
	require.NoError(t, err)
	require.Equal(t, "1-1", info.BusIDString())
	require.NoError(t, importConn.Close())
	require.Positive(t, store.lastSockfd("1-1"))

	reuseConn, err := net.Dial("tcp", serverAddr.String())
	require.NoError(t, err)
	defer reuseConn.Close()
	require.NoError(t, WriteOpReqImportExt(reuseConn, ImportExtRequest{
		BusID:       "1-1",
		LeaseID:     lease.LeaseID,
		ClientNonce: lease.ClientNonce,
	}))
	header, err = ReadOpHeader(reuseConn)
	require.NoError(t, err)
	require.Equal(t, OpRepImportExt, header.Code)
	require.Equal(t, OpStatusError, header.Status)
}

func TestClientAttemptAttachUsesImportReplyAndVHCIAttach(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedSuper)
	device.BusNum = 7
	device.DevNum = 11
	store := newTestDeviceStore(device)
	store.setStatus("1-1", usbipStatusAvailable)

	serverOps := newTestUSBIPOps(t)
	serverOps.readUsbipStatus = store.readUsbipStatus
	serverOps.readSysfsDevice = store.readSysfsDevice
	serverOps.writeUsbipSockfd = store.writeUsbipSockfd

	server := &ServerService{
		ctx:         ctx,
		cancel:      cancel,
		logger:      newTestLogger(),
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         serverOps,
	}
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	clientOps := newTestUSBIPOps(t)
	var attachedPort int
	var attachedDevID uint32
	var attachedSpeed uint32
	clientOps.vhciPickFreePort = func(speed uint32) (int, error) {
		require.Equal(t, SpeedSuper, speed)
		return 7, nil
	}
	clientOps.vhciAttach = func(port int, _ uintptr, devid uint32, speed uint32) error {
		attachedPort = port
		attachedDevID = devid
		attachedSpeed = speed
		return nil
	}

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: serverAddr,
		ops:        clientOps,
	}

	port, err := client.attemptAttach(ctx, "1-1")
	require.NoError(t, err)
	require.Equal(t, 7, port)
	require.Equal(t, 7, attachedPort)
	info := device.toProtocol()
	require.Equal(t, info.DevID(), attachedDevID)
	require.Equal(t, SpeedSuper, attachedSpeed)
	require.Positive(t, store.lastSockfd("1-1"))
}

func TestClientAttemptAttachUsesImportExtLease(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	controlClient, controlServer := net.Pipe()
	defer controlClient.Close()
	defer controlServer.Close()

	controlSession := newClientControlSession(controlClient, controlCapabilities)
	controlErrCh := make(chan error, 1)
	go func() {
		message, err := readControlMessage(controlServer)
		if err != nil {
			controlErrCh <- err
			return
		}
		if message.Frame.Type != controlFrameLeaseRequest {
			controlErrCh <- fmt.Errorf("unexpected control frame %d", message.Frame.Type)
			return
		}
		var request controlLeaseRequest
		if err := unmarshalControlPayload(message.Payload, &request); err != nil {
			controlErrCh <- err
			return
		}
		if request.BusID != "1-1" {
			controlErrCh <- fmt.Errorf("unexpected lease busid %s", request.BusID)
			return
		}
		controlErrCh <- writeControlMessage(controlServer, controlFrame{
			Type:    controlFrameLeaseResponse,
			Version: controlProtocolVersion,
		}, controlLeaseResponse{
			BusID:       request.BusID,
			LeaseID:     55,
			ClientNonce: request.ClientNonce,
			Generation:  2,
			TTLMillis:   int64(importLeaseTTL / time.Millisecond),
		})
	}()
	deliverErrCh := make(chan error, 1)
	go func() {
		message, err := readControlMessage(controlClient)
		if err != nil {
			deliverErrCh <- err
			return
		}
		if message.Frame.Type != controlFrameLeaseResponse {
			deliverErrCh <- fmt.Errorf("unexpected control response %d", message.Frame.Type)
			return
		}
		var response controlLeaseResponse
		if err := unmarshalControlPayload(message.Payload, &response); err != nil {
			deliverErrCh <- err
			return
		}
		controlSession.deliverLeaseResponse(response)
		deliverErrCh <- nil
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer conn.Close()
		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if header.Code != OpReqImportExt {
			serverErrCh <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		request, readErr := ReadOpReqImportExtBody(conn)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if request.BusID != "1-1" || request.LeaseID != 55 || request.ClientNonce != 1 {
			serverErrCh <- fmt.Errorf("unexpected import-ext request %+v", request)
			return
		}
		info := device.toProtocol()
		serverErrCh <- WriteOpRepImportExt(conn, OpStatusOK, &info)
	}()

	ops := newTestUSBIPOps(t)
	ops.vhciPickFreePort = func(speed uint32) (int, error) {
		require.Equal(t, SpeedHigh, speed)
		return 4, nil
	}
	ops.vhciAttach = func(port int, _ uintptr, devid uint32, speed uint32) error {
		require.Equal(t, 4, port)
		info := device.toProtocol()
		require.Equal(t, info.DevID(), devid)
		require.Equal(t, SpeedHigh, speed)
		return nil
	}

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        ops,
	}
	client.setControlSession(controlSession)
	defer client.clearControlSession(controlSession, errClientControlSessionClosed)

	port, err := client.attemptAttach(ctx, "1-1")
	require.NoError(t, err)
	require.Equal(t, 4, port)
	require.NoError(t, <-controlErrCh)
	require.NoError(t, <-deliverErrCh)
	require.NoError(t, <-serverErrCh)
}

func TestClientAttemptAttachWithOpaqueConnRelay(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	serverConnCh := make(chan net.Conn, 1)
	serverErrCh := make(chan error, 1)
	serverDone := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			_ = conn.Close()
			serverErrCh <- readErr
			return
		}
		if header.Code != OpReqImport {
			_ = conn.Close()
			serverErrCh <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		busid, readErr := ReadOpReqImportBody(conn)
		if readErr != nil {
			_ = conn.Close()
			serverErrCh <- readErr
			return
		}
		if busid != "1-1" {
			_ = conn.Close()
			serverErrCh <- fmt.Errorf("unexpected busid %s", busid)
			return
		}
		info := device.toProtocol()
		if writeErr := WriteOpRepImport(conn, OpStatusOK, &info); writeErr != nil {
			_ = conn.Close()
			serverErrCh <- writeErr
			return
		}
		serverConnCh <- conn
		<-serverDone
		_ = conn.Close()
		serverErrCh <- nil
	}()
	defer close(serverDone)

	kernelConnCh := make(chan net.Conn, 1)
	ops := newTestUSBIPOps(t)
	ops.vhciPickFreePort = func(speed uint32) (int, error) {
		require.Equal(t, SpeedHigh, speed)
		return 4, nil
	}
	ops.vhciAttach = func(port int, fd uintptr, devid uint32, speed uint32) error {
		require.Equal(t, 4, port)
		requireStreamSocketFD(t, fd)
		info := device.toProtocol()
		require.Equal(t, info.DevID(), devid)
		require.Equal(t, SpeedHigh, speed)
		kernelConnCh <- duplicateConnFromFD(t, fd, "usbip-client-test-kernel")
		return nil
	}

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     wrappingDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        ops,
	}

	port, err := client.attemptAttach(ctx, "1-1")
	require.NoError(t, err)
	require.Equal(t, 4, port)

	var serverConn net.Conn
	select {
	case serverConn = <-serverConnCh:
	case err = <-serverErrCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server conn")
	}
	var kernelConn net.Conn
	select {
	case kernelConn = <-kernelConnCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for client relay kernel conn")
	}
	defer kernelConn.Close()
	setConnDeadline(t, serverConn)
	setConnDeadline(t, kernelConn)

	_, err = serverConn.Write([]byte("client-in"))
	require.NoError(t, err)
	requireConnRead(t, kernelConn, []byte("client-in"))

	_, err = kernelConn.Write([]byte("client-out"))
	require.NoError(t, err)
	requireConnRead(t, serverConn, []byte("client-out"))
}

func TestClientAttemptAttachRelayClosesHandoffOnVHCIAttachFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	serverErrCh := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer conn.Close()
		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if header.Code != OpReqImport {
			serverErrCh <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		busid, readErr := ReadOpReqImportBody(conn)
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		if busid != "1-1" {
			serverErrCh <- fmt.Errorf("unexpected busid %s", busid)
			return
		}
		info := device.toProtocol()
		if writeErr := WriteOpRepImport(conn, OpStatusOK, &info); writeErr != nil {
			serverErrCh <- writeErr
			return
		}
		buffer := make([]byte, 1)
		n, readErr := conn.Read(buffer)
		if n != 0 {
			serverErrCh <- fmt.Errorf("unexpected server read bytes after attach failure: %d", n)
			return
		}
		if !errors.Is(readErr, io.EOF) {
			serverErrCh <- readErr
			return
		}
		serverErrCh <- nil
	}()

	expectedErr := errors.New("vhci attach failed")
	kernelConnCh := make(chan net.Conn, 1)
	ops := newTestUSBIPOps(t)
	ops.vhciPickFreePort = func(speed uint32) (int, error) {
		require.Equal(t, SpeedHigh, speed)
		return 4, nil
	}
	ops.vhciAttach = func(port int, fd uintptr, devid uint32, speed uint32) error {
		require.Equal(t, 4, port)
		requireStreamSocketFD(t, fd)
		info := device.toProtocol()
		require.Equal(t, info.DevID(), devid)
		require.Equal(t, SpeedHigh, speed)
		kernelConnCh <- duplicateConnFromFD(t, fd, "usbip-client-vhci-failure-kernel")
		return expectedErr
	}

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     wrappingDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        ops,
	}

	port, err := client.attemptAttach(ctx, "1-1")
	require.Equal(t, -1, port)
	require.ErrorIs(t, err, expectedErr)

	var kernelConn net.Conn
	select {
	case kernelConn = <-kernelConnCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for failed client relay kernel conn")
	}
	defer kernelConn.Close()
	setConnDeadline(t, kernelConn)
	requireConnEOF(t, kernelConn)

	select {
	case err = <-serverErrCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for server side close")
	}
	client.portsMu.Lock()
	_, reserved := client.ports[4]
	client.portsMu.Unlock()
	require.False(t, reserved)
}

func TestClientFetchDevListRejectsUnexpectedReplyVersion(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErr <- readErr
			return
		}
		if header.Code != OpReqDevList {
			serverErr <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		if writeErr := binary.Write(conn, binary.BigEndian, OpHeader{
			Version: ProtocolVersion + 1,
			Code:    OpRepDevList,
			Status:  OpStatusOK,
		}); writeErr != nil {
			serverErr <- writeErr
			return
		}
		if writeErr := binary.Write(conn, binary.BigEndian, uint32(0)); writeErr != nil {
			serverErr <- writeErr
			return
		}
		serverErr <- nil
	}()

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        newTestUSBIPOps(t),
	}

	entries, err := client.fetchDevList(ctx)
	require.Nil(t, entries)
	require.ErrorContains(t, err, "unexpected reply version")
	require.NoError(t, <-serverErr)
}

func TestClientFetchDevListReturnsOnContextCancelWhileServerStalls(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	requestReady := make(chan struct{})
	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErr <- readErr
			return
		}
		if header.Code != OpReqDevList {
			serverErr <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		close(requestReady)

		var buf [1]byte
		_, readErr = conn.Read(buf[:])
		if readErr == nil {
			serverErr <- errors.New("expected client close after cancellation")
			return
		}
		serverErr <- nil
	}()

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        newTestUSBIPOps(t),
	}

	fetchErr := make(chan error, 1)
	go func() {
		_, fetchErrValue := client.fetchDevList(ctx)
		fetchErr <- fetchErrValue
	}()

	select {
	case <-requestReady:
	case <-time.After(3 * time.Second):
		t.Fatal("fetchDevList did not reach stalled read path")
	}
	cancel()

	select {
	case err = <-fetchErr:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("fetchDevList did not exit after cancellation")
	}
	require.NoError(t, <-serverErr)
}

func TestClientSyncRemoteStateAndResetControlStateRebuildsV2Map(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	entry := DeviceEntry{Info: device.toProtocol(), Interfaces: device.Interfaces}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()
		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErr <- readErr
			return
		}
		if header.Code != OpReqDevList {
			serverErr <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		serverErr <- WriteOpRepDevList(conn, []DeviceEntry{entry})
	}()

	client := &ClientService{
		ctx:             ctx,
		cancel:          cancel,
		logger:          newTestLogger(),
		dialer:          testDialer{},
		serverAddr:      M.SocksaddrFromNet(listener.Addr()),
		matches:         []option.USBIPDeviceMatch{{BusID: "unused"}},
		ops:             newTestUSBIPOps(t),
		remoteDevicesV2: map[string]DeviceInfoV2{"stale": {BusID: "stale", State: deviceStateAvailable}},
	}

	require.NoError(t, client.syncRemoteStateAndResetControlState(ctx))
	require.NoError(t, <-serverErr)

	client.remoteMu.Lock()
	devices := client.remoteDevicesV2
	client.remoteMu.Unlock()
	require.Len(t, devices, 1)
	require.Contains(t, devices, "1-1")
	require.Equal(t, deviceStateAvailable, devices["1-1"].State)
	require.Equal(t, uint16(0x1d6b), devices["1-1"].VendorID)
}

func TestClientAttemptAttachRejectsUnexpectedReplyVersion(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	device := newTestDevice("1-1", 0x1d6b, 0x0002, "serial-1", SpeedHigh)
	info := device.toProtocol()

	serverErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverErr <- acceptErr
			return
		}
		defer conn.Close()

		header, readErr := ReadOpHeader(conn)
		if readErr != nil {
			serverErr <- readErr
			return
		}
		if header.Code != OpReqImport {
			serverErr <- fmt.Errorf("unexpected request code 0x%s", hex16(header.Code))
			return
		}
		busid, readErr := ReadOpReqImportBody(conn)
		if readErr != nil {
			serverErr <- readErr
			return
		}
		if busid != "1-1" {
			serverErr <- fmt.Errorf("unexpected busid %s", busid)
			return
		}
		if writeErr := binary.Write(conn, binary.BigEndian, OpHeader{
			Version: ProtocolVersion + 1,
			Code:    OpRepImport,
			Status:  OpStatusOK,
		}); writeErr != nil {
			serverErr <- writeErr
			return
		}
		if writeErr := binary.Write(conn, binary.BigEndian, &info); writeErr != nil {
			serverErr <- writeErr
			return
		}
		serverErr <- nil
	}()

	ops := newTestUSBIPOps(t)
	ops.vhciPickFreePort = func(uint32) (int, error) {
		return -1, errors.New("unexpected vhci attach path")
	}

	client := &ClientService{
		ctx:        ctx,
		cancel:     cancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: M.SocksaddrFromNet(listener.Addr()),
		ops:        ops,
	}

	port, err := client.attemptAttach(ctx, "1-1")
	require.Equal(t, -1, port)
	require.ErrorContains(t, err, "unexpected reply version")
	require.NoError(t, <-serverErr)
}

func TestClientRunControlSessionSyncsAssignmentsOnChanged(t *testing.T) {
	t.Parallel()

	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()

	initialDevice := newTestDevice("1-1", 0x1d6b, 0x0002, "first", SpeedHigh)
	updatedDevice := newTestDevice("1-2", 0x1d6b, 0x0002, "second", SpeedHigh)
	store := newTestDeviceStore(initialDevice)
	store.setStatus("1-1", usbipStatusAvailable)
	store.setStatus("1-2", usbipStatusAvailable)

	serverOps := newTestUSBIPOps(t)
	serverOps.readUsbipStatus = store.readUsbipStatus
	serverOps.readSysfsDevice = store.readSysfsDevice

	server := &ServerService{
		ctx:         serverCtx,
		cancel:      serverCancel,
		logger:      newTestLogger(),
		exports:     map[string]serverExport{"1-1": {busid: "1-1"}},
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         serverOps,
	}
	serverAddr, closeServer := startDispatchServer(t, server)
	defer closeServer()

	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()

	match := option.USBIPDeviceMatch{VendorID: 0x1d6b, ProductID: 0x0002}
	client := &ClientService{
		ctx:        clientCtx,
		cancel:     clientCancel,
		logger:     newTestLogger(),
		dialer:     testDialer{},
		serverAddr: serverAddr,
		matches:    []option.USBIPDeviceMatch{match},
		targets:    []clientTarget{{match: match}},
		assigned:   make([]string, 1),
		ops:        newTestUSBIPOps(t),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.runControlSession()
	}()

	require.Eventually(t, func() bool {
		client.stateMu.Lock()
		defer client.stateMu.Unlock()
		return client.assigned[0] == "1-1"
	}, 3*time.Second, 10*time.Millisecond)

	store.setDevices(updatedDevice)
	server.deleteExport("1-1")
	server.setExport(serverExport{busid: "1-2"})
	server.broadcastChanged()

	require.Eventually(t, func() bool {
		client.stateMu.Lock()
		defer client.stateMu.Unlock()
		return client.assigned[0] == "1-2"
	}, 3*time.Second, 10*time.Millisecond)

	clientCancel()
	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("runControlSession did not exit after cancellation")
	}
}

func TestUSBIPLinuxSmoke(t *testing.T) {
	requireRoot(t)

	requireUSBIPHost(t)
	requireVHCI(t)

	gadget := newTestUSBGadget(t)
	device, err := readSysfsDevice(gadget.busid, sysBusDevicePath(gadget.busid))
	require.NoError(t, err)
	require.Equal(t, gadget.busid, device.BusID)

	server := &ServerService{
		ctx:         context.Background(),
		logger:      newTestLogger(),
		exports:     make(map[string]serverExport),
		controlSubs: make(map[uint64]*serverControlConn),
		ops:         systemUSBIPOps,
	}
	require.NoError(t, server.bindOne(&device))

	_, ok := server.snapshotExports()[gadget.busid]
	require.True(t, ok)

	driver, err := currentDriver(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, "usbip-host", driver)

	status, err := readUsbipStatus(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, usbipStatusAvailable, status)

	require.NoError(t, hostUnbind(gadget.busid))
	require.NoError(t, hostMatchBusID(gadget.busid, false))
	require.NoError(t, bindToDriver(gadget.busid, "usb"))
	server.deleteExport(gadget.busid)

	driver, err = currentDriver(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, "usb", driver)
}
