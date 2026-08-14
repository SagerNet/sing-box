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
)

var commandAPITailscaleTaildropGet = &cobra.Command{
	Use:   "get <name> [output]",
	Short: "Save a received file",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var outputPath string
		if len(args) > 1 {
			outputPath = args[1]
		}
		return runAPITaildropGet(args[0], outputPath)
	},
}

func init() {
	commandAPITailscaleTaildrop.AddCommand(commandAPITailscaleTaildropGet)
}

func runAPITaildropGet(name string, outputPath string) error {
	clientConn, client, err := createAPIClient()
	if err != nil {
		return err
	}
	defer clientConn.Close()
	endpointTag, err := resolveTailscaleEndpointTag(client)
	if err != nil {
		return err
	}
	if outputPath == "" {
		outputPath = name
	} else {
		information, statErr := os.Stat(outputPath)
		if statErr == nil && information.IsDir() {
			outputPath = filepath.Join(outputPath, name)
		}
	}
	ctx, cancel := signal.NotifyContext(globalCtx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stream, err := client.DownloadTaildropFile(ctx, &daemon.DownloadTaildropFileRequest{
		EndpointTag: endpointTag,
		Name:        name,
	})
	if err != nil {
		return err
	}
	firstChunk, err := stream.Recv()
	if err != nil {
		return err
	}
	totalSize := firstChunk.GetSize()
	var outputFile *os.File
	if outputPath == "-" {
		outputFile = os.Stdout
	} else {
		outputFile, err = os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
	}
	var downloaded int64
	writeChunk := func(data []byte) error {
		if len(data) == 0 {
			return nil
		}
		_, writeErr := outputFile.Write(data)
		if writeErr != nil {
			return writeErr
		}
		downloaded += int64(len(data))
		writeProgress(F.ToString(name, ": ", formatTaildropSize(downloaded), " / ", formatTaildropSize(totalSize), "    "))
		return nil
	}
	err = writeChunk(firstChunk.GetData())
	for err == nil {
		var chunk *daemon.DownloadTaildropFileChunk
		chunk, err = stream.Recv()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			break
		}
		err = writeChunk(chunk.GetData())
	}
	if outputPath != "-" {
		closeErr := outputFile.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(outputPath)
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return E.New("interrupted")
		}
		return err
	}
	writeProgress("")
	writeStderrLine(F.ToString("saved ", name, " (", formatTaildropSize(downloaded), ")"))
	return nil
}
