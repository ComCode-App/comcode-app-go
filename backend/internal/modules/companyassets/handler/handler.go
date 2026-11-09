// Package handler exposes company employee assets through the standard API envelope.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct{ svc *usecase.Service }

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func write(w http.ResponseWriter, r *http.Request, status int, out any, err error) {
	var validation *apiquery.ValidationError
	if errors.As(err, &validation) {
		details := make([]response.Detail, 0, len(validation.Details))
		for _, d := range validation.Details {
			details = append(details, response.Detail{Field: d.Field, Message: d.Message, Code: d.Code})
		}
		response.ValidationError(w, r, details)
		return
	}
	switch {
	case err == nil:
		response.JSON(w, r, status, out)
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, "Forbidden")
	case errors.Is(err, model.ErrNotFound):
		response.NotFound(w, r, "Not found")
	case errors.Is(err, model.ErrInvalid):
		response.ValidationError(w, r, []response.Detail{{Field: "input", Message: "Invalid filters, sort or request body"}})
	case errors.Is(err, model.ErrConflict):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		if entitlement.WriteError(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "company asset operation failed")
	}
}

func filter(r *http.Request, cards bool) (model.Filter, error) {
	v := r.URL.Query()
	if len(apiquery.ForbiddenParams(v)) > 0 {
		return model.Filter{}, model.ErrInvalid
	}
	q := apiquery.Parse(v)
	f := model.Filter{
		Q: q.Q, Status: v.Get("status"), Sort: strings.TrimSpace(v.Get("sort")),
		Limit: q.Limit, Offset: q.Offset,
	}
	for _, field := range []struct {
		name string
		dest **uuid.UUID
	}{
		{name: "office_uuid", dest: &f.OfficeUUID}, {name: "group_uuid", dest: &f.GroupUUID},
	} {
		if raw := v.Get(field.name); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil || id == uuid.Nil {
				return f, model.ErrInvalid
			}
			*field.dest = &id
		}
	}
	if raw := v.Get("is_active"); raw != "" {
		if !cards {
			return f, model.ErrInvalid
		}
		active, err := strconv.ParseBool(raw)
		if err != nil {
			return f, model.ErrInvalid
		}
		f.IsActive = &active
	}
	return f, nil
}

func (h *Handler) Cards(w http.ResponseWriter, r *http.Request) {
	f, err := filter(r, true)
	if err != nil {
		write(w, r, 0, nil, err)
		return
	}
	items, total, err := h.svc.Cards(r.Context(), authctx.MustPrincipal(r.Context()), f)
	write(w, r, http.StatusOK, apiquery.NewPage(items, total, f.Limit, f.Offset), err)
}
func (h *Handler) Orders(w http.ResponseWriter, r *http.Request) {
	f, err := filter(r, false)
	if err != nil {
		write(w, r, 0, nil, err)
		return
	}
	items, total, err := h.svc.Orders(r.Context(), authctx.MustPrincipal(r.Context()), f)
	write(w, r, http.StatusOK, apiquery.NewPage(items, total, f.Limit, f.Offset), err)
}

func (h *Handler) meta(w http.ResponseWriter, r *http.Request, cards bool) {
	resource, permission, name := "app.company_orders", rbac.PermAppOrdersRead, "order_number"
	if cards {
		resource, permission, name = "app.company_cards", rbac.PermAppCardsRead, "name"
	}
	if err := h.svc.CheckAccess(r.Context(), authctx.MustPrincipal(r.Context()), permission); err != nil {
		write(w, r, 0, nil, err)
		return
	}
	filters := []string{"office_uuid", "group_uuid", "status"}
	if cards {
		filters = append(filters, "is_active")
	}
	write(w, r, http.StatusOK, resourcemeta.ResourceMeta{
		Resource: resource, DefaultSort: "-created_at", DefaultFields: []string{"uuid", name, "status"},
		Capabilities:     resourcemeta.Capabilities{Read: true, Search: true, Filter: true, Sort: true},
		SearchableFields: []string{name}, SortableFields: []string{"created_at", name, "status"}, FilterableFields: filters,
		Columns: []resourcemeta.Column{}, Filters: []resourcemeta.Filter{}, Includes: []string{},
		BulkActions: []bulkengine.BulkActionDef{},
	}, nil)
}
func (h *Handler) CardsMeta(w http.ResponseWriter, r *http.Request)  { h.meta(w, r, true) }
func (h *Handler) OrdersMeta(w http.ResponseWriter, r *http.Request) { h.meta(w, r, false) }

func decode(w http.ResponseWriter, r *http.Request, in any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(in); err != nil {
		return model.ErrInvalid
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return model.ErrInvalid
	}
	return nil
}
func cardID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, model.ErrInvalid
	}
	return id, nil
}
func (h *Handler) AssignCard(w http.ResponseWriter, r *http.Request) {
	id, err := cardID(r)
	if err != nil {
		write(w, r, 0, nil, err)
		return
	}
	var in struct {
		MemberUUID uuid.UUID `json:"member_uuid"`
	}
	if err = decode(w, r, &in); err == nil {
		err = h.svc.AssignCard(r.Context(), authctx.MustPrincipal(r.Context()), id, in.MemberUUID)
	}
	write(w, r, http.StatusOK, map[string]any{"uuid": id, "member_uuid": in.MemberUUID}, err)
}
func (h *Handler) SetCardActive(w http.ResponseWriter, r *http.Request) {
	id, err := cardID(r)
	if err != nil {
		write(w, r, 0, nil, err)
		return
	}
	var in struct {
		IsActive *bool `json:"is_active"`
	}
	if err = decode(w, r, &in); err == nil {
		if in.IsActive == nil {
			err = model.ErrInvalid
		} else {
			err = h.svc.SetCardActive(r.Context(), authctx.MustPrincipal(r.Context()), id, *in.IsActive)
		}
	}
	write(w, r, http.StatusOK, map[string]any{"uuid": id, "is_active": in.IsActive}, err)
}
func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	var in model.ReorderInput
	if err := decode(w, r, &in); err != nil {
		write(w, r, 0, nil, err)
		return
	}
	out, err := h.svc.Reorder(r.Context(), authctx.MustPrincipal(r.Context()), in)
	write(w, r, http.StatusCreated, out, err)
}
