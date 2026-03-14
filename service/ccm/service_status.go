package ccm

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

	if len(s.options.Users) == 0 {
		writeJSONError(w, r, http.StatusForbidden, "authentication_error", "status endpoint requires user authentication")
		return
	}

	if r.Header.Get("X-Api-Key") != "" || r.Header.Get("Api-Key") != "" {
		writeJSONError(w, r, http.StatusBadRequest, "invalid_request_error",
			"API key authentication is not supported; use Authorization: Bearer with a CCM user token")
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

	userConfig := s.userConfigMap[username]
	if userConfig == nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", "user config not found")
		return
	}

	provider, err := credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, username)
	if err != nil {
		writeJSONError(w, r, http.StatusInternalServerError, "api_error", err.Error())
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

func (s *Service) computeAggregatedUtilization(provider credentialProvider, userConfig *option.CCMUser) (float64, float64, float64) {
	var totalWeightedRemaining5h, totalWeightedRemainingWeekly, totalWeight float64
	for _, credential := range provider.allCredentials() {
		if !credential.isAvailable() {
			continue
		}
		if userConfig.ExternalCredential != "" && credential.tagName() == userConfig.ExternalCredential {
			continue
		}
		if !userConfig.AllowExternalUsage && credential.isExternal() {
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

func (s *Service) rewriteResponseHeadersForExternalUser(headers http.Header, userConfig *option.CCMUser) {
	provider, err := credentialForUser(s.userConfigMap, s.providers, s.legacyProvider, userConfig.Name)
	if err != nil {
		return
	}

	avgFiveHour, avgWeekly, totalWeight := s.computeAggregatedUtilization(provider, userConfig)

	headers.Set("anthropic-ratelimit-unified-5h-utilization", strconv.FormatFloat(avgFiveHour/100, 'f', 6, 64))
	headers.Set("anthropic-ratelimit-unified-7d-utilization", strconv.FormatFloat(avgWeekly/100, 'f', 6, 64))
	if totalWeight > 0 {
		headers.Set("X-CCM-Plan-Weight", strconv.FormatFloat(totalWeight, 'f', -1, 64))
	}
}
