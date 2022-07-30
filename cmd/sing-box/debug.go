//go:build debug

package main

import (
	"net/http"
	_ "net/http/pprof"
)

func init() {
	go http.ListenAndServe("0.0.0.0:8964", nil)
}
