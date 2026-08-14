package libbox

import (
	"os"

	"github.com/sagernet/sing-box/daemon"
)

const TaildropChunkSize = daemon.TaildropChunkSize

type TaildropInbox struct {
	EndpointTag string
	files       []*TaildropFile
	receiving   []*TaildropReceivingFile
}

func (i *TaildropInbox) Files() TaildropFileIterator {
	return newIterator(i.files)
}

func (i *TaildropInbox) Receiving() TaildropReceivingFileIterator {
	return newIterator(i.receiving)
}

type TaildropFile struct {
	Name       string
	Size       int64
	SenderName string
	ModifiedAt int64
}

type TaildropFileIterator interface {
	Next() *TaildropFile
	HasNext() bool
}

type TaildropReceivingFile struct {
	Name          string
	Size          int64
	ReceivedBytes int64
	SenderID      string
	SenderName    string
}

type TaildropReceivingFileIterator interface {
	Next() *TaildropReceivingFile
	HasNext() bool
}

type TaildropInboxHandler interface {
	OnInboxUpdate(inbox *TaildropInbox)
	OnError(message string)
}

type TaildropInboxSubscription struct {
	streamSession
}

func taildropInboxFromGRPC(inbox *daemon.TaildropInbox) *TaildropInbox {
	result := &TaildropInbox{EndpointTag: inbox.EndpointTag}
	for _, file := range inbox.Files {
		result.files = append(result.files, &TaildropFile{
			Name:       file.Name,
			Size:       file.Size,
			SenderName: file.SenderName,
			ModifiedAt: file.ModifiedAt,
		})
	}
	for _, file := range inbox.Receiving {
		result.receiving = append(result.receiving, &TaildropReceivingFile{
			Name:          file.Name,
			Size:          file.Size,
			ReceivedBytes: file.ReceivedBytes,
			SenderID:      file.SenderID,
			SenderName:    file.SenderName,
		})
	}
	return result
}

type TaildropSendOptions struct {
	EndpointTag  string
	PeerStableID string
	files        []*daemon.TaildropOutgoingFile
}

func NewTaildropSendOptions() *TaildropSendOptions {
	return &TaildropSendOptions{}
}

// AddFile queues a file whose content the caller writes through
// TaildropSendSession.WriteChunk in queue order, terminated by FinishFile.
// A negative size declares the size as unknown; a non-negative size is
// verified against the written byte count.
func (o *TaildropSendOptions) AddFile(name string, size int64) {
	o.files = append(o.files, &daemon.TaildropOutgoingFile{
		Name: name,
		Size: size,
	})
}

type TaildropDownloadHandler interface {
	OnProgress(downloaded int64, total int64)
	OnFinish(errorMessage string)
}

type TaildropDownloadSession struct {
	streamSession
}

type TaildropSendHandler interface {
	OnProgress(fileIndex int32, sentBytes int64)
	OnFileCompleted(fileIndex int32, sentBytes int64)
	OnFinish(errorMessage string)
}

type TaildropSendSession struct {
	streamSession
	stream daemon.StartedService_SendTaildropFilesClient
}

func (s *TaildropSendSession) WriteChunk(data []byte) error {
	if s.ctx.Err() != nil {
		return os.ErrClosed
	}
	err := s.stream.Send(&daemon.TaildropSendClientMessage{
		Message: &daemon.TaildropSendClientMessage_Chunk{Chunk: &daemon.TaildropFileChunk{Data: data}},
	})
	if err != nil {
		return s.finishSend()
	}
	return nil
}

func (s *TaildropSendSession) FinishFile() error {
	if s.ctx.Err() != nil {
		return os.ErrClosed
	}
	err := s.stream.Send(&daemon.TaildropSendClientMessage{
		Message: &daemon.TaildropSendClientMessage_FileDone{FileDone: &daemon.TaildropFileDone{}},
	})
	if err != nil {
		return s.finishSend()
	}
	return nil
}

// grpc-go reports the RPC status only through RecvMsg; a Send on a stream
// terminated by the server returns bare io.EOF. Wait for the receive
// goroutine to deliver the status through OnFinish, then return a sentinel
// the caller does not report.
func (s *TaildropSendSession) finishSend() error {
	<-s.closeDone
	return os.ErrClosed
}
