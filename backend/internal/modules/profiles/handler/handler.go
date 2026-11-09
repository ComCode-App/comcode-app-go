package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type QRTracker interface {
	RecordQRScan(context.Context, string, ingestion.HTTPContext)
}

type Handler struct {
	svc            *profilesusecase.Service
	legacyResolver LegacyResolver
	qrTracker      QRTracker
}

func New(svc *profilesusecase.Service) *Handler {
	return &Handler{svc: svc}
}

// SetQRTracker wires fail-soft scan attribution through the existing visitor API.
func (h *Handler) SetQRTracker(tracker QRTracker) { h.qrTracker = tracker }

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.AppProfiles()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())
	statusStr := r.URL.Query().Get("status")

	items, total, err := h.svc.List(r.Context(), p, q.Limit, q.Offset, q.Q, statusStr)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list profiles")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	item, err := h.svc.Get(r.Context(), p, profileUUID)
	if err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		response.InternalErr(w, r, err, "failed to load profile")
		return
	}

	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in model.UpsertProfileInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}

	item, err := h.svc.Create(r.Context(), p, in)
	if err != nil {
		if entitlement.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, profilesusecase.ErrInvalidRequest) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot create profiles")
			return
		}
		response.InternalErr(w, r, err, "failed to create profile")
		return
	}

	response.JSON(w, r, http.StatusCreated, item)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	var in model.UpsertProfileInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}

	item, err := h.svc.Update(r.Context(), p, profileUUID, in)
	if err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "forbidden")
			return
		}
		if errors.Is(err, profilesusecase.ErrInvalidRequest) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to update profile")
		return
	}

	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "avatar file is required")
		return
	}
	defer func() { _ = file.Close() }()

	item, err := h.svc.UploadAvatar(r.Context(), p, profileUUID, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "forbidden")
			return
		}
		response.InternalErr(w, r, err, "failed to upload avatar")
		return
	}

	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("cover")
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "cover file is required")
		return
	}
	defer func() { _ = file.Close() }()

	item, err := h.svc.UploadCover(r.Context(), p, profileUUID, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "forbidden")
			return
		}
		response.InternalErr(w, r, err, "failed to upload cover")
		return
	}

	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), p, profileUUID); err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "forbidden")
			return
		}
		response.InternalErr(w, r, err, "failed to delete profile")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) SetPrimary(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	if err := h.svc.SetPrimary(r.Context(), p, profileUUID); err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		if errors.Is(err, profilesusecase.ErrForbidden) {
			response.Forbidden(w, r, "forbidden")
			return
		}
		response.InternalErr(w, r, err, "failed to set primary profile")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) DownloadVCard(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}

	requestedLocale := r.URL.Query().Get("locale")
	requestedVersion := r.URL.Query().Get("version")
	vcfContent, filename, err := h.svc.GenerateVCard(r.Context(), p, profileUUID, requestedLocale, requestedVersion)
	if err != nil {
		if errors.Is(err, profilesusecase.ErrNotFound) {
			response.NotFound(w, r, "profile not found")
			return
		}
		response.InternalErr(w, r, err, "failed to generate vcard")
		return
	}

	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(vcfContent))
}

func (h *Handler) ParseVCard(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	contentType := r.Header.Get("Content-Type")

	// 1. Multipart form file upload
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			file, _, err = r.FormFile("vcard")
		}
		if err != nil {
			response.BadRequest(w, r, response.CodeValidationError, "vcard file is required")
			return
		}
		defer func() { _ = file.Close() }()

		res, err := h.svc.ParseVCard(r.Context(), p, file)
		if err != nil {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.JSON(w, r, http.StatusOK, res)
		return
	}

	// 2. JSON payload e.g. { "vcard": "BEGIN:VCARD..." }
	if strings.HasPrefix(contentType, "application/json") {
		var req struct {
			VCard string `json:"vcard"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.BadRequest(w, r, response.CodeValidationError, "invalid JSON payload")
			return
		}
		if strings.TrimSpace(req.VCard) == "" {
			response.BadRequest(w, r, response.CodeValidationError, "vcard content is required")
			return
		}

		res, err := h.svc.ParseVCard(r.Context(), p, strings.NewReader(req.VCard))
		if err != nil {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.JSON(w, r, http.StatusOK, res)
		return
	}

	// 3. Raw body
	res, err := h.svc.ParseVCard(r.Context(), p, r.Body)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return
	}
	response.JSON(w, r, http.StatusOK, res)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid JSON body")
		return err
	}
	return nil
}
