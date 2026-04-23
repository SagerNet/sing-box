//go:build darwin && cgo

package usbip

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
)

const (
	darwinFakeBusID     = "test-1"
	darwinFakeVendorID  = 0x1209
	darwinFakeProductID = 0x0001
)

var (
	darwinFakeDeviceDescriptor = []byte{
		18, 1, 0x00, 0x02, 0, 0, 0, 64,
		0x09, 0x12, 0x01, 0x00, 0x00, 0x01, 0, 0, 0, 1,
	}
	darwinFakeConfigurationDescriptor = []byte{
		9, 2, 32, 0, 1, 1, 0, 0x80, 50,
		9, 4, 0, 0, 2, 0xff, 0x42, 0x01, 0,
		7, 5, 0x81, 2, 64, 0, 0,
		7, 5, 0x02, 2, 64, 0, 0,
	}
)

type darwinFakeUSBIPServer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	listener net.Listener
	address  M.Socksaddr
	entry    DeviceEntry

	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	wg       sync.WaitGroup
	closeMux sync.Once

	submitSeen            chan SubmitCommand
	setConfigurationSeen  chan struct{}
	setConfigurationClose sync.Once
}

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("root required")
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

func requireDarwinUserHCI(t *testing.T) {
	t.Helper()

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), left, darwinFakeDeviceEntry().Info)
	hostController, err := darwinCreateUSBHostController(controller, 1, SpeedFull)
	controller.cancel()
	if err != nil {
		t.Skipf("IOUSBHostControllerInterface unavailable: %v", err)
	}
	hostController.Close()
}

func startDarwinFakeUSBIPServer(t *testing.T) *darwinFakeUSBIPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	server := &darwinFakeUSBIPServer{
		ctx:                  ctx,
		cancel:               cancel,
		listener:             listener,
		address:              M.SocksaddrFromNet(listener.Addr()),
		entry:                darwinFakeDeviceEntry(),
		conns:                make(map[net.Conn]struct{}),
		submitSeen:           make(chan SubmitCommand, 32),
		setConfigurationSeen: make(chan struct{}),
	}
	server.wg.Add(1)
	go server.acceptLoop()
	return server
}

func (s *darwinFakeUSBIPServer) Close() {
	s.closeMux.Do(func() {
		s.cancel()
		_ = s.listener.Close()
		s.mu.Lock()
		conns := make([]net.Conn, 0, len(s.conns))
		for conn := range s.conns {
			conns = append(conns, conn)
		}
		s.mu.Unlock()
		for _, conn := range conns {
			_ = conn.Close()
		}
	})

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}

func (s *darwinFakeUSBIPServer) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *darwinFakeUSBIPServer) trackConn(conn net.Conn) {
	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *darwinFakeUSBIPServer) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

func (s *darwinFakeUSBIPServer) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	s.trackConn(conn)
	defer s.untrackConn(conn)

	var prefix [controlPrefaceSize]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return
	}
	if IsControlPreface(prefix[:]) {
		s.handleControlConn(conn)
		return
	}

	switch header := ParseOpHeader(prefix[:]); header.Code {
	case OpReqDevList:
		_ = WriteOpRepDevList(conn, []DeviceEntry{s.entry})
	case OpReqImport:
		s.handleImport(conn)
	}
}

func (s *darwinFakeUSBIPServer) handleControlConn(conn net.Conn) {
	hello, err := ReadControlFrame(conn)
	if err != nil {
		return
	}
	if hello.Type != controlFrameHello || hello.Version != controlProtocolVersion {
		return
	}
	if err := WriteControlAck(conn, 0); err != nil {
		return
	}
	for {
		frame, err := ReadControlFrame(conn)
		if err != nil {
			return
		}
		if frame.Type != controlFramePing {
			return
		}
		if err := WriteControlPong(conn); err != nil {
			return
		}
	}
}

func (s *darwinFakeUSBIPServer) handleImport(conn net.Conn) {
	busid, err := ReadOpReqImportBody(conn)
	if err != nil {
		return
	}
	if busid != darwinFakeBusID {
		_ = WriteOpRepImport(conn, OpStatusError, nil)
		return
	}
	info := s.entry.Info
	if err := WriteOpRepImport(conn, OpStatusOK, &info); err != nil {
		return
	}
	s.handleDataSession(conn)
}

func (s *darwinFakeUSBIPServer) handleDataSession(conn net.Conn) {
	for {
		header, err := ReadDataHeader(conn)
		if err != nil {
			return
		}
		switch header.Command {
		case CmdSubmit:
			command, err := ReadSubmitCommandBody(conn, header)
			if err != nil {
				return
			}
			select {
			case s.submitSeen <- command:
			default:
			}
			if err := WriteSubmitResponse(conn, s.submitResponse(command)); err != nil {
				return
			}
		case CmdUnlink:
			if _, err := ReadUnlinkCommandBody(conn, header); err != nil {
				return
			}
			if err := WriteUnlinkResponse(conn, UnlinkResponse{
				Header: DataHeader{
					Command:   RetUnlink,
					SeqNum:    header.SeqNum,
					DevID:     header.DevID,
					Direction: header.Direction,
					Endpoint:  header.Endpoint,
				},
				Status: 0,
			}); err != nil {
				return
			}
		default:
			return
		}
	}
}

func (s *darwinFakeUSBIPServer) submitResponse(command SubmitCommand) SubmitResponse {
	response := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    command.Header.SeqNum,
			DevID:     command.Header.DevID,
			Direction: command.Header.Direction,
			Endpoint:  command.Header.Endpoint,
		},
		Status: 0,
	}
	if command.Header.Endpoint != 0 {
		return response
	}
	setup := command.Setup
	length := int(setup[6]) | int(setup[7])<<8
	switch setup[1] {
	case 5:
		return response
	case 6:
		if command.Header.Direction != USBIPDirIn {
			return response
		}
		var data []byte
		switch setup[3] {
		case 1:
			data = darwinFakeDeviceDescriptor
		case 2:
			data = darwinFakeConfigurationDescriptor
		}
		response.Buffer = truncateDarwinFakeDescriptor(data, length)
		response.ActualLength = int32(len(response.Buffer))
	case 8:
		if command.Header.Direction == USBIPDirIn && length > 0 {
			response.Buffer = []byte{1}
			response.ActualLength = 1
		}
	case 9:
		s.setConfigurationClose.Do(func() {
			close(s.setConfigurationSeen)
		})
	}
	return response
}

func truncateDarwinFakeDescriptor(data []byte, length int) []byte {
	if length < len(data) {
		data = data[:length]
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

func darwinFakeDeviceEntry() DeviceEntry {
	var info DeviceInfoTruncated
	encodePathField(&info.Path, "fake-darwin-usbip", "codex-usbip-fake")
	copy(info.BusID[:], darwinFakeBusID)
	info.BusNum = 1
	info.DevNum = 1
	info.Speed = SpeedHigh
	info.IDVendor = darwinFakeVendorID
	info.IDProduct = darwinFakeProductID
	info.BCDDevice = 0x0100
	info.BConfigurationValue = 1
	info.BNumConfigurations = 1
	info.BNumInterfaces = 1
	return DeviceEntry{
		Info: info,
		Interfaces: []DeviceInterface{{
			BInterfaceClass:    0xff,
			BInterfaceSubClass: 0x42,
			BInterfaceProtocol: 0x01,
		}},
	}
}

func TestDarwinUSBIPClientImportsFakeServer(t *testing.T) {
	requireRoot(t)
	requireDarwinUserHCI(t)

	fakeServer := startDarwinFakeUSBIPServer(t)
	defer fakeServer.Close()

	serviceInstance, err := NewClientService(context.Background(), newTestLogger(), "usbip-client-darwin-test", option.USBIPClientServiceOptions{
		ServerOptions: option.ServerOptions{
			Server:     fakeServer.address.AddrString(),
			ServerPort: fakeServer.address.Port,
		},
		Devices: []option.USBIPDeviceMatch{{BusID: darwinFakeBusID}},
	})
	require.NoError(t, err)
	client := serviceInstance.(*ClientService)
	defer func() {
		fakeServer.Close()
		_ = client.Close()
	}()
	require.NoError(t, client.Start(adapter.StartStateStart))

	select {
	case command := <-fakeServer.submitSeen:
		require.Equal(t, CmdSubmit, command.Header.Command)
		require.Equal(t, uint32(0), command.Header.Endpoint)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for macOS UserHCI USB/IP submit")
	}

	select {
	case <-fakeServer.setConfigurationSeen:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for macOS UserHCI SET_CONFIGURATION")
	}
}

func TestDarwinUSBIPServerSelectedDeviceConfiguresDevice(t *testing.T) {
	requireRoot(t)

	candidate, ok := darwinSafeCaptureCandidate(t)
	if !ok {
		t.Skip("safe vendor-specific USB capture target unavailable")
	}

	serviceInstance, err := NewServerService(context.Background(), newTestLogger(), "usbip-server-darwin-test", option.USBIPServerServiceOptions{
		ListenOptions: option.ListenOptions{
			Listen:     loopbackListenAddr(),
			ListenPort: pickFreeTCPPort(t),
		},
		Devices: []option.USBIPDeviceMatch{{BusID: candidate.key.BusID}},
	})
	require.NoError(t, err)
	server := serviceInstance.(*ServerService)
	defer server.Close()
	if err := server.Start(adapter.StartStateStart); err != nil {
		t.Skipf("IOUSBHostDevice enumeration unavailable: %v", err)
	}
	if len(server.currentExports()) == 0 {
		t.Skipf("IOUSBHostDevice capture unavailable for %s", candidate.key.BusID)
	}

	destination := M.SocksaddrFromNet(server.listen.Addr())
	entries := darwinFetchDevList(t, destination)
	require.Len(t, entries, 1)
	require.Equal(t, candidate.key.BusID, entries[0].Info.BusIDString())

	conn := darwinDialUSBIP(t, destination)
	defer conn.Close()
	require.NoError(t, WriteOpReqImport(conn, candidate.key.BusID))
	header, err := ReadOpHeader(conn)
	require.NoError(t, err)
	require.Equal(t, OpRepImport, header.Code)
	require.Equal(t, OpStatusOK, header.Status)
	_, err = ReadOpRepImportBody(conn)
	require.NoError(t, err)

	require.NoError(t, WriteSubmitCommand(conn, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    1,
			DevID:     entries[0].Info.DevID(),
			Direction: USBIPDirIn,
			Endpoint:  0,
		},
		TransferBufferLength: 18,
		Setup:                [8]byte{0x80, 6, 0, 1, 0, 0, 18, 0},
	}))
	dataHeader, err := ReadDataHeader(conn)
	require.NoError(t, err)
	require.Equal(t, RetSubmit, dataHeader.Command)
	response, err := ReadSubmitResponseBody(conn, dataHeader)
	require.NoError(t, err)
	require.Equal(t, int32(0), response.Status)
	require.GreaterOrEqual(t, len(response.Buffer), 18)
	require.Equal(t, byte(1), response.Buffer[1])

	response = darwinSubmitControl(t, conn, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    2,
			DevID:     entries[0].Info.DevID(),
			Direction: USBIPDirIn,
			Endpoint:  0,
		},
		TransferBufferLength: 9,
		Setup:                [8]byte{0x80, 6, 0, 2, 0, 0, 9, 0},
	})
	require.Equal(t, int32(0), response.Status)
	require.GreaterOrEqual(t, len(response.Buffer), 9)
	require.Equal(t, byte(2), response.Buffer[1])
	configurationValue := response.Buffer[5]
	require.NotZero(t, configurationValue)

	response = darwinSubmitControl(t, conn, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    3,
			DevID:     entries[0].Info.DevID(),
			Direction: USBIPDirOut,
			Endpoint:  0,
		},
		Setup: [8]byte{0x00, 9, configurationValue, 0, 0, 0, 0, 0},
	})
	require.Equal(t, int32(0), response.Status)

	response = darwinSubmitControl(t, conn, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    4,
			DevID:     entries[0].Info.DevID(),
			Direction: USBIPDirIn,
			Endpoint:  0,
		},
		TransferBufferLength: 1,
		Setup:                [8]byte{0x80, 8, 0, 0, 0, 0, 1, 0},
	})
	require.Equal(t, int32(0), response.Status)
	require.Equal(t, []byte{configurationValue}, response.Buffer)
}

func darwinSubmitControl(t *testing.T, conn net.Conn, command SubmitCommand) SubmitResponse {
	t.Helper()

	require.NoError(t, WriteSubmitCommand(conn, command))
	dataHeader, err := ReadDataHeader(conn)
	require.NoError(t, err)
	require.Equal(t, RetSubmit, dataHeader.Command)
	require.Equal(t, command.Header.SeqNum, dataHeader.SeqNum)
	response, err := ReadSubmitResponseBody(conn, dataHeader)
	require.NoError(t, err)
	return response
}

func darwinSafeCaptureCandidate(t *testing.T) (darwinUSBHostDeviceInfo, bool) {
	t.Helper()

	devices, err := darwinCopyUSBHostDevices()
	if err != nil {
		t.Skipf("IOUSBHostDevice enumeration unavailable: %v", err)
	}
	for i := range devices {
		info := devices[i]
		if info.key.BusID == "" || info.key.VendorID == 0 || info.key.VendorID == 0x05ac {
			continue
		}
		if info.entry.Info.BDeviceClass == 0x09 {
			continue
		}
		device, err := darwinOpenUSBHostDevice(info.registryID, false)
		if err != nil {
			continue
		}
		info = device.info
		device.Close()
		if darwinVendorSpecificOnly(info) {
			return info, true
		}
	}
	return darwinUSBHostDeviceInfo{}, false
}

func darwinVendorSpecificOnly(info darwinUSBHostDeviceInfo) bool {
	if info.entry.Info.BDeviceClass != 0 {
		return info.entry.Info.BDeviceClass == 0xff
	}
	if len(info.entry.Interfaces) == 0 {
		return false
	}
	for _, iface := range info.entry.Interfaces {
		if iface.BInterfaceClass != 0xff {
			return false
		}
	}
	return true
}

func darwinFetchDevList(t *testing.T, destination M.Socksaddr) []DeviceEntry {
	t.Helper()

	conn := darwinDialUSBIP(t, destination)
	defer conn.Close()
	require.NoError(t, WriteOpHeader(conn, OpReqDevList, OpStatusOK))
	header, err := ReadOpHeader(conn)
	require.NoError(t, err)
	require.Equal(t, OpRepDevList, header.Code)
	require.Equal(t, OpStatusOK, header.Status)
	entries, err := ReadOpRepDevListBody(conn)
	require.NoError(t, err)
	return entries
}

func darwinDialUSBIP(t *testing.T, destination M.Socksaddr) net.Conn {
	t.Helper()

	address := net.JoinHostPort(destination.AddrString(), strconv.Itoa(int(destination.Port)))
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	return conn
}
