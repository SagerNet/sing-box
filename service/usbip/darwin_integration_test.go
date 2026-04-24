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
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	M "github.com/sagernet/sing/common/metadata"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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

	access   sync.Mutex
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
		t.Skip("root required; run with go test -exec sudo")
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

func TestDarwinVirtualControllerReadsCompliantSubmitResponsePayload(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	controller := newDarwinVirtualController(context.Background(), newTestLogger(), clientConn, DeviceInfoTruncated{
		BusNum: 1,
		DevNum: 1,
	})
	go controller.readLoop()
	t.Cleanup(controller.Close)

	responseCh := make(chan SubmitResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		response, err := controller.sendSubmit(SubmitCommand{
			Header: DataHeader{
				Command:   CmdSubmit,
				DevID:     0x00010001,
				Direction: USBIPDirIn,
				Endpoint:  1,
			},
			TransferBufferLength: 3,
		})
		if err != nil {
			errCh <- err
			return
		}
		responseCh <- response
	}()

	header, err := ReadDataHeader(serverConn)
	require.NoError(t, err)
	command, err := ReadSubmitCommandBody(serverConn, header)
	require.NoError(t, err)
	require.Equal(t, USBIPDirIn, command.Header.Direction)
	require.Equal(t, int32(nonIsoPacketCount), command.NumberOfPackets)

	require.NoError(t, WriteSubmitResponse(serverConn, SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    header.SeqNum,
			Direction: USBIPDirIn,
		},
		Status:       0,
		ActualLength: 3,
		Buffer:       []byte{1, 2, 3},
	}))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case response := <-responseCh:
		require.Equal(t, DataHeader{Command: RetSubmit, SeqNum: header.SeqNum}, response.Header)
		require.Equal(t, int32(nonIsoPacketCount), response.NumberOfPackets)
		require.Equal(t, []byte{1, 2, 3}, response.Buffer)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for submit response")
	}
}

func TestDarwinHandleIsoTransferPreservesASAPFlag(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	controller := newDarwinVirtualController(context.Background(), newTestLogger(), clientConn, DeviceInfoTruncated{
		BusNum: 1,
		DevNum: 2,
	})
	go controller.readLoop()
	t.Cleanup(controller.Close)

	type transferResult struct {
		status int32
		length int
	}
	resultCh := make(chan transferResult, 1)
	var buffer [8]byte
	go func() {
		status, length := controller.handleIsoTransfer(darwinEndpointKey{device: 1, endpoint: 0x81}, darwinCIMessage{
			control: ciIsochronousTransferControlASAP | (0x7f << ciIsochronousTransferControlFramePhase),
			data0:   uint32(len(buffer)),
			buffer:  unsafe.Pointer(&buffer[0]),
		})
		resultCh <- transferResult{status: status, length: length}
	}()

	header, err := ReadDataHeader(serverConn)
	require.NoError(t, err)
	command, err := ReadSubmitCommandBody(serverConn, header)
	require.NoError(t, err)
	require.Equal(t, CmdSubmit, header.Command)
	require.Equal(t, USBIPDirIn, command.Header.Direction)
	require.Equal(t, uint32(1), command.Header.Endpoint)
	require.Equal(t, int32(usbipTransferFlagIsoASAP), command.TransferFlags)
	require.Zero(t, command.StartFrame)
	require.Equal(t, int32(1), command.NumberOfPackets)
	require.Len(t, command.IsoPackets, 1)

	require.NoError(t, WriteSubmitResponse(serverConn, SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    header.SeqNum,
			Direction: USBIPDirIn,
		},
		Status:          0,
		ActualLength:    0,
		NumberOfPackets: 1,
		IsoPackets:      []IsoPacketDescriptor{{}},
	}))

	select {
	case result := <-resultCh:
		require.Zero(t, result.status)
		require.Zero(t, result.length)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for isochronous submit response")
	}
}

func TestDarwinSubmitInTransferRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), nil, DeviceInfoTruncated{})
	buffer := []byte{0xaa, 0xbb}

	status, length := controller.completeSubmitInTransfer(unsafe.Pointer(&buffer[0]), SubmitResponse{
		Status:       0,
		ActualLength: 3,
		Buffer:       []byte{1, 2, 3},
	}, len(buffer))

	require.Equal(t, -int32(unix.EOVERFLOW), status)
	require.Zero(t, length)
	require.Equal(t, []byte{0xaa, 0xbb}, buffer)
	select {
	case <-controller.ctx.Done():
	default:
		t.Fatal("controller context stayed active after oversized payload")
	}
}

func TestWaitDarwinControllerClosesOnContextCancel(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	controller := newDarwinVirtualController(context.Background(), newTestLogger(), clientConn, DeviceInfoTruncated{})
	go controller.readLoop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		waitDarwinController(ctx, controller)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for controller cancellation")
	}
	select {
	case <-controller.done:
	default:
		t.Fatal("controller read loop still active after cancellation")
	}
}

type fakeDarwinEndpointStateMachine struct {
	transfers              []darwinCITransfer
	processDoorbellStarted chan struct{}
	releaseProcessDoorbell <-chan struct{}
	closeCalled            chan struct{}
	currentRead            int
	completeCalled         int
	closeOnce              sync.Once
}

func (f *fakeDarwinEndpointStateMachine) Close() {
	if f.closeCalled != nil {
		f.closeOnce.Do(func() {
			close(f.closeCalled)
		})
	}
}

func (f *fakeDarwinEndpointStateMachine) respond(darwinCIMessage, int) error {
	return nil
}

func (f *fakeDarwinEndpointStateMachine) processDoorbell(uint32) error {
	if f.processDoorbellStarted != nil {
		close(f.processDoorbellStarted)
	}
	if f.releaseProcessDoorbell != nil {
		<-f.releaseProcessDoorbell
	}
	return nil
}

func (f *fakeDarwinEndpointStateMachine) currentTransfer() darwinCITransfer {
	if f.currentRead >= len(f.transfers) {
		return darwinCITransfer{}
	}
	transfer := f.transfers[f.currentRead]
	f.currentRead++
	return transfer
}

func (f *fakeDarwinEndpointStateMachine) complete(darwinCITransfer, int, int) error {
	f.completeCalled++
	return nil
}

func TestDarwinHandleDoorbellSkipsNoResponseCompletion(t *testing.T) {
	t.Parallel()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), nil, DeviceInfoTruncated{})
	message := darwinCIMessage{
		control: (1 << 15) | (1 << 14) | 0x3c,
		data0:   (uint32(2) << 8) | 1,
	}
	endpoint := &fakeDarwinEndpointStateMachine{
		transfers: []darwinCITransfer{{
			ptr:     unsafe.Pointer(&message),
			message: message,
		}},
	}
	controller.endpoints[darwinEndpointKey{device: 1, endpoint: 2}] = endpoint

	controller.handleDoorbell((uint32(2) << 8) | 1)
	require.Zero(t, endpoint.completeCalled)
}

func TestDarwinHandleDoorbellContinuesAfterNoResponseTransfer(t *testing.T) {
	t.Parallel()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), nil, DeviceInfoTruncated{})
	noResponseMessage := darwinCIMessage{
		control: (1 << 15) | (1 << 14) | 0x3c,
		data0:   (uint32(2) << 8) | 1,
	}
	responseMessage := darwinCIMessage{
		control: (1 << 15) | 0x3c,
		data0:   (uint32(2) << 8) | 1,
	}
	endpoint := &fakeDarwinEndpointStateMachine{
		transfers: []darwinCITransfer{
			{
				ptr:     unsafe.Pointer(&noResponseMessage),
				message: noResponseMessage,
			},
			{
				ptr:     unsafe.Pointer(&responseMessage),
				message: responseMessage,
			},
		},
	}
	controller.endpoints[darwinEndpointKey{device: 1, endpoint: 2}] = endpoint

	controller.handleDoorbell((uint32(2) << 8) | 1)
	require.Equal(t, 1, endpoint.completeCalled)
	require.Equal(t, 2, endpoint.currentRead)
}

func TestDarwinControllerCloseWaitsForEventLoopTeardown(t *testing.T) {
	t.Parallel()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), nil, DeviceInfoTruncated{})
	processStarted := make(chan struct{})
	releaseProcess := make(chan struct{})
	endpointClosed := make(chan struct{})
	endpoint := &fakeDarwinEndpointStateMachine{
		processDoorbellStarted: processStarted,
		releaseProcessDoorbell: releaseProcess,
		closeCalled:            endpointClosed,
	}
	controller.endpoints[darwinEndpointKey{device: 1, endpoint: 2}] = endpoint

	go controller.eventLoop()
	controller.enqueueDoorbell((uint32(2) << 8) | 1)

	select {
	case <-processStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for doorbell processing")
	}

	closeDone := make(chan struct{})
	go func() {
		controller.Close()
		close(closeDone)
	}()

	select {
	case <-endpointClosed:
		t.Fatal("endpoint closed while doorbell processing was active")
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case <-closeDone:
		t.Fatal("controller Close returned while doorbell processing was active")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseProcess)

	select {
	case <-endpointClosed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for endpoint close")
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller Close")
	}
}

func TestDarwinControllerCloseWithNilConn(t *testing.T) {
	t.Parallel()

	controller := newDarwinVirtualController(context.Background(), newTestLogger(), nil, DeviceInfoTruncated{})
	done := make(chan struct{})
	go func() {
		controller.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller Close")
	}
}

func TestDarwinUSBHostDeviceWatcherSmoke(t *testing.T) {
	watcher, err := darwinWatchUSBHostDevices(func() {})
	if err != nil {
		t.Skipf("IOUSBHostDevice watcher unavailable: %v", err)
	}
	watcher.Close()
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
		s.access.Lock()
		conns := make([]net.Conn, 0, len(s.conns))
		for conn := range s.conns {
			conns = append(conns, conn)
		}
		s.access.Unlock()
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
	s.access.Lock()
	s.conns[conn] = struct{}{}
	s.access.Unlock()
}

func (s *darwinFakeUSBIPServer) untrackConn(conn net.Conn) {
	s.access.Lock()
	delete(s.conns, conn)
	s.access.Unlock()
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
	case OpReqImportExt:
		s.handleImportExt(conn)
	}
}

func (s *darwinFakeUSBIPServer) handleControlConn(conn net.Conn) {
	helloMessage, err := readControlMessage(conn)
	if err != nil {
		return
	}
	hello := helloMessage.Frame
	if hello.Type != controlFrameHello || hello.Version != controlProtocolVersion {
		return
	}
	capabilities := negotiatedControlCapabilities(hello.Capabilities)
	if err := writeControlAckWithCapabilities(conn, 0, capabilities); err != nil {
		return
	}
	if supportsControlExtensions(capabilities) {
		_ = writeControlMessage(conn, controlFrame{
			Type:    controlFrameDeviceSnapshot,
			Version: controlProtocolVersion,
		}, controlDeviceSnapshot{
			Devices: []DeviceInfoV2{deviceInfoV2FromEntry(s.entry, "darwin-fake", "darwin-fake:"+s.entry.Info.BusIDString(), deviceStateAvailable, 0, "available")},
		})
	}
	for {
		message, err := readControlMessage(conn)
		if err != nil {
			return
		}
		frame := message.Frame
		if frame.Type != controlFramePing {
			if frame.Type == controlFrameLeaseRequest && supportsControlExtensions(capabilities) {
				var request controlLeaseRequest
				if unmarshalControlPayload(message.Payload, &request) != nil {
					return
				}
				_ = writeControlMessage(conn, controlFrame{
					Type:    controlFrameLeaseResponse,
					Version: controlProtocolVersion,
				}, controlLeaseResponse{
					BusID:       request.BusID,
					LeaseID:     1,
					ClientNonce: request.ClientNonce,
					TTLMillis:   int64(importLeaseTTL / time.Millisecond),
				})
				continue
			}
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

func (s *darwinFakeUSBIPServer) handleImportExt(conn net.Conn) {
	request, err := ReadOpReqImportExtBody(conn)
	if err != nil {
		return
	}
	if request.BusID != s.entry.Info.BusIDString() || request.LeaseID == 0 {
		_ = WriteOpRepImportExt(conn, OpStatusError, nil)
		return
	}
	info := s.entry.Info
	if err := WriteOpRepImportExt(conn, OpStatusOK, &info); err != nil {
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
		Status:          0,
		NumberOfPackets: nonIsoPacketCount,
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
	encodePathField(&info.Path, "fake-darwin-usbip")
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
		Info:   info,
		Serial: "codex-usbip-fake",
		Interfaces: []DeviceInterface{{
			BInterfaceClass:    0xff,
			BInterfaceSubClass: 0x42,
			BInterfaceProtocol: 0x01,
		}},
	}
}

func TestDarwinUSBIPClientSmoke(t *testing.T) {
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

func TestDarwinUSBIPServerSmoke(t *testing.T) {
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
	response, err := ReadSubmitResponseBody(conn, dataHeader, USBIPDirIn)
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
	response, err := ReadSubmitResponseBody(conn, dataHeader, command.Header.Direction)
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
