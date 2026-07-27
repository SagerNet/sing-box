package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"

	"howett.net/plist"
)

type xcFrameworkInfo struct {
	AvailableLibraries []xcFrameworkLibrary `plist:"AvailableLibraries"`
}

type xcFrameworkLibrary struct {
	BinaryPath               string   `plist:"BinaryPath"`
	LibraryIdentifier        string   `plist:"LibraryIdentifier"`
	LibraryPath              string   `plist:"LibraryPath"`
	SupportedArchitectures   []string `plist:"SupportedArchitectures"`
	SupportedPlatform        string   `plist:"SupportedPlatform"`
	SupportedPlatformVariant string   `plist:"SupportedPlatformVariant"`
}

type frameworkSlice struct {
	rootPath string
	library  xcFrameworkLibrary
}

var outputPath string

func init() {
	flag.StringVar(&outputPath, "output", "", "output XCFramework path")
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
		return E.New("missing input XCFramework paths")
	}
	frameworkGroups := make(map[string][]frameworkSlice)
	for _, inputPath := range inputPaths {
		infoFile, err := os.Open(filepath.Join(inputPath, "Info.plist"))
		if err != nil {
			return E.Cause(err, "open XCFramework metadata: ", inputPath)
		}
		var info xcFrameworkInfo
		decoder := plist.NewDecoder(infoFile)
		err = decoder.Decode(&info)
		closeErr := infoFile.Close()
		if err != nil {
			return E.Cause(err, "decode XCFramework metadata: ", inputPath)
		}
		if closeErr != nil {
			return E.Cause(closeErr, "close XCFramework metadata: ", inputPath)
		}
		for _, library := range info.AvailableLibraries {
			groupName := library.SupportedPlatform + "|" + library.SupportedPlatformVariant
			frameworkGroups[groupName] = append(frameworkGroups[groupName], frameworkSlice{
				rootPath: inputPath,
				library:  library,
			})
		}
	}
	groupNames := make([]string, 0, len(frameworkGroups))
	for groupName := range frameworkGroups {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)
	absoluteOutputPath, err := filepath.Abs(outputPath)
	if err != nil {
		return E.Cause(err, "resolve output XCFramework path")
	}
	err = os.MkdirAll(filepath.Dir(absoluteOutputPath), 0o755)
	if err != nil {
		return E.Cause(err, "create output XCFramework directory")
	}
	temporaryDirectory, err := os.MkdirTemp(filepath.Dir(absoluteOutputPath), ".merge-xcframework-*")
	if err != nil {
		return E.Cause(err, "create XCFramework merge directory")
	}
	defer os.RemoveAll(temporaryDirectory)
	frameworkPaths := make([]string, 0, len(groupNames))
	for groupIndex, groupName := range groupNames {
		frameworkSlices := frameworkGroups[groupName]
		firstSlice := frameworkSlices[0]
		firstFrameworkPath := filepath.Join(firstSlice.rootPath, firstSlice.library.LibraryIdentifier, firstSlice.library.LibraryPath)
		if len(frameworkSlices) == 1 {
			frameworkPaths = append(frameworkPaths, firstFrameworkPath)
			continue
		}
		architectures := make(map[string]bool)
		binaryPaths := make([]string, 0, len(frameworkSlices))
		for _, currentSlice := range frameworkSlices {
			if currentSlice.library.LibraryPath != firstSlice.library.LibraryPath || currentSlice.library.BinaryPath != firstSlice.library.BinaryPath {
				return E.New("incompatible XCFramework slices for platform: ", currentSlice.library.SupportedPlatform)
			}
			for _, architecture := range currentSlice.library.SupportedArchitectures {
				if architectures[architecture] {
					return E.New("duplicate XCFramework architecture: ", architecture)
				}
				architectures[architecture] = true
			}
			binaryPaths = append(binaryPaths, filepath.Join(currentSlice.rootPath, currentSlice.library.LibraryIdentifier, currentSlice.library.BinaryPath))
		}
		mergedFrameworkPath := filepath.Join(temporaryDirectory, "framework-"+strconv.Itoa(groupIndex), filepath.Base(firstSlice.library.LibraryPath))
		copyCommand := exec.Command("ditto", firstFrameworkPath, mergedFrameworkPath)
		copyCommand.Stdout = os.Stdout
		copyCommand.Stderr = os.Stderr
		err = copyCommand.Run()
		if err != nil {
			return E.Cause(err, "copy XCFramework slice")
		}
		binaryRelativePath, relativeErr := filepath.Rel(firstSlice.library.LibraryPath, firstSlice.library.BinaryPath)
		if relativeErr != nil {
			return E.Cause(relativeErr, "resolve XCFramework binary path")
		}
		if binaryRelativePath == "." || strings.HasPrefix(binaryRelativePath, ".."+string(filepath.Separator)) {
			return E.New("invalid XCFramework binary path: ", firstSlice.library.BinaryPath)
		}
		mergedBinaryPath := filepath.Join(mergedFrameworkPath, binaryRelativePath)
		temporaryBinaryPath := mergedBinaryPath + ".merged"
		lipoArguments := append([]string{"lipo", "-create"}, binaryPaths...)
		lipoArguments = append(lipoArguments, "-output", temporaryBinaryPath)
		lipoCommand := exec.Command("xcrun", lipoArguments...)
		lipoCommand.Stdout = os.Stdout
		lipoCommand.Stderr = os.Stderr
		err = lipoCommand.Run()
		if err != nil {
			return E.Cause(err, "merge XCFramework binaries")
		}
		err = os.Rename(temporaryBinaryPath, mergedBinaryPath)
		if err != nil {
			return E.Cause(err, "replace merged XCFramework binary")
		}
		frameworkPaths = append(frameworkPaths, mergedFrameworkPath)
	}
	err = os.RemoveAll(absoluteOutputPath)
	if err != nil {
		return E.Cause(err, "remove output XCFramework")
	}
	xcodebuildArguments := []string{"-create-xcframework"}
	for _, frameworkPath := range frameworkPaths {
		xcodebuildArguments = append(xcodebuildArguments, "-framework", frameworkPath)
	}
	xcodebuildArguments = append(xcodebuildArguments, "-output", absoluteOutputPath)
	xcodebuildCommand := exec.Command("xcodebuild", xcodebuildArguments...)
	xcodebuildCommand.Stdout = os.Stdout
	xcodebuildCommand.Stderr = os.Stderr
	err = xcodebuildCommand.Run()
	if err != nil {
		return E.Cause(err, "create XCFramework")
	}
	return nil
}
