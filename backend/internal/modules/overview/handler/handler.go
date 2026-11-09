// Package handler serves GET /v1/platform/overview and its conversion funnel.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/overview/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Handler exposes the platform overview.
type Handler struct {
	svc *usecase.Service
}

// New creates the handler.
func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

// Get answers GET /v1/platform/overview.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load platform overview")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Funnel answers GET /v1/platform/overview/funnel?days=N (TEC-944); days defaults to 30 and must
// be one of model.FunnelPeriods.
func (h *Handler) Funnel(w http.ResponseWriter, r *http.Request) {
	days := model.DefaultFunnelDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			response.BadRequest(w, r, response.CodeValidationError, "days must be one of 7, 30, 90, 365")
			return
		}
		days = n
	}
	out, err := h.svc.Funnel(r.Context(), days)
	if errors.Is(err, usecase.ErrInvalidPeriod) {
		response.BadRequest(w, r, response.CodeValidationError, "days must be one of 7, 30, 90, 365")
		return
	}
	if err != nil {
		response.InternalErr(w, r, err, "failed to load conversion funnel")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}
