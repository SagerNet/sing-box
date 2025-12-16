//go:build !darwin

package ocm

func platformReadCredentials(customPath string) (*oauthCredentials, error) {
	if customPath == "" {
		var err error
		customPath, err = getDefaultCredentialsPath()
		if err != nil {
			return nil, err
		}
	}
	return readCredentialsFromFile(customPath)
}

func platformWriteCredentials(credentials *oauthCredentials, customPath string) error {
	if customPath == "" {
		var err error
		customPath, err = getDefaultCredentialsPath()
		if err != nil {
			return err
		}
	}
	return writeCredentialsToFile(credentials, customPath)
}
