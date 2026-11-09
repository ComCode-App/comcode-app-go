package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/categories/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.PlatformCategories()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	var isActive *bool
	if act := r.URL.Query().Get("is_active"); act != "" {
		b := act == "true" || act == "1"
		isActive = &b
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.List(r.Context(), qStr, isActive, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list categories")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListActive(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to list active categories")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	cat, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "category not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get category")
		return
	}

	response.JSON(w, r, http.StatusOK, cat)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.UpsertCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	cat, err := h.svc.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrNameRequired) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrSlugTaken) {
			response.Conflict(w, r, response.CodeConflict, "slug is already taken")
			return
		}
		response.InternalErr(w, r, err, "failed to create category")
		return
	}

	response.JSON(w, r, http.StatusCreated, cat)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpsertCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	cat, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "category not found")
			return
		}
		if errors.Is(err, usecase.ErrSlugTaken) {
			response.Conflict(w, r, response.CodeConflict, "slug is already taken")
			return
		}
		if errors.Is(err, usecase.ErrNameRequired) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to update category")
		return
	}

	response.JSON(w, r, http.StatusOK, cat)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "category not found")
			return
		}
		response.InternalErr(w, r, err, "failed to delete category")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
