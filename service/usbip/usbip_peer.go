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

type UsbIpPeer struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger log.ContextLogger
	conn   net.Conn

	seq atomic.Uint32

	writeAccess sync.Mutex

	pendingAccess sync.Mutex
	pending       map[uint32]*UrbTransaction

	done      chan struct{}
	closeOnce sync.Once

	errAccess sync.Mutex
	err       error
}

func NewUsbIpPeer(ctx context.Context, logger log.ContextLogger, conn net.Conn) *UsbIpPeer {
	ctx, cancel := context.WithCancel(ctx)
	peer := &UsbIpPeer{
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		conn:    conn,
		pending: make(map[uint32]*UrbTransaction),
		done:    make(chan struct{}),
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
	p.pending[seqnum] = transaction
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
		p.cancel()
		_ = p.conn.Close()
	})
	<-p.done
	return nil
}

func (p *UsbIpPeer) writeUnlink(submitSeqnum uint32) error {
	unlinkSeqnum := p.seq.Add(1)
	p.writeAccess.Lock()
	err := WriteUnlinkCommand(p.conn, UnlinkCommand{
		Header: DataHeader{
			Command: CmdUnlink,
			SeqNum:  unlinkSeqnum,
		},
		SeqNum: submitSeqnum,
	})
	p.writeAccess.Unlock()
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
	return p.pending[seqnum]
}

func (p *UsbIpPeer) consumePending(seqnum uint32) *UrbTransaction {
	p.pendingAccess.Lock()
	defer p.pendingAccess.Unlock()
	if p.pending == nil {
		return nil
	}
	transaction := p.pending[seqnum]
	if transaction != nil {
		delete(p.pending, seqnum)
	}
	return transaction
}

func (p *UsbIpPeer) readLoop() {
	defer close(p.done)
	defer p.cancel()
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
			// The submit seqnum bound to this unlink is unknown without an
			// additional map; we rely on the matching RET_SUBMIT (with
			// status=ECONNRESET on cancel) to finalize the transaction.
		default:
			p.setReadError(E.New(fmt.Sprintf("unexpected USB/IP response 0x%08x", header.Command)))
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
	p.pendingAccess.Unlock()
	for _, transaction := range pending {
		transaction.finalize(SubmitResponse{}, ErrPeerClosed)
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
