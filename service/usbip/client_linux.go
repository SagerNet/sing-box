//go:build linux

package usbip

import (
	"context"
	"errors"
	"net"
	"slices"
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
)

const (
	clientReconnectDelay   = 5 * time.Second
	clientShutdownTimeout  = 15 * time.Second
	clientDetachTimeout    = 10 * time.Second
	clientDetachPoll       = 100 * time.Millisecond
	controlPingInterval    = 10 * time.Second
	controlReadTimeout     = 30 * time.Second
	controlWriteTimeout    = 5 * time.Second
	controlSessionIdleHint = "control session lost"
)

var (
	errImmediateReconnect = errors.New("usbip control reconnect")
	errControlUnsupported = errors.New("usbip control unsupported")
)

type clientTarget struct {
	fixedBusID string
	match      option.USBIPDeviceMatch
}

func (t clientTarget) description() string {
	if t.fixedBusID != "" {
		return describeMatch(option.USBIPDeviceMatch{BusID: t.fixedBusID})
	}
	return describeMatch(t.match)
}

type clientAssignedWorker struct {
	target  clientTarget
	updates chan string
}

type clientBusIDWorker struct {
	cancel context.CancelFunc
}

type ClientService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	matches    []option.USBIPDeviceMatch // empty = import all remote exports
	ops        usbipOps

	stateMu         sync.Mutex
	targets         []clientTarget
	assigned        []string
	assignedWorkers []*clientAssignedWorker
	allWorkers      map[string]*clientBusIDWorker
	allDesired      map[string]struct{}

	attachMu sync.Mutex // serializes vhci port pick + attach
	wg       sync.WaitGroup

	portsMu sync.Mutex
	ports   map[int]struct{}

	activeMu     sync.Mutex
	activeBusIDs map[string]struct{}

	controlMu      sync.Mutex
	controlSession *clientControlSession

	remoteMu        sync.Mutex
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
	if err := c.ops.ensureVHCI(); err != nil {
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
	select {
	case <-done:
	case <-time.After(clientShutdownTimeout):
		c.logger.Warn("shutdown timeout; some vhci ports may remain attached")
	}
	return nil
}

func (c *ClientService) initializeWorkers() {
	targets := c.buildTargets()
	c.stateMu.Lock()
	c.targets = targets
	if len(c.matches) == 0 {
		c.stateMu.Unlock()
		return
	}
	c.assigned = make([]string, len(targets))
	c.assignedWorkers = make([]*clientAssignedWorker, len(targets))
	for i, target := range targets {
		c.assignedWorkers[i] = &clientAssignedWorker{
			target:  target,
			updates: make(chan string, 1),
		}
	}
	workers := append([]*clientAssignedWorker(nil), c.assignedWorkers...)
	c.stateMu.Unlock()

	for _, worker := range workers {
		c.wg.Add(1)
		go c.runAssignedWorker(worker)
	}
}

func (c *ClientService) run() {
	defer c.wg.Done()
	for immediate := true; immediate || sleepCtx(c.ctx, clientReconnectDelay); {
		err := c.runSession()
		if c.ctx.Err() != nil {
			break
		}
		if err != nil {
			c.logger.Error("control ", c.serverAddr, ": ", err)
		}
		immediate = errors.Is(err, errImmediateReconnect)
	}
	c.stopAllWorkers()
}

func (c *ClientService) runSession() error {
	err := c.runControlSession()
	if errors.Is(err, errControlUnsupported) {
		c.logger.Info("control channel unsupported by ", c.serverAddr, "; using standard usbip mode")
		return c.runStandardSession()
	}
	return err
}

func (c *ClientService) runControlSession() error {
	conn, err := c.dialer.DialContext(c.ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return E.Cause(err, "dial ", c.serverAddr)
	}
	defer conn.Close()
	stopCloseOnCancel := closeConnOnContextDone(c.ctx, conn)
	defer stopCloseOnCancel()

	_ = conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
	_ = conn.SetReadDeadline(time.Now().Add(controlWriteTimeout))
	if err := WriteControlPreface(conn); err != nil {
		return E.Cause(errControlUnsupported, "write control preface: ", err)
	}
	if err := WriteControlHello(conn); err != nil {
		return E.Cause(errControlUnsupported, "write control hello: ", err)
	}
	ack, err := ReadControlFrame(conn)
	if err != nil {
		return E.Cause(errControlUnsupported, "read control ack: ", err)
	}
	if ack.Type != controlFrameAck {
		return E.Cause(errControlUnsupported, "unexpected control ack frame ", ack.Type)
	}
	if ack.Version != controlProtocolVersion {
		return E.Cause(errControlUnsupported, "unsupported control version ", ack.Version)
	}
	if ack.Capabilities&controlRequiredCapabilities != controlRequiredCapabilities {
		return E.Cause(errControlUnsupported, "missing control capabilities 0x", ack.Capabilities)
	}
	_ = conn.SetWriteDeadline(time.Time{})
	_ = conn.SetReadDeadline(time.Time{})

	session := newClientControlSession(conn, ack.Capabilities)
	extended := supportsControlExtensions(ack.Capabilities)
	if extended {
		c.setControlSession(session)
		defer c.clearControlSession(session, errClientControlSessionClosed)
	} else if err := c.syncRemoteState(); err != nil {
		return E.Cause(err, "initial devlist sync")
	}

	pingDone := make(chan struct{})
	go c.controlPingLoop(session, pingDone)
	defer close(pingDone)

	lastSeq := ack.Sequence
	for {
		if err := conn.SetReadDeadline(time.Now().Add(controlReadTimeout)); err != nil {
			return err
		}
		message, err := readControlMessage(conn)
		if err != nil {
			return E.Cause(errImmediateReconnect, controlSessionIdleHint, ": ", err)
		}
		frame := message.Frame
		switch frame.Type {
		case controlFrameChanged:
			if frame.Sequence != lastSeq && frame.Sequence != lastSeq+1 {
				return E.Cause(errImmediateReconnect, "control sequence jumped from ", lastSeq, " to ", frame.Sequence)
			}
			lastSeq = frame.Sequence
			if extended {
				err = c.syncRemoteStateAndResetControlState(c.ctx)
			} else {
				err = c.syncRemoteState()
			}
			if err != nil {
				return E.Cause(errImmediateReconnect, "devlist sync after change ", frame.Sequence, ": ", err)
			}
		case controlFrameDeviceSnapshot:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			var snapshot controlDeviceSnapshot
			if err := unmarshalControlPayload(message.Payload, &snapshot); err != nil {
				return E.Cause(errImmediateReconnect, "read device snapshot: ", err)
			}
			lastSeq = frame.Sequence
			c.applyControlSnapshot(snapshot)
		case controlFrameDeviceDelta:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			if frame.Sequence != lastSeq+1 {
				if err := c.syncRemoteStateAndResetControlState(c.ctx); err != nil {
					return E.Cause(errImmediateReconnect, "devlist sync after sequence jump ", frame.Sequence, ": ", err)
				}
				lastSeq = frame.Sequence
				continue
			}
			var delta controlDeviceDelta
			if err := unmarshalControlPayload(message.Payload, &delta); err != nil {
				return E.Cause(errImmediateReconnect, "read device delta: ", err)
			}
			lastSeq = frame.Sequence
			c.applyControlDelta(delta)
		case controlFrameLeaseResponse:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			var response controlLeaseResponse
			if err := unmarshalControlPayload(message.Payload, &response); err != nil {
				return E.Cause(errImmediateReconnect, "read lease response: ", err)
			}
			session.deliverLeaseResponse(response)
		case controlFramePong:
		default:
			return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
		}
	}
}

func (c *ClientService) runStandardSession() error {
	if err := c.syncRemoteState(); err != nil {
		return E.Cause(err, "initial devlist sync")
	}
	<-c.ctx.Done()
	return nil
}

func (c *ClientService) controlPingLoop(session *clientControlSession, done <-chan struct{}) {
	ticker := time.NewTicker(controlPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			if err := session.writeControl(controlFrame{
				Type:    controlFramePing,
				Version: controlProtocolVersion,
			}, nil); err != nil {
				_ = session.conn.Close()
				return
			}
		}
	}
}

func (c *ClientService) syncRemoteState() error {
	return c.syncRemoteStateContext(c.ctx)
}

func (c *ClientService) syncRemoteStateContext(ctx context.Context) error {
	entries, err := c.fetchDevList(ctx)
	if err != nil {
		return err
	}
	c.applyRemoteEntries(entries)
	return nil
}

func (c *ClientService) applyRemoteEntries(entries []DeviceEntry) {
	if len(c.matches) == 0 {
		c.applyRemoteExports(entries)
		return
	}
	c.applyMatchedExports(entries)
}

func (c *ClientService) applyRemoteDeviceState(devices []DeviceInfoV2) {
	availableEntries := deviceInfoV2ToEntries(devices, true)
	if len(c.matches) == 0 {
		c.applyRemoteExports(availableEntries)
		return
	}
	knownKeys := make(map[string]DeviceKey, len(devices))
	for _, device := range devices {
		if device.BusID == "" {
			continue
		}
		knownKeys[device.BusID] = device.key()
	}
	c.applyMatchedExportsWithRetained(availableEntries, knownKeys)
}

func (c *ClientService) applyRemoteExports(entries []DeviceEntry) {
	desired := make(map[string]struct{}, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		desired[busid] = struct{}{}
	}

	c.stateMu.Lock()
	c.allDesired = desired
	stopWorkers := make([]*clientBusIDWorker, 0)
	for busid, worker := range c.allWorkers {
		if _, ok := desired[busid]; ok {
			continue
		}
		if c.isBusIDActive(busid) {
			continue
		}
		stopWorkers = append(stopWorkers, worker)
		delete(c.allWorkers, busid)
	}
	startBusIDs := make([]string, 0)
	for busid := range desired {
		if _, ok := c.allWorkers[busid]; ok {
			continue
		}
		startBusIDs = append(startBusIDs, busid)
	}
	c.stateMu.Unlock()

	for _, worker := range stopWorkers {
		worker.cancel()
	}
	slices.Sort(startBusIDs)
	for _, busid := range startBusIDs {
		c.startRemoteBusIDWorker(busid, busid)
	}
}

func (c *ClientService) applyMatchedExports(entries []DeviceEntry) {
	c.applyMatchedExportsWithRetained(entries, nil)
}

func (c *ClientService) applyMatchedExportsWithRetained(entries []DeviceEntry, knownKeys map[string]DeviceKey) {
	c.stateMu.Lock()
	if len(c.targets) == 0 {
		c.stateMu.Unlock()
		return
	}
	activeCurrent := c.activeCurrentAssignmentsLocked(c.assigned, knownKeys)
	nextAssigned := assignMatchedBusIDsWithRetained(c.targets, c.assigned, entries, knownKeys, activeCurrent)
	workers := append([]*clientAssignedWorker(nil), c.assignedWorkers...)
	previous := append([]string(nil), c.assigned...)
	c.assigned = nextAssigned
	c.stateMu.Unlock()

	for i, worker := range workers {
		if previous[i] == nextAssigned[i] {
			continue
		}
		worker.setDesiredBusID(nextAssigned[i])
	}
}

func (c *ClientService) activeCurrentAssignmentsLocked(current []string, knownKeys map[string]DeviceKey) map[string]struct{} {
	if len(knownKeys) == 0 {
		return nil
	}
	var activeCurrent map[string]struct{}
	for _, busid := range current {
		if busid == "" {
			continue
		}
		if _, ok := knownKeys[busid]; !ok {
			continue
		}
		if !c.isBusIDActive(busid) {
			continue
		}
		if activeCurrent == nil {
			activeCurrent = make(map[string]struct{})
		}
		activeCurrent[busid] = struct{}{}
	}
	return activeCurrent
}

func (c *ClientService) runAssignedWorker(worker *clientAssignedWorker) {
	defer c.wg.Done()

	var current string
	var runnerCancel context.CancelFunc
	var runnerDone chan struct{}

	stopRunner := func() {
		if runnerCancel == nil {
			return
		}
		runnerCancel()
		<-runnerDone
		runnerCancel = nil
		runnerDone = nil
	}

	for {
		select {
		case <-c.ctx.Done():
			stopRunner()
			return
		case desired := <-worker.updates:
			if desired == current {
				continue
			}
			stopRunner()
			current = desired
			if desired == "" {
				continue
			}

			runCtx, cancel := context.WithCancel(c.ctx)
			done := make(chan struct{})
			runnerCancel = cancel
			runnerDone = done

			c.wg.Add(1)
			go func(busid string) {
				defer c.wg.Done()
				defer close(done)
				c.runBusIDLoop(runCtx, busid, worker.target.description())
			}(desired)
		}
	}
}

func (w *clientAssignedWorker) setDesiredBusID(busid string) {
	select {
	case w.updates <- busid:
		return
	default:
	}
	select {
	case <-w.updates:
	default:
	}
	w.updates <- busid
}

func (c *ClientService) startRemoteBusIDWorker(busid, description string) {
	runCtx, cancel := context.WithCancel(c.ctx)
	worker := &clientBusIDWorker{cancel: cancel}

	c.stateMu.Lock()
	c.allWorkers[busid] = worker
	c.stateMu.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.runBusIDLoop(runCtx, busid, description)
	}()
}

func (c *ClientService) stopAllWorkers() {
	c.stateMu.Lock()
	workers := make([]*clientBusIDWorker, 0, len(c.allWorkers))
	for _, worker := range c.allWorkers {
		workers = append(workers, worker)
	}
	c.allWorkers = make(map[string]*clientBusIDWorker)
	c.stateMu.Unlock()

	for _, worker := range workers {
		worker.cancel()
	}
}

func (c *ClientService) buildTargets() []clientTarget {
	if len(c.matches) == 0 {
		return nil
	}
	seenFixed := make(map[string]struct{})
	targets := make([]clientTarget, 0, len(c.matches))
	for _, m := range c.matches {
		if isBusIDOnlyMatch(m) {
			if _, seen := seenFixed[m.BusID]; seen {
				continue
			}
			seenFixed[m.BusID] = struct{}{}
			targets = append(targets, clientTarget{fixedBusID: m.BusID})
			continue
		}
		targets = append(targets, clientTarget{match: m})
	}
	return targets
}

func (c *ClientService) fetchDevList(ctx context.Context) ([]DeviceEntry, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()
	if err := WriteOpHeader(conn, OpReqDevList, OpStatusOK); err != nil {
		return nil, E.Cause(err, "send OP_REQ_DEVLIST")
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_DEVLIST header")
	}
	if header.Version != ProtocolVersion {
		return nil, E.New("unexpected reply version 0x", hex16(header.Version))
	}
	if header.Code != OpRepDevList || header.Status != OpStatusOK {
		return nil, E.New("OP_REP_DEVLIST status=", header.Status, " code=0x", hex16(header.Code))
	}
	return ReadOpRepDevListBody(conn)
}

func (c *ClientService) runBusIDLoop(ctx context.Context, busid, description string) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		port, err := c.attemptAttach(ctx, busid)
		if err != nil {
			c.logger.Error("attach ", description, " (", busid, "): ", err)
			if !sleepCtx(ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		c.logger.Info("attached ", busid, " → vhci port ", port)
		c.setBusIDActive(busid, true)
		c.watchPort(ctx, port, busid)
		c.setBusIDActive(busid, false)
		c.trackPort(port, false)
		if err := ctx.Err(); err != nil {
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

func (c *ClientService) attemptAttach(ctx context.Context, busid string) (int, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return -1, E.Cause(err, "dial ", c.serverAddr)
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
		return -1, err
	}
	expectedReply := OpRepImport
	if lease.Valid {
		expectedReply = OpRepImportExt
		if err := WriteOpReqImportExt(conn, ImportExtRequest{
			BusID:       busid,
			LeaseID:     lease.ID,
			ClientNonce: lease.ClientNonce,
		}); err != nil {
			return -1, E.Cause(err, "write OP_REQ_IMPORT_EXT")
		}
	} else if err := WriteOpReqImport(conn, busid); err != nil {
		return -1, E.Cause(err, "write OP_REQ_IMPORT")
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return -1, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Version != ProtocolVersion {
		return -1, E.New("unexpected reply version 0x", hex16(header.Version))
	}
	if header.Code != expectedReply {
		return -1, E.New("unexpected reply code 0x", hex16(header.Code))
	}
	if header.Status != OpStatusOK {
		return -1, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return -1, E.Cause(err, "read OP_REP_IMPORT body")
	}
	handoff, err := newUSBIPConnHandoff(conn)
	if err != nil {
		return -1, E.Cause(err, "prepare handoff")
	}
	defer func() {
		if !relayStarted {
			_ = handoff.Close()
		}
	}()
	c.logger.Debug("usbip client handoff ", busid, ": ", handoff.mode())
	c.attachMu.Lock()
	defer c.attachMu.Unlock()
	port, err := c.ops.vhciPickFreePort(info.Speed)
	if err != nil {
		return -1, err
	}
	if !c.reservePort(port) {
		return -1, E.New("vhci port ", port, " already reserved")
	}
	if err := c.ops.vhciAttach(port, handoff.kernelFD(), info.DevID(), info.Speed); err != nil {
		c.trackPort(port, false)
		return -1, E.Cause(err, "vhci attach")
	}
	if err := handoff.closeKernelFD(); err != nil {
		c.logger.Debug("close kernel fd ", busid, ": ", err)
	}
	relayStarted = handoff.startRelay(ctx, c.logger, "client", busid)
	return port, nil
}

func (c *ClientService) watchPort(ctx context.Context, port int, busid string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	seenUsed := false
	settleDeadline := time.NewTimer(10 * time.Second)
	defer settleDeadline.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := c.ops.vhciDetach(port); err != nil {
				c.logger.Warn("detach port ", port, " (", busid, "): ", err)
			}
			c.waitVHCIPortIdle(port, busid)
			return
		case <-settleDeadline.C:
			if !seenUsed {
				c.logger.Warn("vhci port ", port, " never reached used state; reattaching ", busid)
				if err := c.ops.vhciDetach(port); err != nil {
					c.logger.Warn("detach port ", port, " (", busid, "): ", err)
				}
				c.waitVHCIPortIdle(port, busid)
				return
			}
		case <-ticker.C:
			used, err := c.ops.vhciPortUsed(port)
			if err != nil {
				c.logger.Debug("poll port ", port, ": ", err)
				continue
			}
			if used {
				if !seenUsed {
					c.logger.Debug("vhci port ", port, " entered used state for ", busid)
				}
				seenUsed = true
				continue
			}
			if seenUsed {
				c.logger.Debug("vhci port ", port, " left used state for ", busid)
				return
			}
		}
	}
}

func (c *ClientService) waitVHCIPortIdle(port int, busid string) {
	deadline := time.Now().Add(clientDetachTimeout)
	for {
		used, err := c.ops.vhciPortUsed(port)
		if err == nil && !used {
			return
		}
		if time.Now().After(deadline) {
			if err != nil {
				c.logger.Warn("poll detached vhci port ", port, " (", busid, "): ", err)
			} else {
				c.logger.Warn("vhci port ", port, " stayed used after detach for ", busid)
			}
			return
		}
		time.Sleep(clientDetachPoll)
	}
}

func (c *ClientService) trackPort(port int, add bool) {
	c.portsMu.Lock()
	defer c.portsMu.Unlock()
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
	c.portsMu.Lock()
	defer c.portsMu.Unlock()
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

func (c *ClientService) setBusIDActive(busid string, active bool) {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	if c.activeBusIDs == nil {
		c.activeBusIDs = make(map[string]struct{})
	}
	if active {
		c.activeBusIDs[busid] = struct{}{}
	} else {
		delete(c.activeBusIDs, busid)
	}
}

func (c *ClientService) isBusIDActive(busid string) bool {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
	_, exists := c.activeBusIDs[busid]
	return exists
}

func (c *ClientService) shouldRetryBusID(ctx context.Context, busid string) bool {
	if len(c.matches) != 0 {
		return true
	}
	if err := c.syncRemoteStateContext(ctx); err != nil {
		c.logger.Warn("refresh remote exports after releasing ", busid, ": ", err)
		return true
	}
	return c.isBusIDRetryDesired(busid)
}

func (c *ClientService) isBusIDRetryDesired(busid string) bool {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if _, registered := c.allWorkers[busid]; !registered {
		return false
	}
	if _, desired := c.allDesired[busid]; desired {
		return true
	}
	return false
}

func isBusIDOnlyMatch(m option.USBIPDeviceMatch) bool {
	return m.BusID != "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == ""
}

func assignMatchedBusIDs(targets []clientTarget, current []string, entries []DeviceEntry) []string {
	return assignMatchedBusIDsWithRetained(targets, current, entries, nil, nil)
}

func assignMatchedBusIDsWithRetained(
	targets []clientTarget,
	current []string,
	entries []DeviceEntry,
	knownKeys map[string]DeviceKey,
	activeCurrent map[string]struct{},
) []string {
	if len(targets) == 0 {
		return nil
	}
	keysByBusID := make(map[string]DeviceKey, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		keysByBusID[busid] = entryDeviceKey(entries[i])
	}
	currentKey := func(busid string) (DeviceKey, bool) {
		if key, ok := keysByBusID[busid]; ok {
			return key, true
		}
		if _, active := activeCurrent[busid]; !active {
			return DeviceKey{}, false
		}
		key, ok := knownKeys[busid]
		return key, ok
	}

	nextAssigned := make([]string, len(targets))
	reserved := make(map[string]struct{}, len(targets))
	for i, target := range targets {
		if target.fixedBusID == "" {
			continue
		}
		if _, ok := keysByBusID[target.fixedBusID]; ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
			continue
		}
		if i >= len(current) || current[i] != target.fixedBusID {
			continue
		}
		if _, ok := currentKey(target.fixedBusID); ok {
			nextAssigned[i] = target.fixedBusID
			reserved[target.fixedBusID] = struct{}{}
		}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || i >= len(current) {
			continue
		}
		if current[i] == "" {
			continue
		}
		if _, ok := reserved[current[i]]; ok {
			continue
		}
		key, ok := currentKey(current[i])
		if !ok || !Matches(target.match, key) {
			continue
		}
		nextAssigned[i] = current[i]
		reserved[current[i]] = struct{}{}
	}
	for i, target := range targets {
		if target.fixedBusID != "" || nextAssigned[i] != "" {
			continue
		}
		nextAssigned[i] = firstMatchingUnclaimedBusID(target.match, entries, reserved)
		if nextAssigned[i] != "" {
			reserved[nextAssigned[i]] = struct{}{}
		}
	}
	return nextAssigned
}

func firstMatchingUnclaimedBusID(match option.USBIPDeviceMatch, entries []DeviceEntry, reserved map[string]struct{}) string {
	for i := range entries {
		key := entryDeviceKey(entries[i])
		if _, claimed := reserved[key.BusID]; claimed {
			continue
		}
		if Matches(match, key) {
			return key.BusID
		}
	}
	return ""
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func closeConnOnContextDone(ctx context.Context, conn net.Conn) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
}
