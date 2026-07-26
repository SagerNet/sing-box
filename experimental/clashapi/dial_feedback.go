package clashapi

import (
	"net/http"
	"strconv"

	"github.com/sagernet/sing-box/protocol/group/smart"

	"github.com/go-chi/render"
)

type dialFeedbackResponse struct {
	Instance string                    `json:"instance"`
	Sequence uint64                    `json:"sequence"`
	Events   []smart.DialFeedbackEvent `json:"events"`
}

type dialFeedbackReadFunc func(since uint64) (uint64, []smart.DialFeedbackEvent)

func dialFeedbackSignalsEnabled(r *http.Request) bool {
	return r.URL.Query().Get("signals") == "1"
}

func getDialFeedback(w http.ResponseWriter, r *http.Request) {
	serveDialFeedback(
		w,
		r,
		smart.DialFeedbackInstance(),
		smart.DialFeedbackSince,
		smart.DialFeedbackDetailedSince,
	)
}

func serveDialFeedback(
	w http.ResponseWriter,
	r *http.Request,
	instance string,
	readLegacy dialFeedbackReadFunc,
	readDetailed dialFeedbackReadFunc,
) {
	w.Header().Set("Cache-Control", "no-store")

	var since uint64
	if rawSince := r.URL.Query().Get("since"); rawSince != "" {
		parsedSince, err := strconv.ParseUint(rawSince, 10, 64)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, ErrBadRequest)
			return
		}
		since = parsedSince
	}

	includeSignals := dialFeedbackSignalsEnabled(r)
	var (
		sequence uint64
		events   []smart.DialFeedbackEvent
	)
	if includeSignals {
		sequence, events = readDetailed(since)
	} else {
		sequence, events = readLegacy(since)
	}
	render.JSON(w, r, dialFeedbackResponse{
		Instance: instance,
		Sequence: sequence,
		Events:   events,
	})
}
