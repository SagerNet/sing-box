//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"io"
	"net"
	"slices"
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

const (
	clientReconnectDelay   = 5 * time.Second
	controlPingInterval    = 10 * time.Second
	controlReadTimeout     = 30 * time.Second
	controlWriteTimeout    = 5 * time.Second
	controlSessionIdleHint = "control session lost"
)

var (
	errImmediateReconnect = errors.New("usbip control reconnect")
	errControlUnsupported = errors.New("usbip control unsupported")
)

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
	matches    []option.USBIPDeviceMatch

	stateMu          sync.Mutex
	targets          []clientTarget
	assigned         []string
	assignedWorkers  []*clientAssignedWorker
	allWorkers       map[string]*clientBusIDWorker
	allDesired       map[string]struct{}
	matchedKnownKeys map[string]DeviceKey

	wg sync.WaitGroup

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
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		c.logger.Warn("shutdown timeout; some macOS USB/IP controllers may remain active")
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
	if ack.Type != controlFrameAck || ack.Version != controlProtocolVersion || ack.Capabilities&controlRequiredCapabilities != controlRequiredCapabilities {
		return E.Cause(errControlUnsupported, "invalid control ack")
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
		if busid != "" {
			desired[busid] = struct{}{}
		}
	}
	c.stateMu.Lock()
	c.allDesired = desired
	stopWorkers := make([]*clientBusIDWorker, 0)
	for busid, worker := range c.allWorkers {
		if _, ok := desired[busid]; ok || c.isBusIDActive(busid) {
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
	assignmentKeys := c.matchedKeysForAssignmentLocked(entries, knownKeys)
	activeCurrent := c.activeCurrentAssignmentsLocked(c.assigned, assignmentKeys)
	nextAssigned := assignMatchedBusIDsWithRetained(c.targets, c.assigned, entries, assignmentKeys, activeCurrent)
	workers := append([]*clientAssignedWorker(nil), c.assignedWorkers...)
	previous := append([]string(nil), c.assigned...)
	c.assigned = nextAssigned
	c.retainMatchedKnownKeysLocked(assignmentKeys, entries, nextAssigned)
	c.stateMu.Unlock()

	for i, worker := range workers {
		if previous[i] != nextAssigned[i] {
			worker.setDesiredBusID(nextAssigned[i])
		}
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
		if err := ctx.Err(); err != nil {
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
		if err := WriteOpReqImportExt(conn, ImportExtRequest{
			BusID:       busid,
			LeaseID:     lease.ID,
			ClientNonce: lease.ClientNonce,
		}); err != nil {
			return nil, E.Cause(err, "write OP_REQ_IMPORT_EXT")
		}
	} else if err := WriteOpReqImport(conn, busid); err != nil {
		return nil, E.Cause(err, "write OP_REQ_IMPORT")
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Version != ProtocolVersion {
		return nil, E.New("unexpected reply version 0x", hex16(header.Version))
	}
	if header.Code != expectedReply {
		return nil, E.New("unexpected reply code 0x", hex16(header.Code))
	}
	if header.Status != OpStatusOK {
		return nil, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT body")
	}
	controller := newDarwinVirtualController(ctx, c.logger, conn, info)
	if err := controller.Start(); err != nil {
		controller.Close()
		return nil, err
	}
	closeConn = false
	return controller, nil
}

func (c *ClientService) setBusIDActive(busid string, active bool) {
	c.activeMu.Lock()
	defer c.activeMu.Unlock()
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
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if _, registered := c.allWorkers[busid]; !registered {
		return false
	}
	_, desired := c.allDesired[busid]
	return desired
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

	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[uint32]darwinPendingSubmit

	stateMu       sync.Mutex
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
			c.logger.Debug("unexpected USB/IP response 0x", hex32(header.Command))
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
	c.stateMu.Lock()
	address := c.nextAddress
	c.nextAddress++
	c.devices[address] = device
	c.stateMu.Unlock()
	return device.respondCreate(message, ciStatusSuccess, address)
}

func (c *darwinVirtualController) handleDeviceCommand(message darwinCIMessage) error {
	address := message.deviceAddress()
	c.stateMu.Lock()
	device := c.devices[address]
	c.stateMu.Unlock()
	if device == nil {
		return nil
	}
	err := device.respond(message, ciStatusSuccess)
	if message.messageType() == ciMsgDeviceDestroy {
		c.stateMu.Lock()
		delete(c.devices, address)
		c.stateMu.Unlock()
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
	c.stateMu.Lock()
	c.endpoints[key] = endpoint
	c.stateMu.Unlock()
	return endpoint.respond(message, ciStatusSuccess)
}

func (c *darwinVirtualController) handleEndpointCommand(message darwinCIMessage) error {
	key := darwinEndpointKey{device: message.deviceAddress(), endpoint: message.endpointAddress()}
	c.stateMu.Lock()
	endpoint := c.endpoints[key]
	c.stateMu.Unlock()
	if endpoint == nil {
		return nil
	}
	err := endpoint.respond(message, ciStatusSuccess)
	if message.messageType() == ciMsgEndpointDestroy {
		c.stateMu.Lock()
		delete(c.endpoints, key)
		delete(c.controlStates, key.device)
		c.stateMu.Unlock()
		endpoint.Close()
	}
	return err
}

func (c *darwinVirtualController) handleDoorbell(doorbell uint32) {
	key := darwinEndpointKey{
		device:   uint8(doorbell & 0xff),
		endpoint: uint8((doorbell >> 8) & 0xff),
	}
	c.stateMu.Lock()
	endpoint := c.endpoints[key]
	c.stateMu.Unlock()
	if endpoint == nil {
		return
	}
	if err := endpoint.processDoorbell(doorbell); err != nil {
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
		if err := endpoint.complete(transfer, darwinUSBIPStatusToCIStatus(status), length); err != nil {
			c.logger.Debug("complete transfer: ", err)
			c.requestClose()
			return
		}
	}
}

func (c *darwinVirtualController) teardownIOUSBHostState() {
	c.stateMu.Lock()
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
	c.stateMu.Unlock()

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
		c.stateMu.Lock()
		c.controlStates[key.device] = darwinControlState{setup: message.setup()}
		c.stateMu.Unlock()
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
	c.stateMu.Lock()
	state, ok := c.controlStates[key.device]
	c.stateMu.Unlock()
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
	c.stateMu.Lock()
	state, ok := c.controlStates[key.device]
	delete(c.controlStates, key.device)
	c.stateMu.Unlock()
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
	c.pendingMu.Lock()
	c.pending[seq] = darwinPendingSubmit{direction: command.Header.Direction, reply: reply}
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, seq)
		c.pendingMu.Unlock()
	}()
	c.writeMu.Lock()
	err := WriteSubmitCommand(c.conn, command)
	c.writeMu.Unlock()
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
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	pending, ok := c.pending[seq]
	if !ok {
		return 0, false
	}
	return pending.direction, true
}

func (c *darwinVirtualController) deliverSubmit(response SubmitResponse) {
	c.pendingMu.Lock()
	pending := c.pending[response.Header.SeqNum]
	c.pendingMu.Unlock()
	reply := pending.reply
	if reply == nil {
		return
	}
	select {
	case reply <- response:
	default:
	}
}

func (c *darwinVirtualController) failPending() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
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
