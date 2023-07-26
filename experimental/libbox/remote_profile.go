package libbox

import (
	"net/url"
)

func GenerateRemoteProfileImportLink(name string, remoteURL string) string {
	importLink := &url.URL{
		Scheme:   "sing-box",
		Host:     "import-remote-profile",
		RawQuery: url.Values{"url": []string{remoteURL}}.Encode(),
		Fragment: name,
	}
	return importLink.String()
}

type ImportRemoteProfile struct {
	Name string
	URL  string
	Host string
}

func ParseRemoteProfileImportLink(importLink string) (*ImportRemoteProfile, error) {
	importURL, err := url.Parse(importLink)
	if err != nil {
		return nil, err
	}
	remoteURL, err := url.Parse(importURL.Query().Get("url"))
	if err != nil {
		return nil, err
	}
	name := importURL.Fragment
	if name == "" {
		name = remoteURL.Host
	}
	return &ImportRemoteProfile{
		Name: name,
		URL:  remoteURL.String(),
		Host: remoteURL.Host,
	}, nil
}
