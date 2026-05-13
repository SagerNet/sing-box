//go:build linux

package usbip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type testLogWriter struct {
	access sync.Mutex
	buffer bytes.Buffer
}

func (w *testLogWriter) Write(p []byte) (int, error) {
	w.access.Lock()
	defer w.access.Unlock()
	return w.buffer.Write(p)
}

func (w *testLogWriter) String() string {
	w.access.Lock()
	defer w.access.Unlock()
	return w.buffer.String()
}

type opaqueConn struct {
	net.Conn
}

func newTestLogger(t testing.TB) log.ContextLogger {
	t.Helper()
	writer := new(testLogWriter)
	factory := log.NewDefaultFactory(
		context.Background(),
		log.Formatter{
			BaseTime:      time.Now(),
			DisableColors: true,
		},
		writer,
		"",
		nil,
		false,
	)
	factory.SetLevel(log.LevelTrace)
	t.Cleanup(func() {
		if output := writer.String(); t.Failed() && output != "" {
			t.Logf("USB/IP log:\n%s", output)
		}
		_ = factory.Close()
	})
	return factory.NewLogger("usbip")
}

func duplicateConnFromFD(t *testing.T, fd uintptr, name string) net.Conn {
	t.Helper()

	dupFD, err := unix.Dup(int(fd))
	require.NoError(t, err)
	file := os.NewFile(uintptr(dupFD), name)
	conn, err := net.FileConn(file)
	closeErr := file.Close()
	require.NoError(t, err)
	require.NoError(t, closeErr)
	return conn
}

func duplicateHandoffKernelConn(t *testing.T, handoff *kernelHandoffSession) net.Conn {
	t.Helper()

	conn := duplicateConnFromFD(t, handoff.file.Fd(), "usbip-test-kernel")
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
	err := ensureKernelPath(sysUsbipHostDriver, "usbip-host", "usbip-host driver")
	if err != nil {
		t.Skipf("usbip-host unavailable: %v", err)
	}
}

func requireVHCI(t *testing.T) {
	t.Helper()
	err := ensureKernelPath(sysVHCIControllerV0, "vhci-hcd", "vhci_hcd.0")
	if err != nil {
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
			driver, err := currentDriver(gadget.busid)
			if err == nil {
				switch driver {
				case "usbip-host":
					_ = writeSysfs(filepath.Join(sysUsbipHostDriver, "unbind"), gadget.busid)
					_ = writeSysfs(filepath.Join(sysUsbipHostDriver, "match_busid"), "del "+gadget.busid)
					_ = writeSysfs("/sys/bus/usb/drivers/usb/bind", gadget.busid)
				case "usb":
				case "":
				default:
					_ = writeSysfs("/sys/bus/usb/drivers/usb/bind", gadget.busid)
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

func setLinuxExport(host *linuxExportHost, exp *linuxExport) {
	host.access.Lock()
	defer host.access.Unlock()
	host.exports[exp.busid] = exp
}

func deleteLinuxExport(host *linuxExportHost, busid string) {
	host.access.Lock()
	defer host.access.Unlock()
	delete(host.exports, busid)
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

	handoff, err := newKernelHandoffSession(conn)
	require.NoError(t, err)
	defer handoff.Close()

	require.Nil(t, handoff.relayConn)
	requireStreamSocketFD(t, handoff.file.Fd())
	handoff.Start(context.Background(), newTestLogger(t), "test", "direct")

	_, err = conn.Write([]byte("closed"))
	require.Error(t, err)
	require.NoError(t, acceptedConn.Close())
	select {
	case <-handoff.Done():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for direct handoff monitor")
	}
}

func TestUSBIPConnHandoffRelaySocketpairCopies(t *testing.T) {
	t.Parallel()

	left, right := net.Pipe()
	defer right.Close()
	handoff, err := newKernelHandoffSession(opaqueConn{Conn: left})
	require.NoError(t, err)
	defer handoff.Close()
	require.NotNil(t, handoff.relayConn)
	requireStreamSocketFD(t, handoff.file.Fd())

	kernelConn := duplicateHandoffKernelConn(t, handoff)
	defer kernelConn.Close()
	setConnDeadline(t, right)
	setConnDeadline(t, kernelConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handoff.Start(ctx, newTestLogger(t), "test", "relay")

	_, err = right.Write([]byte("ping"))
	require.NoError(t, err)
	requireConnRead(t, kernelConn, []byte("ping"))

	_, err = kernelConn.Write([]byte("pong"))
	require.NoError(t, err)
	requireConnRead(t, right, []byte("pong"))

	require.NoError(t, right.Close())
	require.NoError(t, kernelConn.Close())
	select {
	case <-handoff.Done():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for relay handoff")
	}
}

func TestUSBIPLinuxSmoke(t *testing.T) {
	requireRoot(t)

	requireUSBIPHost(t)
	requireVHCI(t)

	gadget := newTestUSBGadget(t)
	device, err := readSysfsDevice(gadget.busid, filepath.Join(sysBusUSBDevices, gadget.busid))
	require.NoError(t, err)
	require.Equal(t, gadget.busid, device.BusID)

	host := newLinuxExportHost(newTestLogger(t), nil)
	exp, err := host.bindOne(&device)
	require.NoError(t, err)
	setLinuxExport(host, exp)

	host.access.Lock()
	_, present := host.exports[gadget.busid]
	host.access.Unlock()
	require.True(t, present)

	driver, err := currentDriver(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, "usbip-host", driver)

	status, err := readUsbipStatus(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, usbipStatusAvailable, status)

	require.NoError(t, writeSysfs(filepath.Join(sysUsbipHostDriver, "unbind"), gadget.busid))
	require.NoError(t, writeSysfs(filepath.Join(sysUsbipHostDriver, "match_busid"), "del "+gadget.busid))
	require.NoError(t, writeSysfs("/sys/bus/usb/drivers/usb/bind", gadget.busid))
	deleteLinuxExport(host, gadget.busid)

	driver, err = currentDriver(gadget.busid)
	require.NoError(t, err)
	require.Equal(t, "usb", driver)
}
