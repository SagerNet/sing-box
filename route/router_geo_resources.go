package route

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/geoip"
	"github.com/sagernet/sing-box/common/geosite"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service/filemanager"

	"github.com/fsnotify/fsnotify"
)

func (r *Router) TryUpdateGeoDatabase() {
	if r.needGeoIPDatabase {
		go func() {
			geoPath := r.loadGeoIPPath()
			tempGeoPath := geoPath + ".tmp"
			err := r.tryDownloadGeoIPDatabase(tempGeoPath)
			if err != nil {
				r.logger.Error(E.Cause(err, "download geoip database"))
				return
			}
			os.Rename(tempGeoPath, geoPath)
		}()
	}
	if r.needGeositeDatabase {
		go func() {
			geoPath := r.loadGeositePath()
			tempGeoPath := geoPath + ".tmp"
			err := r.tryDownloadGeositeDatabase(tempGeoPath)
			if err != nil {
				r.logger.Error(E.Cause(err, "download geosite database"))
				return
			}
			os.Rename(tempGeoPath, geoPath)
		}()
	}
}

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
	rule, err = NewDefaultRule(r, nil, geosite.Compile(items))
	if err != nil {
		return nil, err
	}
	r.geositeCache[code] = rule
	return rule, nil
}

func (r *Router) loadGeoIPPath() string {
	var geoPath string
	if r.geoIPOptions.Path != "" {
		geoPath = r.geoIPOptions.Path
	} else {
		geoPath = "geoip.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	if !rw.FileExists(geoPath) {
		geoPath = filemanager.BasePath(r.ctx, geoPath)
	}
	return geoPath
}

func (r *Router) prepareGeoIPDatabase() error {
	geoPath := r.loadGeoIPPath()
	if stat, err := os.Stat(geoPath); err == nil {
		if stat.IsDir() {
			return E.New("geoip path is a directory: ", geoPath)
		}
		if stat.Size() == 0 {
			os.Remove(geoPath)
		}
	}
	if !rw.FileExists(geoPath) {
		r.logger.Warn("geoip database not exists: ", geoPath)
		err := r.tryDownloadGeoIPDatabase(geoPath)
		if err != nil {
			return err
		}
	}
	return r.loadGeoIPDatabase(geoPath)
}

func (r *Router) tryDownloadGeoIPDatabase(geoPath string) error {
	os.Remove(geoPath)
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = r.downloadGeoIPDatabase(geoPath)
		if err == nil {
			r.logger.Info("download geoip database success")
			break
		}
		r.logger.Error("download geoip database: ", err)
		os.Remove(geoPath)
		// time.Sleep(10 * time.Second)
	}
	return err
}

func (r *Router) loadGeoIPDatabase(geoPath string) error {
	geoReader, codes, err := geoip.Open(geoPath)
	if err != nil {
		err = E.Cause(err, "open geoip database")
		return err
	}
	r.logger.Info("loaded geoip database: ", len(codes), " codes")
	r.geoIPReader = geoReader
	return nil
}

func (r *Router) loadGeositePath() string {
	var geoPath string
	if r.geositeOptions.Path != "" {
		geoPath = r.geositeOptions.Path
	} else {
		geoPath = "geosite.db"
		if foundPath, loaded := C.FindPath(geoPath); loaded {
			geoPath = foundPath
		}
	}
	if !rw.FileExists(geoPath) {
		geoPath = filemanager.BasePath(r.ctx, geoPath)
	}
	return geoPath
}

func (r *Router) prepareGeositeDatabase() error {
	geoPath := r.loadGeositePath()
	if stat, err := os.Stat(geoPath); err == nil {
		if stat.IsDir() {
			return E.New("geoip path is a directory: ", geoPath)
		}
		if stat.Size() == 0 {
			os.Remove(geoPath)
		}
	}
	if !rw.FileExists(geoPath) {
		r.logger.Warn("geosite database not exists: ", geoPath)
		err := r.tryDownloadGeositeDatabase(geoPath)
		if err != nil {
			return err
		}
	}
	return r.loadGeositeDatabase(geoPath)
}

func (r *Router) tryDownloadGeositeDatabase(geoPath string) error {
	os.Remove(geoPath)
	var err error
	for attempts := 0; attempts < 3; attempts++ {
		err = r.downloadGeositeDatabase(geoPath)
		if err == nil {
			r.logger.Info("download geosite database success")
			break
		}
		r.logger.Error("download geosite database: ", err)
		os.Remove(geoPath)
	}
	return err
}

func (r *Router) loadGeositeDatabase(geoPath string) error {
	geoReader, codes, err := geosite.Open(geoPath)
	if err != nil {
		return E.Cause(err, "open geosite database")
	}
	r.logger.Info("loaded geosite database: ", len(codes), " codes")
	r.geositeReader = geoReader
	return nil
}

func (r *Router) loadGeositeRule() {
	r.geositeCache = make(map[string]adapter.Rule)
	for _, rule := range r.rules {
		err := rule.UpdateGeosite()
		if err != nil {
			r.logger.Error("failed to initialize geosite: ", err)
		}
	}
	for _, rule := range r.dnsRules {
		err := rule.UpdateGeosite()
		if err != nil {
			r.logger.Error("failed to initialize geosite: ", err)
		}
	}
	err := common.Close(r.geositeReader)
	if err != nil {
		r.logger.Error("close geosite reader: ", err)
	}
	r.geositeCache = nil
	r.geositeReader = nil
}

func (r *Router) startGeoWatcher() error {
	geoWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	r.geoWatcher = geoWatcher
	var geoIPPath string
	if r.needGeoIPDatabase {
		geoIPPath = r.loadGeoIPPath()
		err = geoWatcher.Add(filepath.Dir(geoIPPath))
		if err != nil {
			return err
		}
		r.logger.Debug("geo resource watcher: watching ", geoIPPath)
	}
	var geositePath string
	if r.needGeositeDatabase {
		geositePath = r.loadGeositePath()
		err = geoWatcher.Add(filepath.Dir(geositePath))
		if err != nil {
			return err
		}
		r.logger.Debug("geo resource watcher: watching ", geositePath)
	}
	go r.loopGeoUpdate(geoIPPath, geositePath)
	r.logger.Debug("geo resource watcher started")
	return nil
}

func (r *Router) loopGeoUpdate(geoIPPath, geositePath string) {
	var geoIPUpdateLock sync.Mutex
	var geositeUpdateLock sync.Mutex
	for {
		select {
		case event, ok := <-r.geoWatcher.Events:
			if !ok {
				return
			}
			if !(r.needGeoIPDatabase && event.Name == geoIPPath) && !(r.needGeositeDatabase && event.Name == geositePath) {
				continue
			}
			if event.Op.Has(fsnotify.Remove | fsnotify.Chmod) {
				continue
			}
			if r.needGeoIPDatabase && event.Name == geoIPPath {
				if geoIPUpdateLock.TryLock() {
					go func() {
						defer geoIPUpdateLock.Unlock()
						r.logger.Info("geoip file changed, try to reload...")
						err := r.loadGeoIPDatabase(geoIPPath)
						if err != nil {
							r.logger.Error(E.Cause(err, "reload geoip database"))
							return
						}
						r.logger.Info("geoip database reloaded")
					}()
				}
			}
			if r.needGeositeDatabase && event.Name == geositePath {
				if geositeUpdateLock.TryLock() {
					go func() {
						defer geositeUpdateLock.Unlock()
						r.logger.Info("geosite file changed, try to reload...")
						err := r.loadGeositeDatabase(geositePath)
						if err != nil {
							r.logger.Error(E.Cause(err, "reload geosite database"))
							return
						}
						r.loadGeositeRule()
						r.logger.Info("geosite database reloaded")
					}()
				}
			}
		case err, ok := <-r.geoWatcher.Errors:
			if !ok {
				return
			}
			r.logger.Error(E.Cause(err, "geo resource watcher: fsnotify error"))
		}
	}
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

	saveFile, err := filemanager.Create(r.ctx, savePath)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	defer saveFile.Close()

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
	_, err = io.Copy(saveFile, response.Body)
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

	saveFile, err := filemanager.Create(r.ctx, savePath)
	if err != nil {
		return E.Cause(err, "open output file: ", downloadURL)
	}
	defer saveFile.Close()

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
	_, err = io.Copy(saveFile, response.Body)
	return err
}

func hasRule(rules []option.Rule, cond func(rule option.DefaultRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			for _, subRule := range rule.LogicalOptions.Rules {
				if cond(subRule) {
					return true
				}
			}
		}
	}
	return false
}

func hasDNSRule(rules []option.DNSRule, cond func(rule option.DefaultDNSRule) bool) bool {
	for _, rule := range rules {
		switch rule.Type {
		case C.RuleTypeDefault:
			if cond(rule.DefaultOptions) {
				return true
			}
		case C.RuleTypeLogical:
			for _, subRule := range rule.LogicalOptions.Rules {
				if cond(subRule) {
					return true
				}
			}
		}
	}
	return false
}

func isGeoIPRule(rule option.DefaultRule) bool {
	return len(rule.SourceGeoIP) > 0 && common.Any(rule.SourceGeoIP, notPrivateNode) || len(rule.GeoIP) > 0 && common.Any(rule.GeoIP, notPrivateNode)
}

func isGeoIPDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.SourceGeoIP) > 0 && common.Any(rule.SourceGeoIP, notPrivateNode)
}

func isGeositeRule(rule option.DefaultRule) bool {
	return len(rule.Geosite) > 0
}

func isGeositeDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.Geosite) > 0
}

func isProcessRule(rule option.DefaultRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.PackageName) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func isProcessDNSRule(rule option.DefaultDNSRule) bool {
	return len(rule.ProcessName) > 0 || len(rule.ProcessPath) > 0 || len(rule.PackageName) > 0 || len(rule.User) > 0 || len(rule.UserID) > 0
}

func notPrivateNode(code string) bool {
	return code != "private"
}
