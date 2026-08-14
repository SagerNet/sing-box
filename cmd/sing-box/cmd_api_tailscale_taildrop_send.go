package main

import (
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sagernet/sing-box/daemon"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var commandAPITailscaleTaildropSend = &cobra.Command{
	Use:   "send <peer> <file>...",
	Short: "Send files to a Tailscale peer",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAPITaildropSend(args[0], args[1:])
	},
}

func init() {
	commandAPITailscaleTaildrop.AddCommand(commandAPITailscaleTaildropSend)
}

func runAPITaildropSend(selector string, filePaths []string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpoint, err := fetchTailscaleEndpoint(client)
	if err != nil {
		return err
	}
	if !endpoint.GetCanShareFiles() {
		return E.New("file sharing not enabled by Tailscale admin")
	}
	entry, err := resolveTailscalePeer(tailscalePeerEntries(endpoint), selector)
	if err != nil {
		return err
	}
	if entry.self {
		return E.New("cannot send files to this device")
	}
	if !entry.peer.GetCanReceiveFiles() {
		return E.New("peer cannot receive files: ", tailscalePeerName(entry.peer))
	}

	type outgoingFile struct {
		path string
		name string
		size int64
	}
	files := make([]outgoingFile, 0, len(filePaths))
	manifest := make([]*daemon.TaildropOutgoingFile, 0, len(filePaths))
	for _, filePath := range filePaths {
		information, statErr := os.Stat(filePath)
		if statErr != nil {
			return statErr
		}
		if information.IsDir() {
			return E.New("directories are not supported: ", filePath)
		}
		file := outgoingFile{
			path: filePath,
			name: filepath.Base(filePath),
			size: information.Size(),
		}
		files = append(files, file)
		manifest = append(manifest, &daemon.TaildropOutgoingFile{
			Name: file.name,
			Size: file.size,
		})
	}

	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stream, err := client.SendTaildropFiles(ctx)
	if err != nil {
		return err
	}
	err = stream.Send(&daemon.TaildropSendClientMessage{
		Message: &daemon.TaildropSendClientMessage_Start{Start: &daemon.TaildropSendStart{
			EndpointTag:  endpoint.GetEndpointTag(),
			PeerStableID: entry.peer.GetStableID(),
			Files:        manifest,
		}},
	})
	if err != nil {
		return err
	}

	var uploadErr error
	uploadDone := make(chan struct{})
	go func() {
		defer close(uploadDone)
		failUpload := func(cause error) {
			uploadErr = cause
			cancel()
		}
		buffer := make([]byte, daemon.TaildropChunkSize)
		for _, file := range files {
			sourceFile, openErr := os.Open(file.path)
			if openErr != nil {
				failUpload(openErr)
				return
			}
			for {
				n, readErr := sourceFile.Read(buffer)
				if n > 0 {
					sendErr := stream.Send(&daemon.TaildropSendClientMessage{
						Message: &daemon.TaildropSendClientMessage_Chunk{Chunk: &daemon.TaildropFileChunk{Data: buffer[:n]}},
					})
					if sendErr != nil {
						sourceFile.Close()
						failUpload(sendErr)
						return
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					sourceFile.Close()
					failUpload(readErr)
					return
				}
			}
			sourceFile.Close()
			sendErr := stream.Send(&daemon.TaildropSendClientMessage{
				Message: &daemon.TaildropSendClientMessage_FileDone{FileDone: &daemon.TaildropFileDone{}},
			})
			if sendErr != nil {
				failUpload(sendErr)
				return
			}
		}
	}()

	peerName := tailscalePeerName(entry.peer)
	for {
		message, recvErr := stream.Recv()
		if recvErr != nil {
			cancel()
			<-uploadDone
			if uploadErr != nil && !E.IsCanceled(uploadErr) && status.Code(uploadErr) != codes.Canceled {
				return uploadErr
			}
			if ctx.Err() != nil {
				return E.New("interrupted")
			}
			if recvErr == io.EOF {
				writeProgress("")
				writeStderrLine(F.ToString("sent ", len(files), " file(s) to ", peerName))
				return nil
			}
			return recvErr
		}
		progress := message.GetProgress()
		if progress == nil {
			continue
		}
		fileIndex := progress.FileIndex
		if fileIndex < 0 || int(fileIndex) >= len(files) {
			return E.New("invalid file index: ", fileIndex)
		}
		file := files[fileIndex]
		if progress.FileCompleted {
			writeProgress("")
			writeStderrLine(F.ToString(file.name, ": done"))
		} else {
			writeProgress(F.ToString(file.name, ": ", formatTaildropSize(progress.SentBytes), " / ", formatTaildropSize(file.size), "    "))
		}
	}
}
