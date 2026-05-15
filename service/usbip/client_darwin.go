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

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

type darwinControllerEvent struct {
	command  *darwinCIMessage
	doorbell uint32
}

type darwinEndpointKey struct {
	device   uint8
	endpoint uint8
}

type darwinPendingSubmit struct {
	direction uint32
	reply     chan SubmitResponse
}

var _ AttachedSession = (*darwinVirtualController)(nil)

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
	closeErr     error
	runErr       error
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
	endpoints     map[darwinEndpointKey]*darwinUSBHostEndpointSM
	controlStates map[uint8][8]byte
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
		endpoints:     make(map[darwinEndpointKey]*darwinUSBHostEndpointSM),
		controlStates: make(map[uint8][8]byte),
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

func (c *darwinVirtualController) Close() error {
	c.requestClose()
	if c.eventStarted.Load() {
		<-c.eventDone
	}
	return c.closeErr
}

func (c *darwinVirtualController) requestClose() {
	c.closeOnce.Do(func() {
		c.cancel()
		if c.conn != nil {
			c.closeErr = c.conn.Close()
		}
	})
}

func (c *darwinVirtualController) Done() <-chan struct{} {
	return c.done
}

func (c *darwinVirtualController) Err() error {
	return c.runErr
}

func (c *darwinVirtualController) Description() string {
	return "IOUSBHostControllerInterface"
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
				if !E.IsClosedOrCanceled(err) {
					c.runErr = err
				}
			}
			c.failPending()
			return
		}
		switch header.Command {
		case RetSubmit:
			c.pendingAccess.Lock()
			pending, ok := c.pending[header.SeqNum]
			c.pendingAccess.Unlock()
			payloadDirection := header.Direction
			if ok {
				payloadDirection = pending.direction
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
		if transfer.message.control&(1<<14) != 0 {
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
	endpoints := make([]*darwinUSBHostEndpointSM, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	c.endpoints = make(map[darwinEndpointKey]*darwinUSBHostEndpointSM)
	devices := make([]*darwinUSBHostDeviceSM, 0, len(c.devices))
	for _, device := range c.devices {
		devices = append(devices, device)
	}
	c.devices = make(map[uint8]*darwinUSBHostDeviceSM)
	c.controlStates = make(map[uint8][8]byte)
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
		c.controlStates[key.device] = message.setup()
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
	setup, ok := c.controlStates[key.device]
	c.stateAccess.Unlock()
	if !ok {
		return -int32(unix.EPROTO), 0
	}
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if setup[0]&0x80 != 0 {
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
		Setup:                setup,
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
	setup, ok := c.controlStates[key.device]
	delete(c.controlStates, key.device)
	c.stateAccess.Unlock()
	if !ok {
		return 0, 0
	}
	if setup[6] != 0 || setup[7] != 0 {
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
		Setup:           setup,
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
	startFrame := int32(message.control >> ciIsochronousTransferControlFramePhase & 0xff)
	transferFlags := int32(0)
	if message.control&ciIsochronousTransferControlASAP != 0 {
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
	copyLength := min(actualLength, len(response.Buffer))
	if copyLength > 0 && ptr != nil {
		if len(response.IsoPackets) > 0 {
			dst := unsafe.Slice((*byte)(ptr), requestLength)
			scatterIsoInResponseBuffer(dst, response.Buffer[:copyLength], response.IsoPackets)
		} else {
			copy(unsafe.Slice((*byte)(ptr), copyLength), response.Buffer[:copyLength])
		}
	}
	return response.Status, actualLength
}

func scatterIsoInResponseBuffer(dst []byte, payload []byte, packets []IsoPacketDescriptor) {
	cursor := 0
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			continue
		}
		if cursor+length > len(payload) {
			length = len(payload) - cursor
			if length <= 0 {
				return
			}
		}
		offset := int(packets[i].Offset)
		if offset < 0 || offset >= len(dst) {
			cursor += length
			continue
		}
		end := min(offset+length, len(dst))
		copy(dst[offset:end], payload[cursor:cursor+(end-offset)])
		cursor += length
	}
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
