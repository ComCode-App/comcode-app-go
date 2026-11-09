package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/socialplatforms/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.PlatformSocialPlatforms()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	var category *string
	if cat := r.URL.Query().Get("category"); cat != "" {
		category = &cat
	}
	var isActive *bool
	if act := r.URL.Query().Get("is_active"); act != "" {
		b := act == "true" || act == "1"
		isActive = &b
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.List(r.Context(), qStr, category, isActive, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list social platforms")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) ListActive(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListActive(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to list active social platforms")
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

	platform, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "social platform not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get social platform")
		return
	}

	response.JSON(w, r, http.StatusOK, platform)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.UpsertSocialPlatformInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	platform, err := h.svc.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidInput) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrSlugTaken) {
			response.Conflict(w, r, response.CodeConflict, "slug is already taken")
			return
		}
		response.InternalErr(w, r, err, "failed to create social platform")
		return
	}

	response.JSON(w, r, http.StatusCreated, platform)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpsertSocialPlatformInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	platform, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "social platform not found")
			return
		}
		if errors.Is(err, usecase.ErrSlugTaken) {
			response.Conflict(w, r, response.CodeConflict, "slug is already taken")
			return
		}
		if errors.Is(err, usecase.ErrInvalidInput) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to update social platform")
		return
	}

	response.JSON(w, r, http.StatusOK, platform)
}

func (h *Handler) UploadIcon(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("icon")
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "icon file is required")
		return
	}
	defer func() { _ = file.Close() }()

	platform, err := h.svc.UploadIcon(r.Context(), id, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "social platform not found")
			return
		}
		response.InternalErr(w, r, err, "failed to upload icon")
		return
	}

	response.JSON(w, r, http.StatusOK, platform)
}

func (h *Handler) FetchIcon(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	platform, err := h.svc.FetchIcon(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "social platform not found")
			return
		}
		response.InternalErr(w, r, err, "failed to fetch icon via OpenGraph")
		return
	}

	response.JSON(w, r, http.StatusOK, platform)
}

type BulkFetchIconsRequest struct {
	UUIDs []string `json:"uuids"`
}

func (h *Handler) BulkFetchIcons(w http.ResponseWriter, r *http.Request) {
	var req BulkFetchIconsRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	var targetUUIDs []uuid.UUID
	for _, raw := range req.UUIDs {
		if id, err := uuid.Parse(raw); err == nil {
			targetUUIDs = append(targetUUIDs, id)
		}
	}

	updatedCount, err := h.svc.BulkFetchIcons(r.Context(), targetUUIDs)
	if err != nil {
		response.InternalErr(w, r, err, "failed to bulk fetch icons")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"updated_count": updatedCount,
		"success":       true,
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "social platform not found")
			return
		}
		response.InternalErr(w, r, err, "failed to delete social platform")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
