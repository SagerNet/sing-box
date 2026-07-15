//go:build !darwin || !cgo

package ccm

import "context"

func platformReadCredentials(ctx context.Context, customPath string) (*oauthCredentials, error) {
	if customPath == "" {
		var err error
		customPath, err = getDefaultCredentialsPath()
		if err != nil {
			return nil, err
		}
	}
	return readCredentialsFromFile(ctx, customPath)
}

func platformWriteCredentials(ctx context.Context, oauthCredentials *oauthCredentials, customPath string) error {
	if customPath == "" {
		var err error
		customPath, err = getDefaultCredentialsPath()
		if err != nil {
			return err
		}
	}
	return writeCredentialsToFile(ctx, oauthCredentials, customPath)
}
