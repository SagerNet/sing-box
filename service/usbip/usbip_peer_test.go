//go:build linux || (darwin && cgo)

package usbip

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sagernet/sing-box/log"

	"github.com/stretchr/testify/require"
)

func nopLogger() log.ContextLogger {
	return log.NewNOPFactory().Logger()
}

type wireServer struct {
	conn net.Conn
}

func (s *wireServer) readSubmit(tb testing.TB) SubmitCommand {
	tb.Helper()
	header, err := ReadDataHeader(s.conn)
	require.NoError(tb, err)
	require.Equal(tb, CmdSubmit, header.Command)
	submit, err := ReadSubmitCommandBody(s.conn, header)
	require.NoError(tb, err)
	return submit
}

func (s *wireServer) readUnlink(tb testing.TB) UnlinkCommand {
	tb.Helper()
	header, err := ReadDataHeader(s.conn)
	require.NoError(tb, err)
	require.Equal(tb, CmdUnlink, header.Command)
	unlink, err := ReadUnlinkCommandBody(s.conn, header)
	require.NoError(tb, err)
	return unlink
}

func (s *wireServer) writeSubmitResponse(tb testing.TB, requestDirection uint32, seqnum uint32, status int32, buffer []byte, isoPackets []IsoPacketDescriptor) {
	tb.Helper()
	err := WriteSubmitResponse(s.conn, SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    seqnum,
			Direction: requestDirection,
		},
		Status:       status,
		ActualLength: int32(len(buffer)),
		Buffer:       buffer,
		IsoPackets:   isoPackets,
	})
	require.NoError(tb, err)
}

func (s *wireServer) writeUnlinkResponse(tb testing.TB, seqnum uint32, status int32) {
	tb.Helper()
	err := WriteUnlinkResponse(s.conn, UnlinkResponse{
		Header: DataHeader{Command: RetUnlink, SeqNum: seqnum},
		Status: status,
	})
	require.NoError(tb, err)
}

func newPeerPair(tb testing.TB) (*UsbIpPeer, *wireServer, context.CancelFunc) {
	tb.Helper()
	clientConn, serverConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	peer := NewUsbIpPeer(ctx, nopLogger(), clientConn)
	server := &wireServer{conn: serverConn}
	tb.Cleanup(func() {
		_ = peer.Close()
		_ = serverConn.Close()
		cancel()
	})
	return peer, server, cancel
}

func TestUsbIpPeerSubmitRoundTrip(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	payload := []byte{1, 2, 3, 4}
	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		require.Equal(t, USBIPDirIn, submit.Header.Direction)
		require.Equal(t, uint32(7), submit.Header.DevID)
		require.Equal(t, int32(len(payload)), submit.TransferBufferLength)
		server.writeSubmitResponse(t, USBIPDirIn, submit.Header.SeqNum, 0, payload, nil)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     7,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: int32(len(payload)),
	})
	require.NoError(t, err)

	response, err := transaction.Wait(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(0), response.Status)
	require.Equal(t, int32(len(payload)), response.ActualLength)
	require.Equal(t, payload, response.Buffer)

	serverDone.Wait()
}

func TestUsbIpPeerSessionCloseFailsPending(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	go func() {
		_ = server.readSubmit(t)
		// Do not reply.
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     7,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 16,
	})
	require.NoError(t, err)

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = peer.Close()
	}()

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrPeerClosed)
}

// TestUsbIpPeerCancelMidFlight uses the canonical Linux cancel wire:
// after CMD_UNLINK the server emits only RET_UNLINK (status
// ECONNRESET), never a parallel RET_SUBMIT. The peer finalizes the
// transaction off RET_UNLINK alone.
func TestUsbIpPeerCancelMidFlight(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	const dataLen = 32
	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		require.Equal(t, USBIPDirIn, submit.Header.Direction)
		unlink := server.readUnlink(t)
		require.Equal(t, submit.Header.SeqNum, unlink.SeqNum)
		require.Equal(t, submit.Header.DevID, unlink.Header.DevID)
		server.writeUnlinkResponse(t, unlink.Header.SeqNum, usbipStatusECONNRESET)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirIn,
			Endpoint:  2,
		},
		TransferBufferLength: dataLen,
	})
	require.NoError(t, err)

	go func() {
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, transaction.Cancel(context.Background()))
	}()

	_, err = transaction.Wait(context.Background())
	require.ErrorIs(t, err, ErrCanceled)
	serverDone.Wait()
}

func TestUsbIpPeerCancelAfterCompletion(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	var serverDone sync.WaitGroup
	serverDone.Add(1)
	go func() {
		defer serverDone.Done()
		submit := server.readSubmit(t)
		server.writeSubmitResponse(t, USBIPDirOut, submit.Header.SeqNum, 0, nil, nil)
	}()

	transaction, err := peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     1,
			Direction: USBIPDirOut,
			Endpoint:  1,
		},
		TransferBufferLength: 4,
		Buffer:               []byte{9, 9, 9, 9},
	})
	require.NoError(t, err)

	_, err = transaction.Wait(context.Background())
	require.NoError(t, err)
	serverDone.Wait()

	// Cancel after completion must be a no-op; if it tried to write CMD_UNLINK
	// the test server would have already exited and the wire would either block
	// (net.Pipe drops writes when there's no reader) or fail. The call must not
	// hang.
	cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = transaction.Cancel(cancelCtx)
	require.NoError(t, err)
}

func TestUsbIpPeerUnknownSeqnumClosesPeer(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	go func() {
		err := WriteSubmitResponse(server.conn, SubmitResponse{
			Header: DataHeader{
				Command:   RetSubmit,
				SeqNum:    99,
				Direction: USBIPDirIn,
			},
			NumberOfPackets: nonIsoPacketCount,
		})
		if err != nil {
			return
		}
	}()

	select {
	case <-peer.Done():
	case <-time.After(time.Second):
		t.Fatal("peer did not close after unknown RET_SUBMIT")
	}
	require.ErrorContains(t, peer.Err(), "unexpected RET_SUBMIT seq 99")
}

func TestUsbIpPeerConcurrentSubmits(t *testing.T) {
	peer, server, _ := newPeerPair(t)

	const submits = 16
	serverErrCh := make(chan error, submits)
	go func() {
		for range submits {
			submit, err := func() (SubmitCommand, error) {
				header, err := ReadDataHeader(server.conn)
				if err != nil {
					return SubmitCommand{}, err
				}
				if header.Command != CmdSubmit {
					return SubmitCommand{}, errors.New("unexpected command")
				}
				return ReadSubmitCommandBody(server.conn, header)
			}()
			if err != nil {
				serverErrCh <- err
				return
			}
			payload := make([]byte, submit.TransferBufferLength)
			for i := range payload {
				payload[i] = byte(submit.Header.SeqNum)
			}
			err = WriteSubmitResponse(server.conn, SubmitResponse{
				Header: DataHeader{
					Command:   RetSubmit,
					SeqNum:    submit.Header.SeqNum,
					Direction: USBIPDirIn,
				},
				Status:       0,
				ActualLength: int32(len(payload)),
				Buffer:       payload,
			})
			if err != nil {
				serverErrCh <- err
				return
			}
		}
		close(serverErrCh)
	}()

	var wg sync.WaitGroup
	var got atomic.Int64
	wg.Add(submits)
	for range submits {
		go func() {
			defer wg.Done()
			transaction, err := peer.Submit(SubmitCommand{
				Header: DataHeader{
					Command:   CmdSubmit,
					DevID:     1,
					Direction: USBIPDirIn,
					Endpoint:  3,
				},
				TransferBufferLength: 8,
			})
			require.NoError(t, err)
			response, err := transaction.Wait(context.Background())
			require.NoError(t, err)
			require.Equal(t, int32(8), response.ActualLength)
			require.Equal(t, byte(transaction.SeqNum()), response.Buffer[0])
			got.Add(1)
		}()
	}
	wg.Wait()
	require.Equal(t, int64(submits), got.Load())

	err, ok := <-serverErrCh
	if ok && err != nil {
		t.Fatalf("server side: %v", err)
	}
}
