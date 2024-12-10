package libbox

import (
	"os"
	"path/filepath"
)

func serviceErrorPath() string {
	return filepath.Join(sWorkingPath, "network_extension_error")
}

func ClearServiceError() {
	os.Remove(serviceErrorPath())
}

func ReadServiceError() (*StringBox, error) {
	data, err := os.ReadFile(serviceErrorPath())
	if err == nil {
		os.Remove(serviceErrorPath())
	}
	return wrapString(string(data)), err
}

func WriteServiceError(message string) error {
	errorFile, err := os.Create(serviceErrorPath())
	if err != nil {
		return err
	}
	errorFile.WriteString(message)
	errorFile.Chown(sUserID, sGroupID)
	return errorFile.Close()
}
