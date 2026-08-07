//go:build windows && with_external_windivert

package windivert

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	E "github.com/sagernet/sing/common/exceptions"
)

func driverAsset() ([]byte, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, E.Cause(err, "windivert: locate executable")
	}
	assetPath := filepath.Join(filepath.Dir(executablePath), driverAssetName)
	content, err := os.ReadFile(assetPath)
	if err != nil {
		return nil, E.Cause(err, "windivert: read ", assetPath)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != driverAssetDigest {
		return nil, E.New("windivert: ", assetPath, " does not match the WinDivert ", AssetVersion, " digest")
	}
	return content, nil
}
