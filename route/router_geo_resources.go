package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/deprecated"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service/filemanager"
)

func (r *Router) GeoIPReader() *geoip.Reader {
	return r.geoIPReader
}

func (r *Router) LoadGeosite(code string) (adapter.Rule, error) {
	rule, cached := r.geositeCache[code]
	if cached {
		return rule, nil
	}
	items, err := r.geositeReader.Read(code)
	if err != nil {
		return nil, err
	}
	rule, err = NewDefaultRule(r.ctx, r, nil, geosite.Compile(items))
	if err != nil {
		return nil, err
	}
	r.geositeCache[code] = rule
	return rule, nil
}

func (r *Router) prepareGeoIPDatabase() error {
	deprecated.Report(r.ctx, deprecated.OptionGEOIP)
	var geoPath string
	if r.geoIPOptions.Path != "" {
		geoPath = r.geoIPOptions.Path
	} else {
		geoPath = "geoip.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	if !rw.IsFile(geoPath) {
		geoPath = filemanager.BasePath(r.ctx, geoPath)
	}
	if stat, err := os.Stat(geoPath); err == nil {
		if stat.IsDir() {
			return E.New("geoip path is a directory: ", geoPath)
		}
		if stat.Size() == 0 {
			os.Remove(geoPath)
		}
	}
	if !rw.IsFile(geoPath) {
		r.logger.Warn("geoip database not exists: ", geoPath)
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			err = r.downloadGeoIPDatabase(geoPath)
			if err == nil {
				break
			}
			r.logger.Error("download geoip database: ", err)
			os.Remove(geoPath)
			// time.Sleep(10 * time.Second)
		}
		if err != nil {
			return err
		}
	}
	geoReader, codes, err := geoip.Open(geoPath)
	if err != nil {
		return E.Cause(err, "open geoip database")
	}
	r.logger.Info("loaded geoip database: ", len(codes), " codes")
	r.geoIPReader = geoReader
	return nil
}

func (r *Router) prepareGeositeDatabase() error {
	deprecated.Report(r.ctx, deprecated.OptionGEOSITE)
	var geoPath string
	if r.geositeOptions.Path != "" {
		geoPath = r.geositeOptions.Path
	} else {
		geoPath = "geosite.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	if !rw.IsFile(geoPath) {
		geoPath = filemanager.BasePath(r.ctx, geoPath)
	}
	if stat, err := os.Stat(geoPath); err == nil {
		if stat.IsDir() {
			return E.New("geoip path is a directory: ", geoPath)
		}
		if stat.Size() == 0 {
			os.Remove(geoPath)
		}
	}
	if !rw.IsFile(geoPath) {
		r.logger.Warn("geosite database not exists: ", geoPath)
		var err error
		for attempts := 0; attempts < 3; attempts++ {
			err = r.downloadGeositeDatabase(geoPath)
			if err == nil {
				break
			}
			r.logger.Error("download geosite database: ", err)
			os.Remove(geoPath)
		}
		if err != nil {
			return err
		}
	}
	geoReader, codes, err := geosite.Open(geoPath)
	if err == nil {
		r.logger.Info("loaded geosite database: ", len(codes), " codes")
		r.geositeReader = geoReader
	} else {
		return E.Cause(err, "open geosite database")
	}
	return nil
}

func (r *Router) downloadGeoIPDatabase(savePath string) error {
	var downloadURL string
	if r.geoIPOptions.DownloadURL != "" {
		downloadURL = r.geoIPOptions.DownloadURL
	} else {
		downloadURL = "https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
	}
	r.logger.Info("downloading geoip database")
	var detour adapter.Outbound
	if r.geoIPOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geoIPOptions.DownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", r.geoIPOptions.DownloadDetour)
		}
		detour = outbound
	} else {
		detour = r.defaultOutboundForConnection
	}

	if parentDir := filepath.Dir(savePath); parentDir != "" {
		filemanager.MkdirAll(r.ctx, parentDir, 0o755)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: C.TCPTimeout,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	request, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request.WithContext(r.ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	saveFile, err := filemanager.Create(r.ctx, savePath)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	_, err = io.Copy(saveFile, response.Body)
	saveFile.Close()
	if err != nil {
		filemanager.Remove(r.ctx, savePath)
	}
	return err
}

func (r *Router) downloadGeositeDatabase(savePath string) error {
	var downloadURL string
	if r.geositeOptions.DownloadURL != "" {
		downloadURL = r.geositeOptions.DownloadURL
	} else {
		downloadURL = "https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
	}
	r.logger.Info("downloading geosite database")
	var detour adapter.Outbound
	if r.geositeOptions.DownloadDetour != "" {
		outbound, loaded := r.Outbound(r.geositeOptions.DownloadDetour)
		if !loaded {
			return E.New("detour outbound not found: ", r.geositeOptions.DownloadDetour)
		}
		detour = outbound
	} else {
		detour = r.defaultOutboundForConnection
	}

	if parentDir := filepath.Dir(savePath); parentDir != "" {
		filemanager.MkdirAll(r.ctx, parentDir, 0o755)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: C.TCPTimeout,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return detour.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
		},
	}
	defer httpClient.CloseIdleConnections()
	request, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request.WithContext(r.ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	saveFile, err := filemanager.Create(r.ctx, savePath)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	_, err = io.Copy(saveFile, response.Body)
	saveFile.Close()
	if err != nil {
		filemanager.Remove(r.ctx, savePath)
	}
	return err
}
