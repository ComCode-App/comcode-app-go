package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/model"
	storageusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/storage/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc *storageusecase.Service
	// maxUploadBytes caps a single multipart upload (STORAGE_MAX_BYTES).
	maxUploadBytes int64
}

// New builds the storage handler. maxUploadBytes is STORAGE_MAX_BYTES; a
// non-positive value falls back to upload.DefaultMaxBytes.
func New(svc *storageusecase.Service, maxUploadBytes int64) *Handler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = upload.DefaultMaxBytes
	}
	return &Handler{svc: svc, maxUploadBytes: maxUploadBytes}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.PlatformStorage())
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	if err := apiquery.ValidateSort(q.Sort, apiquery.StorageSort); err != nil {
		var ve *apiquery.ValidationError
		if errors.As(err, &ve) {
			details := make([]response.Detail, 0, len(ve.Details))
			for _, d := range ve.Details {
				details = append(details, response.Detail{Field: d.Field, Message: d.Message, Code: d.Code})
			}
			response.ValidationError(w, r, details)
			return
		}
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return
	}
	sortSpec := "name"
	if field, desc, ok := apiquery.PrimarySort(q.Sort, apiquery.StorageSort); ok {
		if desc {
			sortSpec = "-" + field
		} else {
			sortSpec = field
		}
	}
	vals := r.URL.Query()
	out, err := h.svc.List(r.Context(), actor(r), model.ListInput{
		Prefix:       vals.Get("prefix"),
		View:         vals.Get("view"),
		Q:            q.Q,
		Kind:         vals.Get("kind"),
		Access:       vals.Get("access"),
		Sort:         sortSpec,
		Limit:        q.Limit,
		Offset:       q.Offset,
		ModifiedFrom: vals.Get("modified_from"),
		ModifiedTo:   vals.Get("modified_to"),
		Recursive:    vals.Get("recursive") == "true",
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	obj, err := h.svc.Get(r.Context(), actor(r), r.URL.Query().Get("key"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, obj)
}

func (h *Handler) Usage(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Usage(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) {
	var in model.CreateFolderInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	obj, err := h.svc.CreateFolder(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, obj)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	// Cap the body before reading it (STORAGE_MAX_BYTES); oversize is 413.
	file, header, ok := upload.FormFile(w, r, h.maxUploadBytes, "file")
	defer upload.Cleanup(r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()
	obj, err := h.svc.Upload(r.Context(), actor(r), model.UploadMeta{
		Prefix:      r.FormValue("prefix"),
		Key:         r.FormValue("key"),
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
	}, file)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, obj)
}

func (h *Handler) CreateUploadSession(w http.ResponseWriter, r *http.Request) {
	var in model.UploadMeta
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.CreateUploadSession(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	obj, err := h.svc.CompleteUpload(r.Context(), actor(r), in.Key)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, obj)
}

func (h *Handler) Copy(w http.ResponseWriter, r *http.Request) {
	h.copyMove(w, r, false)
}

func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
	h.copyMove(w, r, true)
}

func (h *Handler) copyMove(w http.ResponseWriter, r *http.Request, move bool) {
	var in model.CopyMoveInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	var (
		obj model.Object
		err error
	)
	if move {
		obj, err = h.svc.Move(r.Context(), actor(r), in)
	} else {
		obj, err = h.svc.Copy(r.Context(), actor(r), in)
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, obj)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	var in model.RenameInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	obj, err := h.svc.Rename(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, obj)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	var in model.KeysInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if err := h.svc.Delete(r.Context(), actor(r), in.Keys); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "trashed"})
}

func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	var in model.KeysInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if err := h.svc.Restore(r.Context(), actor(r), in.Keys); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "restored"})
}

func (h *Handler) Purge(w http.ResponseWriter, r *http.Request) {
	var in model.KeysInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if err := h.svc.Purge(r.Context(), actor(r), in.Keys); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	h.stream(w, r, true)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	h.stream(w, r, false)
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request, download bool) {
	opened, err := h.svc.Open(
		r.Context(),
		r.URL.Query().Get("key"),
		r.URL.Query().Get("version_id"),
		download,
	)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = opened.Body.Close() }()
	writeFile(w, opened)
}

func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Versions(r.Context(), r.URL.Query().Get("key"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	var in model.RestoreVersionInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	obj, err := h.svc.RestoreVersion(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, obj)
}

func (h *Handler) DeleteVersion(w http.ResponseWriter, r *http.Request) {
	var in model.RestoreVersionInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if err := h.svc.DeleteVersion(r.Context(), actor(r), in.Key, in.VersionID); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.Activity(r.Context(), r.URL.Query().Get("key"), q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Star(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if err := h.svc.Star(r.Context(), actor(r), in.Key); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "starred"})
}

func (h *Handler) Unstar(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unstar(r.Context(), actor(r), r.URL.Query().Get("key")); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "unstarred"})
}

func (h *Handler) Shares(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Shares(r.Context(), r.URL.Query().Get("key"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Share(w http.ResponseWriter, r *http.Request) {
	var in model.CreateShareInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	item, err := h.svc.Share(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, item)
}

func (h *Handler) Unshare(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unshare(r.Context(), actor(r), r.PathValue("uuid")); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) Links(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Links(r.Context(), r.URL.Query().Get("key"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	var in model.CreateLinkInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	item, err := h.svc.CreateLink(r.Context(), actor(r), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, item)
}

func (h *Handler) RevokeLink(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RevokeLink(r.Context(), actor(r), r.PathValue("uuid")); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handler) PublicSlug(w http.ResponseWriter, r *http.Request) {
	download := r.URL.Query().Get("download") == "1"
	opened, err := h.svc.ResolvePublic(r.Context(), r.PathValue("slug"), download)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = opened.Body.Close() }()
	writeFile(w, opened)
}

func (h *Handler) PublicSigned(w http.ResponseWriter, r *http.Request) {
	download := r.URL.Query().Get("download") == "1"
	opened, err := h.svc.ResolveSigned(r.Context(), r.PathValue("token"), download)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = opened.Body.Close() }()
	writeFile(w, opened)
}

func (h *Handler) StreamMedia(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		response.NotFound(w, r, "key required")
		return
	}
	driver := h.svc.Driver()
	if driver == nil {
		response.NotFound(w, r, "storage not configured")
		return
	}
	rc, size, err := driver.Download(r.Context(), key)
	if err != nil {
		response.NotFound(w, r, "media not found")
		return
	}
	defer func() { _ = rc.Close() }()

	// The type comes from the server-chosen key extension; active types
	// (SVG, HTML, XML, ...) are served as attachments with nosniff and a
	// sandbox CSP so they never run as a document on our origin.
	upload.SetServeHeaders(w, upload.ContentTypeForKey(key), path.Base(key), false)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	_, _ = io.Copy(w, rc)
}

// writeFile streams a stored object. The stored MIME type may be
// uploader-declared, so only inline-safe types render inline; everything else
// (HTML, SVG, XML, scripts, unknown) downloads, always with nosniff.
func writeFile(w http.ResponseWriter, opened model.OpenObject) {
	upload.SetServeHeaders(w, opened.Object.MimeType, opened.Object.Name, opened.Download)
	if opened.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(opened.Size, 10))
	}
	_, _ = io.Copy(w, opened.Body)
}

func actor(r *http.Request) model.Actor {
	p, ok := authctx.PrincipalFrom(r.Context())
	if !ok {
		return model.Actor{}
	}
	return model.Actor{UserID: p.UserInternal, Name: p.Email}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, storageusecase.ErrNotFound):
		response.NotFound(w, r, "object not found")
	case errors.Is(err, storageusecase.ErrInvalidKey):
		response.BadRequest(w, r, response.CodeValidationError, "invalid object key")
	case errors.Is(err, storageusecase.ErrConflict):
		response.Conflict(w, r, response.CodeConflict, "object already exists")
	case errors.Is(err, storageusecase.ErrForbidden):
		response.Forbidden(w, r, "access denied")
	case errors.Is(err, storageusecase.ErrLinkExpired):
		response.Forbidden(w, r, "link expired")
	case errors.Is(err, storageusecase.ErrLinkRevoked):
		response.Forbidden(w, r, "link revoked")
	default:
		response.InternalErr(w, r, err, "storage operation failed")
	}
}
