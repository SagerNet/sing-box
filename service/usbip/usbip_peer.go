//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

type peerClosedError struct{}

func (peerClosedError) Error() string { return "USB/IP peer closed" }

type urbCanceledError struct{}

func (urbCanceledError) Error() string { return "USB/IP urb canceled" }

var (
	ErrPeerClosed error = peerClosedError{}
	ErrCanceled   error = urbCanceledError{}
)

// pendingSubmit retains the per-submit metadata the peer needs to
// emit a protocol-conformant CMD_UNLINK and to bind RET_UNLINK back
// to the originating submit. DevID is required by Linux's stub_rx
// valid_request check; without it cancellation packets are silently
// rejected.
type pendingSubmit struct {
	transaction *UrbTransaction
	devID       uint32
}

type UsbIpPeer struct {
	ctx      context.Context
	closeCtx context.CancelFunc
	logger   log.ContextLogger
	conn     net.Conn

	seq atomic.Uint32

	writeAccess sync.Mutex

	// pendingAccess guards both pending and unlinkBindings: a
	// CMD_UNLINK write registers a binding and inspects pending
	// atomically so RET_UNLINK arriving immediately afterwards can
	// always be matched back to its submit.
	pendingAccess  sync.Mutex
	pending        map[uint32]*pendingSubmit
	unlinkBindings map[uint32]uint32

	done      chan struct{}
	closeOnce sync.Once

	errAccess sync.Mutex
	err       error
}

func NewUsbIpPeer(ctx context.Context, logger log.ContextLogger, conn net.Conn) *UsbIpPeer {
	ctx, closeCtx := context.WithCancel(ctx)
	peer := &UsbIpPeer{
		ctx:            ctx,
		closeCtx:       closeCtx,
		logger:         logger,
		conn:           conn,
		pending:        make(map[uint32]*pendingSubmit),
		unlinkBindings: make(map[uint32]uint32),
		done:           make(chan struct{}),
	}
	go peer.readLoop()
	return peer
}

func (p *UsbIpPeer) Submit(command SubmitCommand) (*UrbTransaction, error) {
	seqnum := p.seq.Add(1)
	command.Header.SeqNum = seqnum
	if command.NumberOfPackets == 0 && len(command.IsoPackets) == 0 {
		command.NumberOfPackets = nonIsoPacketCount
	}

	transaction := &UrbTransaction{
		peer:      p,
		seqnum:    seqnum,
		direction: command.Header.Direction,
		done:      make(chan struct{}),
	}

	p.pendingAccess.Lock()
	if p.pending == nil {
		p.pendingAccess.Unlock()
		return nil, ErrPeerClosed
	}
	p.pending[seqnum] = &pendingSubmit{
		transaction: transaction,
		devID:       command.Header.DevID,
	}
	p.pendingAccess.Unlock()

	p.writeAccess.Lock()
	err := WriteSubmitCommand(p.conn, command)
	p.writeAccess.Unlock()
	if err != nil {
		p.removePending(seqnum)
		return nil, err
	}
	return transaction, nil
}

func (p *UsbIpPeer) Done() <-chan struct{} {
	return p.done
}

func (p *UsbIpPeer) Err() error {
	p.errAccess.Lock()
	defer p.errAccess.Unlock()
	return p.err
}

func (p *UsbIpPeer) Close() error {
	p.closeOnce.Do(func() {
		p.closeCtx()
		_ = p.conn.Close()
	})
	<-p.done
	return nil
}

// cancel sends CMD_UNLINK for the submit identified by submitSeqnum.
// It is a no-op when the submit is no longer pending (already
// finalized, or never registered): Linux's RET_UNLINK in that race
// carries status 0 and adds no observable state to the client. When
// the wire write fails the unlink binding is rolled back so a later
// retry can re-allocate without leaking the binding slot.
func (p *UsbIpPeer) cancel(submitSeqnum uint32) error {
	unlinkSeqnum := p.seq.Add(1)

	p.pendingAccess.Lock()
	if p.pending == nil {
		p.pendingAccess.Unlock()
		return nil
	}
	submit, found := p.pending[submitSeqnum]
	if !found {
		p.pendingAccess.Unlock()
		return nil
	}
	devID := submit.devID
	p.unlinkBindings[unlinkSeqnum] = submitSeqnum
	p.pendingAccess.Unlock()

	p.writeAccess.Lock()
	err := WriteUnlinkCommand(p.conn, UnlinkCommand{
		Header: DataHeader{
			Command: CmdUnlink,
			SeqNum:  unlinkSeqnum,
			DevID:   devID,
		},
		SeqNum: submitSeqnum,
	})
	p.writeAccess.Unlock()
	if err != nil {
		p.pendingAccess.Lock()
		delete(p.unlinkBindings, unlinkSeqnum)
		p.pendingAccess.Unlock()
	}
	return err
}

func (p *UsbIpPeer) removePending(seqnum uint32) {
	p.pendingAccess.Lock()
	if p.pending != nil {
		delete(p.pending, seqnum)
	}
	p.pendingAccess.Unlock()
}

func (p *UsbIpPeer) lookupPending(seqnum uint32) *UrbTransaction {
	p.pendingAccess.Lock()
	defer p.pendingAccess.Unlock()
	if p.pending == nil {
		return nil
	}
	submit, found := p.pending[seqnum]
	if !found {
		return nil
	}
	return submit.transaction
}

func (p *UsbIpPeer) consumePending(seqnum uint32) *UrbTransaction {
	p.pendingAccess.Lock()
	defer p.pendingAccess.Unlock()
	if p.pending == nil {
		return nil
	}
	submit, found := p.pending[seqnum]
	if !found {
		return nil
	}
	delete(p.pending, seqnum)
	return submit.transaction
}

// consumeUnlink resolves a RET_UNLINK back to its submit transaction
// in one critical section: the binding entry is removed regardless,
// and the submit is consumed only when it has not already been
// finalized by an earlier RET_SUBMIT.
func (p *UsbIpPeer) consumeUnlink(unlinkSeqnum uint32) *UrbTransaction {
	p.pendingAccess.Lock()
	defer p.pendingAccess.Unlock()
	if p.unlinkBindings == nil || p.pending == nil {
		return nil
	}
	submitSeqnum, found := p.unlinkBindings[unlinkSeqnum]
	if !found {
		return nil
	}
	delete(p.unlinkBindings, unlinkSeqnum)
	submit, stillPending := p.pending[submitSeqnum]
	if !stillPending {
		return nil
	}
	delete(p.pending, submitSeqnum)
	return submit.transaction
}

func (p *UsbIpPeer) readLoop() {
	defer close(p.done)
	defer p.closeCtx()
	defer p.drainPending()

	for {
		header, err := ReadDataHeader(p.conn)
		if err != nil {
			p.setReadError(err)
			return
		}
		switch header.Command {
		case RetSubmit:
			transaction := p.lookupPending(header.SeqNum)
			if transaction == nil {
				p.setReadError(E.New("unexpected RET_SUBMIT seq ", header.SeqNum))
				return
			}
			response, err := ReadSubmitResponseBody(p.conn, header, transaction.direction)
			if err != nil {
				p.setReadError(err)
				return
			}
			if p.consumePending(header.SeqNum) == nil {
				continue
			}
			p.deliverSubmit(transaction, response)
		case RetUnlink:
			_, err := ReadUnlinkResponseBody(p.conn, header)
			if err != nil {
				p.setReadError(err)
				return
			}
			transaction := p.consumeUnlink(header.SeqNum)
			if transaction == nil {
				continue
			}
			transaction.finalize(SubmitResponse{}, ErrCanceled)
		default:
			p.setReadError(E.New("unexpected USB/IP response ", fmt.Sprintf("0x%08x", header.Command)))
			return
		}
	}
}

func (p *UsbIpPeer) deliverSubmit(transaction *UrbTransaction, response SubmitResponse) {
	transaction.access.Lock()
	canceling := transaction.canceling
	transaction.access.Unlock()
	if canceling {
		transaction.finalize(SubmitResponse{}, ErrCanceled)
		return
	}
	transaction.finalize(response, nil)
}

func (p *UsbIpPeer) drainPending() {
	p.pendingAccess.Lock()
	pending := p.pending
	p.pending = nil
	p.unlinkBindings = nil
	p.pendingAccess.Unlock()
	for _, submit := range pending {
		submit.transaction.finalize(SubmitResponse{}, ErrPeerClosed)
	}
}

func (p *UsbIpPeer) setReadError(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, io.EOF) || E.IsClosedOrCanceled(err) || p.ctx.Err() != nil {
		return
	}
	p.errAccess.Lock()
	if p.err == nil {
		p.err = err
	}
	p.errAccess.Unlock()
	p.logger.Debug("USB/IP read loop: ", err)
}
