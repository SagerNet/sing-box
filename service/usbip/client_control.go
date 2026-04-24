//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"net"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

var errClientControlSessionClosed = E.New("usbip control session closed")

type clientImportLease struct {
	Valid       bool
	ID          uint64
	ClientNonce uint64
}

type clientControlSession struct {
	conn         net.Conn
	capabilities uint32
	writeAccess  sync.Mutex
	access       sync.Mutex
	nextNonce    uint64
	pending      map[uint64]chan clientLeaseResult
	closed       bool
}

type clientLeaseResult struct {
	response controlLeaseResponse
	err      error
}

func newClientControlSession(conn net.Conn, capabilities uint32) *clientControlSession {
	return &clientControlSession{
		conn:         conn,
		capabilities: capabilities,
		pending:      make(map[uint64]chan clientLeaseResult),
	}
}

func (s *clientControlSession) supportsImportLease() bool {
	return supportsControlExtensions(s.capabilities)
}

func (s *clientControlSession) writeControl(frame controlFrame, payload any) error {
	s.writeAccess.Lock()
	defer s.writeAccess.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
	err := writeControlMessage(s.conn, frame, payload)
	_ = s.conn.SetWriteDeadline(time.Time{})
	return err
}

func (s *clientControlSession) requestLease(ctx context.Context, busid string) (controlLeaseResponse, error) {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return controlLeaseResponse{}, errClientControlSessionClosed
	}
	s.nextNonce++
	nonce := s.nextNonce
	waiter := make(chan clientLeaseResult, 1)
	s.pending[nonce] = waiter
	s.access.Unlock()

	request := controlLeaseRequest{
		BusID:       busid,
		ClientNonce: nonce,
	}
	err := s.writeControl(controlFrame{
		Type:    controlFrameLeaseRequest,
		Version: controlProtocolVersion,
	}, request)
	if err != nil {
		s.removeLeaseWaiter(nonce)
		return controlLeaseResponse{}, err
	}

	select {
	case result := <-waiter:
		return result.response, result.err
	case <-ctx.Done():
		s.removeLeaseWaiter(nonce)
		return controlLeaseResponse{}, ctx.Err()
	}
}

func (s *clientControlSession) deliverLeaseResponse(response controlLeaseResponse) bool {
	s.access.Lock()
	waiter, ok := s.pending[response.ClientNonce]
	if ok {
		delete(s.pending, response.ClientNonce)
	}
	s.access.Unlock()
	if !ok {
		return false
	}
	waiter <- clientLeaseResult{response: response}
	close(waiter)
	return true
}

func (s *clientControlSession) removeLeaseWaiter(nonce uint64) {
	s.access.Lock()
	delete(s.pending, nonce)
	s.access.Unlock()
}

func (s *clientControlSession) closeWithError(err error) {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return
	}
	s.closed = true
	pending := s.pending
	s.pending = make(map[uint64]chan clientLeaseResult)
	s.access.Unlock()

	for _, waiter := range pending {
		waiter <- clientLeaseResult{err: err}
		close(waiter)
	}
}

func (c *ClientService) setControlSession(session *clientControlSession) {
	c.controlAccess.Lock()
	c.controlSession = session
	c.controlAccess.Unlock()
}

func (c *ClientService) clearControlSession(session *clientControlSession, err error) {
	c.controlAccess.Lock()
	if c.controlSession == session {
		c.controlSession = nil
	}
	c.controlAccess.Unlock()
	session.closeWithError(err)
}

func (c *ClientService) currentControlSession() *clientControlSession {
	c.controlAccess.Lock()
	defer c.controlAccess.Unlock()
	return c.controlSession
}

func (c *ClientService) requestImportLease(ctx context.Context, busid string) (clientImportLease, error) {
	session := c.currentControlSession()
	if session == nil || !session.supportsImportLease() {
		return clientImportLease{}, nil
	}
	response, err := session.requestLease(ctx, busid)
	if err != nil {
		return clientImportLease{}, E.Cause(err, "request import lease")
	}
	if response.ErrorCode != "" {
		return clientImportLease{}, E.New("remote rejected import lease (", response.ErrorCode, ": ", response.ErrorMessage, ")")
	}
	return clientImportLease{
		Valid:       true,
		ID:          response.LeaseID,
		ClientNonce: response.ClientNonce,
	}, nil
}

func (c *ClientService) runStandardSessionWithInterval(interval time.Duration) error {
	for {
		err := c.syncRemoteStateContext(c.ctx)
		if err != nil {
			return E.Cause(err, "devlist sync")
		}
		if !sleepCtx(c.ctx, interval) {
			return nil
		}
	}
}

func (c *ClientService) applyControlSnapshot(snapshot controlDeviceSnapshot) {
	devices := deviceInfoV2Map(snapshot.Devices)
	values := sortedDeviceInfoV2Values(devices)
	c.remoteAccess.Lock()
	c.remoteDevicesV2 = devices
	c.remoteAccess.Unlock()
	c.applyRemoteDeviceState(values)
}

func (c *ClientService) applyControlDelta(delta controlDeviceDelta) {
	c.remoteAccess.Lock()
	if c.remoteDevicesV2 == nil {
		c.remoteDevicesV2 = make(map[string]DeviceInfoV2)
	}
	for _, busid := range delta.Removed {
		delete(c.remoteDevicesV2, busid)
	}
	for _, device := range delta.Added {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	for _, device := range delta.Updated {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	values := sortedDeviceInfoV2Values(c.remoteDevicesV2)
	c.remoteAccess.Unlock()
	c.applyRemoteDeviceState(values)
}

func (c *ClientService) clearControlDeviceState() {
	c.remoteAccess.Lock()
	c.remoteDevicesV2 = nil
	c.remoteAccess.Unlock()
}

func (c *ClientService) syncRemoteStateAndResetControlState(ctx context.Context) error {
	entries, err := c.fetchDevList(ctx)
	if err != nil {
		return err
	}
	c.resetControlDeviceStateFromEntries(entries)
	c.applyRemoteEntries(entries)
	return nil
}

func (c *ClientService) resetControlDeviceStateFromEntries(entries []DeviceEntry) {
	devices := make(map[string]DeviceInfoV2, len(entries))
	for _, entry := range entries {
		device := deviceInfoV2FromEntry(entry, "", "", deviceStateAvailable, 0, deviceStateAvailable)
		if device.BusID == "" {
			continue
		}
		devices[device.BusID] = device
	}
	c.remoteAccess.Lock()
	c.remoteDevicesV2 = devices
	c.remoteAccess.Unlock()
}

func (c *ClientService) matchedKeysForAssignmentLocked(entries []DeviceEntry, knownKeys map[string]DeviceKey) map[string]DeviceKey {
	if len(c.matchedKnownKeys) == 0 && len(entries) == 0 && len(knownKeys) == 0 {
		return nil
	}
	assignmentKeys := make(map[string]DeviceKey, len(c.matchedKnownKeys)+len(entries)+len(knownKeys))
	for busid, key := range c.matchedKnownKeys {
		assignmentKeys[busid] = key
	}
	for i := range entries {
		key := entryDeviceKey(entries[i])
		if key.BusID == "" {
			continue
		}
		assignmentKeys[key.BusID] = key
	}
	for busid, key := range knownKeys {
		if busid == "" {
			continue
		}
		assignmentKeys[busid] = key
	}
	return assignmentKeys
}

func (c *ClientService) retainMatchedKnownKeysLocked(assignmentKeys map[string]DeviceKey, entries []DeviceEntry, assigned []string) {
	if len(assignmentKeys) == 0 {
		c.matchedKnownKeys = nil
		return
	}
	retained := make(map[string]DeviceKey, len(entries)+len(assigned))
	for i := range entries {
		busid := entries[i].Info.BusIDString()
		if busid == "" {
			continue
		}
		if key, ok := assignmentKeys[busid]; ok {
			retained[busid] = key
		}
	}
	for _, busid := range assigned {
		if busid == "" {
			continue
		}
		if key, ok := assignmentKeys[busid]; ok {
			retained[busid] = key
		}
	}
	if len(retained) == 0 {
		c.matchedKnownKeys = nil
		return
	}
	c.matchedKnownKeys = retained
}
