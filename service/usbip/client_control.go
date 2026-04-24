//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

var errClientControlSessionClosed = errors.New("usbip control session closed")

type clientImportLease struct {
	Valid       bool
	ID          uint64
	ClientNonce uint64
}

type clientControlSession struct {
	conn         net.Conn
	capabilities uint32
	writeMu      sync.Mutex
	mu           sync.Mutex
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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
	err := writeControlMessage(s.conn, frame, payload)
	_ = s.conn.SetWriteDeadline(time.Time{})
	return err
}

func (s *clientControlSession) requestLease(ctx context.Context, busid string) (controlLeaseResponse, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return controlLeaseResponse{}, errClientControlSessionClosed
	}
	s.nextNonce++
	nonce := s.nextNonce
	waiter := make(chan clientLeaseResult, 1)
	s.pending[nonce] = waiter
	s.mu.Unlock()

	request := controlLeaseRequest{
		BusID:       busid,
		ClientNonce: nonce,
	}
	if err := s.writeControl(controlFrame{
		Type:    controlFrameLeaseRequest,
		Version: controlProtocolVersion,
	}, request); err != nil {
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
	s.mu.Lock()
	waiter, ok := s.pending[response.ClientNonce]
	if ok {
		delete(s.pending, response.ClientNonce)
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	waiter <- clientLeaseResult{response: response}
	close(waiter)
	return true
}

func (s *clientControlSession) removeLeaseWaiter(nonce uint64) {
	s.mu.Lock()
	delete(s.pending, nonce)
	s.mu.Unlock()
}

func (s *clientControlSession) closeWithError(err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	pending := s.pending
	s.pending = make(map[uint64]chan clientLeaseResult)
	s.mu.Unlock()

	for _, waiter := range pending {
		waiter <- clientLeaseResult{err: err}
		close(waiter)
	}
}

func (c *ClientService) setControlSession(session *clientControlSession) {
	c.controlMu.Lock()
	c.controlSession = session
	c.controlMu.Unlock()
}

func (c *ClientService) clearControlSession(session *clientControlSession, err error) {
	c.controlMu.Lock()
	if c.controlSession == session {
		c.controlSession = nil
	}
	c.controlMu.Unlock()
	session.closeWithError(err)
}

func (c *ClientService) currentControlSession() *clientControlSession {
	c.controlMu.Lock()
	defer c.controlMu.Unlock()
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

func (c *ClientService) applyControlSnapshot(snapshot controlDeviceSnapshot) {
	devices := deviceInfoV2Map(snapshot.Devices)
	values := sortedDeviceInfoV2Values(devices)
	c.remoteMu.Lock()
	c.remoteDevicesV2 = devices
	c.remoteMu.Unlock()
	c.applyRemoteEntries(deviceInfoV2ToEntries(values, true))
}

func (c *ClientService) applyControlDelta(delta controlDeviceDelta) {
	c.remoteMu.Lock()
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
	c.remoteMu.Unlock()
	c.applyRemoteEntries(deviceInfoV2ToEntries(values, true))
}

func (c *ClientService) clearControlDeviceState() {
	c.remoteMu.Lock()
	c.remoteDevicesV2 = nil
	c.remoteMu.Unlock()
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
		device := deviceInfoV2FromEntry(entry, "", "", deviceStateAvailable, 0, "available")
		if device.BusID == "" {
			continue
		}
		devices[device.BusID] = device
	}
	c.remoteMu.Lock()
	c.remoteDevicesV2 = devices
	c.remoteMu.Unlock()
}
