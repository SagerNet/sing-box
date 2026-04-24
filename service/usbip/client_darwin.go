//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/sys/unix"
)

type ClientService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	matches    []option.USBIPDeviceMatch

	stateAccess      sync.Mutex
	targets          []clientTarget
	assigned         []string
	assignedWorkers  []*clientAssignedWorker
	allWorkers       map[string]*clientBusIDWorker
	allDesired       map[string]struct{}
	matchedKnownKeys map[string]DeviceKey

	wg sync.WaitGroup

	activeAccess sync.Mutex
	activeBusIDs map[string]struct{}

	controlAccess  sync.Mutex
	controlSession *clientControlSession

	remoteAccess    sync.Mutex
	remoteDevicesV2 map[string]DeviceInfoV2
}

func NewClientService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPClientServiceOptions) (adapter.Service, error) {
	for i, m := range options.Devices {
		if m.IsZero() {
			return nil, E.New("devices[", i, "]: at least one of busid/vendor_id/product_id/serial is required")
		}
	}
	if options.ServerPort == 0 {
		options.ServerPort = DefaultPort
	}
	if options.Server == "" {
		return nil, E.New("missing server address")
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerOptions.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &ClientService{
		Adapter:      boxService.NewAdapter(C.TypeUSBIPClient, tag),
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
		dialer:       outboundDialer,
		serverAddr:   options.ServerOptions.Build(),
		matches:      options.Devices,
		allWorkers:   make(map[string]*clientBusIDWorker),
		allDesired:   make(map[string]struct{}),
		activeBusIDs: make(map[string]struct{}),
	}, nil
}

func (c *ClientService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	c.initializeWorkers()
	c.wg.Add(1)
	go c.run()
	return nil
}

func (c *ClientService) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(clientShutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		c.logger.Warn("shutdown timeout; some macOS USB/IP controllers may remain active")
	}
	return nil
}

func (c *ClientService) runBusIDLoop(ctx context.Context, busid, description string) {
	for {
		if ctx.Err() != nil {
			return
		}
		controller, err := c.attemptAttach(ctx, busid)
		if err != nil {
			c.logger.Error("attach ", description, " (", busid, "): ", err)
			if !sleepCtx(ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		c.logger.Info("attached ", busid, " through IOUSBHostControllerInterface")
		c.setBusIDActive(busid, true)
		waitDarwinController(ctx, controller)
		c.setBusIDActive(busid, false)
		if ctx.Err() != nil {
			controller.Close()
			return
		}
		controller.Close()
		if !c.shouldRetryBusID(ctx, busid) {
			c.logger.Info("remote export ", busid, " disappeared; stopping import worker")
			return
		}
		if !sleepCtx(ctx, clientReconnectDelay) {
			return
		}
	}
}

func waitDarwinController(ctx context.Context, controller *darwinVirtualController) {
	select {
	case <-controller.done:
	case <-ctx.Done():
		controller.Close()
		controller.Wait()
	}
}

func (c *ClientService) attemptAttach(ctx context.Context, busid string) (*darwinVirtualController, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial ", c.serverAddr)
	}
	closeConn := true
	defer func() {
		if closeConn {
			_ = conn.Close()
		}
	}()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()
	lease, err := c.requestImportLease(ctx, busid)
	if err != nil {
		return nil, err
	}
	expectedReply := OpRepImport
	if lease.Valid {
		expectedReply = OpRepImportExt
		err = WriteOpReqImportExt(conn, ImportExtRequest{
			BusID:       busid,
			LeaseID:     lease.ID,
			ClientNonce: lease.ClientNonce,
		})
		if err != nil {
			return nil, E.Cause(err, "write OP_REQ_IMPORT_EXT")
		}
	} else {
		err = WriteOpReqImport(conn, busid)
		if err != nil {
			return nil, E.Cause(err, "write OP_REQ_IMPORT")
		}
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Version != ProtocolVersion {
		return nil, E.New(fmt.Sprintf("unexpected reply version 0x%04x", header.Version))
	}
	if header.Code != expectedReply {
		return nil, E.New(fmt.Sprintf("unexpected reply code 0x%04x", header.Code))
	}
	if header.Status != OpStatusOK {
		return nil, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT body")
	}
	controller := newDarwinVirtualController(ctx, c.logger, conn, info)
	err = controller.Start()
	if err != nil {
		controller.Close()
		return nil, err
	}
	closeConn = false
	return controller, nil
}

type darwinControllerEvent struct {
	command  *darwinCIMessage
	doorbell uint32
}

type darwinEndpointKey struct {
	device   uint8
	endpoint uint8
}

type darwinControlState struct {
	setup [8]byte
}

type darwinPendingSubmit struct {
	direction uint32
	reply     chan SubmitResponse
}

type darwinEndpointStateMachine interface {
	Close()
	respond(message darwinCIMessage, status int) error
	processDoorbell(doorbell uint32) error
	currentTransfer() darwinCITransfer
	complete(transfer darwinCITransfer, status int, length int) error
}

type darwinVirtualController struct {
	ctx       context.Context
	cancel    context.CancelFunc
	logger    log.ContextLogger
	conn      net.Conn
	info      DeviceInfoTruncated
	startTime time.Time

	controller   *darwinUSBHostController
	events       chan darwinControllerEvent
	done         chan struct{}
	eventDone    chan struct{}
	closeOnce    sync.Once
	eventStarted atomic.Bool
	seq          atomic.Uint32

	writeAccess   sync.Mutex
	pendingAccess sync.Mutex
	pending       map[uint32]darwinPendingSubmit

	stateAccess   sync.Mutex
	powered       bool
	connected     bool
	nextAddress   uint8
	devices       map[uint8]*darwinUSBHostDeviceSM
	endpoints     map[darwinEndpointKey]darwinEndpointStateMachine
	controlStates map[uint8]darwinControlState
}

func newDarwinVirtualController(ctx context.Context, logger log.ContextLogger, conn net.Conn, info DeviceInfoTruncated) *darwinVirtualController {
	ctx, cancel := context.WithCancel(ctx)
	return &darwinVirtualController{
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger,
		conn:          conn,
		info:          info,
		startTime:     time.Now(),
		events:        make(chan darwinControllerEvent, 64),
		done:          make(chan struct{}),
		eventDone:     make(chan struct{}),
		pending:       make(map[uint32]darwinPendingSubmit),
		nextAddress:   1,
		devices:       make(map[uint8]*darwinUSBHostDeviceSM),
		endpoints:     make(map[darwinEndpointKey]darwinEndpointStateMachine),
		controlStates: make(map[uint8]darwinControlState),
	}
}

func (c *darwinVirtualController) Start() error {
	controller, err := darwinCreateUSBHostController(c, 1, c.info.Speed)
	if err != nil {
		return err
	}
	c.controller = controller
	c.eventStarted.Store(true)
	go c.readLoop()
	go c.eventLoop()
	return nil
}

func (c *darwinVirtualController) Close() {
	c.requestClose()
	if c.eventStarted.Load() {
		<-c.eventDone
	}
}

func (c *darwinVirtualController) requestClose() {
	c.closeOnce.Do(func() {
		c.cancel()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (c *darwinVirtualController) Wait() {
	<-c.done
}

func (c *darwinVirtualController) enqueueCommand(message darwinCIMessage) {
	c.enqueueEvent(darwinControllerEvent{command: &message})
}

func (c *darwinVirtualController) enqueueDoorbell(doorbell uint32) {
	c.enqueueEvent(darwinControllerEvent{doorbell: doorbell})
}

func (c *darwinVirtualController) enqueueEvent(event darwinControllerEvent) {
	select {
	case c.events <- event:
	case <-c.ctx.Done():
	default:
		c.logger.Warn("IOUSBHostControllerInterface event queue overflow")
		c.requestClose()
	}
}

func (c *darwinVirtualController) readLoop() {
	defer close(c.done)
	defer c.cancel()
	for {
		header, err := ReadDataHeader(c.conn)
		if err != nil {
			if c.ctx.Err() == nil && !errors.Is(err, io.EOF) {
				c.logger.Debug("read USB/IP data header: ", err)
			}
			c.failPending()
			return
		}
		switch header.Command {
		case RetSubmit:
			payloadDirection, ok := c.pendingSubmitDirection(header.SeqNum)
			if !ok {
				payloadDirection = header.Direction
			}
			response, err := ReadSubmitResponseBody(c.conn, header, payloadDirection)
			if err != nil {
				c.logger.Debug("read RET_SUBMIT: ", err)
				c.failPending()
				return
			}
			c.deliverSubmit(response)
		case RetUnlink:
			_, err := ReadUnlinkResponseBody(c.conn, header)
			if err != nil {
				c.logger.Debug("read RET_UNLINK: ", err)
				c.failPending()
				return
			}
		default:
			c.logger.Debug(fmt.Sprintf("unexpected USB/IP response 0x%08x", header.Command))
			c.failPending()
			return
		}
	}
}

func (c *darwinVirtualController) eventLoop() {
	c.eventStarted.Store(true)
	defer close(c.eventDone)
	defer c.teardownIOUSBHostState()
	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.events:
			if event.command != nil {
				c.handleCommand(*event.command)
			} else {
				c.handleDoorbell(event.doorbell)
			}
			if c.ctx.Err() != nil {
				return
			}
		}
	}
}

func (c *darwinVirtualController) handleCommand(message darwinCIMessage) {
	var err error
	switch message.messageType() {
	case ciMsgControllerPowerOn, ciMsgControllerPowerOff, ciMsgControllerStart, ciMsgControllerPause:
		err = c.controller.respond(message, ciStatusSuccess)
	case ciMsgControllerFrameNumber:
		frame := uint64(time.Since(c.startTime) / time.Millisecond)
		err = c.controller.respondFrame(message, ciStatusSuccess, frame, darwinCIFrameTimestamp())
	case ciMsgPortPowerOn:
		c.powered = true
		err = c.controller.respondPort(message, ciStatusSuccess, c.powered, c.connected, c.info.Speed)
	case ciMsgPortPowerOff:
		c.powered = false
		c.connected = false
		err = c.controller.respondPort(message, ciStatusSuccess, c.powered, c.connected, c.info.Speed)
	case ciMsgPortReset, ciMsgPortStatus, ciMsgPortResume:
		if c.powered {
			c.connected = true
		}
		err = c.controller.respondPort(message, ciStatusSuccess, c.powered, c.connected, c.info.Speed)
	case ciMsgPortSuspend, ciMsgPortDisable:
		err = c.controller.respondPort(message, ciStatusSuccess, c.powered, c.connected, c.info.Speed)
	case ciMsgDeviceCreate:
		err = c.handleDeviceCreate(message)
	case ciMsgDeviceDestroy, ciMsgDeviceStart, ciMsgDevicePause, ciMsgDeviceUpdate:
		err = c.handleDeviceCommand(message)
	case ciMsgEndpointCreate:
		err = c.handleEndpointCreate(message)
	case ciMsgEndpointDestroy, ciMsgEndpointPause, ciMsgEndpointUpdate, ciMsgEndpointReset, ciMsgEndpointSetNext:
		err = c.handleEndpointCommand(message)
	default:
		c.logger.Debug("unhandled IOUSBHostCI command 0x", hex8(message.messageType()))
	}
	if err != nil {
		c.logger.Debug("IOUSBHostCI command 0x", hex8(message.messageType()), ": ", err)
		c.requestClose()
		return
	}
}

func (c *darwinVirtualController) handleDeviceCreate(message darwinCIMessage) error {
	device, err := c.controller.createDeviceSM(message)
	if err != nil {
		return err
	}
	c.stateAccess.Lock()
	address := c.nextAddress
	c.nextAddress++
	c.devices[address] = device
	c.stateAccess.Unlock()
	return device.respondCreate(message, ciStatusSuccess, address)
}

func (c *darwinVirtualController) handleDeviceCommand(message darwinCIMessage) error {
	address := message.deviceAddress()
	c.stateAccess.Lock()
	device := c.devices[address]
	c.stateAccess.Unlock()
	if device == nil {
		return nil
	}
	err := device.respond(message, ciStatusSuccess)
	if message.messageType() == ciMsgDeviceDestroy {
		c.stateAccess.Lock()
		delete(c.devices, address)
		c.stateAccess.Unlock()
		device.Close()
	}
	return err
}

func (c *darwinVirtualController) handleEndpointCreate(message darwinCIMessage) error {
	endpoint, err := c.controller.createEndpointSM(message)
	if err != nil {
		return err
	}
	key := darwinEndpointKey{device: message.deviceAddress(), endpoint: message.endpointAddress()}
	c.stateAccess.Lock()
	c.endpoints[key] = endpoint
	c.stateAccess.Unlock()
	return endpoint.respond(message, ciStatusSuccess)
}

func (c *darwinVirtualController) handleEndpointCommand(message darwinCIMessage) error {
	key := darwinEndpointKey{device: message.deviceAddress(), endpoint: message.endpointAddress()}
	c.stateAccess.Lock()
	endpoint := c.endpoints[key]
	c.stateAccess.Unlock()
	if endpoint == nil {
		return nil
	}
	err := endpoint.respond(message, ciStatusSuccess)
	if message.messageType() == ciMsgEndpointDestroy {
		c.stateAccess.Lock()
		delete(c.endpoints, key)
		delete(c.controlStates, key.device)
		c.stateAccess.Unlock()
		endpoint.Close()
	}
	return err
}

func (c *darwinVirtualController) handleDoorbell(doorbell uint32) {
	key := darwinEndpointKey{
		device:   uint8(doorbell & 0xff),
		endpoint: uint8((doorbell >> 8) & 0xff),
	}
	c.stateAccess.Lock()
	endpoint := c.endpoints[key]
	c.stateAccess.Unlock()
	if endpoint == nil {
		return
	}
	err := endpoint.processDoorbell(doorbell)
	if err != nil {
		c.logger.Debug("process doorbell: ", err)
		return
	}
	var previousNoResponse unsafe.Pointer
	for {
		transfer := endpoint.currentTransfer()
		if transfer.ptr == nil || !transfer.message.valid() {
			return
		}
		if transfer.message.noResponse() {
			if transfer.ptr == previousNoResponse {
				return
			}
			previousNoResponse = transfer.ptr
			c.handleTransfer(key, transfer.message)
			continue
		}
		previousNoResponse = nil
		status, length := c.handleTransfer(key, transfer.message)
		err = endpoint.complete(transfer, darwinUSBIPStatusToCIStatus(status), length)
		if err != nil {
			c.logger.Debug("complete transfer: ", err)
			c.requestClose()
			return
		}
	}
}

func (c *darwinVirtualController) teardownIOUSBHostState() {
	c.stateAccess.Lock()
	endpoints := make([]darwinEndpointStateMachine, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	c.endpoints = make(map[darwinEndpointKey]darwinEndpointStateMachine)
	devices := make([]*darwinUSBHostDeviceSM, 0, len(c.devices))
	for _, device := range c.devices {
		devices = append(devices, device)
	}
	c.devices = make(map[uint8]*darwinUSBHostDeviceSM)
	c.controlStates = make(map[uint8]darwinControlState)
	controller := c.controller
	c.controller = nil
	c.stateAccess.Unlock()

	for _, endpoint := range endpoints {
		endpoint.Close()
	}
	for _, device := range devices {
		device.Close()
	}
	if controller != nil {
		controller.Close()
	}
}

func (c *darwinVirtualController) handleTransfer(key darwinEndpointKey, message darwinCIMessage) (int32, int) {
	switch message.messageType() {
	case ciMsgSetupTransfer:
		c.stateAccess.Lock()
		c.controlStates[key.device] = darwinControlState{setup: message.setup()}
		c.stateAccess.Unlock()
		return 0, 0
	case ciMsgStatusTransfer:
		return c.handleControlStatusTransfer(key)
	case ciMsgNormalTransfer:
		if key.endpoint == 0 {
			return c.handleControlDataTransfer(key, message)
		}
		return c.handleNormalTransfer(key, message)
	case ciMsgIsochronousTransfer:
		return c.handleIsoTransfer(key, message)
	default:
		return -int32(unix.EIO), 0
	}
}

func (c *darwinVirtualController) handleControlDataTransfer(key darwinEndpointKey, message darwinCIMessage) (int32, int) {
	c.stateAccess.Lock()
	state, ok := c.controlStates[key.device]
	c.stateAccess.Unlock()
	if !ok {
		return -int32(unix.EPROTO), 0
	}
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if state.setup[0]&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	response, err := c.sendSubmit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     c.info.DevID(),
			Direction: direction,
			Endpoint:  0,
		},
		TransferBufferLength: int32(length),
		NumberOfPackets:      nonIsoPacketCount,
		Setup:                state.setup,
		Buffer:               buffer,
	})
	if err != nil {
		return -int32(unix.EIO), 0
	}
	if direction == USBIPDirIn {
		return c.completeSubmitInTransfer(message.bufferPointer(), response, length)
	}
	return response.Status, int(response.ActualLength)
}

func (c *darwinVirtualController) handleControlStatusTransfer(key darwinEndpointKey) (int32, int) {
	c.stateAccess.Lock()
	state, ok := c.controlStates[key.device]
	delete(c.controlStates, key.device)
	c.stateAccess.Unlock()
	if !ok {
		return 0, 0
	}
	if state.setup[6] != 0 || state.setup[7] != 0 {
		return 0, 0
	}
	response, err := c.sendSubmit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     c.info.DevID(),
			Direction: USBIPDirOut,
			Endpoint:  0,
		},
		NumberOfPackets: nonIsoPacketCount,
		Setup:           state.setup,
	})
	if err != nil {
		return -int32(unix.EIO), 0
	}
	return response.Status, 0
}

func (c *darwinVirtualController) handleNormalTransfer(key darwinEndpointKey, message darwinCIMessage) (int32, int) {
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if key.endpoint&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	response, err := c.sendSubmit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     c.info.DevID(),
			Direction: direction,
			Endpoint:  uint32(key.endpoint & 0x0f),
		},
		TransferBufferLength: int32(length),
		NumberOfPackets:      nonIsoPacketCount,
		Buffer:               buffer,
	})
	if err != nil {
		return -int32(unix.EIO), 0
	}
	if direction == USBIPDirIn {
		return c.completeSubmitInTransfer(message.bufferPointer(), response, length)
	}
	return response.Status, int(response.ActualLength)
}

func (c *darwinVirtualController) handleIsoTransfer(key darwinEndpointKey, message darwinCIMessage) (int32, int) {
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if key.endpoint&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	startFrame := message.isoFrame()
	transferFlags := int32(0)
	if message.isoASAP() {
		startFrame = 0
		transferFlags = usbipTransferFlagIsoASAP
	}
	response, err := c.sendSubmit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     c.info.DevID(),
			Direction: direction,
			Endpoint:  uint32(key.endpoint & 0x0f),
		},
		TransferFlags:        transferFlags,
		TransferBufferLength: int32(length),
		StartFrame:           startFrame,
		NumberOfPackets:      1,
		Buffer:               buffer,
		IsoPackets: []IsoPacketDescriptor{{
			Offset: 0,
			Length: int32(length),
		}},
	})
	if err != nil {
		return -int32(unix.EIO), 0
	}
	if direction == USBIPDirIn {
		return c.completeSubmitInTransfer(message.bufferPointer(), response, length)
	}
	return response.Status, int(response.ActualLength)
}

func (c *darwinVirtualController) completeSubmitInTransfer(ptr unsafe.Pointer, response SubmitResponse, requestLength int) (int32, int) {
	if response.ActualLength < 0 {
		c.logger.Debug("RET_SUBMIT actual_length is negative: ", response.ActualLength)
		c.requestClose()
		return -int32(unix.EPROTO), 0
	}
	actualLength := int(response.ActualLength)
	if actualLength > requestLength || len(response.Buffer) > requestLength {
		c.logger.Debug("RET_SUBMIT actual_length ", actualLength, " exceeds request length ", requestLength)
		c.requestClose()
		return -int32(unix.EOVERFLOW), 0
	}
	copyLength := actualLength
	if copyLength > len(response.Buffer) {
		copyLength = len(response.Buffer)
	}
	if copyLength > 0 {
		copyToUnsafe(ptr, response.Buffer[:copyLength])
	}
	return response.Status, actualLength
}

func (c *darwinVirtualController) sendSubmit(command SubmitCommand) (SubmitResponse, error) {
	seq := c.seq.Add(1)
	command.Header.SeqNum = seq
	if command.NumberOfPackets == 0 && len(command.IsoPackets) == 0 {
		command.NumberOfPackets = nonIsoPacketCount
	}
	reply := make(chan SubmitResponse, 1)
	c.pendingAccess.Lock()
	c.pending[seq] = darwinPendingSubmit{direction: command.Header.Direction, reply: reply}
	c.pendingAccess.Unlock()
	defer func() {
		c.pendingAccess.Lock()
		delete(c.pending, seq)
		c.pendingAccess.Unlock()
	}()
	c.writeAccess.Lock()
	err := WriteSubmitCommand(c.conn, command)
	c.writeAccess.Unlock()
	if err != nil {
		return SubmitResponse{}, err
	}
	select {
	case response, ok := <-reply:
		if !ok {
			return SubmitResponse{}, E.New("USB/IP data session closed")
		}
		return response, nil
	case <-c.ctx.Done():
		return SubmitResponse{}, c.ctx.Err()
	}
}

func (c *darwinVirtualController) pendingSubmitDirection(seq uint32) (uint32, bool) {
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	pending, ok := c.pending[seq]
	if !ok {
		return 0, false
	}
	return pending.direction, true
}

func (c *darwinVirtualController) deliverSubmit(response SubmitResponse) {
	c.pendingAccess.Lock()
	pending, ok := c.pending[response.Header.SeqNum]
	if ok {
		delete(c.pending, response.Header.SeqNum)
	}
	c.pendingAccess.Unlock()
	if !ok || pending.reply == nil {
		return
	}
	pending.reply <- response
}

func (c *darwinVirtualController) failPending() {
	c.pendingAccess.Lock()
	defer c.pendingAccess.Unlock()
	for seq, pending := range c.pending {
		delete(c.pending, seq)
		close(pending.reply)
	}
}

func bytesFromUnsafe(ptr unsafe.Pointer, length int) []byte {
	if ptr == nil || length == 0 {
		return nil
	}
	out := make([]byte, length)
	copy(out, unsafe.Slice((*byte)(ptr), length))
	return out
}

func copyToUnsafe(ptr unsafe.Pointer, data []byte) {
	if ptr == nil || len(data) == 0 {
		return
	}
	copy(unsafe.Slice((*byte)(ptr), len(data)), data)
}
