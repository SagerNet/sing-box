//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

const (
	clientReconnectDelay   = 5 * time.Second
	clientShutdownTimeout  = 15 * time.Second
	controlPingInterval    = 10 * time.Second
	controlReadTimeout     = 30 * time.Second
	controlWriteTimeout    = 5 * time.Second
	controlSessionIdleHint = "control session lost"
)

var (
	errImmediateReconnect = E.New("usbip control reconnect")
	errControlUnsupported = E.New("usbip control unsupported")
)

type clientAssignedWorker struct {
	target  clientTarget
	updates chan string
}

type clientBusIDWorker struct {
	cancel context.CancelFunc
}

func (c *ClientService) initializeWorkers() {
	if !c.assignment.Matched() {
		return
	}
	targets := c.assignment.Targets()
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
		return c.runStandardSessionWithInterval(clientReconnectDelay)
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
	err = WriteControlPreface(conn)
	if err != nil {
		return E.Cause(errControlUnsupported, "write control preface: ", err)
	}
	err = WriteControlHello(conn)
	if err != nil {
		return E.Cause(errControlUnsupported, "write control hello: ", err)
	}
	var ack controlFrame
	ack, err = ReadControlFrame(conn)
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
	} else {
		err = c.syncRemoteStateContext(c.ctx)
		if err != nil {
			return E.Cause(err, "initial devlist sync")
		}
	}

	pingDone := make(chan struct{})
	go c.controlPingLoop(session, pingDone)
	defer close(pingDone)

	lastSeq := ack.Sequence
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
		case controlFrameChanged:
			if frame.Sequence != lastSeq && frame.Sequence != lastSeq+1 {
				return E.Cause(errImmediateReconnect, "control sequence jumped from ", lastSeq, " to ", frame.Sequence)
			}
			lastSeq = frame.Sequence
			if extended {
				err = c.syncRemoteStateAndResetControlState(c.ctx)
			} else {
				err = c.syncRemoteStateContext(c.ctx)
			}
			if err != nil {
				return E.Cause(errImmediateReconnect, "devlist sync after change ", frame.Sequence, ": ", err)
			}
		case controlFrameDeviceSnapshot:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			var snapshot controlDeviceSnapshot
			err = unmarshalControlPayload(message.Payload, &snapshot)
			if err != nil {
				return E.Cause(errImmediateReconnect, "read device snapshot: ", err)
			}
			lastSeq = frame.Sequence
			c.applyControlSnapshot(snapshot)
		case controlFrameDeviceDelta:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			if frame.Sequence != lastSeq+1 {
				err = c.syncRemoteStateAndResetControlState(c.ctx)
				if err != nil {
					return E.Cause(errImmediateReconnect, "devlist sync after sequence jump ", frame.Sequence, ": ", err)
				}
				lastSeq = frame.Sequence
				continue
			}
			var delta controlDeviceDelta
			err = unmarshalControlPayload(message.Payload, &delta)
			if err != nil {
				return E.Cause(errImmediateReconnect, "read device delta: ", err)
			}
			lastSeq = frame.Sequence
			c.applyControlDelta(delta)
		case controlFrameLeaseResponse:
			if !extended {
				return E.Cause(errImmediateReconnect, "unexpected control frame ", frame.Type)
			}
			var response controlLeaseResponse
			err = unmarshalControlPayload(message.Payload, &response)
			if err != nil {
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
			err := session.writeControl(controlFrame{
				Type:    controlFramePing,
				Version: controlProtocolVersion,
			}, nil)
			if err != nil {
				_ = session.conn.Close()
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
	c.applyRemoteEntries(entries)
	return nil
}

func (c *ClientService) applyRemoteEntries(entries []DeviceEntry) {
	if !c.assignment.Matched() {
		c.applyRemoteExports(entries)
		return
	}
	c.applyMatchedExportsWithRetained(entries, nil)
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
		knownKeys[device.BusID] = device.key()
	}
	c.applyMatchedExportsWithRetained(availableEntries, knownKeys)
}

func (c *ClientService) applyRemoteExports(entries []DeviceEntry) {
	start, stop := c.assignment.ApplyAll(entries)

	c.workerAccess.Lock()
	stopWorkers := make([]*clientBusIDWorker, 0, len(stop))
	for _, busid := range stop {
		worker, ok := c.allWorkers[busid]
		if !ok {
			continue
		}
		stopWorkers = append(stopWorkers, worker)
		delete(c.allWorkers, busid)
	}
	c.workerAccess.Unlock()

	for _, worker := range stopWorkers {
		worker.cancel()
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

	c.workerAccess.Lock()
	c.allWorkers[busid] = worker
	c.workerAccess.Unlock()

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.runBusIDLoop(runCtx, busid, description)
	}()
}

func (c *ClientService) stopAllWorkers() {
	c.assignment.ClearRegistered()

	c.workerAccess.Lock()
	workers := make([]*clientBusIDWorker, 0, len(c.allWorkers))
	for _, worker := range c.allWorkers {
		workers = append(workers, worker)
	}
	c.allWorkers = make(map[string]*clientBusIDWorker)
	c.workerAccess.Unlock()

	for _, worker := range workers {
		worker.cancel()
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
		return nil, E.New(fmt.Sprintf("unexpected reply version 0x%04x", header.Version))
	}
	if header.Code != OpRepDevList || header.Status != OpStatusOK {
		return nil, E.New(fmt.Sprintf("OP_REP_DEVLIST status=%d code=0x%04x", header.Status, header.Code))
	}
	return ReadOpRepDevListBody(conn)
}

func (c *ClientService) setBusIDActive(busid string, active bool) {
	c.assignment.SetActive(busid, active)
}

func (c *ClientService) isBusIDActive(busid string) bool {
	return c.assignment.IsActive(busid)
}

func (c *ClientService) shouldRetryBusID(ctx context.Context, busid string) bool {
	if c.assignment.Matched() {
		return true
	}
	err := c.syncRemoteStateContext(ctx)
	if err != nil {
		c.logger.Warn("refresh remote exports after releasing ", busid, ": ", err)
		return true
	}
	return c.assignment.IsRetryDesired(busid)
}
