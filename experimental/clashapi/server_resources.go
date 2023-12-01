package clashapi

import (
	"archive/zip"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service/filemanager"
)

func (s *Server) checkAndDownloadExternalUI() {
	if s.externalUI == "" {
		return
	}
	entries, err := os.ReadDir(s.externalUI)
	if err != nil {
		os.MkdirAll(s.externalUI, 0o755)
	}
	if len(entries) == 0 {
		err = s.downloadExternalUI()
		if err != nil {
			s.logger.Error("download external ui error: ", err)
		}
	}
}

func (s *Server) downloadExternalUI() error {
	var downloadURL string
	if s.externalUIDownloadURL != "" {
		downloadURL = s.externalUIDownloadURL
	} else {
		downloadURL = "https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip"
	}
	s.logger.Info("downloading external ui")
	var detour adapter.Outbound
	if s.externalUIDownloadDetour != "" {
		outbound, loaded := s.router.Outbound(s.externalUIDownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", s.externalUIDownloadDetour)
		}
		detour = outbound
	} else {
		outbound, err := s.router.DefaultOutbound(N.NetworkTCP)
		if err != nil {
			return err
		}
		detour = outbound
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	response, err := httpClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return E.New("download external ui failed: ", response.Status)
	}
	err = s.downloadZIP(filepath.Base(downloadURL), response.Body, s.externalUI)
	if err != nil {
		removeAllInDirectory(s.externalUI)
	}
	return err
}

func (s *Server) downloadZIP(name string, body io.Reader, output string) error {
	tempFile, err := filemanager.CreateTemp(s.ctx, name)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	_, err = io.Copy(tempFile, body)
	tempFile.Close()
	if err != nil {
		return err
	}
	reader, err := zip.OpenReader(tempFile.Name())
	if err != nil {
		return err
	}
	defer reader.Close()
	trimDir := zipIsInSingleDirectory(reader.File)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		pathElements := strings.Split(file.Name, "/")
		if trimDir {
			pathElements = pathElements[1:]
		}
		saveDirectory := output
		if len(pathElements) > 1 {
			saveDirectory = filepath.Join(saveDirectory, filepath.Join(pathElements[:len(pathElements)-1]...))
		}
		err = os.MkdirAll(saveDirectory, 0o755)
		if err != nil {
			return err
		}
		savePath := filepath.Join(saveDirectory, pathElements[len(pathElements)-1])
		err = downloadZIPEntry(s.ctx, file, savePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func downloadZIPEntry(ctx context.Context, zipFile *zip.File, savePath string) error {
	saveFile, err := filemanager.Create(ctx, savePath)
	if err != nil {
		return err
	}
	defer saveFile.Close()
	reader, err := zipFile.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return common.Error(io.Copy(saveFile, reader))
}

func removeAllInDirectory(directory string) {
	dirEntries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, dirEntry := range dirEntries {
		os.RemoveAll(filepath.Join(directory, dirEntry.Name()))
	}
}

func zipIsInSingleDirectory(files []*zip.File) bool {
	var singleDirectory string
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		pathElements := strings.Split(file.Name, "/")
		if len(pathElements) == 0 {
			return false
		}
		if singleDirectory == "" {
			singleDirectory = pathElements[0]
		} else if singleDirectory != pathElements[0] {
			return false
		}
	}
	return true
}
