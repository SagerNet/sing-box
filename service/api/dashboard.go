package api

import (
	"archive/zip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
)

const (
	dashboardRoutePrefix  = "/dashboard/"
	dashboardEtagFileName = ".etag"
	defaultDashboardURL   = "https://github.com/SagerNet/sing-box-dashboard/archive/refs/heads/gh-pages.zip"
)

type dashboardStatus int

const (
	dashboardEmpty dashboardStatus = iota
	dashboardManaged
	dashboardUserProvided
)

type dashboard struct {
	ctx            context.Context
	cancel         context.CancelFunc
	logger         log.ContextLogger
	options        option.APIDashboardOptions
	path           string
	url            string
	updateInterval time.Duration
	fileServer     http.Handler
	httpClient     *http.Client
	lastEtag       string
	lastUpdated    time.Time
}

func newDashboard(ctx context.Context, logger log.ContextLogger, options option.APIDashboardOptions) *dashboard {
	ctx, cancel := context.WithCancel(ctx)
	path := options.Path
	if path == "" {
		path = "dashboard"
	}
	path = filemanager.BasePath(ctx, os.ExpandEnv(path))
	url := options.DownloadURL
	if url == "" {
		url = defaultDashboardURL
	}
	updateInterval := 24 * time.Hour
	if options.UpdateInterval > 0 {
		updateInterval = time.Duration(options.UpdateInterval)
	}
	return &dashboard{
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		options:        options,
		path:           path,
		url:            url,
		updateInterval: updateInterval,
		fileServer:     http.StripPrefix(dashboardRoutePrefix, http.FileServer(dashboardDir(path))),
	}
}

func (d *dashboard) start() error {
	transport, err := d.resolveTransport()
	if err != nil {
		return E.Cause(err, "create dashboard http client")
	}
	d.httpClient = &http.Client{Transport: transport}
	go d.loopUpdate()
	return nil
}

func (d *dashboard) close() error {
	d.cancel()
	if d.httpClient != nil {
		d.httpClient.CloseIdleConnections()
	}
	return nil
}

func (d *dashboard) resolveTransport() (adapter.HTTPTransport, error) {
	httpClientManager := service.FromContext[adapter.HTTPClientManager](d.ctx)
	if httpClientManager == nil {
		return nil, E.New("missing http client manager in context")
	}
	if d.options.HTTPClient != nil && !d.options.HTTPClient.IsEmpty() {
		return httpClientManager.ResolveTransport(d.ctx, d.logger, *d.options.HTTPClient)
	}
	defaultTransport := httpClientManager.DefaultTransport()
	if defaultTransport == nil {
		return nil, E.New("default http client transport is not initialized")
	}
	return defaultTransport, nil
}

func (d *dashboard) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, dashboardRoutePrefix) {
		d.fileServer.ServeHTTP(writer, request)
		return
	}
	http.Redirect(writer, request, dashboardRoutePrefix, http.StatusFound)
}

func (d *dashboard) loopUpdate() {
	status := d.loadState()
	if status == dashboardUserProvided {
		d.logger.Info("dashboard: serving user-provided files at ", d.path, ", auto-update disabled")
		return
	}
	var nextUpdate time.Time
	if status == dashboardManaged {
		nextUpdate = d.lastUpdated.Add(d.updateInterval)
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-timer.C:
		}
		now := time.Now()
		if !now.Before(nextUpdate) {
			err := d.fetch(d.ctx)
			if err != nil {
				d.logger.Error(E.Cause(err, "update dashboard"))
				nextUpdate = now.Add(d.updateInterval)
			} else {
				nextUpdate = d.lastUpdated.Add(d.updateInterval)
			}
		}
		timer.Reset(max(time.Until(nextUpdate), 0))
	}
}

func (d *dashboard) loadState() dashboardStatus {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return dashboardEmpty
	}
	if len(entries) == 0 {
		return dashboardEmpty
	}
	etagPath := filepath.Join(d.path, dashboardEtagFileName)
	etagBytes, err := os.ReadFile(etagPath)
	if err != nil {
		return dashboardUserProvided
	}
	d.lastEtag = strings.TrimSpace(string(etagBytes))
	info, err := os.Stat(etagPath)
	if err == nil {
		d.lastUpdated = info.ModTime()
	}
	return dashboardManaged
}

func (d *dashboard) fetch(ctx context.Context) error {
	d.logger.Info("updating dashboard from URL: ", d.url)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return err
	}
	if d.lastEtag != "" {
		request.Header.Set("If-None-Match", d.lastEtag)
	}
	defer d.httpClient.CloseIdleConnections()
	response, err := d.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		d.lastUpdated = time.Now()
		err = filemanager.WriteFile(d.ctx, filepath.Join(d.path, dashboardEtagFileName), []byte(d.lastEtag), 0o644)
		if err != nil {
			d.logger.Warn(E.Cause(err, "save dashboard update time"))
		}
		d.logger.Info("dashboard: not modified")
		return nil
	default:
		return E.New("unexpected status: ", response.Status)
	}
	etag := response.Header.Get("Etag")
	err = d.extract(response.Body, etag)
	if err != nil {
		return err
	}
	d.lastEtag = etag
	d.lastUpdated = time.Now()
	d.logger.Info("dashboard: updated")
	return nil
}

func (d *dashboard) extract(body io.Reader, etag string) error {
	tempFile, err := filemanager.CreateTemp(d.ctx, "sing-box-dashboard-*.zip")
	if err != nil {
		return err
	}
	tempZipPath := tempFile.Name()
	defer os.Remove(tempZipPath)
	_, err = io.Copy(tempFile, body)
	tempFile.Close()
	if err != nil {
		return err
	}
	reader, err := zip.OpenReader(tempZipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	tempDir := d.path + ".tmp"
	err = filemanager.RemoveAll(d.ctx, tempDir)
	if err != nil {
		return err
	}
	err = filemanager.MkdirAll(d.ctx, tempDir, 0o755)
	if err != nil {
		return err
	}
	trimDir := zipIsInSingleDirectory(reader.File)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		pathElements := strings.Split(file.Name, "/")
		if trimDir {
			pathElements = pathElements[1:]
		}
		if len(pathElements) == 0 {
			continue
		}
		relativePath := filepath.Join(pathElements...)
		if !filepath.IsLocal(relativePath) {
			filemanager.RemoveAll(d.ctx, tempDir)
			return E.New("invalid dashboard archive entry: ", file.Name)
		}
		savePath := filepath.Join(tempDir, relativePath)
		err = filemanager.MkdirAll(d.ctx, filepath.Dir(savePath), 0o755)
		if err != nil {
			filemanager.RemoveAll(d.ctx, tempDir)
			return err
		}
		err = extractZipEntry(d.ctx, file, savePath)
		if err != nil {
			filemanager.RemoveAll(d.ctx, tempDir)
			return err
		}
	}
	err = filemanager.WriteFile(d.ctx, filepath.Join(tempDir, dashboardEtagFileName), []byte(etag), 0o644)
	if err != nil {
		filemanager.RemoveAll(d.ctx, tempDir)
		return err
	}
	err = filemanager.RemoveAll(d.ctx, d.path)
	if err != nil {
		return err
	}
	return os.Rename(tempDir, d.path)
}

func extractZipEntry(ctx context.Context, file *zip.File, savePath string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	writer, err := filemanager.Create(ctx, savePath)
	if err != nil {
		return err
	}
	defer writer.Close()
	_, err = io.Copy(writer, reader)
	return err
}

// GitHub archives wrap every file under a single "<repo>-<branch>/" top-level directory.
func zipIsInSingleDirectory(files []*zip.File) bool {
	var dirName string
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		pathElements := strings.Split(file.Name, "/")
		if len(pathElements) < 2 {
			return false
		}
		if dirName == "" {
			dirName = pathElements[0]
		} else if dirName != pathElements[0] {
			return false
		}
	}
	return dirName != ""
}
