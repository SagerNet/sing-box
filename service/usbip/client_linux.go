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
	controlPingInterval    = 10 * time.Second
	controlReadTimeout     = 30 * time.Second
	controlWriteTimeout    = 5 * time.Second
	controlSessionIdleHint = "control session lost"
)

var errImmediateReconnect = errors.New("usbip control reconnect")

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

	stateMu         sync.Mutex
	targets         []clientTarget
	assigned        []string
	assignedWorkers []*clientAssignedWorker
	allWorkers      map[string]*clientBusIDWorker

	attachMu sync.Mutex // serializes vhci port pick + attach
	wg       sync.WaitGroup

	portsMu sync.Mutex
	ports   map[int]struct{}
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
		Adapter:    boxService.NewAdapter(C.TypeUSBIPClient, tag),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		dialer:     outboundDialer,
		serverAddr: options.ServerOptions.Build(),
		matches:    options.Devices,
		allWorkers: make(map[string]*clientBusIDWorker),
		ports:      make(map[int]struct{}),
	}, nil
}

func (c *ClientService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	if err := ensureVHCI(); err != nil {
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
	case <-time.After(5 * time.Second):
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
	immediate := true
	for {
		if !immediate && !sleepCtx(c.ctx, clientReconnectDelay) {
			break
		}
		err := c.runControlSession()
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
		return E.Cause(err, "write control preface")
	}
	if err := WriteControlHello(conn); err != nil {
		return E.Cause(err, "write control hello")
	}
	ack, err := ReadControlFrame(conn)
	if err != nil {
		return E.Cause(err, "read control ack")
	}
	if ack.Type != controlFrameAck {
		return E.New("unexpected control ack frame ", ack.Type)
	}
	if ack.Version != controlProtocolVersion {
		return E.New("unsupported control version ", ack.Version)
	}
	if ack.Capabilities&controlCapabilities != controlCapabilities {
		return E.New("missing control capabilities 0x", ack.Capabilities)
	}
	_ = conn.SetWriteDeadline(time.Time{})
	_ = conn.SetReadDeadline(time.Time{})

	if err := c.syncRemoteState(); err != nil {
		return E.Cause(err, "initial devlist sync")
	}

	pingDone := make(chan struct{})
	go c.controlPingLoop(conn, pingDone)
	defer close(pingDone)

	lastSeq := ack.Sequence
	for {
		if err := conn.SetReadDeadline(time.Now().Add(controlReadTimeout)); err != nil {
			return err
		}
		frame, err := ReadControlFrame(conn)
		if err != nil {
			return E.Cause(errImmediateReconnect, controlSessionIdleHint, ": ", err)
		}
		switch frame.Type {
		case controlFrameChanged:
			if frame.Sequence != lastSeq+1 {
				return E.Cause(errImmediateReconnect, "control sequence jumped from ", lastSeq, " to ", frame.Sequence)
			}
			lastSeq = frame.Sequence
			if err := c.syncRemoteState(); err != nil {
				return E.Cause(errImmediateReconnect, "devlist sync after change ", frame.Sequence, ": ", err)
			}
		case controlFramePong:
		default:
			return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
		}
	}
}

func (c *ClientService) controlPingLoop(conn net.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(controlPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
			if err := WriteControlPing(conn); err != nil {
				_ = conn.Close()
				return
			}
			_ = conn.SetWriteDeadline(time.Time{})
		}
	}
}

func (c *ClientService) syncRemoteState() error {
	entries, err := c.fetchDevList(c.ctx)
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
	stopWorkers := make([]*clientBusIDWorker, 0)
	for busid, worker := range c.allWorkers {
		if _, ok := desired[busid]; ok {
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
	keysByBusID := make(map[string]DeviceKey, len(entries))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		keysByBusID[busid] = DeviceKey{
			BusID:     busid,
			VendorID:  entries[i].Info.IDVendor,
			ProductID: entries[i].Info.IDProduct,
			Serial:    entries[i].Info.SerialString(),
		}
	}

	c.stateMu.Lock()
	if len(c.targets) == 0 {
		c.stateMu.Unlock()
		return
	}

	nextAssigned := make([]string, len(c.targets))
	reserved := make(map[string]struct{}, len(c.targets))
	for i, target := range c.targets {
		if target.fixedBusID == "" {
			continue
		}
		if _, ok := keysByBusID[target.fixedBusID]; !ok {
			continue
		}
		nextAssigned[i] = target.fixedBusID
		reserved[target.fixedBusID] = struct{}{}
	}
	for i, target := range c.targets {
		if target.fixedBusID != "" {
			continue
		}
		current := c.assigned[i]
		if current == "" {
			continue
		}
		if _, ok := reserved[current]; ok {
			continue
		}
		key, ok := keysByBusID[current]
		if !ok || !Matches(target.match, key) {
			continue
		}
		nextAssigned[i] = current
		reserved[current] = struct{}{}
	}
	for i, target := range c.targets {
		if target.fixedBusID != "" || nextAssigned[i] != "" {
			continue
		}
		nextAssigned[i] = firstMatchingUnclaimedBusID(target.match, entries, reserved)
		if nextAssigned[i] != "" {
			reserved[nextAssigned[i]] = struct{}{}
		}
	}

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
		c.trackPort(port, true)
		c.watchPort(ctx, port, busid)
		c.trackPort(port, false)
		if err := ctx.Err(); err != nil {
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
	defer conn.Close()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()
	if err := WriteOpReqImport(conn, busid); err != nil {
		return -1, E.Cause(err, "write OP_REQ_IMPORT")
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return -1, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Code != OpRepImport {
		return -1, E.New("unexpected reply code 0x", hex16(header.Code))
	}
	if header.Status != OpStatusOK {
		return -1, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return -1, E.Cause(err, "read OP_REP_IMPORT body")
	}
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return -1, E.New("dialed conn is not *net.TCPConn (type=", conn, ")")
	}
	file, err := tcp.File()
	if err != nil {
		return -1, E.Cause(err, "dup socket fd")
	}
	defer file.Close()
	c.attachMu.Lock()
	defer c.attachMu.Unlock()
	port, err := vhciPickFreePort(info.Speed)
	if err != nil {
		return -1, err
	}
	if err := vhciAttach(port, file.Fd(), info.DevID(), info.Speed); err != nil {
		return -1, E.Cause(err, "vhci attach")
	}
	return port, nil
}

func (c *ClientService) watchPort(ctx context.Context, port int, busid string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := vhciDetach(port); err != nil {
				c.logger.Warn("detach port ", port, " (", busid, "): ", err)
			}
			return
		case <-ticker.C:
			used, err := vhciPortUsed(port)
			if err != nil {
				c.logger.Debug("poll port ", port, ": ", err)
				continue
			}
			if !used {
				return
			}
		}
	}
}

func (c *ClientService) trackPort(port int, add bool) {
	c.portsMu.Lock()
	defer c.portsMu.Unlock()
	if add {
		c.ports[port] = struct{}{}
	} else {
		delete(c.ports, port)
	}
}

func isBusIDOnlyMatch(m option.USBIPDeviceMatch) bool {
	return m.BusID != "" && m.VendorID == 0 && m.ProductID == 0 && m.Serial == ""
}

func firstMatchingUnclaimedBusID(match option.USBIPDeviceMatch, entries []DeviceEntry, reserved map[string]struct{}) string {
	for i := range entries {
		key := DeviceKey{
			BusID:     entries[i].Info.BusIDString(),
			VendorID:  entries[i].Info.IDVendor,
			ProductID: entries[i].Info.IDProduct,
			Serial:    entries[i].Info.SerialString(),
		}
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
