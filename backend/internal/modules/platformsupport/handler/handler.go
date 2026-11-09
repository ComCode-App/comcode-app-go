// Package handler serves the platform card and route support API (TEC-950).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/platformsupport/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

type listResult[T any] struct {
	Items []T `json:"items"`
}

func actor(r *http.Request) usecase.Actor {
	p := authctx.MustPrincipal(r.Context())
	return usecase.Actor{UserID: p.UserInternal, Request: r}
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.NotFound(w, r, "Not found")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) SearchCards(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.SearchCards(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, listResult[any]{Items: toAny(items)})
}

func (h *Handler) GetCard(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	card, err := h.svc.GetCard(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, card)
}

func (h *Handler) RetireCard(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	card, err := h.svc.RetireCard(r.Context(), actor(r), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, card)
}

type reassignRequest struct {
	ProfileUUID string `json:"profile_uuid"`
}

func (h *Handler) ReassignCard(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var body reassignRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "Invalid request body")
		return
	}
	profileUUID, err := uuid.Parse(body.ProfileUUID)
	if err != nil {
		response.ValidationError(w, r, []response.Detail{{Field: "profile_uuid", Message: "A valid profile is required", Code: "invalid"}})
		return
	}
	card, err := h.svc.ReassignCard(r.Context(), actor(r), id, profileUUID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, card)
}

func (h *Handler) SearchRoutes(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.SearchRoutes(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, listResult[any]{Items: toAny(items)})
}

func (h *Handler) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	rt, err := h.svc.GetRoute(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, rt)
}

type releaseRequest struct {
	Path string `json:"path"`
}

func (h *Handler) ReleaseQuarantine(w http.ResponseWriter, r *http.Request) {
	var body releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "Invalid request body")
		return
	}
	res, err := h.svc.ReleasePath(r.Context(), actor(r), body.Path)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, res)
}

func toAny[T any](items []T) []any {
	out := make([]any, len(items))
	for i := range items {
		out[i] = items[i]
	}
	return out
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrQueryTooShort):
		response.ValidationError(w, r, []response.Detail{{Field: "q", Message: "Enter at least 2 characters", Code: "too_short"}})
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "Not found")
	case errors.Is(err, usecase.ErrProfileNotFound):
		response.ValidationError(w, r, []response.Detail{{Field: "profile_uuid", Message: "Profile not found", Code: "not_found"}})
	case errors.Is(err, usecase.ErrProfileHasNoOwner):
		response.ValidationError(w, r, []response.Detail{{Field: "profile_uuid", Message: "Profile has no owner to hold the card", Code: "no_owner"}})
	case errors.Is(err, usecase.ErrCardRetired):
		response.Conflict(w, r, "CARD_RETIRED", "Card is already retired")
	case errors.Is(err, usecase.ErrCardHasNoRoute):
		response.Conflict(w, r, "CARD_HAS_NO_ROUTE", "Card has no card route")
	case errors.Is(err, usecase.ErrCardNotActivated):
		response.Conflict(w, r, "CARD_NOT_ACTIVATED", "Card is not activated yet")
	case errors.Is(err, usecase.ErrPathNotQuarantined):
		response.Conflict(w, r, "PATH_NOT_QUARANTINED", "Path is not quarantined")
	default:
		response.InternalErr(w, r, err, "Support request failed")
	}
}
