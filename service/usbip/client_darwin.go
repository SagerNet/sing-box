//go:build darwin && cgo

package usbip

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sagernet/sing-box/log"
)

type darwinControllerEvent struct {
	command  *darwinCIMessage
	doorbell uint32
}

type darwinEndpointKey struct {
	device   uint8
	endpoint uint8
}

var _ AttachedSession = (*darwinVirtualController)(nil)

type darwinVirtualController struct {
	ctx       context.Context
	cancel    context.CancelFunc
	logger    log.ContextLogger
	conn      net.Conn
	info      DeviceInfoTruncated
	startTime time.Time

	peer *UsbIpPeer
	iso  *IsoScheduler

	controller   *darwinUSBHostController
	events       chan darwinControllerEvent
	done         chan struct{}
	eventDone    chan struct{}
	closeOnce    sync.Once
	runErr       error
	eventStarted atomic.Bool

	stateAccess sync.Mutex
	powered     bool
	connected   bool
	nextAddress uint8
	devices     map[uint8]*darwinUSBHostDeviceSM
	endpoints   map[darwinEndpointKey]*darwinEndpoint
}

func newDarwinVirtualController(ctx context.Context, logger log.ContextLogger, conn net.Conn, info DeviceInfoTruncated) *darwinVirtualController {
	ctx, cancel := context.WithCancel(ctx)
	c := &darwinVirtualController{
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		conn:        conn,
		info:        info,
		startTime:   time.Now(),
		events:      make(chan darwinControllerEvent, 64),
		done:        make(chan struct{}),
		eventDone:   make(chan struct{}),
		nextAddress: 1,
		devices:     make(map[uint8]*darwinUSBHostDeviceSM),
		endpoints:   make(map[darwinEndpointKey]*darwinEndpoint),
	}
	c.iso = NewIsoScheduler(c)
	return c
}

func (c *darwinVirtualController) CurrentFrame() uint64 {
	return uint64(time.Since(c.startTime) / time.Millisecond)
}

func (c *darwinVirtualController) Start() error {
	c.peer = NewUsbIpPeer(c.ctx, c.logger, c.conn)
	controller, err := darwinCreateUSBHostController(c, 1, c.info.Speed)
	if err != nil {
		_ = c.peer.Close()
		return err
	}
	c.controller = controller
	c.eventStarted.Store(true)
	go c.watchPeer()
	go c.eventLoop()
	return nil
}

func (c *darwinVirtualController) Close() error {
	c.requestClose()
	if c.eventStarted.Load() {
		<-c.eventDone
	}
	return nil
}

func (c *darwinVirtualController) requestClose() {
	c.closeOnce.Do(func() {
		c.cancel()
		if c.peer != nil {
			_ = c.peer.Close()
		} else if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (c *darwinVirtualController) watchPeer() {
	defer close(c.done)
	defer c.cancel()
	<-c.peer.Done()
	c.runErr = c.peer.Err()
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
		err = c.controller.respondFrame(message, ciStatusSuccess, c.CurrentFrame(), darwinCIFrameTimestamp())
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
	sm, err := c.controller.createEndpointSM(message)
	if err != nil {
		return err
	}
	key := darwinEndpointKey{device: message.deviceAddress(), endpoint: message.endpointAddress()}
	endpoint := newDarwinEndpoint(c.ctx, c.logger, sm, c.peer, c.iso, c.info.DevID(), key)
	c.stateAccess.Lock()
	c.endpoints[key] = endpoint
	c.stateAccess.Unlock()
	return sm.respond(message, ciStatusSuccess)
}

func (c *darwinVirtualController) handleEndpointCommand(message darwinCIMessage) error {
	key := darwinEndpointKey{device: message.deviceAddress(), endpoint: message.endpointAddress()}
	destroy := message.messageType() == ciMsgEndpointDestroy
	c.stateAccess.Lock()
	endpoint := c.endpoints[key]
	if destroy {
		delete(c.endpoints, key)
	}
	c.stateAccess.Unlock()
	if endpoint == nil {
		return nil
	}
	endpoint.Command(message)
	if destroy {
		endpoint.Wait()
	}
	return nil
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
	endpoint.EnqueueDoorbell(doorbell)
}

func (c *darwinVirtualController) teardownIOUSBHostState() {
	c.stateAccess.Lock()
	endpoints := make([]*darwinEndpoint, 0, len(c.endpoints))
	for _, endpoint := range c.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	c.endpoints = make(map[darwinEndpointKey]*darwinEndpoint)
	devices := make([]*darwinUSBHostDeviceSM, 0, len(c.devices))
	for _, device := range c.devices {
		devices = append(devices, device)
	}
	c.devices = make(map[uint8]*darwinUSBHostDeviceSM)
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
