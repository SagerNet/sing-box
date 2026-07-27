package main

import (
	"archive/zip"
	"crypto/sha256"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var outputPath string

func init() {
	flag.StringVar(&outputPath, "output", "", "output AAR path")
}

func main() {
	flag.Parse()
	err := merge()
	if err != nil {
		log.Fatal(err)
	}
}

func merge() error {
	inputPaths := flag.Args()
	if outputPath == "" {
		return E.New("missing output path")
	}
	if len(inputPaths) == 0 {
		return E.New("missing input AAR paths")
	}
	archiveReaders := make([]*zip.ReadCloser, 0, len(inputPaths))
	for _, inputPath := range inputPaths {
		archiveReader, err := zip.OpenReader(inputPath)
		if err != nil {
			return E.Cause(err, "open input AAR: ", inputPath)
		}
		archiveReaders = append(archiveReaders, archiveReader)
	}
	defer func() {
		for _, archiveReader := range archiveReaders {
			archiveReader.Close()
		}
	}()

	referenceEntries := make(map[string][sha256.Size]byte)
	selectedEntries := make([]*zip.File, 0)
	selectedJNIEntries := make(map[string]bool)
	for inputIndex, archiveReader := range archiveReaders {
		seenEntries := make(map[string]bool)
		for _, archiveFile := range archiveReader.File {
			if strings.HasPrefix(archiveFile.Name, "jni/") {
				if archiveFile.FileInfo().IsDir() {
					continue
				}
				if selectedJNIEntries[archiveFile.Name] {
					return E.New("duplicate AAR JNI entry: ", archiveFile.Name)
				}
				selectedJNIEntries[archiveFile.Name] = true
				selectedEntries = append(selectedEntries, archiveFile)
				continue
			}
			entryDigest, err := digestEntry(archiveFile)
			if err != nil {
				return E.Cause(err, "read AAR entry: ", archiveFile.Name)
			}
			if inputIndex == 0 {
				referenceEntries[archiveFile.Name] = entryDigest
				selectedEntries = append(selectedEntries, archiveFile)
			} else {
				referenceDigest, loaded := referenceEntries[archiveFile.Name]
				if !loaded {
					return E.New("unexpected AAR entry: ", archiveFile.Name)
				}
				if referenceDigest != entryDigest {
					return E.New("AAR entry differs between architectures: ", archiveFile.Name)
				}
			}
			seenEntries[archiveFile.Name] = true
		}
		if inputIndex > 0 {
			for referenceName := range referenceEntries {
				if !seenEntries[referenceName] {
					return E.New("missing AAR entry: ", referenceName)
				}
			}
		}
	}

	absoluteOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return E.Cause(err, "resolve output AAR path")
	}
	err = os.MkdirAll(filepath.Dir(absoluteOutputPath), 0o755)
	if err != nil {
		return E.Cause(err, "create output AAR directory")
	}
	temporaryFile, err := os.CreateTemp(filepath.Dir(absoluteOutputPath), ".merge-aar-*.aar")
	if err != nil {
		return E.Cause(err, "create temporary output AAR")
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	archiveWriter := zip.NewWriter(temporaryFile)
	for _, archiveFile := range selectedEntries {
		rawReader, openErr := archiveFile.OpenRaw()
		if openErr != nil {
			archiveWriter.Close()
			temporaryFile.Close()
			return E.Cause(openErr, "open raw AAR entry: ", archiveFile.Name)
		}
		header := archiveFile.FileHeader
		rawWriter, createErr := archiveWriter.CreateRaw(&header)
		if createErr != nil {
			archiveWriter.Close()
			temporaryFile.Close()
			return E.Cause(createErr, "create output AAR entry: ", archiveFile.Name)
		}
		_, copyErr := io.Copy(rawWriter, rawReader)
		if copyErr != nil {
			archiveWriter.Close()
			temporaryFile.Close()
			return E.Cause(copyErr, "copy output AAR entry: ", archiveFile.Name)
		}
	}
	err = archiveWriter.Close()
	if err != nil {
		temporaryFile.Close()
		return E.Cause(err, "finalize output AAR")
	}
	err = temporaryFile.Close()
	if err != nil {
		return E.Cause(err, "close output AAR")
	}
	err = os.Rename(temporaryPath, absoluteOutputPath)
	if err != nil {
		return E.Cause(err, "replace output AAR")
	}
	return nil
}

func digestEntry(archiveFile *zip.File) ([sha256.Size]byte, error) {
	entryReader, err := archiveFile.Open()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	digest := sha256.New()
	_, err = io.Copy(digest, entryReader)
	closeErr := entryReader.Close()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	if closeErr != nil {
		return [sha256.Size]byte{}, closeErr
	}
	var result [sha256.Size]byte
	copy(result[:], digest.Sum(nil))
	return result, nil
}
