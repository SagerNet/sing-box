package clashapi

import (
	"context"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/miekg/dns"
)

func dnsRouter(router adapter.DNSRouter) http.Handler {
	r := chi.NewRouter()
	r.Get("/query", queryDNS(router))
	return r
}

func queryDNS(router adapter.DNSRouter) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		qTypeStr := r.URL.Query().Get("type")
		if qTypeStr == "" {
			qTypeStr = "A"
		}

		qType, exist := dns.StringToType[qTypeStr]
		if !exist {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("invalid query type"))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), C.DNSTimeout)
		defer cancel()

		msg := dns.Msg{}
		msg.SetQuestion(dns.Fqdn(name), qType)
		resp, err := router.Exchange(ctx, &msg, adapter.DNSQueryOptions{})
		if err != nil {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, newError(err.Error()))
			return
		}

		responseData := render.M{
			"Status":   resp.Rcode,
			"Question": resp.Question,
			"Server":   "internal",
			"TC":       resp.Truncated,
			"RD":       resp.RecursionDesired,
			"RA":       resp.RecursionAvailable,
			"AD":       resp.AuthenticatedData,
			"CD":       resp.CheckingDisabled,
		}

		rr2Json := func(rr dns.RR) render.M {
			header := rr.Header()
			return render.M{
				"name": header.Name,
				"type": header.Rrtype,
				"TTL":  header.Ttl,
				"data": rr.String()[len(header.String()):],
			}
		}

		if len(resp.Answer) > 0 {
			responseData["Answer"] = common.Map(resp.Answer, rr2Json)
		}
		if len(resp.Ns) > 0 {
			responseData["Authority"] = common.Map(resp.Ns, rr2Json)
		}
		if len(resp.Extra) > 0 {
			responseData["Additional"] = common.Map(resp.Extra, rr2Json)
		}

		render.JSON(w, r, responseData)
	}
}
