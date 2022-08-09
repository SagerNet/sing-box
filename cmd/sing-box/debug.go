//go:build debug

package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/sagernet/sing-box/log"
)

func init() {
	go func() {
		err := http.ListenAndServe("0.0.0.0:8964", nil)
		if err != nil {
			log.Debug(err)
		}
	}()
}
