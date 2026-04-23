//go:build linux

package usbip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

const (
	testVendorID     uint16 = 0x1d6b
	testACMProductID uint16 = 0x0104
	testHIDProductID uint16 = 0x0105
	testUDCCount            = 2
)

var testHIDReportDescriptor = []byte{
	0x06, 0x00, 0xff,
	0x09, 0x01,
	0xa1, 0x01,
	0x15, 0x00,
	0x26, 0xff, 0x00,
	0x75, 0x08,
	0x95, 0x08,
	0x09, 0x01,
	0x81, 0x02,
	0x95, 0x08,
	0x09, 0x01,
	0x91, 0x02,
	0xc0,
}

type testUSBIPTools struct {
	usbip  string
	usbipd string
}

type testVirtualFunction struct {
	name        string
	instance    string
	nodePattern string
	configure   func(functionPath string) error
}

type testVirtualGadget struct {
	path       string
	serial     string
	busid      string
	functions  []testVirtualFunction
	nodes      map[string]string
	closeOnce  sync.Once
	removeOnce sync.Once
	udcName    string
}

type testACMGadget struct {
	*testVirtualGadget
	ttyPath string
}

type testHIDGadget struct {
	*testVirtualGadget
	hidPath string
}

type rawFile struct {
	file  *os.File
	state *term.State
}

type readResult struct {
	data []byte
	err  error
}

var (
	testUDCMu        sync.Mutex
	testAllocatedUDC = make(map[string]struct{})
)

func requireUSBIPTools(t *testing.T) testUSBIPTools {
	t.Helper()
	requireRoot(t)

	usbipPath, usbipErr := exec.LookPath("usbip")
	usbipdPath, usbipdErr := exec.LookPath("usbipd")
	if usbipErr != nil || usbipdErr != nil {
		t.Skip("usbip and usbipd are required")
	}
	return testUSBIPTools{
		usbip:  usbipPath,
		usbipd: usbipdPath,
	}
}

func currentUDCNames() []string {
	entries, err := os.ReadDir("/sys/class/udc")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func ensureTestUDCs(t *testing.T, minCount int) []string {
	t.Helper()

	requireKernelModule(t, "configfs")
	requireKernelModule(t, "libcomposite")

	udcs := currentUDCNames()
	if len(udcs) >= minCount {
		return udcs
	}

	modprobePath, err := findModprobePath()
	require.NoError(t, err)

	command := exec.Command(modprobePath, "-r", "dummy_hcd")
	command.Env = os.Environ()
	_, _ = command.CombinedOutput()

	command = exec.Command(modprobePath, "dummy_hcd", "num="+strconv.Itoa(minCount))
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "modprobe dummy_hcd num=%d\n%s", minCount, string(output))

	require.Eventually(t, func() bool {
		return len(currentUDCNames()) >= minCount
	}, 5*time.Second, 100*time.Millisecond)

	return currentUDCNames()
}

func reserveTestUDC(t *testing.T) string {
	t.Helper()

	testUDCMu.Lock()
	defer testUDCMu.Unlock()

	udcs := ensureTestUDCs(t, testUDCCount)
	for _, udc := range udcs {
		if _, inUse := testAllocatedUDC[udc]; inUse {
			continue
		}
		testAllocatedUDC[udc] = struct{}{}
		return udc
	}

	t.Fatal("no free test UDC available")
	return ""
}

func releaseTestUDC(name string) {
	if name == "" {
		return
	}
	testUDCMu.Lock()
	delete(testAllocatedUDC, name)
	testUDCMu.Unlock()
}

func resetUSBIPInteropState(t *testing.T) {
	t.Helper()
	requireRoot(t)

	records, err := readVHCIStatus()
	if err == nil {
		for _, record := range records {
			if record.state == 6 {
				_ = vhciDetach(record.port)
			}
		}
		require.Eventually(t, func() bool {
			records, err = readVHCIStatus()
			if err != nil {
				return false
			}
			for _, record := range records {
				if record.state == 6 {
					return false
				}
			}
			return true
		}, 10*time.Second, 100*time.Millisecond)
	}

	devices, err := listUSBDevices()
	if err != nil {
		return
	}
	for _, device := range devices {
		if !strings.HasPrefix(device.Serial, "codex-usbip-") {
			continue
		}
		driver, err := currentDriver(device.BusID)
		if err != nil || driver != "usbip-host" {
			continue
		}
		_ = hostUnbind(device.BusID)
		_ = hostMatchBusID(device.BusID, false)
		_ = bindToDriver(device.BusID, "usb")
	}

	paths, _ := filepath.Glob("/sys/kernel/config/usb_gadget/codex_usbip_*")
	for _, path := range paths {
		_ = writeSysfsLine(filepath.Join(path, "UDC"), "")

		links, _ := filepath.Glob(filepath.Join(path, "configs", "*", "*"))
		for _, link := range links {
			info, err := os.Lstat(link)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(link)
			}
		}

		functions, _ := filepath.Glob(filepath.Join(path, "functions", "*"))
		for _, functionPath := range functions {
			_ = os.RemoveAll(functionPath)
		}
		_ = os.RemoveAll(filepath.Join(path, "configs"))
		_ = os.RemoveAll(filepath.Join(path, "strings"))
		_ = os.RemoveAll(path)
	}
	require.Eventually(t, func() bool {
		paths, _ := filepath.Glob("/sys/kernel/config/usb_gadget/codex_usbip_*")
		return len(paths) == 0
	}, 10*time.Second, 100*time.Millisecond)
	require.Eventually(t, func() bool {
		return len(importedNodeSnapshot("/dev/ttyACM*")) == 0 && len(importedNodeSnapshot("/dev/hidraw*")) == 0
	}, 10*time.Second, 100*time.Millisecond)

	testUDCMu.Lock()
	testAllocatedUDC = make(map[string]struct{})
	testUDCMu.Unlock()
}

func loopbackListenAddr() *badoption.Addr {
	addr := badoption.Addr(netip.MustParseAddr("127.0.0.1"))
	return &addr
}

func pickFreeTCPPort(t *testing.T) uint16 {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return uint16(listener.Addr().(*net.TCPAddr).Port)
}

func startRealUSBIPServer(t *testing.T, devices []option.USBIPDeviceMatch) (*ServerService, M.Socksaddr) {
	t.Helper()

	serviceInstance, err := NewServerService(context.Background(), newTestLogger(), "usbip-server-test", option.USBIPServerServiceOptions{
		ListenOptions: option.ListenOptions{
			Listen:     loopbackListenAddr(),
			ListenPort: pickFreeTCPPort(t),
		},
		Devices: devices,
	})
	require.NoError(t, err)

	server := serviceInstance.(*ServerService)
	require.NoError(t, server.Start(adapter.StartStateStart))
	t.Cleanup(func() {
		_ = server.Close()
	})

	return server, M.SocksaddrFromNet(server.listenFD.Addr())
}

func startRealUSBIPClient(t *testing.T, destination M.Socksaddr, devices []option.USBIPDeviceMatch) *ClientService {
	t.Helper()

	serviceInstance, err := NewClientService(context.Background(), newTestLogger(), "usbip-client-test", option.USBIPClientServiceOptions{
		ServerOptions: option.ServerOptions{
			Server:     destination.AddrString(),
			ServerPort: destination.Port,
		},
		Devices: devices,
	})
	require.NoError(t, err)

	client := serviceInstance.(*ClientService)
	require.NoError(t, client.Start(adapter.StartStateStart))
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func runCommand(t *testing.T, name string, args ...string) string {
	t.Helper()

	command := exec.Command(name, args...)
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "%s %s\n%s", name, strings.Join(args, " "), string(output))
	return string(output)
}

func runUSBIP(t *testing.T, tools testUSBIPTools, args ...string) string {
	t.Helper()
	return runCommand(t, tools.usbip, args...)
}

func startUSBIPD(t *testing.T, tools testUSBIPTools, port uint16) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, tools.usbipd, "--debug", "--tcp-port", strconv.Itoa(int(port)))
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	command.Env = os.Environ()
	require.NoError(t, command.Start())
	waitForTCPPort(t, port)
	t.Cleanup(func() {
		cancel()
		done := make(chan error, 1)
		go func() {
			done <- command.Wait()
		}()
		select {
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			<-done
		case <-done:
		}
	})
}

func waitForTCPPort(t *testing.T, port uint16) {
	t.Helper()

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)
}

func snapshotPaths(pattern string) map[string]struct{} {
	paths, _ := filepath.Glob(pattern)
	snapshot := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		snapshot[path] = struct{}{}
	}
	return snapshot
}

func newPaths(pattern string, before map[string]struct{}) []string {
	paths, _ := filepath.Glob(pattern)
	var out []string
	for _, path := range paths {
		if _, found := before[path]; found {
			continue
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func waitForNewPath(t *testing.T, pattern string, before map[string]struct{}) string {
	t.Helper()

	var found string
	require.Eventually(t, func() bool {
		paths := newPaths(pattern, before)
		if len(paths) == 0 {
			return false
		}
		found = paths[0]
		return true
	}, 5*time.Second, 100*time.Millisecond)
	return found
}

func importedNodeSnapshot(pattern string) map[string]struct{} {
	paths, _ := filepath.Glob(pattern)
	snapshot := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if isVHCINode(path) {
			snapshot[path] = struct{}{}
		}
	}
	return snapshot
}

func isVHCINode(path string) bool {
	base := filepath.Base(path)
	var sysfsPath string
	switch {
	case strings.HasPrefix(base, "ttyACM"):
		sysfsPath = filepath.Join("/sys/class/tty", base, "device")
	case strings.HasPrefix(base, "hidraw"):
		sysfsPath = filepath.Join("/sys/class/hidraw", base, "device")
	default:
		return false
	}
	realPath, err := filepath.EvalSymlinks(sysfsPath)
	if err != nil {
		return false
	}
	return strings.Contains(realPath, "vhci_hcd")
}

func waitForNewImportedNode(t *testing.T, pattern string, before map[string]struct{}) string {
	t.Helper()

	var found string
	require.Eventually(t, func() bool {
		paths, _ := filepath.Glob(pattern)
		var candidates []string
		for _, path := range paths {
			if !isVHCINode(path) {
				continue
			}
			if _, present := before[path]; present {
				continue
			}
			candidates = append(candidates, path)
		}
		if len(candidates) == 0 {
			return false
		}
		sort.Strings(candidates)
		found = candidates[0]
		return true
	}, 20*time.Second, 100*time.Millisecond)
	return found
}

func waitForImportedNodePresent(t *testing.T, pattern string, path string) string {
	t.Helper()

	if path != "" {
		if _, err := os.Stat(path); err == nil && isVHCINode(path) {
			return path
		}
	}

	var found string
	require.Eventually(t, func() bool {
		paths, _ := filepath.Glob(pattern)
		var candidates []string
		for _, candidate := range paths {
			if !isVHCINode(candidate) {
				continue
			}
			candidates = append(candidates, candidate)
		}
		if len(candidates) == 0 {
			return false
		}
		sort.Strings(candidates)
		found = candidates[0]
		return true
	}, 10*time.Second, 100*time.Millisecond)
	return found
}

func waitForPathGone(t *testing.T, path string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return os.IsNotExist(err)
	}, 10*time.Second, 100*time.Millisecond)
}

func ensureNoNewImportedNode(t *testing.T, pattern string, before map[string]struct{}, duration time.Duration) {
	t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		paths, _ := filepath.Glob(pattern)
		for _, path := range paths {
			if !isVHCINode(path) {
				continue
			}
			if _, present := before[path]; !present {
				t.Fatalf("unexpected imported node %s", path)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func usedVHCIPorts(t *testing.T) map[int]struct{} {
	t.Helper()

	records, err := readVHCIStatus()
	require.NoError(t, err)

	ports := make(map[int]struct{})
	for _, record := range records {
		if record.state == 6 {
			ports[record.port] = struct{}{}
		}
	}
	return ports
}

func waitForNewUsedVHCIPort(t *testing.T, before map[int]struct{}) int {
	t.Helper()

	var port int
	require.Eventually(t, func() bool {
		records, err := readVHCIStatus()
		if err != nil {
			return false
		}
		for _, record := range records {
			if record.state != 6 {
				continue
			}
			if _, found := before[record.port]; found {
				continue
			}
			port = record.port
			return true
		}
		return false
	}, 10*time.Second, 100*time.Millisecond)
	return port
}

func readExactlyAsync(reader io.Reader, size int) <-chan readResult {
	results := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, size)
		_, err := io.ReadFull(reader, buffer)
		results <- readResult{
			data: buffer,
			err:  err,
		}
	}()
	return results
}

func requireRead(t *testing.T, results <-chan readResult, expected []byte) {
	t.Helper()

	select {
	case result := <-results:
		require.NoError(t, result.err)
		require.Equal(t, expected, result.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for device I/O")
	}
}

func readExactlyWithin(reader io.Reader, size int, timeout time.Duration) ([]byte, error) {
	results := readExactlyAsync(reader, size)
	select {
	case result := <-results:
		return result.data, result.err
	case <-time.After(timeout):
		return nil, context.DeadlineExceeded
	}
}

func openRawTTY(t *testing.T, path string) *rawFile {
	t.Helper()

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	state, err := term.MakeRaw(int(file.Fd()))
	require.NoError(t, err)
	return &rawFile{
		file:  file,
		state: state,
	}
}

func (r *rawFile) Close() {
	if r == nil || r.file == nil {
		return
	}
	_ = term.Restore(int(r.file.Fd()), r.state)
	_ = r.file.Close()
}

func openBinaryDevice(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	return file
}

func newTestVirtualGadget(t *testing.T, productID uint16, productName string, functions []testVirtualFunction) *testVirtualGadget {
	t.Helper()
	requireRoot(t)

	requireKernelModule(t, "configfs")
	requireKernelModule(t, "libcomposite")

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	resolvedFunctions := make([]testVirtualFunction, len(functions))
	for i, function := range functions {
		resolvedFunctions[i] = function
		typeName, _, hasInstance := strings.Cut(function.name, ".")
		if hasInstance {
			resolvedFunctions[i].instance = typeName + ".codex" + suffix
		} else {
			resolvedFunctions[i].instance = function.name + "codex" + suffix
		}
	}

	snapshots := make(map[string]map[string]struct{})
	for _, function := range resolvedFunctions {
		if function.nodePattern == "" {
			continue
		}
		snapshots[function.name] = snapshotPaths(function.nodePattern)
	}

	gadget := &testVirtualGadget{
		path:      filepath.Join("/sys/kernel/config/usb_gadget", fmt.Sprintf("codex_usbip_%d", time.Now().UnixNano())),
		serial:    fmt.Sprintf("codex-usbip-%d", time.Now().UnixNano()),
		functions: resolvedFunctions,
		nodes:     make(map[string]string, len(resolvedFunctions)),
		udcName:   reserveTestUDC(t),
	}

	require.NoError(t, os.MkdirAll(filepath.Join(gadget.path, "strings/0x409"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(gadget.path, "configs/c.1/strings/0x409"), 0o755))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "idVendor"), fmt.Sprintf("0x%04x", testVendorID)))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "idProduct"), fmt.Sprintf("0x%04x", productID)))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/serialnumber"), gadget.serial))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/manufacturer"), "OpenAI"))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "strings/0x409/product"), productName))
	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "configs/c.1/strings/0x409/configuration"), "config-1"))

	for _, function := range resolvedFunctions {
		functionPath := filepath.Join(gadget.path, "functions", function.instance)
		require.NoError(t, os.Mkdir(functionPath, 0o755))
		if function.configure != nil {
			require.NoError(t, function.configure(functionPath))
		}
		require.NoError(t, os.Symlink(functionPath, filepath.Join(gadget.path, "configs/c.1", function.instance)))
	}

	require.NoError(t, writeSysfs(filepath.Join(gadget.path, "UDC"), gadget.udcName))

	require.Eventually(t, func() bool {
		devices, err := listUSBDevices()
		if err != nil {
			return false
		}
		for i := range devices {
			if devices[i].VendorID == testVendorID &&
				devices[i].ProductID == productID &&
				devices[i].Serial == gadget.serial {
				gadget.busid = devices[i].BusID
				return true
			}
		}
		return false
	}, 10*time.Second, 100*time.Millisecond)

	for _, function := range functions {
		if function.nodePattern == "" {
			continue
		}
		gadget.nodes[function.name] = waitForNewPath(t, function.nodePattern, snapshots[function.name])
	}

	t.Cleanup(func() {
		gadget.Close()
	})

	return gadget
}

func (g *testVirtualGadget) Close() {
	g.closeOnce.Do(func() {
		defer releaseTestUDC(g.udcName)

		if g.busid != "" {
			if driver, err := currentDriver(g.busid); err == nil && driver == "usbip-host" {
				_ = hostUnbind(g.busid)
				_ = hostMatchBusID(g.busid, false)
			}
		}

		_ = writeSysfsLine(filepath.Join(g.path, "UDC"), "")

		for _, function := range g.functions {
			_ = os.Remove(filepath.Join(g.path, "configs/c.1", function.instance))
		}
		for _, function := range g.functions {
			_ = os.RemoveAll(filepath.Join(g.path, "functions", function.instance))
		}
		_ = os.RemoveAll(filepath.Join(g.path, "configs/c.1/strings/0x409"))
		_ = os.RemoveAll(filepath.Join(g.path, "configs/c.1"))
		_ = os.RemoveAll(filepath.Join(g.path, "strings/0x409"))
		_ = os.RemoveAll(g.path)

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(g.path); err == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if g.busid != "" {
				if _, err := os.Stat(sysBusDevicePath(g.busid)); err == nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			remainingNode := false
			for _, path := range g.nodes {
				if _, err := os.Stat(path); err == nil {
					remainingNode = true
					break
				}
			}
			if !remainingNode {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

func newTestACMGadget(t *testing.T) *testACMGadget {
	t.Helper()

	gadget := newTestVirtualGadget(t, testACMProductID, "Codex USBIP ACM", []testVirtualFunction{{
		name:        "acm.usb0",
		nodePattern: "/dev/ttyGS*",
	}})
	return &testACMGadget{
		testVirtualGadget: gadget,
		ttyPath:           gadget.nodes["acm.usb0"],
	}
}

func newTestHIDGadget(t *testing.T) *testHIDGadget {
	t.Helper()

	gadget := newTestVirtualGadget(t, testHIDProductID, "Codex USBIP HID", []testVirtualFunction{{
		name:        "hid.usb0",
		nodePattern: "/dev/hidg*",
		configure: func(functionPath string) error {
			if err := writeSysfs(functionPath+"/protocol", "0"); err != nil {
				return err
			}
			if err := writeSysfs(functionPath+"/subclass", "0"); err != nil {
				return err
			}
			if err := writeSysfs(functionPath+"/report_length", "8"); err != nil {
				return err
			}
			return os.WriteFile(functionPath+"/report_desc", testHIDReportDescriptor, 0o644)
		},
	}})
	return &testHIDGadget{
		testVirtualGadget: gadget,
		hidPath:           gadget.nodes["hid.usb0"],
	}
}

func (g *testACMGadget) exerciseImportedIO(t *testing.T, importedTTY string) {
	t.Helper()

	gadgetTTY := openRawTTY(t, g.ttyPath)
	imported := openRawTTY(t, importedTTY)
	defer gadgetTTY.Close()
	defer imported.Close()

	gadgetToHost := []byte("acm-g2h!")
	hostToGadget := []byte("acm-h2g!")

	hostRead := readExactlyAsync(imported.file, len(gadgetToHost))
	_, err := gadgetTTY.file.Write(gadgetToHost)
	require.NoError(t, err)
	requireRead(t, hostRead, gadgetToHost)

	gadgetRead := readExactlyAsync(gadgetTTY.file, len(hostToGadget))
	_, err = imported.file.Write(hostToGadget)
	require.NoError(t, err)
	requireRead(t, gadgetRead, hostToGadget)
}

func (g *testHIDGadget) exerciseImportedIO(t *testing.T, importedHID string) {
	t.Helper()

	gadgetToHost := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	hostToGadget := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	require.Eventually(t, func() bool {
		gadgetHID, err := os.OpenFile(g.hidPath, os.O_RDWR, 0)
		if err != nil {
			return false
		}
		defer gadgetHID.Close()

		imported, err := os.OpenFile(importedHID, os.O_RDWR, 0)
		if err != nil {
			return false
		}
		defer imported.Close()

		if _, err = gadgetHID.Write(gadgetToHost); err != nil {
			return false
		}
		readBack, err := readExactlyWithin(imported, len(gadgetToHost), time.Second)
		if err != nil || !bytes.Equal(readBack, gadgetToHost) {
			return false
		}

		if _, err = imported.Write(hostToGadget); err != nil {
			return false
		}
		readBack, err = readExactlyWithin(gadgetHID, len(hostToGadget), time.Second)
		return err == nil && bytes.Equal(readBack, hostToGadget)
	}, 10*time.Second, 100*time.Millisecond)
}

func bindWithOfficialUSBIP(t *testing.T, tools testUSBIPTools, busid string) {
	t.Helper()

	if driver, err := currentDriver(busid); err == nil && driver == "usbip-host" {
		return
	}
	runUSBIP(t, tools, "bind", "--busid="+busid)
	require.Eventually(t, func() bool {
		driver, err := currentDriver(busid)
		return err == nil && driver == "usbip-host"
	}, 5*time.Second, 100*time.Millisecond)
}

func unbindWithOfficialUSBIP(t *testing.T, tools testUSBIPTools, busid string) {
	t.Helper()
	runUSBIP(t, tools, "unbind", "--busid="+busid)
	require.Eventually(t, func() bool {
		driver, err := currentDriver(busid)
		return err == nil && driver != "usbip-host"
	}, 5*time.Second, 100*time.Millisecond)
}

func TestUSBIPInteropOurServerWithOfficialClientACM(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	tools := requireUSBIPTools(t)
	require.NoError(t, ensureVHCI())

	gadget := newTestACMGadget(t)
	server, address := startRealUSBIPServer(t, []option.USBIPDeviceMatch{{Serial: gadget.serial}})
	beforePorts := usedVHCIPorts(t)
	beforeTTY := importedNodeSnapshot("/dev/ttyACM*")

	listOutput := runUSBIP(t, tools, "--tcp-port", strconv.Itoa(int(address.Port)), "list", "--remote=127.0.0.1")
	require.Contains(t, listOutput, gadget.busid)

	runUSBIP(t, tools, "--tcp-port", strconv.Itoa(int(address.Port)), "attach", "--remote=127.0.0.1", "--busid="+gadget.busid)
	port := waitForNewUsedVHCIPort(t, beforePorts)
	importedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", beforeTTY)
	gadget.exerciseImportedIO(t, importedTTY)

	portOutput := runUSBIP(t, tools, "port")
	require.Contains(t, portOutput, fmt.Sprintf("Port %02d", port))

	runUSBIP(t, tools, "detach", "--port="+strconv.Itoa(port))
	waitForPathGone(t, importedTTY)

	_ = server
}

func TestUSBIPInteropOurServerWithOfficialClientHID(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	tools := requireUSBIPTools(t)
	require.NoError(t, ensureVHCI())

	gadget := newTestHIDGadget(t)
	_, address := startRealUSBIPServer(t, []option.USBIPDeviceMatch{{Serial: gadget.serial}})
	beforePorts := usedVHCIPorts(t)
	beforeHID := importedNodeSnapshot("/dev/hidraw*")

	listOutput := runUSBIP(t, tools, "--tcp-port", strconv.Itoa(int(address.Port)), "list", "--remote=127.0.0.1")
	require.Contains(t, listOutput, gadget.busid)

	runUSBIP(t, tools, "--tcp-port", strconv.Itoa(int(address.Port)), "attach", "--remote=127.0.0.1", "--busid="+gadget.busid)
	port := waitForNewUsedVHCIPort(t, beforePorts)
	importedHID := waitForNewImportedNode(t, "/dev/hidraw*", beforeHID)
	gadget.exerciseImportedIO(t, importedHID)

	portOutput := runUSBIP(t, tools, "port")
	require.Contains(t, portOutput, fmt.Sprintf("Port %02d", port))

	runUSBIP(t, tools, "detach", "--port="+strconv.Itoa(port))
	waitForPathGone(t, importedHID)
}

func TestUSBIPInteropOurClientWithOfficialServerACM(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	tools := requireUSBIPTools(t)
	require.NoError(t, ensureVHCI())

	gadget := newTestACMGadget(t)
	bindWithOfficialUSBIP(t, tools, gadget.busid)
	t.Cleanup(func() {
		unbindWithOfficialUSBIP(t, tools, gadget.busid)
	})

	port := pickFreeTCPPort(t)
	startUSBIPD(t, tools, port)
	beforeTTY := importedNodeSnapshot("/dev/ttyACM*")

	client := startRealUSBIPClient(t, M.ParseSocksaddrHostPort("127.0.0.1", port), []option.USBIPDeviceMatch{{
		VendorID:  option.USBIPHexUint16(testVendorID),
		ProductID: option.USBIPHexUint16(testACMProductID),
	}})
	_ = client

	importedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", beforeTTY)
	gadget.exerciseImportedIO(t, importedTTY)

	require.NoError(t, client.Close())
	waitForPathGone(t, importedTTY)
}

func TestUSBIPInteropOurClientWithOfficialServerHID(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	tools := requireUSBIPTools(t)
	require.NoError(t, ensureVHCI())

	gadget := newTestHIDGadget(t)
	bindWithOfficialUSBIP(t, tools, gadget.busid)
	t.Cleanup(func() {
		unbindWithOfficialUSBIP(t, tools, gadget.busid)
	})

	port := pickFreeTCPPort(t)
	startUSBIPD(t, tools, port)
	beforeHID := importedNodeSnapshot("/dev/hidraw*")

	client := startRealUSBIPClient(t, M.ParseSocksaddrHostPort("127.0.0.1", port), []option.USBIPDeviceMatch{{
		VendorID:  option.USBIPHexUint16(testVendorID),
		ProductID: option.USBIPHexUint16(testHIDProductID),
	}})
	_ = client

	importedHID := waitForNewImportedNode(t, "/dev/hidraw*", beforeHID)
	gadget.exerciseImportedIO(t, importedHID)

	require.NoError(t, client.Close())
	waitForPathGone(t, importedHID)
}

func TestUSBIPOfficialServerHasStaticDiscoveryOnly(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	tools := requireUSBIPTools(t)
	require.NoError(t, ensureVHCI())

	first := newTestACMGadget(t)
	bindWithOfficialUSBIP(t, tools, first.busid)
	t.Cleanup(func() {
		unbindWithOfficialUSBIP(t, tools, first.busid)
	})

	port := pickFreeTCPPort(t)
	startUSBIPD(t, tools, port)
	beforeTTY := importedNodeSnapshot("/dev/ttyACM*")

	client := startRealUSBIPClient(t, M.ParseSocksaddrHostPort("127.0.0.1", port), nil)
	_ = client

	importedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", beforeTTY)
	first.exerciseImportedIO(t, importedTTY)

	second := newTestHIDGadget(t)
	bindWithOfficialUSBIP(t, tools, second.busid)
	t.Cleanup(func() {
		unbindWithOfficialUSBIP(t, tools, second.busid)
	})

	beforeHID := importedNodeSnapshot("/dev/hidraw*")
	ensureNoNewImportedNode(t, "/dev/hidraw*", beforeHID, 3*time.Second)

	require.NoError(t, client.Close())
	waitForPathGone(t, importedTTY)
}

func TestUSBIPControlHotplugACMReattach(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	require.NoError(t, ensureVHCI())
	ensureTestUDCs(t, testUDCCount)

	server, address := startRealUSBIPServer(t, []option.USBIPDeviceMatch{{
		VendorID:  option.USBIPHexUint16(testVendorID),
		ProductID: option.USBIPHexUint16(testACMProductID),
	}})
	client := startRealUSBIPClient(t, address, []option.USBIPDeviceMatch{{
		VendorID:  option.USBIPHexUint16(testVendorID),
		ProductID: option.USBIPHexUint16(testACMProductID),
	}})
	_ = client

	beforeTTY := importedNodeSnapshot("/dev/ttyACM*")
	first := newTestACMGadget(t)
	firstImportedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", beforeTTY)
	first.exerciseImportedIO(t, firstImportedTTY)

	first.Close()
	waitForPathGone(t, firstImportedTTY)
	require.Eventually(t, func() bool {
		return len(server.currentExports()) == 0
	}, 5*time.Second, 100*time.Millisecond)

	secondBefore := importedNodeSnapshot("/dev/ttyACM*")
	second := newTestACMGadget(t)
	secondImportedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", secondBefore)
	second.exerciseImportedIO(t, secondImportedTTY)
}

func TestUSBIPControlImportAllACMAndHID(t *testing.T) {
	requireRoot(t)
	resetUSBIPInteropState(t)
	require.NoError(t, ensureVHCI())
	ensureTestUDCs(t, testUDCCount)

	_, address := startRealUSBIPServer(t, []option.USBIPDeviceMatch{
		{VendorID: option.USBIPHexUint16(testVendorID), ProductID: option.USBIPHexUint16(testACMProductID)},
		{VendorID: option.USBIPHexUint16(testVendorID), ProductID: option.USBIPHexUint16(testHIDProductID)},
	})
	client := startRealUSBIPClient(t, address, nil)
	_ = client

	beforeTTY := importedNodeSnapshot("/dev/ttyACM*")
	beforeHID := importedNodeSnapshot("/dev/hidraw*")

	acm := newTestACMGadget(t)
	hid := newTestHIDGadget(t)

	importedTTY := waitForNewImportedNode(t, "/dev/ttyACM*", beforeTTY)
	importedHID := waitForNewImportedNode(t, "/dev/hidraw*", beforeHID)
	importedTTY = waitForImportedNodePresent(t, "/dev/ttyACM*", importedTTY)
	importedHID = waitForImportedNodePresent(t, "/dev/hidraw*", importedHID)

	acm.exerciseImportedIO(t, importedTTY)
	hid.exerciseImportedIO(t, importedHID)
}
