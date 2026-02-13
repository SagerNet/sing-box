package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/rw"
)

var target string

func init() {
	flag.StringVar(&target, "target", "android", "target platform (android or apple)")
}

func main() {
	flag.Parse()

	args := []string{
		"generate",
		"-v",
		"--config", "experimental/libbox/ffi.json",
		"--platform-type", target,
	}
	command := exec.Command("sing-ffi", args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		log.Fatal(err)
	}

	copyArtifacts(target)
}

func copyArtifacts(target string) {
	switch target {
	case "android":
		copyPath := filepath.Join("..", "sing-box-for-android", "app", "libs")
		if rw.IsDir(copyPath) {
			copyPath, _ = filepath.Abs(copyPath)
			for _, name := range []string{"libbox.aar", "libbox-legacy.aar"} {
				artifactPath, found := findArtifactPath(name)
				if !found {
					continue
				}
				targetPath := filepath.Join(target, artifactPath)
				os.RemoveAll(targetPath)
				err := os.Rename(artifactPath, targetPath)
				if err != nil {
					log.Fatal(err)
				}
				log.Info("copied ", name, " to ", copyPath)
			}
		}
	case "apple":
		copyPath := filepath.Join("..", "sing-box-for-apple")
		if rw.IsDir(copyPath) {
			sourceDir, found := findArtifactPath("Libbox.xcframework")
			if !found {
				log.Fatal("Libbox.xcframework not found in current directory or experimental/libbox")
			}

			targetDir := filepath.Join(copyPath, "Libbox.xcframework")
			targetDir, _ = filepath.Abs(targetDir)
			err := os.RemoveAll(targetDir)
			if err != nil {
				log.Fatal(err)
			}
			err = os.Rename(sourceDir, targetDir)
			if err != nil {
				log.Fatal(err)
			}
			log.Info("copied ", sourceDir, " to ", targetDir)
		}
	}
}

func findArtifactPath(name string) (string, bool) {
	candidates := []string{
		name,
		filepath.Join("experimental", "libbox", name),
	}
	for _, candidate := range candidates {
		if rw.IsFile(candidate) || rw.IsDir(candidate) {
			return candidate, true
		}
	}
	return "", false
}
