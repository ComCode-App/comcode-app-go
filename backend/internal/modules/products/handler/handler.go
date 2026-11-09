package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
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
	meta := resourcemeta.PlatformProducts()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())

	var status *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}

	var productType *string
	if pt := r.URL.Query().Get("product_type"); pt != "" {
		productType = &pt
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.List(r.Context(), qStr, nil, status, productType, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list products")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	prod, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get product")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		response.BadRequest(w, r, response.CodeValidationError, "slug is required")
		return
	}

	prod, err := h.svc.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get product by slug")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input model.UpsertProductInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	prod, err := h.svc.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, usecase.ErrNameRequired) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrSlugTaken) {
			response.Conflict(w, r, response.CodeConflict, "slug is already taken")
			return
		}
		response.InternalErr(w, r, err, "failed to create product")
		return
	}

	response.JSON(w, r, http.StatusCreated, prod)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpsertProductInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	prod, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
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
		response.InternalErr(w, r, err, "failed to update product")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

func (h *Handler) Update3DViewbox(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.Update3DViewboxInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	prod, err := h.svc.Update3DViewbox(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to update 3d viewbox")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

// Upload limits (TEC-1097): the body is capped before it is read.
const (
	maxModelBytes        = 50 << 20
	maxProductImageBytes = 10 << 20
)

func (h *Handler) Upload3DModel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	file, header, ok := upload.FormFile(w, r, maxModelBytes, "model")
	defer upload.Cleanup(r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()

	isUSDZ := r.URL.Query().Get("format") == "usdz"

	prod, err := h.svc.Upload3DModel(r.Context(), id, file, header.Filename, header.Header.Get("Content-Type"), header.Size, isUSDZ)
	if err != nil {
		if upload.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to upload 3d model")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	file, header, ok := upload.FormFile(w, r, maxProductImageBytes, "image")
	defer upload.Cleanup(r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()

	prod, err := h.svc.UploadImage(r.Context(), id, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		if upload.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to upload image")
		return
	}

	response.JSON(w, r, http.StatusOK, prod)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to delete product")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// Variants
func (h *Handler) ListVariants(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid product uuid")
		return
	}

	variants, err := h.svc.ListVariants(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		response.InternalErr(w, r, err, "failed to list variants")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"items": variants})
}

func (h *Handler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid product uuid")
		return
	}

	var input model.UpsertVariantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	variant, err := h.svc.CreateVariant(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "product not found")
			return
		}
		if errors.Is(err, usecase.ErrNameRequired) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to create variant")
		return
	}

	response.JSON(w, r, http.StatusCreated, variant)
}

func (h *Handler) UpdateVariant(w http.ResponseWriter, r *http.Request) {
	variantID, err := uuid.Parse(r.PathValue("variantUuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid variant uuid")
		return
	}

	var input model.UpsertVariantInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	variant, err := h.svc.UpdateVariant(r.Context(), variantID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrVariantNotFound) {
			response.NotFound(w, r, "variant not found")
			return
		}
		response.InternalErr(w, r, err, "failed to update variant")
		return
	}

	response.JSON(w, r, http.StatusOK, variant)
}

func (h *Handler) DeleteVariant(w http.ResponseWriter, r *http.Request) {
	variantID, err := uuid.Parse(r.PathValue("variantUuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid variant uuid")
		return
	}

	if err := h.svc.DeleteVariant(r.Context(), variantID); err != nil {
		response.InternalErr(w, r, err, "failed to delete variant")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
