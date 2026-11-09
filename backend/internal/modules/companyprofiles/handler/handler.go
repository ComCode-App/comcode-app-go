// Package handler serves the member-profile routes of company accounts (TEC-573).
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc  *usecase.Service
	bulk bulkengine.BulkAdapter
}

func New(svc *usecase.Service, bulk bulkengine.BulkAdapter) *Handler {
	return &Handler{svc: svc, bulk: bulk}
}

func write(w http.ResponseWriter, r *http.Request, out any, err error) {
	switch {
	case err == nil:
		response.JSON(w, r, http.StatusOK, out)
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, "Forbidden")
	case errors.Is(err, model.ErrNotFound):
		response.NotFound(w, r, "Not found")
	case errors.Is(err, model.ErrInvalid):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, model.ErrConflict):
		response.Conflict(w, r, response.CodeConflict, "Profile is linked to another member")
	default:
		response.InternalErr(w, r, err, "company profiles operation failed")
	}
}

func optionalUUID(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, model.ErrInvalid
	}
	return &id, nil
}

// MemberProfiles lists members of the active company with assignment and linked profile.
func (h *Handler) MemberProfiles(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	values := r.URL.Query()
	f := model.MemberProfileFilter{Q: values.Get("q"), Limit: q.Limit, Offset: q.Offset}
	var err error
	if f.OfficeUUID, err = optionalUUID(values.Get("office_uuid")); err != nil {
		write(w, r, nil, err)
		return
	}
	if f.GroupUUID, err = optionalUUID(values.Get("group_uuid")); err != nil {
		write(w, r, nil, err)
		return
	}
	switch values.Get("has_profile") {
	case "true", "1":
		v := true
		f.HasProfile = &v
	case "false", "0":
		v := false
		f.HasProfile = &v
	}
	items, total, err := h.svc.MemberProfiles(r.Context(), authctx.MustPrincipal(r.Context()), f)
	if err != nil {
		write(w, r, nil, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	write(w, r, apiquery.NewPage(items, total, limit, q.Offset), nil)
}

// Meta describes the member-profile list and its bulk actions.
func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	write(w, r, resourcemeta.ResourceMeta{
		Resource:         model.ResourceCompanyProfiles,
		DefaultSort:      "name",
		DefaultFields:    []string{"name", "office_uuid", "group_uuid", "profile"},
		Capabilities:     resourcemeta.Capabilities{Read: true, Search: true, Filter: true, Bulk: true, Import: true},
		SearchableFields: []string{"name", "surname", "email"},
		SortableFields:   []string{},
		FilterableFields: []string{"office_uuid", "group_uuid", "has_profile"},
		Columns:          []resourcemeta.Column{},
		Filters:          []resourcemeta.Filter{},
		Includes:         []string{},
		BulkActions:      h.bulk.BulkActions(),
	}, nil)
}

// LinkProfile sets or clears the member's employee profile.
func (h *Handler) LinkProfile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil || id == uuid.Nil {
		write(w, r, nil, model.ErrInvalid)
		return
	}
	var in struct {
		ProfileUUID *uuid.UUID `json:"profile_uuid"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		write(w, r, nil, model.ErrInvalid)
		return
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		write(w, r, nil, model.ErrInvalid)
		return
	}
	err = h.svc.LinkProfile(r.Context(), authctx.MustPrincipal(r.Context()), id, in.ProfileUUID)
	write(w, r, map[string]any{"user_uuid": id, "profile_uuid": in.ProfileUUID}, err)
}
