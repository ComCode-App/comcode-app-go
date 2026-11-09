package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

const activityResource = "accounts"

// EntitlementOverview resolves the active account's plan and usage (entitlement.Service).
type EntitlementOverview interface {
	OverviewForAccount(ctx context.Context, accountID int64) (entitlement.Overview, error)
}

type Handler struct {
	svc          *usecase.Service
	activity     *activity.Recorder
	entitlements EntitlementOverview
}

func New(svc *usecase.Service, rec *activity.Recorder, entitlements EntitlementOverview) *Handler {
	return &Handler{svc: svc, activity: rec, entitlements: entitlements}
}

func (h *Handler) record(r *http.Request, action string, accountUUID uuid.UUID, payload map[string]any) {
	if h.activity == nil {
		return
	}
	p := authctx.MustPrincipal(r.Context())
	h.activity.Record(r.Context(), &p.UserInternal, action, activityResource, &accountUUID, payload, r)
}

// ---- App: active account ----

func (h *Handler) Current(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	out, err := h.svc.Current(r.Context(), p)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Entitlements returns the active account's plan, every catalog feature and quota usage, so the
// app can show limits and upgrade hints. Any member may read it.
func (h *Handler) Entitlements(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	out, err := h.entitlements.OverviewForAccount(r.Context(), p.Account.ID)
	if err != nil {
		response.InternalErr(w, r, err, "failed to load plan entitlements")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.Rename(r.Context(), p, in.Name)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.updated", p.Account.UUID, map[string]any{"name": out.Name})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	items, err := h.svc.Members(r.Context(), p)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	m, err := h.svc.AddMember(r.Context(), p, in.Email, in.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_added", p.Account.UUID, map[string]any{"user_uuid": m.UserUUID.String(), "role": m.Role})
	response.JSON(w, r, http.StatusCreated, m)
}

func (h *Handler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	userUUID, ok := pathUUID(w, r, "user_uuid")
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	m, err := h.svc.ChangeRole(r.Context(), p, userUUID, in.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_role_changed", p.Account.UUID, map[string]any{"user_uuid": userUUID.String(), "role": m.Role})
	response.JSON(w, r, http.StatusOK, m)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	userUUID, ok := pathUUID(w, r, "user_uuid")
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(r.Context(), p, userUUID); err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_removed", p.Account.UUID, map[string]any{"user_uuid": userUUID.String()})
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "removed"})
}

// ---- Platform ----

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.PlatformAccounts())
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.List(r.Context(), q.Q, r.URL.Query().Get("type"), r.URL.Query().Get("status"), q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) CreateCompany(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name          string    `json:"name"`
		Slug          string    `json:"slug"`
		OwnerUserUUID uuid.UUID `json:"owner_user_uuid"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.CreateCompany(r.Context(), in.Name, in.Slug, in.OwnerUserUUID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.created", out.UUID, map[string]any{"name": out.Name, "owner_user_uuid": in.OwnerUserUUID.String()})
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	var in struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.Patch(r.Context(), id, in.Name, in.Status)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.updated", id, map[string]any{"name": out.Name, "status": out.Status})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) PlatformAddMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	var in struct {
		UserUUID uuid.UUID `json:"user_uuid"`
		Role     string    `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	m, err := h.svc.PlatformAddMember(r.Context(), id, in.UserUUID, in.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_added", id, map[string]any{"user_uuid": in.UserUUID.String(), "role": m.Role})
	response.JSON(w, r, http.StatusCreated, m)
}

func (h *Handler) PlatformChangeRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	userUUID, ok := pathUUID(w, r, "user_uuid")
	if !ok {
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	m, err := h.svc.PlatformChangeRole(r.Context(), id, userUUID, in.Role)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_role_changed", id, map[string]any{"user_uuid": userUUID.String(), "role": m.Role})
	response.JSON(w, r, http.StatusOK, m)
}

func (h *Handler) PlatformRemoveMember(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	userUUID, ok := pathUUID(w, r, "user_uuid")
	if !ok {
		return
	}
	if err := h.svc.PlatformRemoveMember(r.Context(), id, userUUID); err != nil {
		writeErr(w, r, err)
		return
	}
	h.record(r, "accounts.member_removed", id, map[string]any{"user_uuid": userUUID.String()})
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "removed"})
}

// ---- helpers ----

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
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
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, err.Error())
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, err.Error())
	case errors.Is(err, usecase.ErrInvalidRequest), errors.Is(err, usecase.ErrPersonalAccount):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, usecase.ErrConflict), errors.Is(err, usecase.ErrLastOwner):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		response.InternalErr(w, r, err, "account operation failed")
	}
}
