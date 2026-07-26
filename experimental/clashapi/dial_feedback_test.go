package clashapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sagernet/sing-box/protocol/group/smart"
)

func TestGetDialFeedback(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/dart/dial-feedback?signals=1&since="+strconv.FormatUint(^uint64(0), 10),
		nil,
	)
	recorder := httptest.NewRecorder()

	getDialFeedback(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("cache-control=%q", cacheControl)
	}
	var response dialFeedbackResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Events == nil || len(response.Events) != 0 {
		t.Fatalf("events=%v", response.Events)
	}
	if response.Instance != smart.DialFeedbackInstance() {
		t.Fatalf("instance=%q", response.Instance)
	}
}

func TestDialFeedbackSignalsRequireExplicitOptIn(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"signals=1", true},
		{"signals=0", false},
		{"signals=true", false},
		{"", false},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, "/dart/dial-feedback?"+test.query, nil)
		if got := dialFeedbackSignalsEnabled(request); got != test.want {
			t.Errorf("query=%q got %v want %v", test.query, got, test.want)
		}
	}
}

func TestDialFeedbackAPISelectsLegacyAndDetailedProjection(t *testing.T) {
	legacyCalls := 0
	detailedCalls := 0
	readLegacy := func(since uint64) (uint64, []smart.DialFeedbackEvent) {
		legacyCalls++
		if since != 7 {
			t.Fatalf("legacy since=%d", since)
		}
		return 9, []smart.DialFeedbackEvent{{
			Sequence:   8,
			Outbound:   "node",
			Network:    "tcp",
			Signal:     "handshake",
			Success:    true,
			DurationMs: 20,
		}}
	}
	readDetailed := func(since uint64) (uint64, []smart.DialFeedbackEvent) {
		detailedCalls++
		if since != 7 {
			t.Fatalf("detailed since=%d", since)
		}
		return 9, []smart.DialFeedbackEvent{{
			Sequence:   8,
			Outbound:   "node",
			Network:    "tcp",
			Signal:     "handshake",
			Success:    true,
			DurationMs: 2,
		}}
	}
	requestProjection := func(suffix string) dialFeedbackResponse {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/dart/dial-feedback?since=7"+suffix, nil)
		recorder := httptest.NewRecorder()
		serveDialFeedback(recorder, request, "test-instance", readLegacy, readDetailed)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var response dialFeedbackResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return response
	}

	legacy := requestProjection("")
	detailed := requestProjection("&signals=1")
	if legacy.Instance != "test-instance" || detailed.Instance != legacy.Instance {
		t.Fatalf("instances legacy=%q detailed=%q", legacy.Instance, detailed.Instance)
	}
	if legacy.Sequence != 9 || detailed.Sequence != 9 {
		t.Fatalf("sequences legacy=%d detailed=%d", legacy.Sequence, detailed.Sequence)
	}
	if len(legacy.Events) != 1 || legacy.Events[0].DurationMs != 20 {
		t.Fatalf("legacy events=%+v", legacy.Events)
	}
	if len(detailed.Events) != 1 || detailed.Events[0].DurationMs != 2 {
		t.Fatalf("detailed events=%+v", detailed.Events)
	}
	if legacyCalls != 1 || detailedCalls != 1 {
		t.Fatalf("calls legacy=%d detailed=%d", legacyCalls, detailedCalls)
	}
}

func TestGetDialFeedbackRejectsInvalidCursor(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dart/dial-feedback?since=invalid", nil)
	recorder := httptest.NewRecorder()

	getDialFeedback(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
