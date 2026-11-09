package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	errortracking "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/errortracking/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc *errortracking.Service
}

func New(svc *errortracking.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load error tracking settings")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var in errortracking.PatchInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := dec.Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return
	}
	s, err := h.svc.Patch(r.Context(), in)
	if err != nil {
		if errors.Is(err, errortracking.ErrInvalidDSN) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to update error tracking settings")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

// SendTest sends a test error through the backend DSN.
func (h *Handler) SendTest(w http.ResponseWriter, r *http.Request) {
	id, err := h.svc.SendTest(r.Context())
	if err != nil {
		if errors.Is(err, errortracking.ErrReportingOff) {
			response.Conflict(w, r, response.CodeConflict, "backend error reporting is off")
			return
		}
		response.InternalErr(w, r, err, "failed to send test error")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"event_id": id})
}

// PublicConfig returns the browser SDK config. Visitors and panel pages both call it, so it
// is served from memory and may be cached briefly.
func (h *Handler) PublicConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	response.JSON(w, r, http.StatusOK, h.svc.FrontendConfig())
}
