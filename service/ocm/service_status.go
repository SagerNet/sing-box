package ocm

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/option"
)

func (s *Service) handleStatusEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, r, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}

	var provider credentialProvider
	var userConfig *option.OCMUser
	if len(s.options.Users) > 0 {
		if r.Header.Get("X-Api-Key") != "" || r.Header.Get("Api-Key") != "" {
			writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error",
				"API key authentication is not supported; use Authorization: Bearer with an OCM user token")
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "missing api key")
			return
		}
		clientToken := strings.TrimPrefix(authHeader, "Bearer ")
		if clientToken == authHeader {
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key format")
			return
		}
		username, ok := s.userManager.Authenticate(clientToken)
		if !ok {
			writeJSONError(w, r, http.StatusUnauthorized, "authentication_error", "invalid api key")
			return
		}

		userConfig = s.userConfigMap[username]
		var err error
		provider, err = credentialForUser(s.userConfigMap, s.providers, username)
		if err != nil {
			writeJSONError(w, r, http.StatusInternalServerError, "api_error", err.Error())
			return
		}
	} else {
		provider = s.providers[s.options.Credentials[0].Tag]
	}
	if provider == nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "no credential available")
		return
	}

	provider.pollIfStale(r.Context())
	avgFiveHour, avgWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]float64{
		"five_hour_utilization": avgFiveHour,
		"weekly_utilization":    avgWeekly,
		"plan_weight":           totalWeight,
	})
}

func (s *Service) computeAggregatedUtilization(provider credentialProvider, userConfig *option.OCMUser) (float64, float64, float64) {
	var totalWeightedRemaining5h, totalWeightedRemainingWeekly, totalWeight float64
	for _, credential := range provider.allCredentials() {
		if !credential.isAvailable() {
			continue
		}
		if userConfig != nil && userConfig.ExternalCredential != "" && credential.tagName() == userConfig.ExternalCredential {
			continue
		}
		if userConfig != nil && !userConfig.AllowExternalUsage && credential.isExternal() {
			continue
		}
		weight := credential.planWeight()
		remaining5h := credential.fiveHourCap() - credential.fiveHourUtilization()
		if remaining5h < 0 {
			remaining5h = 0
		}
		remainingWeekly := credential.weeklyCap() - credential.weeklyUtilization()
		if remainingWeekly < 0 {
			remainingWeekly = 0
		}
		totalWeightedRemaining5h += remaining5h * weight
		totalWeightedRemainingWeekly += remainingWeekly * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 100, 100, 0
	}
	return 100 - totalWeightedRemaining5h/totalWeight,
		100 - totalWeightedRemainingWeekly/totalWeight,
		totalWeight
}

func (s *Service) rewriteResponseHeadersForExternalUser(headers http.Header, provider credentialProvider, userConfig *option.OCMUser) {
	avgFiveHour, avgWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	activeLimitIdentifier := normalizeRateLimitIdentifier(headers.Get("x-codex-active-limit"))
	if activeLimitIdentifier == "" {
		activeLimitIdentifier = "codex"
	}

	headers.Set("x-"+activeLimitIdentifier+"-primary-used-percent", strconv.FormatFloat(avgFiveHour, 'f', 2, 64))
	headers.Set("x-"+activeLimitIdentifier+"-secondary-used-percent", strconv.FormatFloat(avgWeekly, 'f', 2, 64))
	if totalWeight > 0 {
		headers.Set("X-OCM-Plan-Weight", strconv.FormatFloat(totalWeight, 'f', -1, 64))
	}
}
