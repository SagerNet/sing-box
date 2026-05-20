//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"time"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const (
	clientReconnectDelay         = 5 * time.Second
	controlPingInterval          = 10 * time.Second
	controlReadTimeout           = 30 * time.Second
	controlWriteTimeout          = 5 * time.Second
	controlSessionIdleHint       = "control session lost"
	controlHandshakeBackoffStart = time.Second
	controlHandshakeBackoffMax   = 30 * time.Second
)

var (
	errImmediateReconnect = E.New("usbip control reconnect")
	errControlUnsupported = E.New("usbip control unsupported")
	errControlTransient   = E.New("usbip control transient")
)

type clientAssignedWorker struct {
	target  clientTarget
	updates chan string
}

func (c *ClientService) initializeWorkers() {
	if !c.assignment.Matched() {
		return
	}
	targets := c.assignment.targets
	workers := make([]*clientAssignedWorker, len(targets))
	for i, target := range targets {
		workers[i] = &clientAssignedWorker{
			target:  target,
			updates: make(chan string, 1),
		}
	}
	c.workerAccess.Lock()
	c.assignedWorkers = workers
	c.workerAccess.Unlock()

	for _, worker := range workers {
		go c.runAssignedWorker(worker)
	}
}

func (c *ClientService) run() {
	defer c.stopAllWorkers()

	var transientStreak int
	backoff := controlHandshakeBackoffStart
	immediate := true

	for {
		if !immediate {
			delay := clientReconnectDelay
			if transientStreak > 0 {
				delay = backoff
				backoff *= 2
				if backoff > controlHandshakeBackoffMax {
					backoff = controlHandshakeBackoffMax
				}
			}
			if !sleepCtx(c.ctx, delay) {
				return
			}
		}
		immediate = false

		err := c.runControlSession()
		if c.ctx.Err() != nil {
			return
		}

		if errors.Is(err, errControlUnsupported) {
			c.logger.Info("control channel unsupported by ", c.serverAddr, "; using standard usbip static discovery")
			err = c.runStandardStaticMode()
			if c.ctx.Err() != nil {
				return
			}
			if err != nil {
				c.logger.Error("control ", c.serverAddr, ": ", err)
			}
			transientStreak = 0
			backoff = controlHandshakeBackoffStart
			continue
		}

		if errors.Is(err, errControlTransient) {
			transientStreak++
			c.logger.Warn("control handshake ", c.serverAddr, ": ", err)
			continue
		}

		if err != nil {
			c.logger.Error("control ", c.serverAddr, ": ", err)
		}
		transientStreak = 0
		backoff = controlHandshakeBackoffStart
		immediate = errors.Is(err, errImmediateReconnect)
	}
}

// Dynamic export discovery and hotplug updates are provided by the sing-box
// USB/IP control extensions. Standard USB/IP implementations expose only a
// static DEVLIST snapshot, so we seed assignments once and do not keep polling
// for newly exported devices.
func (c *ClientService) runStandardStaticMode() error {
	err := c.syncRemoteStateContext(c.ctx)
	if err != nil {
		return E.Cause(err, "initial static devlist sync")
	}
	<-c.ctx.Done()
	return nil
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
	_, err = conn.Write(controlPreface[:])
	if err != nil {
		return E.Cause(errControlTransient, "write control preface: ", err)
	}
	err = writeControlMessage(conn, controlFrame{
		Type:    controlFrameHello,
		Version: controlProtocolVersion,
	}, nil)
	if err != nil {
		return E.Cause(errControlTransient, "write control hello: ", err)
	}
	var cr controlReader
	ackMessage, err := cr.read(conn)
	if err != nil {
		// A plain usbipd reads our preface as an op-header, finds a bogus
		// version, and closes cleanly: the client sees io.EOF. That means the
		// peer lacks the sing-box USB/IP control extensions, including dynamic
		// export discovery and hotplug updates. Other I/O errors (timeout, RST,
		// partial read) point at a transient network problem instead.
		if errors.Is(err, io.EOF) {
			return E.Cause(errControlUnsupported, "read control ack: ", err)
		}
		return E.Cause(errControlTransient, "read control ack: ", err)
	}
	if len(ackMessage.Payload) > 0 {
		return E.Cause(errControlUnsupported, "unexpected control ack payload length ", len(ackMessage.Payload))
	}
	ack := ackMessage.Frame
	if ack.Type != controlFrameAck {
		return E.Cause(errControlUnsupported, "unexpected control ack frame ", ack.Type)
	}
	if ack.Version != controlProtocolVersion {
		return E.Cause(errControlUnsupported, "unsupported control version ", ack.Version)
	}
	_ = conn.SetWriteDeadline(time.Time{})
	_ = conn.SetReadDeadline(time.Time{})

	pingDone := make(chan struct{})
	go c.controlPingLoop(conn, pingDone)
	defer close(pingDone)

	var reader controlReader
	for {
		err = conn.SetReadDeadline(time.Now().Add(controlReadTimeout))
		if err != nil {
			return err
		}
		var message controlMessage
		message, err = reader.read(conn)
		if err != nil {
			return E.Cause(errImmediateReconnect, controlSessionIdleHint, ": ", err)
		}
		frame := message.Frame
		switch frame.Type {
		case controlFrameDeviceSnapshot:
			var snapshot controlDeviceSnapshot
			err = unmarshalControlPayload(message.Payload, &snapshot)
			if err != nil {
				return E.Cause(errImmediateReconnect, "read device snapshot: ", err)
			}
			devices := deviceInfoV2Map(snapshot.Devices)
			values := sortedDeviceInfoV2Values(devices)
			c.remoteAccess.Lock()
			c.remoteDevicesV2 = devices
			c.remoteAccess.Unlock()
			c.applyRemoteDeviceState(values)
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
			err := writeControlMessage(conn, controlFrame{
				Type:    controlFramePing,
				Version: controlProtocolVersion,
			}, nil)
			_ = conn.SetWriteDeadline(time.Time{})
			if err != nil {
				_ = conn.Close()
				return
			}
		}
	}
}

func (c *ClientService) syncRemoteStateContext(ctx context.Context) error {
	entries, err := c.fetchDevList(ctx)
	if err != nil {
		return err
	}
	if !c.assignment.Matched() {
		c.applyRemoteExports(entries)
		return nil
	}
	c.applyMatchedExportsWithRetained(entries, nil)
	return nil
}

func (c *ClientService) applyRemoteDeviceState(devices []DeviceInfoV2) {
	availableEntries := deviceInfoV2ToEntries(devices, true)
	if !c.assignment.Matched() {
		c.applyRemoteExports(availableEntries)
		return
	}
	knownKeys := make(map[string]DeviceKey, len(devices))
	for _, device := range devices {
		if device.BusID == "" {
			continue
		}
		knownKeys[device.BusID] = DeviceKey{
			BusID:     device.BusID,
			VendorID:  device.VendorID,
			ProductID: device.ProductID,
			Serial:    device.Serial,
		}
	}
	c.applyMatchedExportsWithRetained(availableEntries, knownKeys)
}

func (c *ClientService) applyRemoteExports(entries []DeviceEntry) {
	start, stop := c.assignment.ApplyAll(entries)

	c.workerAccess.Lock()
	stopCancels := make([]context.CancelFunc, 0, len(stop))
	for _, busid := range stop {
		cancel, ok := c.allWorkers[busid]
		if !ok {
			continue
		}
		stopCancels = append(stopCancels, cancel)
		delete(c.allWorkers, busid)
	}
	c.workerAccess.Unlock()

	for _, cancel := range stopCancels {
		cancel()
	}
	slices.Sort(start)
	for _, busid := range start {
		c.startRemoteBusIDWorker(busid, busid)
	}
}

func (c *ClientService) applyMatchedExportsWithRetained(entries []DeviceEntry, knownKeys map[string]DeviceKey) {
	next, previous := c.assignment.ApplyMatched(entries, knownKeys)
	if next == nil {
		return
	}
	c.workerAccess.Lock()
	workers := append([]*clientAssignedWorker(nil), c.assignedWorkers...)
	c.workerAccess.Unlock()
	for i, worker := range workers {
		if previous[i] == next[i] {
			continue
		}
		worker.setDesiredBusID(next[i])
	}
}

func (c *ClientService) runAssignedWorker(worker *clientAssignedWorker) {
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

			match := worker.target.match
			if worker.target.fixedBusID != "" {
				match = option.USBIPDeviceMatch{BusID: worker.target.fixedBusID}
			}
			go func(busid, description string) {
				defer close(done)
				c.runBusIDLoop(runCtx, busid, description)
			}(desired, describeMatch(match))
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

	c.workerAccess.Lock()
	c.allWorkers[busid] = cancel
	c.workerAccess.Unlock()

	go func() {
		c.runBusIDLoop(runCtx, busid, description)
	}()
}

func (c *ClientService) stopAllWorkers() {
	c.assignment.access.Lock()
	c.assignment.registered = make(map[string]struct{})
	c.assignment.access.Unlock()

	c.workerAccess.Lock()
	cancels := make([]context.CancelFunc, 0, len(c.allWorkers))
	for _, cancel := range c.allWorkers {
		cancels = append(cancels, cancel)
	}
	c.allWorkers = make(map[string]context.CancelFunc)
	c.workerAccess.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (c *ClientService) fetchDevList(ctx context.Context) ([]DeviceEntry, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()
	err = WriteOpHeader(conn, OpReqDevList, OpStatusOK)
	if err != nil {
		return nil, E.Cause(err, "send OP_REQ_DEVLIST")
	}
	var header OpHeader
	header, err = ReadOpHeader(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_DEVLIST header")
	}
	if header.Version != ProtocolVersion {
		return nil, E.New("unexpected reply version ", fmt.Sprintf("0x%04x", header.Version))
	}
	if header.Code != OpRepDevList || header.Status != OpStatusOK {
		return nil, E.New("OP_REP_DEVLIST status=", header.Status, " code=", fmt.Sprintf("0x%04x", header.Code))
	}
	return ReadOpRepDevListBody(conn)
}
