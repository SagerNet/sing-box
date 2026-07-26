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

func getDialFeedback(w http.ResponseWriter, r *http.Request) {
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

	sequence, events := smart.DialFeedbackSince(since)
	render.JSON(w, r, dialFeedbackResponse{
		Instance: smart.DialFeedbackInstance(),
		Sequence: sequence,
		Events:   events,
	})
}
