//go:build windows && !with_external_windivert

package windivert

func driverAsset() ([]byte, error) {
	return sysBytes, nil
}
