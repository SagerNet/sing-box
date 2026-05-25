//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ DataSession = (*userspaceURBSession)(nil)

type userspaceURBSession struct {
	ctx    context.Context
	logger log.ContextLogger
	conn   net.Conn
	engine URBEngine

	writeAccess sync.Mutex
	access      sync.Mutex
	pending     map[uint32]userspaceSubmitState
	endpoints   map[uint8]*userspaceEndpointState
	wg          sync.WaitGroup

	done     chan struct{}
	doneOnce sync.Once
	runErr   error

	stateAccess sync.Mutex
	started     bool
	closed      bool
	closeOnce   sync.Once
	closeErr    error
}

type userspaceSubmitState struct {
	command  SubmitCommand
	endpoint uint8
	started  bool
	unlinked bool
	drained  chan struct{}
}

type userspaceEndpointState struct {
	active uint32
	queued []uint32
}

type userspaceNextSubmit struct {
	sequence uint32
	command  SubmitCommand
}

func newUserspaceURBSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, engine URBEngine) *userspaceURBSession {
	return &userspaceURBSession{
		ctx:       ctx,
		logger:    logger,
		conn:      conn,
		engine:    engine,
		pending:   make(map[uint32]userspaceSubmitState),
		endpoints: make(map[uint8]*userspaceEndpointState),
		done:      make(chan struct{}),
	}
}

func (s *userspaceURBSession) Done() <-chan struct{} {
	return s.done
}

func (s *userspaceURBSession) Err() error {
	return s.runErr
}

func (s *userspaceURBSession) Start() error {
	s.stateAccess.Lock()
	defer s.stateAccess.Unlock()
	if s.started || s.closed {
		return nil
	}
	s.started = true
	go s.run()
	return nil
}

func (s *userspaceURBSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = common.Close(s.conn)
	})
	s.stateAccess.Lock()
	started := s.started
	s.closed = true
	s.stateAccess.Unlock()
	if started {
		<-s.done
	} else {
		s.markDone(nil)
	}
	_ = s.engine.Close()
	return s.closeErr
}

func (s *userspaceURBSession) markDone(err error) {
	s.doneOnce.Do(func() {
		s.runErr = err
		close(s.done)
	})
}

func (s *userspaceURBSession) run() {
	err := s.serve()
	if err != nil && (errors.Is(err, io.EOF) || E.IsClosedOrCanceled(err)) {
		err = nil
	}
	s.markDone(err)
}

func (s *userspaceURBSession) serve() error {
	stopCloseOnCancel := closeConnOnContextDone(s.ctx, s.conn)
	defer stopCloseOnCancel()
	defer func() {
		s.abortPendingSubmits()
		s.wg.Wait()
	}()
	for {
		header, err := ReadDataHeader(s.conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch header.Command {
		case CmdSubmit:
			command, err := ReadSubmitCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			next, shouldStart := s.enqueueSubmit(command)
			if shouldStart {
				s.startSubmit(next)
			}
		case CmdUnlink:
			command, err := ReadUnlinkCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			status := int32(0)
			endpoint, drained, shouldAbort, found := s.unlinkSubmit(command.SeqNum)
			if found {
				if shouldAbort {
					abortErr := s.engine.AbortEndpoint(endpoint)
					if abortErr != nil {
						s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", abortErr)
					}
				}
				<-drained
				status = usbipStatusECONNRESET
			}
			s.writeAccess.Lock()
			err = WriteUnlinkResponse(s.conn, UnlinkResponse{
				Header: DataHeader{Command: RetUnlink, SeqNum: header.SeqNum, DevID: header.DevID, Direction: header.Direction, Endpoint: header.Endpoint},
				Status: status,
			})
			s.writeAccess.Unlock()
			if err != nil {
				return err
			}
		default:
			return E.New("unexpected USB/IP command ", fmt.Sprintf("0x%08x", header.Command))
		}
	}
}

func (s *userspaceURBSession) enqueueSubmit(command SubmitCommand) (userspaceNextSubmit, bool) {
	endpoint := submitScheduleEndpoint(command)
	sequence := command.Header.SeqNum

	s.access.Lock()
	defer s.access.Unlock()

	state := userspaceSubmitState{
		command:  command,
		endpoint: endpoint,
	}
	endpointState, found := s.endpoints[endpoint]
	if !found {
		endpointState = &userspaceEndpointState{}
		s.endpoints[endpoint] = endpointState
	}
	if endpointState.active == 0 {
		state.started = true
		s.pending[sequence] = state
		endpointState.active = sequence
		return userspaceNextSubmit{
			sequence: sequence,
			command:  command,
		}, true
	}
	s.pending[sequence] = state
	endpointState.queued = append(endpointState.queued, sequence)
	return userspaceNextSubmit{}, false
}

func (s *userspaceURBSession) startSubmit(next userspaceNextSubmit) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		response := s.handleSubmit(next.command)
		shouldSend, followUp, hasFollowUp := s.finishSubmit(next.sequence)
		if shouldSend {
			s.writeAccess.Lock()
			err := WriteSubmitResponse(s.conn, response)
			s.writeAccess.Unlock()
			if err != nil {
				_ = s.conn.Close()
			}
		}
		if hasFollowUp {
			s.startSubmit(followUp)
		}
	}()
}

func (s *userspaceURBSession) handleSubmit(command SubmitCommand) SubmitResponse {
	response := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    command.Header.SeqNum,
			DevID:     command.Header.DevID,
			Direction: command.Header.Direction,
			Endpoint:  command.Header.Endpoint,
		},
		StartFrame:      command.StartFrame,
		NumberOfPackets: command.NumberOfPackets,
		IsoPackets:      slices.Clone(command.IsoPackets),
	}
	buffer := command.Buffer
	if command.Header.Direction == USBIPDirIn && command.TransferBufferLength > 0 {
		buffer = make([]byte, int(command.TransferBufferLength))
	}
	endpoint := commandEndpoint(command)
	result := s.engine.Submit(URBRequest{
		Command:    command,
		Endpoint:   endpoint,
		Buffer:     buffer,
		IsoPackets: response.IsoPackets,
	})
	if result.Error != nil {
		s.logger.Debug("submit seq ", command.Header.SeqNum, " endpoint 0x", hex8(endpoint), ": ", result.Error)
		response.Status = usbipStatusEIO
		return response
	}
	response.Status = result.Status
	if result.IsoPackets != nil {
		response.IsoPackets = result.IsoPackets
	}
	actual := result.ActualLength
	if actual < 0 {
		actual = 0
	}
	response.ActualLength = actual
	if command.Header.Direction == USBIPDirIn && actual > 0 {
		if command.NumberOfPackets > 0 {
			response.Buffer = packIsoInResponseBuffer(result.Buffer, response.IsoPackets)
			response.ActualLength = int32(len(response.Buffer))
		} else {
			response.Buffer = result.Buffer[:min(int(actual), len(result.Buffer))]
		}
	}
	return response
}

func packIsoInResponseBuffer(buffer []byte, packets []IsoPacketDescriptor) []byte {
	var total int
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			packets[i].ActualLength = 0
			continue
		}
		offset := int(packets[i].Offset)
		if offset < 0 || offset >= len(buffer) {
			packets[i].ActualLength = 0
			continue
		}
		if offset+length > len(buffer) {
			length = len(buffer) - offset
			packets[i].ActualLength = int32(length)
		}
		total += length
	}
	if total == 0 {
		return nil
	}
	packed := make([]byte, 0, total)
	for i := range packets {
		length := int(packets[i].ActualLength)
		if length <= 0 {
			continue
		}
		offset := int(packets[i].Offset)
		packed = append(packed, buffer[offset:offset+length]...)
	}
	return packed
}

func (s *userspaceURBSession) unlinkSubmit(seq uint32) (uint8, <-chan struct{}, bool, bool) {
	var drained chan struct{}

	s.access.Lock()
	pending, found := s.pending[seq]
	if !found {
		s.access.Unlock()
		return 0, nil, false, false
	}
	if pending.drained == nil {
		pending.drained = make(chan struct{})
	}
	drained = pending.drained
	if !pending.started {
		endpointState := s.endpoints[pending.endpoint]
		if endpointState != nil {
			endpointState.queued = removeQueuedSequence(endpointState.queued, seq)
			if endpointState.active == 0 && len(endpointState.queued) == 0 {
				delete(s.endpoints, pending.endpoint)
			}
		}
		delete(s.pending, seq)
		s.access.Unlock()
		close(drained)
		return pending.endpoint, drained, false, true
	}
	shouldAbort := !pending.unlinked
	pending.unlinked = true
	s.pending[seq] = pending
	s.access.Unlock()
	return pending.endpoint, drained, shouldAbort, true
}

func (s *userspaceURBSession) finishSubmit(seq uint32) (bool, userspaceNextSubmit, bool) {
	var drained chan struct{}
	var followUp userspaceNextSubmit
	var hasFollowUp bool

	s.access.Lock()
	pending, found := s.pending[seq]
	if !found {
		s.access.Unlock()
		return true, userspaceNextSubmit{}, false
	}
	endpointState := s.endpoints[pending.endpoint]
	if endpointState != nil && endpointState.active == seq {
		endpointState.active = 0
	}
	delete(s.pending, seq)
	if endpointState != nil {
		for len(endpointState.queued) > 0 {
			nextSequence := endpointState.queued[0]
			endpointState.queued = endpointState.queued[1:]
			nextPending, nextFound := s.pending[nextSequence]
			if !nextFound {
				continue
			}
			nextPending.started = true
			s.pending[nextSequence] = nextPending
			endpointState.active = nextSequence
			followUp = userspaceNextSubmit{
				sequence: nextSequence,
				command:  nextPending.command,
			}
			hasFollowUp = true
			break
		}
		if endpointState.active == 0 && len(endpointState.queued) == 0 {
			delete(s.endpoints, pending.endpoint)
		}
	}
	drained = pending.drained
	unlinked := pending.unlinked
	s.access.Unlock()
	if drained != nil {
		close(drained)
	}
	return !unlinked, followUp, hasFollowUp
}

func (s *userspaceURBSession) abortPendingSubmits() {
	var (
		activeEndpoints []uint8
		drained         []chan struct{}
	)

	s.access.Lock()
	seen := make(map[uint8]struct{})
	for seq, pending := range s.pending {
		if !pending.started {
			delete(s.pending, seq)
			if pending.drained != nil {
				drained = append(drained, pending.drained)
			}
			continue
		}
		if !pending.unlinked {
			seen[pending.endpoint] = struct{}{}
		}
		pending.unlinked = true
		s.pending[seq] = pending
	}
	for endpoint := range s.endpoints {
		endpointState := s.endpoints[endpoint]
		if endpointState != nil {
			endpointState.queued = nil
		}
	}
	s.access.Unlock()

	for _, drainedChannel := range drained {
		close(drainedChannel)
	}
	activeEndpoints = make([]uint8, 0, len(seen))
	for endpoint := range seen {
		activeEndpoints = append(activeEndpoints, endpoint)
	}
	slices.Sort(activeEndpoints)
	for _, endpoint := range activeEndpoints {
		err := s.engine.AbortEndpoint(endpoint)
		if err != nil {
			s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", err)
		}
	}
}

func removeQueuedSequence(queue []uint32, sequence uint32) []uint32 {
	for index, current := range queue {
		if current != sequence {
			continue
		}
		return append(queue[:index], queue[index+1:]...)
	}
	return queue
}

func submitScheduleEndpoint(command SubmitCommand) uint8 {
	if command.Header.Endpoint == 0 {
		return 0
	}
	return commandEndpoint(command)
}

func commandEndpoint(command SubmitCommand) uint8 {
	endpoint := uint8(command.Header.Endpoint & 0x0f)
	if command.Header.Direction == USBIPDirIn {
		endpoint |= 0x80
	}
	return endpoint
}
