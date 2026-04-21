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

type ClientService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	matches    []option.USBIPDeviceMatch // empty = import all remote exports

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

// run resolves the desired busid set (once) and spawns a worker per busid.
func (c *ClientService) run() {
	defer c.wg.Done()
	busids := c.resolveBusIDs()
	if len(busids) == 0 {
		c.logger.Warn("no devices to import; client idle")
		return
	}
	for _, busid := range busids {
		c.wg.Add(1)
		go c.worker(busid)
	}
}

// resolveBusIDs connects once, issues OP_REQ_DEVLIST, and returns the busids
// to attach. For the empty-matches case, returns every remote busid.
// For filtered mode, returns the first match per criterion.
func (c *ClientService) resolveBusIDs() []string {
	// Busid-only matches don't require enumeration.
	if len(c.matches) > 0 && everyMatchBusIDOnly(c.matches) {
		out := make([]string, 0, len(c.matches))
		for _, m := range c.matches {
			out = append(out, m.BusID)
		}
		return dedupe(out)
	}
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
		if len(c.matches) == 0 {
			out := make([]string, 0, len(entries))
			for i := range entries {
				out = append(out, entries[i].Info.BusIDString())
			}
			return out
		}
		var out []string
		for _, m := range c.matches {
			picked := ""
			for i := range entries {
				key := DeviceKey{
					BusID:     entries[i].Info.BusIDString(),
					VendorID:  entries[i].Info.IDVendor,
					ProductID: entries[i].Info.IDProduct,
					Serial:    entries[i].Info.SerialString(),
				}
				if Matches(m, key) {
					picked = key.BusID
					break
				}
			}
			if picked == "" {
				c.logger.Warn("no remote device matched ", describeMatch(m))
				continue
			}
			out = append(out, picked)
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

// worker keeps one remote busid attached to vhci_hcd.0. On any error or
// kernel-side detach, waits clientReconnectDelay and retries.
func (c *ClientService) worker(busid string) {
	defer c.wg.Done()
	for {
		if err := c.ctx.Err(); err != nil {
			return
		}
		port, err := c.attemptAttach(busid)
		if err != nil {
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
		if err := c.ctx.Err(); err != nil {
			return
		}
		c.logger.Info("vhci port ", port, " released; reattaching ", busid)
		if !sleepCtx(c.ctx, clientReconnectDelay) {
			return
		}
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
	success := false
	defer func() {
		if !success {
			conn.Close()
		}
	}()
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
	success = true
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

func everyMatchBusIDOnly(matches []option.USBIPDeviceMatch) bool {
	for _, m := range matches {
		if m.BusID == "" || m.VendorID != 0 || m.ProductID != 0 || m.Serial != "" {
			return false
		}
	}
	return true
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
