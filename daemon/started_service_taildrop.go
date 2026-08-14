package daemon

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	// gRPC splits DATA frames at the unexported transport.http2MaxFrameLen (16384),
	// counting the 5-byte message prefix. A chunk message that stays inside one frame
	// arrives as a single mem.Buffer, so BufferSlice.MaterializeToBuffer returns it
	// without taking a pooled buffer, and codecV2.Marshal fits the 16 KiB pool tier.
	TaildropChunkSize = 16*1024 - 64

	TaildropProgressMinInterval = 200 * time.Millisecond

	// Kept well below the sender's flow control window, so a sender that stops at
	// the window edge always has an acknowledgement pending.
	TaildropReceiveAckInterval = 1 << 20
)

func resolveTailscaleProvider(instance *Instance, tag string) (adapter.TailscaleEndpoint, string, error) {
	if tag != "" {
		endpoint, err := resolveTailscaleEndpoint(instance, tag)
		if err != nil {
			return nil, "", err
		}
		provider, loaded := endpoint.(adapter.TailscaleEndpoint)
		if !loaded {
			return nil, "", status.Error(codes.FailedPrecondition, "endpoint does not support tailscale")
		}
		return provider, endpoint.Tag(), nil
	}
	endpointManager := service.FromContext[adapter.EndpointManager](instance.ctx)
	if endpointManager == nil {
		return nil, "", status.Error(codes.FailedPrecondition, "endpoint manager not available")
	}
	for _, endpoint := range endpointManager.Endpoints() {
		if endpoint.Type() != C.TypeTailscale {
			continue
		}
		provider, loaded := endpoint.(adapter.TailscaleEndpoint)
		if loaded {
			return provider, endpoint.Tag(), nil
		}
	}
	return nil, "", status.Error(codes.NotFound, "no Tailscale endpoint found")
}

func taildropError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return status.Error(codes.NotFound, err.Error())
	}
	return err
}

func (s *StartedService) SubscribeTaildropInbox(
	request *SubscribeTaildropInboxRequest,
	server grpc.ServerStreamingServer[TaildropInbox],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	return s.followInstance(server.Context(), func(ctx context.Context, instance *Instance) error {
		var (
			provider    adapter.TailscaleEndpoint
			endpointTag string
		)
		if instance != nil {
			var resolveErr error
			provider, endpointTag, resolveErr = resolveTailscaleProvider(instance, request.EndpointTag)
			if resolveErr != nil && status.Code(resolveErr) != codes.NotFound {
				return resolveErr
			}
		}
		if provider == nil {
			sendErr := server.Send(&TaildropInbox{})
			if sendErr != nil {
				return sendErr
			}
			<-ctx.Done()
			return nil
		}
		subscribeCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		var sendErr error
		subscribeErr := provider.SubscribeTaildropInbox(subscribeCtx, func(inbox *adapter.TaildropInbox) {
			if sendErr != nil {
				return
			}
			sendErr = server.Send(taildropInboxToProto(endpointTag, inbox))
			if sendErr != nil {
				cancel()
			}
		})
		if sendErr != nil {
			return sendErr
		}
		if subscribeErr != nil && !errors.Is(subscribeErr, context.Canceled) {
			if !errors.Is(subscribeErr, os.ErrClosed) {
				return taildropError(subscribeErr)
			}
			sendErr = server.Send(&TaildropInbox{EndpointTag: endpointTag})
			if sendErr != nil {
				return sendErr
			}
		}
		<-ctx.Done()
		return nil
	})
}

func taildropInboxToProto(endpointTag string, inbox *adapter.TaildropInbox) *TaildropInbox {
	result := &TaildropInbox{EndpointTag: endpointTag}
	for _, file := range inbox.Files {
		result.Files = append(result.Files, &TaildropFile{
			Name:       file.Name,
			Size:       file.Size,
			SenderName: file.SenderName,
			ModifiedAt: file.ModifiedAt,
		})
	}
	for _, file := range inbox.Receiving {
		result.Receiving = append(result.Receiving, &TaildropReceivingFile{
			Name:          file.Name,
			Size:          file.Size,
			ReceivedBytes: file.ReceivedBytes,
			SenderID:      file.SenderID,
			SenderName:    file.SenderName,
		})
	}
	return result
}

func (s *StartedService) MarkTaildropInboxRead(ctx context.Context, request *MarkTaildropInboxReadRequest) (*emptypb.Empty, error) {
	err := s.waitForStarted(ctx)
	if err != nil {
		return nil, err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	provider, _, err := resolveTailscaleProvider(boxService, request.EndpointTag)
	if err != nil {
		return nil, err
	}
	err = provider.MarkTaildropInboxRead()
	if err != nil {
		return nil, taildropError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) SendTaildropFiles(
	server grpc.BidiStreamingServer[TaildropSendClientMessage, TaildropSendServerMessage],
) error {
	streamCtx := server.Context()
	err := s.waitForStarted(streamCtx)
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	firstMessage, err := server.Recv()
	if err != nil {
		return err
	}
	start := firstMessage.GetStart()
	if start == nil {
		return status.Error(codes.InvalidArgument, "expected start message")
	}
	if len(start.Files) == 0 {
		return status.Error(codes.InvalidArgument, "no files to send")
	}
	provider, _, err := resolveTailscaleProvider(boxService, start.EndpointTag)
	if err != nil {
		return err
	}

	var sendAccess sync.Mutex
	sendMessage := func(message *TaildropSendServerMessage) {
		sendAccess.Lock()
		defer sendAccess.Unlock()
		_ = server.Send(message)
	}

	chunkSource := &taildropChunkReader{server: server}
	chunkSource.reportReceived = func(receivedBytes int64) {
		sendMessage(&TaildropSendServerMessage{
			Message: &TaildropSendServerMessage_ReceivedBytes{ReceivedBytes: receivedBytes},
		})
	}
	for index, file := range start.Files {
		fileIndex := int32(index)
		chunkSource.beginFile()
		var lastProgress time.Time
		err = provider.SendTaildropFile(streamCtx, start.PeerStableID, file.Name, file.Size, chunkSource, func(sentBytes int64) {
			now := time.Now()
			if now.Sub(lastProgress) < TaildropProgressMinInterval {
				return
			}
			lastProgress = now
			sendMessage(&TaildropSendServerMessage{
				Message: &TaildropSendServerMessage_Progress{
					Progress: &TaildropSendProgress{FileIndex: fileIndex, SentBytes: sentBytes},
				},
			})
		})
		if err != nil {
			return taildropError(err)
		}
		err = chunkSource.finishFile(file.Size)
		if err != nil {
			return err
		}
		sendMessage(&TaildropSendServerMessage{
			Message: &TaildropSendServerMessage_Progress{
				Progress: &TaildropSendProgress{FileIndex: fileIndex, SentBytes: chunkSource.fileReceived, FileCompleted: true},
			},
		})
	}
	return nil
}

type taildropChunkReader struct {
	server         grpc.BidiStreamingServer[TaildropSendClientMessage, TaildropSendServerMessage]
	buffer         []byte
	fileDone       bool
	fileReceived   int64
	reportReceived func(receivedBytes int64)
	received       int64
	acknowledged   int64
}

func (r *taildropChunkReader) beginFile() {
	r.fileDone = false
	r.fileReceived = 0
}

func (r *taildropChunkReader) Read(destination []byte) (int, error) {
	for len(r.buffer) == 0 {
		if r.fileDone {
			return 0, io.EOF
		}
		message, err := r.server.Recv()
		if err != nil {
			return 0, err
		}
		switch content := message.Message.(type) {
		case *TaildropSendClientMessage_Chunk:
			r.buffer = content.Chunk.Data
		case *TaildropSendClientMessage_FileDone:
			r.fileDone = true
		default:
			return 0, status.Error(codes.InvalidArgument, "expected chunk message")
		}
	}
	n := copy(destination, r.buffer)
	r.buffer = r.buffer[n:]
	r.fileReceived += int64(n)
	r.received += int64(n)
	if r.reportReceived != nil && r.received-r.acknowledged >= TaildropReceiveAckInterval {
		r.acknowledged = r.received
		r.reportReceived(r.received)
	}
	return n, nil
}

func (r *taildropChunkReader) finishFile(declaredSize int64) error {
	for !r.fileDone {
		if len(r.buffer) > 0 {
			return status.Error(codes.InvalidArgument, "file changed while sending")
		}
		message, err := r.server.Recv()
		if err != nil {
			return err
		}
		switch content := message.Message.(type) {
		case *TaildropSendClientMessage_Chunk:
			r.buffer = content.Chunk.Data
		case *TaildropSendClientMessage_FileDone:
			r.fileDone = true
		default:
			return status.Error(codes.InvalidArgument, "expected chunk message")
		}
	}
	if len(r.buffer) > 0 {
		return status.Error(codes.InvalidArgument, "file changed while sending")
	}
	if declaredSize >= 0 && r.fileReceived != declaredSize {
		return status.Error(codes.InvalidArgument, "file changed while sending")
	}
	return nil
}

func (s *StartedService) DownloadTaildropFile(
	request *DownloadTaildropFileRequest,
	server grpc.ServerStreamingServer[DownloadTaildropFileChunk],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	provider, _, err := resolveTailscaleProvider(boxService, request.EndpointTag)
	if err != nil {
		return err
	}
	file, size, err := provider.OpenTaildropFile(request.Name)
	if err != nil {
		return taildropError(err)
	}
	defer file.Close()
	err = server.Send(&DownloadTaildropFileChunk{Size: size})
	if err != nil {
		return err
	}
	buffer := make([]byte, TaildropChunkSize)
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			err = server.Send(&DownloadTaildropFileChunk{Data: buffer[:n]})
			if err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (s *StartedService) DeleteTaildropFile(ctx context.Context, request *DeleteTaildropFileRequest) (*emptypb.Empty, error) {
	err := s.waitForStarted(ctx)
	if err != nil {
		return nil, err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	provider, _, err := resolveTailscaleProvider(boxService, request.EndpointTag)
	if err != nil {
		return nil, err
	}
	err = provider.DeleteTaildropFile(request.Name)
	if err != nil {
		return nil, taildropError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) CancelTaildropReceiving(ctx context.Context, request *CancelTaildropReceivingRequest) (*emptypb.Empty, error) {
	err := s.waitForStarted(ctx)
	if err != nil {
		return nil, err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	provider, _, err := resolveTailscaleProvider(boxService, request.EndpointTag)
	if err != nil {
		return nil, err
	}
	err = provider.CancelTaildropReceiving(request.SenderID, request.Name)
	if err != nil {
		return nil, taildropError(err)
	}
	return &emptypb.Empty{}, nil
}
