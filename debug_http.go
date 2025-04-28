package box

import (
	"net/http"
	"net/http/pprof"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/byteformats"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/go-chi/chi/v5"
)

var debugHTTPServer *http.Server

func applyDebugListenOption(options option.DebugOptions) {
	if debugHTTPServer != nil {
		debugHTTPServer.Close()
		debugHTTPServer = nil
	}
	if options.Listen == "" {
		return
	}
	r := chi.NewMux()
	r.Route("/debug", func(r chi.Router) {
		r.Get("/gc", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNoContent)
			go debug.FreeOSMemory()
		})
		r.Get("/memory", func(writer http.ResponseWriter, request *http.Request) {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			var memObject badjson.JSONObject
			memObject.Put("heap", byteformats.FormatMemoryBytes(memStats.HeapInuse))
			memObject.Put("stack", byteformats.FormatMemoryBytes(memStats.StackInuse))
			memObject.Put("idle", byteformats.FormatMemoryBytes(memStats.HeapIdle-memStats.HeapReleased))
			memObject.Put("goroutines", runtime.NumGoroutine())
			memObject.Put("rss", rusageMaxRSS())

			encoder := json.NewEncoder(writer)
			encoder.SetIndent("", "  ")
			encoder.Encode(&memObject)
		})
		r.Route("/pprof", func(r chi.Router) {
			r.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
				if !strings.HasSuffix(request.URL.Path, "/") {
					http.Redirect(writer, request, request.URL.Path+"/", http.StatusMovedPermanently)
				} else {
					pprof.Index(writer, request)
				}
			})
			r.HandleFunc("/*", pprof.Index)
			r.HandleFunc("/cmdline", pprof.Cmdline)
			r.HandleFunc("/profile", pprof.Profile)
			r.HandleFunc("/symbol", pprof.Symbol)
			r.HandleFunc("/trace", pprof.Trace)
		})
	})
	debugHTTPServer = &http.Server{
		Addr:    options.Listen,
		Handler: r,
	}
	go func() {
		err := debugHTTPServer.ListenAndServe()
		if err != nil && !E.IsClosed(err) {
			log.Error(E.Cause(err, "serve debug HTTP server"))
		}
	}()
}
