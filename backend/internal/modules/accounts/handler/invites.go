package handler

import (
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Invite error codes the invite page switches on.
const (
	codeInviteExpired       = "INVITE_EXPIRED"
	codeInviteClosed        = "INVITE_CLOSED"
	codeInviteEmailMismatch = "INVITE_EMAIL_MISMATCH"
)

// InvitesHandler serves company email invites (TEC-945).
type InvitesHandler struct {
	svc      *usecase.Invites
	activity *activity.Recorder
}

func NewInvites(svc *usecase.Invites, rec *activity.Recorder) *InvitesHandler {
	return &InvitesHandler{svc: svc, activity: rec}
}

func (h *InvitesHandler) record(r *http.Request, action string, p authctx.Principal, payload map[string]any) {
	if h.activity == nil {
		return
	}
	id := p.Account.UUID
	h.activity.Record(r.Context(), &p.UserInternal, action, activityResource, &id, payload, r)
}

func (h *InvitesHandler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.List(r.Context(), p, usecase.InviteFilter{
		Status: r.URL.Query().Get("status"), Q: q.Q, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		writeInviteErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *InvitesHandler) Create(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.Create(r.Context(), p, in.Email, in.Role)
	if err != nil {
		writeInviteErr(w, r, err)
		return
	}
	h.record(r, "accounts.invite_created", p, map[string]any{"invite_uuid": out.UUID.String(), "role": out.Role})
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *InvitesHandler) Resend(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.Resend(r.Context(), p, id)
	if err != nil {
		writeInviteErr(w, r, err)
		return
	}
	h.record(r, "accounts.invite_resent", p, map[string]any{"invite_uuid": id.String()})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *InvitesHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), p, id); err != nil {
		writeInviteErr(w, r, err)
		return
	}
	h.record(r, "accounts.invite_revoked", p, map[string]any{"invite_uuid": id.String()})
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "revoked"})
}

// Preview is public: the token is the credential.
func (h *InvitesHandler) Preview(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Preview(r.Context(), r.PathValue("token"))
	if err != nil {
		writeInviteErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *InvitesHandler) Accept(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	out, err := h.svc.Accept(r.Context(), p.UserInternal, r.PathValue("token"))
	if err != nil {
		writeInviteErr(w, r, err)
		return
	}
	if h.activity != nil {
		h.activity.Record(r.Context(), &p.UserInternal, "accounts.invite_accepted", activityResource, &out.AccountUUID,
			map[string]any{"role": out.Role}, r)
	}
	response.JSON(w, r, http.StatusOK, out)
}

func writeInviteErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrInviteExpired):
		response.Error(w, r, http.StatusGone, codeInviteExpired, err.Error())
	case errors.Is(err, usecase.ErrInviteClosed):
		response.Conflict(w, r, codeInviteClosed, err.Error())
	case errors.Is(err, usecase.ErrInviteEmailMismatch):
		response.Error(w, r, http.StatusForbidden, codeInviteEmailMismatch, err.Error())
	default:
		writeErr(w, r, err)
	}
}
