package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// envFile returns the name of the Go environment configuration file.
// Copy from https://github.com/golang/go/blob/c4f2a9788a7be04daf931ac54382fbe2cb754938/src/cmd/go/internal/cfg/cfg.go#L150-L166
func envFile() (string, error) {
	if file := os.Getenv("GOENV"); file != "" {
		if file == "off" {
			return "", fmt.Errorf("GOENV=off")
		}
		return file, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", fmt.Errorf("missing user-config dir")
	}
	return filepath.Join(dir, "go", "env"), nil
}

// GetRuntimeEnv returns the value of runtime environment variable,
// that is set by running following command: `go env -w key=value`.
func GetRuntimeEnv(key string) (string, error) {
	file, err := envFile()
	if err != nil {
		return "", err
	}
	if file == "" {
		return "", fmt.Errorf("missing runtime env file")
	}
	var data []byte
	var runtimeEnv string
	data, readErr := os.ReadFile(file)
	if readErr != nil {
		return "", readErr
	}
	envStrings := strings.Split(string(data), "\n")
	for _, envItem := range envStrings {
		envItem = strings.TrimSuffix(envItem, "\r")
		envKeyValue := strings.Split(envItem, "=")
		if strings.EqualFold(strings.TrimSpace(envKeyValue[0]), key) {
			runtimeEnv = strings.TrimSpace(envKeyValue[1])
		}
	}
	return runtimeEnv, nil
}

// GetGOBIN returns GOBIN environment variable as a string. It will NOT be empty.
func GetGOBIN() string {
	// The one set by user explicitly by `export GOBIN=/path` or `env GOBIN=/path command`
	GOBIN := os.Getenv("GOBIN")
	if GOBIN == "" {
		var err error
		// The one set by user by running `go env -w GOBIN=/path`
		GOBIN, err = GetRuntimeEnv("GOBIN")
		if err != nil {
			// The default one that Golang uses
			return filepath.Join(build.Default.GOPATH, "bin")
		}
		if GOBIN == "" {
			return filepath.Join(build.Default.GOPATH, "bin")
		}
		return GOBIN
	}
	return GOBIN
}

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Can not get current working directory.")
		os.Exit(1)
	}

	GOBIN := GetGOBIN()
	binPath := os.Getenv("PATH")
	pathSlice := []string{pwd, GOBIN, binPath}
	binPath = strings.Join(pathSlice, string(os.PathListSeparator))
	os.Setenv("PATH", binPath)

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	protoc := "protoc"

	if linkPath, err := os.Readlink(protoc); err == nil {
		protoc = linkPath
	}

	protoFilesMap := make(map[string][]string)
	walkErr := filepath.Walk("./", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if info.IsDir() {
			return nil
		}

		dir := filepath.Dir(path)
		filename := filepath.Base(path)
		if strings.HasSuffix(filename, ".proto") &&
			filename != "typed_message.proto" &&
			filename != "descriptor.proto" {
			protoFilesMap[dir] = append(protoFilesMap[dir], path)
		}

		return nil
	})
	if walkErr != nil {
		fmt.Println(walkErr)
		os.Exit(1)
	}

	for _, files := range protoFilesMap {
		for _, relProtoFile := range files {
			args := []string{
				"-I", ".",
				"--go_out", pwd,
				"--go_opt", "paths=source_relative",
				"--go-grpc_out", pwd,
				"--go-grpc_opt", "paths=source_relative",
				"--plugin", "protoc-gen-go=" + filepath.Join(GOBIN, "protoc-gen-go"+suffix),
				"--plugin", "protoc-gen-go-grpc=" + filepath.Join(GOBIN, "protoc-gen-go-grpc"+suffix),
			}
			args = append(args, relProtoFile)
			cmd := exec.Command(protoc, args...)
			cmd.Env = append(cmd.Env, os.Environ()...)
			output, cmdErr := cmd.CombinedOutput()
			if len(output) > 0 {
				fmt.Println(string(output))
			}
			if cmdErr != nil {
				fmt.Println(cmdErr)
				os.Exit(1)
			}
		}
	}

	normalizeWalkErr := filepath.Walk("./", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if info.IsDir() {
			return nil
		}

		filename := filepath.Base(path)
		if strings.HasSuffix(filename, ".pb.go") &&
			path != "config.pb.go" {
			if err := NormalizeGeneratedProtoFile(path); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		return nil
	})
	if normalizeWalkErr != nil {
		fmt.Println(normalizeWalkErr)
		os.Exit(1)
	}
}

func NormalizeGeneratedProtoFile(path string) error {
	fd, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}

	_, err = fd.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(fd)
	valid := false
	for scanner.Scan() {
		if !valid && !strings.HasPrefix(scanner.Text(), "package ") {
			continue
		}
		valid = true
		out.Write(scanner.Bytes())
		out.Write([]byte("\n"))
	}
	_, err = fd.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = fd.Truncate(0)
	if err != nil {
		return err
	}
	_, err = io.Copy(fd, bytes.NewReader(out.Bytes()))
	if err != nil {
		return err
	}
	return nil
}
