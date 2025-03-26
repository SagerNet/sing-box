package main

import (
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/shell"

	"github.com/spf13/cobra"
)

var commandInstallCACertificate = &cobra.Command{
	Use:   "install-ca <path to certificate>",
	Short: "Install CA certificate to system",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := installCACertificate(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandTools.AddCommand(commandInstallCACertificate)
}

func installCACertificate(path string) error {
	switch runtime.GOOS {
	case "windows":
		return shell.Exec("powershell", "-Command", "Import-Certificate -FilePath \""+path+"\" -CertStoreLocation Cert:\\LocalMachine\\Root").Attach().Run()
	case "darwin":
		return shell.Exec("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", path).Attach().Run()
	case "linux":
		updateCertPath, updateCertPathNotFoundErr := exec.LookPath("update-ca-certificates")
		if updateCertPathNotFoundErr == nil {
			publicDer, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			err = os.MkdirAll("/usr/local/share/ca-certificates", 0o755)
			if err != nil {
				if errors.Is(err, os.ErrPermission) {
					log.Info("Try running with sudo")
					return shell.Exec("sudo", os.Args...).Attach().Run()
				}
				return err
			}
			fileName := filepath.Base(updateCertPath)
			if !strings.HasSuffix(fileName, ".crt") {
				fileName = fileName + ".crt"
			}
			filePath, _ := filepath.Abs(filepath.Join("/usr/local/share/ca-certificates", fileName))
			err = os.WriteFile(filePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicDer}), 0o644)
			if err != nil {
				if errors.Is(err, os.ErrPermission) {
					log.Info("Try running with sudo")
					return shell.Exec("sudo", os.Args...).Attach().Run()
				}
				return err
			}
			log.Info("certificate written to " + filePath + "\n")
			err = shell.Exec(updateCertPath).Attach().Run()
			if err != nil {
				return err
			}
			log.Info("certificate installed")
			return nil
		}
		updateTrustPath, updateTrustPathNotFoundErr := exec.LookPath("update-ca-trust")
		if updateTrustPathNotFoundErr == nil {
			publicDer, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fileName := filepath.Base(updateTrustPath)
			fileExt := filepath.Ext(path)
			if fileExt != "" {
				fileName = fileName[:len(fileName)-len(fileExt)]
			}
			filePath, _ := filepath.Abs(filepath.Join("/etc/pki/ca-trust/source/anchors/", fileName+".pem"))
			err = os.WriteFile(filePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicDer}), 0o644)
			if err != nil {
				if errors.Is(err, os.ErrPermission) {
					log.Info("Try running with sudo")
					return shell.Exec("sudo", os.Args...).Attach().Run()
				}
				return err
			}
			log.Info("certificate written to " + filePath + "\n")
			err = shell.Exec(updateTrustPath, "extract").Attach().Run()
			if err != nil {
				return err
			}
			log.Info("certificate installed")
		}
		return E.New("update-ca-certificates or update-ca-trust not found")
	default:
		return E.New("unsupported operating system: ", runtime.GOOS)
	}
}
