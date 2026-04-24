//go:build linux

package usbip

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
	ops        usbipOps

	stateAccess      sync.Mutex
	targets          []clientTarget
	assigned         []string
	assignedWorkers  []*clientAssignedWorker
	allWorkers       map[string]*clientBusIDWorker
	allDesired       map[string]struct{}
	matchedKnownKeys map[string]DeviceKey

	portAssignAccess sync.Mutex
	wg               sync.WaitGroup

	portsAccess sync.Mutex
	ports       map[int]struct{}

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
		ops:          systemUSBIPOps,
		allWorkers:   make(map[string]*clientBusIDWorker),
		allDesired:   make(map[string]struct{}),
		ports:        make(map[int]struct{}),
		activeBusIDs: make(map[string]struct{}),
	}, nil
}

func (c *ClientService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := c.ops.ensureVHCI()
	if err != nil {
		return err
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
		c.logger.Warn("shutdown timeout; some vhci ports may remain attached")
	}
	return nil
}

func (c *ClientService) runBusIDLoop(ctx context.Context, busid, description string) {
	for {
		if ctx.Err() != nil {
			return
		}
		c.setBusIDActive(busid, true)
		port, done, err := c.attemptAttach(ctx, busid)
		if err != nil {
			c.setBusIDActive(busid, false)
			c.logger.Error("attach ", description, " (", busid, "): ", err)
			if !sleepCtx(ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		c.logger.Info("attached ", busid, " → vhci port ", port)
		c.waitPortSession(ctx, port, busid, done)
		c.setBusIDActive(busid, false)
		c.trackPort(port, false)
		if ctx.Err() != nil {
			return
		}
		if !c.shouldRetryBusID(ctx, busid) {
			c.logger.Info("remote export ", busid, " disappeared; stopping import worker")
			return
		}
		c.logger.Info("vhci port ", port, " released; reattaching ", busid)
		if !sleepCtx(ctx, clientReconnectDelay) {
			return
		}
	}
}

func (c *ClientService) attemptAttach(ctx context.Context, busid string) (int, <-chan struct{}, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return -1, nil, E.Cause(err, "dial ", c.serverAddr)
	}
	relayStarted := false
	defer func() {
		if !relayStarted {
			_ = conn.Close()
		}
	}()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()
	lease, err := c.requestImportLease(ctx, busid)
	if err != nil {
		return -1, nil, err
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
			return -1, nil, E.Cause(err, "write OP_REQ_IMPORT_EXT")
		}
	} else {
		err = WriteOpReqImport(conn, busid)
		if err != nil {
			return -1, nil, E.Cause(err, "write OP_REQ_IMPORT")
		}
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return -1, nil, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Version != ProtocolVersion {
		return -1, nil, E.New(fmt.Sprintf("unexpected reply version 0x%04x", header.Version))
	}
	if header.Code != expectedReply {
		return -1, nil, E.New(fmt.Sprintf("unexpected reply code 0x%04x", header.Code))
	}
	if header.Status != OpStatusOK {
		return -1, nil, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return -1, nil, E.Cause(err, "read OP_REP_IMPORT body")
	}
	handoff, err := newUSBIPConnHandoff(conn)
	if err != nil {
		return -1, nil, E.Cause(err, "prepare handoff")
	}
	defer func() {
		if !relayStarted {
			_ = handoff.Close()
		}
	}()
	c.logger.Debug("usbip client handoff ", busid, ": ", handoff.mode())
	c.portAssignAccess.Lock()
	defer c.portAssignAccess.Unlock()
	triedPorts := make(map[int]struct{})
	for {
		port, err := c.ops.vhciPickFreePort(info.Speed, triedPorts)
		if err != nil {
			return -1, nil, err
		}
		if !c.reservePort(port) {
			triedPorts[port] = struct{}{}
			continue
		}
		err = c.ops.vhciAttach(port, handoff.kernelFD(), info.DevID(), info.Speed)
		if err != nil {
			c.trackPort(port, false)
			if errors.Is(err, unix.EBUSY) {
				triedPorts[port] = struct{}{}
				continue
			}
			return -1, nil, E.Cause(err, "vhci attach")
		}
		err = handoff.closeKernelFD()
		if err != nil {
			c.logger.Debug("close kernel fd ", busid, ": ", err)
		}
		done := handoff.startRelay(ctx, c.logger, "client", busid)
		relayStarted = true
		return port, done, nil
	}
}

func (c *ClientService) waitPortSession(ctx context.Context, port int, busid string, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		err := c.ops.vhciDetach(port)
		if err != nil {
			c.logger.Warn("detach port ", port, " (", busid, "): ", err)
		}
	case <-done:
		c.logger.Debug("vhci port ", port, " session ended for ", busid)
	}
}

func (c *ClientService) trackPort(port int, add bool) {
	c.portsAccess.Lock()
	defer c.portsAccess.Unlock()
	if c.ports == nil {
		c.ports = make(map[int]struct{})
	}
	if add {
		c.logger.Debug("reserve vhci port ", port)
		c.ports[port] = struct{}{}
	} else {
		c.logger.Debug("release vhci port ", port)
		delete(c.ports, port)
	}
}

func (c *ClientService) reservePort(port int) bool {
	c.portsAccess.Lock()
	defer c.portsAccess.Unlock()
	if c.ports == nil {
		c.ports = make(map[int]struct{})
	}
	if _, exists := c.ports[port]; exists {
		c.logger.Debug("vhci port ", port, " already reserved locally")
		return false
	}
	c.logger.Debug("reserve vhci port ", port)
	c.ports[port] = struct{}{}
	return true
}
