package api

import "net/http"

type dashboardDir http.Dir

func (d dashboardDir) Open(name string) (http.File, error) {
	file, err := http.Dir(d).Open(name)
	if err != nil {
		return nil, err
	}
	return &fileWrapper{file}, nil
}

// workaround for #2345 #2596
type fileWrapper struct {
	http.File
}
