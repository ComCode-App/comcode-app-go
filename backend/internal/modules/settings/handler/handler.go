package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"

	settingsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/settings/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc     *settingsusecase.Service
	storage storage.Driver
	sysSvc  *settingsusecase.SystemService
}

func New(svc *settingsusecase.Service, store storage.Driver, sysSvc *settingsusecase.SystemService) *Handler {
	return &Handler{svc: svc, storage: store, sysSvc: sysSvc}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load settings")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	var in settingsusecase.PatchInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	s, err := h.svc.Patch(r.Context(), in)
	if err != nil {
		response.InternalErr(w, r, err, "failed to update settings")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

func (h *Handler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	file, header, ok := upload.FormFile(w, r, storage.MaxLogoBytes, "logo")
	defer upload.Cleanup(r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()
	// Type and extension come from the bytes: raster or a static SVG that
	// passes upload.ValidateSVG; the client Content-Type is ignored.
	sniffed, err := upload.SniffLogo(file)
	if err != nil {
		if upload.WriteError(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "logo upload failed")
		return
	}
	key := storage.AppLogoObjectKey(sniffed.Ext)
	if err := h.storage.Upload(r.Context(), storage.File{
		Body: sniffed.Body, Size: header.Size, ContentType: sniffed.ContentType, Filename: header.Filename,
	}, key); err != nil {
		response.InternalErr(w, r, err, "logo upload failed")
		return
	}
	s, err := h.svc.SetLogo(r.Context(), key)
	if err != nil {
		response.InternalErr(w, r, err, "failed to save logo")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

func (h *Handler) StreamLogo(w http.ResponseWriter, r *http.Request) {
	key, err := h.svc.LogoObjectKey(r.Context())
	if err != nil {
		if errors.Is(err, settingsusecase.ErrNotFound) {
			response.NotFound(w, r, "logo not set")
			return
		}
		response.InternalErr(w, r, err, "failed to load logo")
		return
	}
	rc, _, err := h.storage.Download(r.Context(), key)
	if err != nil {
		response.NotFound(w, r, "logo not found")
		return
	}
	defer func() { _ = rc.Close() }()
	upload.SetServeHeaders(w, storage.MIMEFromLogoKey(key), path.Base(key), false)
	_, _ = io.Copy(w, rc)
}

func (h *Handler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	key, err := h.svc.LogoObjectKey(r.Context())
	if err == nil {
		_ = h.storage.Delete(r.Context(), key)
	}
	s, err := h.svc.ClearLogo(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to clear logo")
		return
	}
	response.JSON(w, r, http.StatusOK, s)
}

func (h *Handler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	if h.sysSvc == nil {
		response.InternalErr(w, r, errors.New("system service unavailable"), "system diagnostics unavailable")
		return
	}
	info := h.sysSvc.GetInfo(r.Context())
	response.JSON(w, r, http.StatusOK, info)
}

func (h *Handler) PingSystem(w http.ResponseWriter, r *http.Request) {
	if h.sysSvc == nil {
		response.InternalErr(w, r, errors.New("system service unavailable"), "system diagnostics unavailable")
		return
	}
	info := h.sysSvc.GetInfo(r.Context())
	response.JSON(w, r, http.StatusOK, info)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
