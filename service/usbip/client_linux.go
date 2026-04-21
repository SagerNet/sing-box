//go:build linux

package usbip

import (
	"context"
	"encoding/binary"
	"net"
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

const clientReconnectDelay = 5 * time.Second

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

type ClientService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	matches    []option.USBIPDeviceMatch // empty = import all remote exports

	assignMu sync.Mutex
	targets  []clientTarget
	assigned []string

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
	c.wg.Add(1)
	go c.run()
	return nil
}

func (c *ClientService) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	// Wait for workers to detach, bounded by 5s.
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

// run prepares the desired targets and spawns one worker per target.
func (c *ClientService) run() {
	defer c.wg.Done()
	targets := c.buildTargets()
	if len(targets) == 0 {
		c.logger.Warn("no devices to import; client idle")
		return
	}
	c.assignMu.Lock()
	c.targets = targets
	c.assigned = make([]string, len(targets))
	for i := range targets {
		c.assigned[i] = targets[i].fixedBusID
	}
	c.assignMu.Unlock()
	for i := range targets {
		c.wg.Add(1)
		go c.worker(i)
	}
}

func (c *ClientService) buildTargets() []clientTarget {
	if len(c.matches) == 0 {
		busids := c.snapshotRemoteBusIDs()
		targets := make([]clientTarget, 0, len(busids))
		for _, busid := range busids {
			targets = append(targets, clientTarget{fixedBusID: busid})
		}
		return targets
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

// snapshotRemoteBusIDs connects once, issues OP_REQ_DEVLIST, and returns the
// currently exported remote busids.
func (c *ClientService) snapshotRemoteBusIDs() []string {
	for {
		if err := c.ctx.Err(); err != nil {
			return nil
		}
		entries, err := c.fetchDevList()
		if err != nil {
			c.logger.Error("enumerate ", c.serverAddr, ": ", err)
			if !sleepCtx(c.ctx, clientReconnectDelay) {
				return nil
			}
			continue
		}
		out := make([]string, 0, len(entries))
		for i := range entries {
			out = append(out, entries[i].Info.BusIDString())
		}
		return dedupe(out)
	}
}

func (c *ClientService) fetchDevList() ([]DeviceEntry, error) {
	conn, err := c.dialer.DialContext(c.ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := binary.Write(conn, binary.BigEndian, OpHeader{Version: ProtocolVersion, Code: OpReqDevList, Status: OpStatusOK}); err != nil {
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

// worker keeps one target attached to vhci_hcd.0. On any error or kernel-side
// detach, waits clientReconnectDelay and retries.
func (c *ClientService) worker(targetIndex int) {
	defer c.wg.Done()
	target := c.targets[targetIndex]
	for {
		if err := c.ctx.Err(); err != nil {
			return
		}
		busid, err := c.claimTargetBusID(targetIndex)
		if err != nil {
			c.logger.Error("assign ", target.description(), ": ", err)
			if !sleepCtx(c.ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		if busid == "" {
			if !sleepCtx(c.ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		port, err := c.attemptAttach(busid)
		if err != nil {
			c.releaseTargetBusID(targetIndex, busid)
			c.logger.Error("attach ", busid, ": ", err)
			if !sleepCtx(c.ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		c.logger.Info("attached ", busid, " → vhci port ", port)
		c.trackPort(port, true)
		c.watchPort(port, busid)
		c.trackPort(port, false)
		c.releaseTargetBusID(targetIndex, busid)
		if err := c.ctx.Err(); err != nil {
			return
		}
		c.logger.Info("vhci port ", port, " released; reattaching ", busid)
		if !sleepCtx(c.ctx, clientReconnectDelay) {
			return
		}
	}
}

func (c *ClientService) claimTargetBusID(targetIndex int) (string, error) {
	target := c.targets[targetIndex]
	if target.fixedBusID != "" {
		return target.fixedBusID, nil
	}
	c.assignMu.Lock()
	current := c.assigned[targetIndex]
	c.assignMu.Unlock()
	if current != "" {
		return current, nil
	}
	entries, err := c.fetchDevList()
	if err != nil {
		return "", err
	}
	return c.refreshAssignments(targetIndex, entries), nil
}

func (c *ClientService) refreshAssignments(targetIndex int, entries []DeviceEntry) string {
	c.assignMu.Lock()
	defer c.assignMu.Unlock()
	if c.assigned[targetIndex] != "" {
		return c.assigned[targetIndex]
	}
	reserved := make(map[string]struct{}, len(c.assigned))
	for _, busid := range c.assigned {
		if busid == "" {
			continue
		}
		reserved[busid] = struct{}{}
	}
	for i, target := range c.targets {
		if target.fixedBusID != "" || c.assigned[i] != "" {
			continue
		}
		busid := firstMatchingUnclaimedBusID(target.match, entries, reserved)
		if busid == "" {
			continue
		}
		c.assigned[i] = busid
		reserved[busid] = struct{}{}
	}
	return c.assigned[targetIndex]
}

func (c *ClientService) releaseTargetBusID(targetIndex int, busid string) {
	if c.targets[targetIndex].fixedBusID != "" {
		return
	}
	c.assignMu.Lock()
	defer c.assignMu.Unlock()
	if c.assigned[targetIndex] == busid {
		c.assigned[targetIndex] = ""
	}
}

// attemptAttach performs one dial → OP_REQ_IMPORT → vhci attach sequence.
// The returned TCP socket is handed to the kernel on success; on failure the
// connection is closed before return.
func (c *ClientService) attemptAttach(busid string) (int, error) {
	conn, err := c.dialer.DialContext(c.ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return -1, E.Cause(err, "dial ", c.serverAddr)
	}
	defer conn.Close()
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

// watchPort polls vhci status every 2s and returns when the port is no longer
// in VDEV_ST_USED, or when ctx is canceled (in which case it detaches the port).
func (c *ClientService) watchPort(port int, busid string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
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
