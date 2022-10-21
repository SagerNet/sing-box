package conf

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	myjson "github.com/sagernet/sing-box/common/json"
	E "github.com/sagernet/sing/common/exceptions"

	"github.com/qjebbs/go-jsons"
)

var (
	// formatName is the format name of JSON.
	formatName jsons.Format = "json"
	// formatExtensions are the extension names of JSON format.
	formatExtensions = []string{".json", ".jsonc"}
)

// Merge merges inputs into a single json.
func Merge(inputs ...interface{}) ([]byte, error) {
	return newMerger().MergeAs(formatName, inputs...)
}

// NewMerger creates a new json files Merger.
func newMerger() *jsons.Merger {
	m := jsons.NewMerger()
	m.RegisterLoader(
		formatName,
		formatExtensions,
		func(b []byte) (map[string]interface{}, error) {
			m := make(map[string]interface{})
			decoder := json.NewDecoder(myjson.NewCommentFilter(bytes.NewReader(b)))
			err := decoder.Decode(&m)
			if err != nil {
				return nil, err
			}
			return m, nil
		},
	)
	return m
}

// ResolveFiles expands folder path (if any and it exists) to file paths.
// Any other paths, like file, even URL, it returns them as is.
func ResolveFiles(paths []string, recursively bool) ([]string, error) {
	return resolveFiles(paths, formatExtensions, recursively)
}

func resolveFiles(paths []string, extensions []string, recursively bool) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	dirReader := readDir
	if recursively {
		dirReader = readDirRecursively
	}
	files := make([]string, 0)
	for _, p := range paths {
		if isRemote(p) {
			return nil, E.New("remote files are not supported")
		}
		if !isDir(p) {
			files = append(files, p)
			continue
		}
		fs, err := dirReader(p, extensions)
		if err != nil {
			return nil, E.Cause(err, "read dir")
		}
		files = append(files, fs...)
	}
	return files, nil
}

// readDir finds files according to extensions in the dir
func readDir(dir string, extensions []string) ([]string, error) {
	confs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for _, f := range confs {
		ext := filepath.Ext(f.Name())
		for _, e := range extensions {
			if strings.EqualFold(ext, e) {
				files = append(files, filepath.Join(dir, f.Name()))
				break
			}
		}
	}
	return files, nil
}

// readDirRecursively finds files according to extensions in the dir recursively
func readDirRecursively(dir string, extensions []string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		ext := filepath.Ext(path)
		for _, e := range extensions {
			if strings.EqualFold(ext, e) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func isRemote(p string) bool {
	u, err := url.Parse(p)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func isDir(p string) bool {
	i, err := os.Stat(p)
	if err != nil {
		return false
	}
	return i.IsDir()
}
