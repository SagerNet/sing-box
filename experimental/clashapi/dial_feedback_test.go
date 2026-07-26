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
	request := httptest.NewRequest(http.MethodGet, "/dart/dial-feedback?since="+strconv.FormatUint(^uint64(0), 10), nil)
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

func TestGetDialFeedbackRejectsInvalidCursor(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/dart/dial-feedback?since=invalid", nil)
	recorder := httptest.NewRecorder()

	getDialFeedback(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
