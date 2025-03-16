package dns

import (
	"context"
	"strings"

	"github.com/sagernet/sing/common/logger"

	"github.com/miekg/dns"
)

func logCachedResponse(logger logger.ContextLogger, ctx context.Context, response *dns.Msg, ttl int) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugContext(ctx, "cached ", domain, " ", dns.RcodeToString[response.Rcode], " ", ttl)
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoContext(ctx, "cached ", dns.Type(record.Header().Rrtype).String(), " ", FormatQuestion(record.String()))
		}
	}
}

func logExchangedResponse(logger logger.ContextLogger, ctx context.Context, response *dns.Msg, ttl uint32) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	domain := FqdnToDomain(response.Question[0].Name)
	logger.DebugContext(ctx, "exchanged ", domain, " ", dns.RcodeToString[response.Rcode], " ", ttl)
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoContext(ctx, "exchanged ", dns.Type(record.Header().Rrtype).String(), " ", FormatQuestion(record.String()))
		}
	}
}

func logRejectedResponse(logger logger.ContextLogger, ctx context.Context, response *dns.Msg) {
	if logger == nil || len(response.Question) == 0 {
		return
	}
	for _, recordList := range [][]dns.RR{response.Answer, response.Ns, response.Extra} {
		for _, record := range recordList {
			logger.InfoContext(ctx, "rejected ", dns.Type(record.Header().Rrtype).String(), " ", FormatQuestion(record.String()))
		}
	}
}

func FqdnToDomain(fqdn string) string {
	if dns.IsFqdn(fqdn) {
		return fqdn[:len(fqdn)-1]
	}
	return fqdn
}

func FormatQuestion(string string) string {
	for strings.HasPrefix(string, ";") {
		string = string[1:]
	}
	string = strings.ReplaceAll(string, "\t", " ")
	string = strings.ReplaceAll(string, "\n", " ")
	string = strings.ReplaceAll(string, ";; ", " ")
	string = strings.ReplaceAll(string, "; ", " ")

	for strings.Contains(string, "  ") {
		string = strings.ReplaceAll(string, "  ", " ")
	}
	return strings.TrimSpace(string)
}
