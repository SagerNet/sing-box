package warp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
)

var identityFile = "wgcf-identity.json"

func saveIdentity(a Identity, path string) error {
	file, err := os.Create(filepath.Join(path, identityFile))
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(a)
	if err != nil {
		return err
	}

	return file.Close()
}

func LoadOrCreateIdentity(l *slog.Logger, path, license string) (*Identity, error) {
	l = l.With("subsystem", "warp/account")

	i, err := LoadIdentity(path)
	if err != nil {
		l.Info("failed to load identity", "path", path, "error", err)
		if err := os.RemoveAll(path); err != nil {
			return nil, err
		}

		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}

		i, err = CreateIdentity(l, license)
		if err != nil {
			return nil, err
		}

		if err = saveIdentity(i, path); err != nil {
			return nil, err
		}
	}

	if license != "" && i.Account.License != license {
		l.Info("updating account license key")
		_, err := UpdateAccount(i.Token, i.ID, license)
		if err != nil {
			return nil, err
		}

		iAcc, err := GetAccount(i.Token, i.ID)
		if err != nil {
			return nil, err
		}
		i.Account = iAcc

		if err = saveIdentity(i, path); err != nil {
			return nil, err
		}
	}

	l.Info("successfully loaded warp identity")
	return &i, nil
}

func LoadIdentity(path string) (Identity, error) {
	identityPath := filepath.Join(path, identityFile)
	_, err := os.Stat(identityPath)
	if err != nil {
		return Identity{}, err
	}

	fileBytes, err := os.ReadFile(identityPath)
	if err != nil {
		return Identity{}, err
	}

	i := &Identity{}
	err = json.Unmarshal(fileBytes, i)
	if err != nil {
		return Identity{}, err
	}

	if len(i.Config.Peers) < 1 {
		return Identity{}, errors.New("identity contains 0 peers")
	}

	return *i, nil
}

func CreateIdentity(l *slog.Logger, license string) (Identity, error) {
	priv, err := GeneratePrivateKey()
	if err != nil {
		return Identity{}, err
	}

	privateKey, publicKey := priv.String(), priv.PublicKey().String()

	l.Info("creating new identity")
	i, err := Register(publicKey)
	if err != nil {
		return Identity{}, err
	}

	if license != "" {
		l.Info("updating account license key")
		_, err := UpdateAccount(i.Token, i.ID, license)
		if err != nil {
			return Identity{}, err
		}

		ac, err := GetAccount(i.Token, i.ID)
		if err != nil {
			return Identity{}, err
		}
		i.Account = ac
	}

	i.PrivateKey = privateKey

	return i, nil
}
