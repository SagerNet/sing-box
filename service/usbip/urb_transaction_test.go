package usbip

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUrbTransactionWaitContextCancel(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	go func() {
		_ = server.readSubmit(t)
		// Never reply.
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 8,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = transaction.Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestUrbTransactionCancelIdempotent exercises the canonical Linux
// cancel wire: after CMD_UNLINK the server replies only with
// RET_UNLINK (status ECONNRESET), never a parallel RET_SUBMIT. The
// peer must finalize the transaction off RET_UNLINK alone, and N
// concurrent Cancel() callers must produce exactly one CMD_UNLINK on
// the wire.
func TestUrbTransactionCancelIdempotent(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var unlinks atomic.Int32
	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		unlink := server.readUnlink(t)
		unlinks.Add(1)
		require.Equal(t, submit.Header.SeqNum, unlink.SeqNum)
		require.Equal(t, submit.Header.DevID, unlink.Header.DevID)
		server.writeUnlinkResponse(t, unlink.Header.SeqNum, usbipStatusECONNRESET)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     0xCAFEF00D,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 8,
	})
	require.NoError(t, err)

	const callers = 8
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			err := transaction.Cancel(context.Background())
			require.NoError(t, err)
		}()
	}
	wg.Wait()

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrCanceled)

	serverDone.Wait()
	require.Equal(t, int32(1), unlinks.Load(), "Cancel wrote CMD_UNLINK more than once")
}

// TestUrbTransactionCancelAfterUrbCompleted models Linux's
// stub_recv_cmd_unlink race: the URB completes on the server before
// CMD_UNLINK arrives, so the server still emits RET_SUBMIT for the
// completed transfer and then emits RET_UNLINK with status 0. The
// client called Cancel mid-flight; the peer must finalize as
// ErrCanceled (canceling flag wins) and absorb the trailing
// RET_UNLINK without crashing the read loop.
func TestUrbTransactionCancelAfterUrbCompleted(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		unlink := server.readUnlink(t)
		require.Equal(t, submit.Header.SeqNum, unlink.SeqNum)
		// URB completed before unlink: real status data goes on the
		// wire, then RET_UNLINK status=0 reports "nothing to cancel".
		server.writeSubmitResponse(t, USBIPDirIn, submit.Header.SeqNum, 0, []byte{1, 2, 3, 4}, nil)
		server.writeUnlinkResponse(t, unlink.Header.SeqNum, 0)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 4,
	})
	require.NoError(t, err)

	require.NoError(t, transaction.Cancel(context.Background()))

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrCanceled)

	serverDone.Wait()

	// The peer must remain healthy after absorbing the trailing
	// RET_UNLINK; close it cleanly.
	require.NoError(t, peer.Close())
}

// TestUrbTransactionCancelWireCarriesDevID pins down the Linux
// stub_rx valid_request check: CMD_UNLINK must carry the original
// submit's DevID. The bytes travel through a real socket pair into a
// real ReadDataHeader, so this observes the actual wire effect rather
// than a builder property.
func TestUrbTransactionCancelWireCarriesDevID(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		unlink := server.readUnlink(t)
		require.Equal(t, uint32(0xDEADBEEF), unlink.Header.DevID)
		require.Equal(t, submit.Header.SeqNum, unlink.SeqNum)
		server.writeUnlinkResponse(t, unlink.Header.SeqNum, usbipStatusECONNRESET)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     0xDEADBEEF,
			Direction: USBIPDirOut,
			Endpoint:  2,
		},
		TransferBufferLength: 4,
		Buffer:               []byte{5, 6, 7, 8},
	})
	require.NoError(t, err)

	require.NoError(t, transaction.Cancel(context.Background()))

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrCanceled)

	serverDone.Wait()
}

func TestUrbTransactionCancelAfterTerminalNoWire(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		server.writeSubmitResponse(t, USBIPDirOut, submit.Header.SeqNum, 0, nil, nil)
		// Any further read MUST be EOF (no CMD_UNLINK in flight).
		_, err := ReadDataHeader(server.conn)
		require.Error(t, err)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirOut,
			Endpoint:  1,
		},
		TransferBufferLength: 4,
		Buffer:               []byte{1, 2, 3, 4},
	})
	require.NoError(t, err)

	_, err = transaction.Wait(context.Background())
	require.NoError(t, err)

	require.NoError(t, transaction.Cancel(context.Background()))

	require.NoError(t, peer.Close())
	serverDone.Wait()
}
