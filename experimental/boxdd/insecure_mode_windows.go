//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/locale"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
	"github.com/sagernet/sing/service/filemanager"
)

func registerSecurityPolicy(ctx context.Context, daemon *Daemon) {
	service.MustRegister[adapter.SecurityPolicy](ctx, &daemonSecurityPolicy{daemon})
	service.MustRegister[filemanager.Manager](ctx, &restrictedFileManager{daemon})
}

func insecureModeAvailable() bool {
	return true
}

func loadSecuritySettings(directory string) (securitySettings, error) {
	content, err := os.ReadFile(filepath.Join(directory, securitySettingsFileName))
	if err != nil {
		return securitySettings{}, err
	}
	settings, err := json.UnmarshalExtended[securitySettings](content)
	if err != nil {
		return securitySettings{}, err
	}
	return settings, nil
}

func (d *Daemon) insecureModeEnabled() bool {
	settings, err := loadSecuritySettings(workingDirectory)
	if err != nil {
		return false
	}
	return settings.InsecureModeEnabled
}

func insecureFeatureError(feature string) error {
	return E.New(fmt.Sprintf(locale.Current().InsecureFeatureMessage, feature))
}

type daemonSecurityPolicy struct {
	daemon *Daemon
}

func (p *daemonSecurityPolicy) CheckFeature(feature string) error {
	if p.daemon.insecureModeEnabled() {
		return nil
	}
	return insecureFeatureError(feature)
}

type restrictedFileManager struct {
	daemon *Daemon
}

func (m *restrictedFileManager) BasePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	currentDirectory, err := os.Getwd()
	if err != nil {
		return name
	}
	return filepath.Join(currentDirectory, name)
}

func (m *restrictedFileManager) TempPath() string {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return currentDirectory
}

func (m *restrictedFileManager) checkPath(name string) (string, error) {
	path, err := filepath.Abs(m.BasePath(name))
	if err != nil {
		return "", err
	}
	if m.daemon.insecureModeEnabled() {
		return path, nil
	}
	currentDirectory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	normalizedRoot := strings.ToLower(filepath.Clean(currentDirectory))
	normalizedPath := strings.ToLower(filepath.Clean(path))
	if normalizedPath != normalizedRoot && !strings.HasPrefix(normalizedPath, normalizedRoot+string(filepath.Separator)) {
		return "", E.New(fmt.Sprintf(locale.Current().ExternalPathFeature, path))
	}
	existingPath := path
	for {
		_, err = os.Lstat(existingPath)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parentPath := filepath.Dir(existingPath)
		if parentPath == existingPath {
			return "", err
		}
		existingPath = parentPath
	}
	resolvedRoot, err := filepath.EvalSymlinks(currentDirectory)
	if err != nil {
		return "", err
	}
	resolvedExistingPath, err := filepath.EvalSymlinks(existingPath)
	if err != nil {
		return "", err
	}
	remainingPath, err := filepath.Rel(existingPath, path)
	if err != nil {
		return "", err
	}
	resolvedPath := filepath.Join(resolvedExistingPath, remainingPath)
	normalizedResolvedRoot := strings.ToLower(filepath.Clean(resolvedRoot))
	normalizedResolvedPath := strings.ToLower(filepath.Clean(resolvedPath))
	if normalizedResolvedPath != normalizedResolvedRoot && !strings.HasPrefix(normalizedResolvedPath, normalizedResolvedRoot+string(filepath.Separator)) {
		return "", E.New(fmt.Sprintf(locale.Current().ExternalPathFeature, path))
	}
	return path, nil
}

func (m *restrictedFileManager) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	path, err := m.checkPath(name)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(path, flag, perm)
}

func (m *restrictedFileManager) Create(name string) (*os.File, error) {
	path, err := m.checkPath(name)
	if err != nil {
		return nil, err
	}
	return os.Create(path)
}

func (m *restrictedFileManager) CreateTemp(pattern string) (*os.File, error) {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return os.CreateTemp(currentDirectory, pattern)
}

func (m *restrictedFileManager) Chown(path string) error {
	return nil
}

func (m *restrictedFileManager) Mkdir(path string, perm os.FileMode) error {
	checkedPath, err := m.checkPath(path)
	if err != nil {
		return err
	}
	return os.Mkdir(checkedPath, perm)
}

func (m *restrictedFileManager) MkdirAll(path string, perm os.FileMode) error {
	checkedPath, err := m.checkPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(checkedPath, perm)
}

func (m *restrictedFileManager) Remove(path string) error {
	checkedPath, err := m.checkPath(path)
	if err != nil {
		return err
	}
	return os.Remove(checkedPath)
}

func (m *restrictedFileManager) RemoveAll(path string) error {
	checkedPath, err := m.checkPath(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(checkedPath)
}

func (m *restrictedFileManager) Rename(oldPath string, newPath string) error {
	checkedOldPath, err := m.checkPath(oldPath)
	if err != nil {
		return err
	}
	checkedNewPath, err := m.checkPath(newPath)
	if err != nil {
		return err
	}
	return os.Rename(checkedOldPath, checkedNewPath)
}
