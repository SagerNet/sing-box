package libbox

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

const fdroidUserAgent = "F-Droid 1.21.1"

type FDroidUpdateInfo struct {
	VersionCode int32
	VersionName string
	DownloadURL string
	FileSize    int64
	FileSHA256  string
}

type FDroidPingResult struct {
	URL       string
	LatencyMs int32
	Error     string
}

type FDroidPingResultIterator interface {
	Len() int32
	HasNext() bool
	Next() *FDroidPingResult
}

type fdroidAPIResponse struct {
	PackageName          string             `json:"packageName"`
	SuggestedVersionCode int32              `json:"suggestedVersionCode"`
	Packages             []fdroidAPIPackage `json:"packages"`
}

type fdroidAPIPackage struct {
	VersionName string `json:"versionName"`
	VersionCode int32  `json:"versionCode"`
}

type fdroidEntry struct {
	Timestamp int64                      `json:"timestamp"`
	Version   int                        `json:"version"`
	Index     fdroidEntryFile            `json:"index"`
	Diffs     map[string]fdroidEntryFile `json:"diffs"`
}

type fdroidEntryFile struct {
	Name        string `json:"name"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	NumPackages int    `json:"numPackages"`
}

type fdroidIndexV2 struct {
	Packages map[string]fdroidV2Package `json:"packages"`
}

type fdroidV2Package struct {
	Versions map[string]fdroidV2Version `json:"versions"`
}

type fdroidV2Version struct {
	Manifest fdroidV2Manifest `json:"manifest"`
	File     fdroidV2File     `json:"file"`
}

type fdroidV2Manifest struct {
	VersionCode int32  `json:"versionCode"`
	VersionName string `json:"versionName"`
}

type fdroidV2File struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type fdroidIndexV1 struct {
	Packages map[string][]fdroidV1Package `json:"packages"`
}

type fdroidV1Package struct {
	VersionCode int32  `json:"versionCode"`
	VersionName string `json:"versionName"`
	ApkName     string `json:"apkName"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	HashType    string `json:"hashType"`
}

type fdroidCache struct {
	MirrorURL string `json:"mirrorURL"`
	Timestamp int64  `json:"timestamp"`
	ETag      string `json:"etag"`
	IsV1      bool   `json:"isV1,omitempty"`
}

func CheckFDroidUpdate(mirrorURL, packageName string, currentVersionCode int32, cachePath string) (*FDroidUpdateInfo, error) {
	mirrorURL = strings.TrimRight(mirrorURL, "/")
	if strings.Contains(mirrorURL, "f-droid.org") {
		return checkFDroidAPI(mirrorURL, packageName, currentVersionCode)
	}
	client := newFDroidHTTPClient()
	defer client.CloseIdleConnections()
	cache := loadFDroidCache(cachePath, mirrorURL)
	if cache != nil && cache.IsV1 {
		return checkFDroidV1(client, mirrorURL, packageName, currentVersionCode, cachePath, cache)
	}
	return checkFDroidV2(client, mirrorURL, packageName, currentVersionCode, cachePath, cache)
}

func PingFDroidMirrors(mirrorURLs string) (FDroidPingResultIterator, error) {
	urls := strings.Split(mirrorURLs, ",")
	results := make([]*FDroidPingResult, len(urls))
	var waitGroup sync.WaitGroup
	for i, rawURL := range urls {
		waitGroup.Add(1)
		go func(index int, target string) {
			defer waitGroup.Done()
			target = strings.TrimSpace(target)
			result := &FDroidPingResult{URL: target}
			latency, err := pingTLS(target)
			if err != nil {
				result.LatencyMs = -1
				result.Error = err.Error()
			} else {
				result.LatencyMs = int32(latency.Milliseconds())
			}
			results[index] = result
		}(i, rawURL)
	}
	waitGroup.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].LatencyMs < 0 {
			return false
		}
		if results[j].LatencyMs < 0 {
			return true
		}
		return results[i].LatencyMs < results[j].LatencyMs
	})
	return newIterator(results), nil
}

func PingFDroidMirror(mirrorURL string) *FDroidPingResult {
	mirrorURL = strings.TrimSpace(mirrorURL)
	result := &FDroidPingResult{URL: mirrorURL}
	latency, err := pingTLS(mirrorURL)
	if err != nil {
		result.LatencyMs = -1
		result.Error = err.Error()
	} else {
		result.LatencyMs = int32(latency.Milliseconds())
	}
	return result
}

func newFDroidHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

func newFDroidRequest(requestURL string) (*http.Request, error) {
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", fdroidUserAgent)
	return request, nil
}

func checkFDroidAPI(mirrorURL, packageName string, currentVersionCode int32) (*FDroidUpdateInfo, error) {
	client := newFDroidHTTPClient()
	defer client.CloseIdleConnections()

	apiURL := "https://f-droid.org/api/v1/packages/" + packageName
	request, err := newFDroidRequest(apiURL)
	if err != nil {
		return nil, err
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, E.New("HTTP ", response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse fdroidAPIResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, err
	}

	var bestCode int32
	var bestName string
	for _, pkg := range apiResponse.Packages {
		if pkg.VersionCode > currentVersionCode && pkg.VersionCode > bestCode {
			bestCode = pkg.VersionCode
			bestName = pkg.VersionName
		}
	}

	if bestCode == 0 {
		return nil, nil
	}

	return &FDroidUpdateInfo{
		VersionCode: bestCode,
		VersionName: bestName,
		DownloadURL: "https://f-droid.org/repo/" + packageName + "_" + strconv.FormatInt(int64(bestCode), 10) + ".apk",
	}, nil
}

func checkFDroidV2(client *http.Client, mirrorURL, packageName string, currentVersionCode int32, cachePath string, cache *fdroidCache) (*FDroidUpdateInfo, error) {
	entryURL := mirrorURL + "/entry.jar"
	request, err := newFDroidRequest(entryURL)
	if err != nil {
		return nil, err
	}
	if cache != nil && cache.ETag != "" {
		request.Header.Set("If-None-Match", cache.ETag)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if response.StatusCode == http.StatusNotFound {
		writeFDroidCache(cachePath, mirrorURL, 0, "", true)
		return checkFDroidV1(client, mirrorURL, packageName, currentVersionCode, cachePath, nil)
	}
	if response.StatusCode != http.StatusOK {
		return nil, E.New("HTTP ", response.Status, ": ", entryURL)
	}

	jarData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	etag := response.Header.Get("ETag")

	var entry fdroidEntry
	err = readJSONFromJar(jarData, "entry.json", &entry)
	if err != nil {
		return nil, E.Cause(err, "read entry.jar")
	}

	if entry.Timestamp == 0 {
		return nil, E.New("entry.json not found in entry.jar")
	}

	if cache != nil && cache.Timestamp == entry.Timestamp {
		writeFDroidCache(cachePath, mirrorURL, entry.Timestamp, etag, false)
		return nil, nil
	}

	var indexURL string
	if cache != nil {
		cachedTimestamp := strconv.FormatInt(cache.Timestamp, 10)
		if diff, ok := entry.Diffs[cachedTimestamp]; ok {
			indexURL = mirrorURL + "/" + diff.Name
		}
	}
	if indexURL == "" {
		indexURL = mirrorURL + "/" + entry.Index.Name
	}

	indexRequest, err := newFDroidRequest(indexURL)
	if err != nil {
		return nil, err
	}

	indexResponse, err := client.Do(indexRequest)
	if err != nil {
		return nil, err
	}
	defer indexResponse.Body.Close()

	if indexResponse.StatusCode != http.StatusOK {
		return nil, E.New("HTTP ", indexResponse.Status, ": ", indexURL)
	}

	indexData, err := io.ReadAll(indexResponse.Body)
	if err != nil {
		return nil, err
	}

	var index fdroidIndexV2
	err = json.Unmarshal(indexData, &index)
	if err != nil {
		return nil, err
	}

	writeFDroidCache(cachePath, mirrorURL, entry.Timestamp, etag, false)

	pkg, ok := index.Packages[packageName]
	if !ok {
		return nil, nil
	}

	var bestCode int32
	var bestVersion fdroidV2Version
	for _, version := range pkg.Versions {
		if version.Manifest.VersionCode > currentVersionCode && version.Manifest.VersionCode > bestCode {
			bestCode = version.Manifest.VersionCode
			bestVersion = version
		}
	}

	if bestCode == 0 {
		return nil, nil
	}

	return &FDroidUpdateInfo{
		VersionCode: bestCode,
		VersionName: bestVersion.Manifest.VersionName,
		DownloadURL: mirrorURL + "/" + bestVersion.File.Name,
		FileSize:    bestVersion.File.Size,
		FileSHA256:  bestVersion.File.SHA256,
	}, nil
}

func checkFDroidV1(client *http.Client, mirrorURL, packageName string, currentVersionCode int32, cachePath string, cache *fdroidCache) (*FDroidUpdateInfo, error) {
	indexURL := mirrorURL + "/index-v1.jar"

	request, err := newFDroidRequest(indexURL)
	if err != nil {
		return nil, err
	}
	if cache != nil && cache.ETag != "" {
		request.Header.Set("If-None-Match", cache.ETag)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, E.New("HTTP ", response.Status, ": ", indexURL)
	}

	jarData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	etag := response.Header.Get("ETag")

	var index fdroidIndexV1
	err = readJSONFromJar(jarData, "index-v1.json", &index)
	if err != nil {
		return nil, E.Cause(err, "read index-v1.jar")
	}

	writeFDroidCache(cachePath, mirrorURL, 0, etag, true)

	packages, ok := index.Packages[packageName]
	if !ok {
		return nil, nil
	}

	var bestCode int32
	var bestPackage fdroidV1Package
	for _, pkg := range packages {
		if pkg.VersionCode > currentVersionCode && pkg.VersionCode > bestCode {
			bestCode = pkg.VersionCode
			bestPackage = pkg
		}
	}

	if bestCode == 0 {
		return nil, nil
	}

	return &FDroidUpdateInfo{
		VersionCode: bestCode,
		VersionName: bestPackage.VersionName,
		DownloadURL: mirrorURL + "/" + bestPackage.ApkName,
		FileSize:    bestPackage.Size,
		FileSHA256:  bestPackage.Hash,
	}, nil
}

func readJSONFromJar(jarData []byte, fileName string, destination any) error {
	zipReader, err := zip.NewReader(bytes.NewReader(jarData), int64(len(jarData)))
	if err != nil {
		return err
	}
	for _, file := range zipReader.File {
		if file.Name != fileName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return err
		}
		return json.Unmarshal(data, destination)
	}
	return nil
}

func pingTLS(mirrorURL string) (time.Duration, error) {
	parsed, err := url.Parse(mirrorURL)
	if err != nil {
		return 0, err
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", host, &tls.Config{})
	if err != nil {
		return 0, err
	}
	latency := time.Since(start)
	conn.Close()
	return latency, nil
}

func loadFDroidCache(cachePath, mirrorURL string) *fdroidCache {
	cacheFile := filepath.Join(cachePath, "fdroid_cache.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}
	var cache fdroidCache
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil
	}
	if cache.MirrorURL != mirrorURL {
		return nil
	}
	return &cache
}

func writeFDroidCache(cachePath, mirrorURL string, timestamp int64, etag string, isV1 bool) {
	cache := fdroidCache{
		MirrorURL: mirrorURL,
		Timestamp: timestamp,
		ETag:      etag,
		IsV1:      isV1,
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	os.MkdirAll(cachePath, 0o755)
	os.WriteFile(filepath.Join(cachePath, "fdroid_cache.json"), data, 0o644)
}
