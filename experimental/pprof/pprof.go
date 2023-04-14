package pprof

import (
	"context"
	"github.com/sagernet/sing-box/log"
	"net/http"
	"net/http/pprof"
)

func NewPprof(ctx context.Context, listen string) {
	server := &http.Server{}
	serverMux := http.NewServeMux()

	serverMux.HandleFunc("/debug/pprof/", pprof.Index)
	serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server.Handler = serverMux
	server.Addr = listen

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("pprof server fail: ", err)
		}
	}()
}
