package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// DialogSubmitter processes a validated dialog submission and returns
// per-field validation errors (if any) or a processing error.
type DialogSubmitter func(ctx context.Context, req *model.SubmitDialogRequest) (fieldErrors map[string]string, err error)

// DialogHandler handles interactive dialog submissions from Mattermost.
type DialogHandler struct {
	Submit DialogSubmitter
}

// HandleSubmit decodes a dialog submission, delegates to the submitter, and
// writes the appropriate SubmitDialogResponse.
func (h *DialogHandler) HandleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.Submit == nil {
		http.Error(w, "dialog submitter not configured", http.StatusInternalServerError)
		return
	}

	var req model.SubmitDialogRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Cancelled {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&model.SubmitDialogResponse{})
		return
	}

	// pass the HTTP request context through to the submitter
	fieldErrors, err := h.Submit(r.Context(), &req)

	w.Header().Set("Content-Type", "application/json")

	if len(fieldErrors) > 0 {
		_ = json.NewEncoder(w).Encode(&model.SubmitDialogResponse{Errors: fieldErrors})
		return
	}
	if err != nil {
		_ = json.NewEncoder(w).Encode(&model.SubmitDialogResponse{Error: err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(&model.SubmitDialogResponse{})
}

// StringField safely extracts a string value from a dialog submission map.
func StringField(submission map[string]interface{}, key string) string {
	if submission == nil {
		return ""
	}
	value, _ := submission[key].(string)
	return value
}
