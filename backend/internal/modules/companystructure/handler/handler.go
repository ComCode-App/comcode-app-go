package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc     *usecase.Service
	catalog *usecase.Catalog
	admin   *usecase.AdminService
}

func New(s *usecase.Service, c *usecase.Catalog, a *usecase.AdminService) *Handler {
	return &Handler{svc: s, catalog: c, admin: a}
}
func write(w http.ResponseWriter, r *http.Request, out any, err error, status int) {
	switch {
	case err == nil:
		response.JSON(w, r, status, out)
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, err.Error())
	case errors.Is(err, model.ErrNotFound):
		response.NotFound(w, r, err.Error())
	case errors.Is(err, model.ErrInvalid):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, model.ErrConflict):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		response.InternalErr(w, r, err, "company structure operation failed")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		write(w, r, nil, model.ErrInvalid, 0)
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		write(w, r, nil, model.ErrInvalid, 0)
		return false
	}
	return true
}
func pathID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil || id == uuid.Nil {
		write(w, r, nil, model.ErrInvalid, 0)
		return uuid.Nil, false
	}
	return id, true
}
func listQuery(w http.ResponseWriter, r *http.Request, defaultSort string) (apiquery.Query, bool) {
	q := apiquery.Parse(r.URL.Query())
	// These small catalogs have a documented stable order; arbitrary sorting is unsupported.
	if sort := r.URL.Query().Get("sort"); sort != "" && sort != defaultSort {
		response.ValidationError(w, r, []response.Detail{{Field: "sort", Message: "unsupported sort", Code: "invalid"}})
		return q, false
	}
	return q, true
}

func (h *Handler) Offices(w http.ResponseWriter, r *http.Request) {
	q, ok := listQuery(w, r, "sort_order,name")
	if !ok {
		return
	}
	items, total, err := h.svc.Offices(r.Context(), authctx.MustPrincipal(r.Context()), q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(items, total, q.Limit, q.Offset), err, http.StatusOK)
}
func (h *Handler) GetOffice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.Office(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) CreateOffice(w http.ResponseWriter, r *http.Request) {
	var in model.OfficeInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.SaveOffice(r.Context(), authctx.MustPrincipal(r.Context()), uuid.Nil, in)
	write(w, r, out, err, http.StatusCreated)
}
func (h *Handler) UpdateOffice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	var in model.OfficeInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.SaveOffice(r.Context(), authctx.MustPrincipal(r.Context()), id, in)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) DeleteOffice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	err := h.svc.DeleteOffice(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, map[string]string{"status": "deleted"}, err, http.StatusOK)
}

func (h *Handler) Groups(w http.ResponseWriter, r *http.Request) {
	q, ok := listQuery(w, r, "name")
	if !ok {
		return
	}
	var f model.GroupFilter
	// office_id takes an office UUID, or "none" for company-wide groups (no office).
	switch v := r.URL.Query().Get("office_id"); v {
	case "":
	case "none":
		f.CompanyWide = true
	default:
		id, err := uuid.Parse(v)
		if err != nil || id == uuid.Nil {
			response.ValidationError(w, r, []response.Detail{{Field: "office_id", Message: "must be an office uuid or none", Code: "invalid"}})
			return
		}
		f.OfficeUUID = &id
	}
	items, total, err := h.svc.Groups(r.Context(), authctx.MustPrincipal(r.Context()), f, q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(items, total, q.Limit, q.Offset), err, http.StatusOK)
}
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.Group(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var in model.GroupInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.SaveGroup(r.Context(), authctx.MustPrincipal(r.Context()), uuid.Nil, in)
	write(w, r, out, err, http.StatusCreated)
}
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	var in model.GroupInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.SaveGroup(r.Context(), authctx.MustPrincipal(r.Context()), id, in)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	err := h.svc.DeleteGroup(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, map[string]string{"status": "deleted"}, err, http.StatusOK)
}

func (h *Handler) Assignments(w http.ResponseWriter, r *http.Request) {
	q, ok := listQuery(w, r, "member")
	if !ok {
		return
	}
	items, total, err := h.svc.Assignments(r.Context(), authctx.MustPrincipal(r.Context()), q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(items, total, q.Limit, q.Offset), err, http.StatusOK)
}
func (h *Handler) Assignment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := h.svc.Assignment(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) SetAssignment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		OfficeUUID *uuid.UUID `json:"office_uuid"`
		GroupUUID  *uuid.UUID `json:"group_uuid"`
	}
	if !decode(w, r, &in) {
		return
	}
	out, err := h.svc.SetAssignment(r.Context(), authctx.MustPrincipal(r.Context()), model.Assignment{UserUUID: id, OfficeUUID: in.OfficeUUID, GroupUUID: in.GroupUUID})
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) Scopes(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	out, err := h.svc.Scopes(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, map[string]any{"office_uuids": out}, err, http.StatusOK)
}
func (h *Handler) SetScopes(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		OfficeUUIDs []uuid.UUID `json:"office_uuids"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.OfficeUUIDs == nil {
		write(w, r, nil, model.ErrInvalid, 0)
		return
	}
	err := h.svc.SetScopes(r.Context(), authctx.MustPrincipal(r.Context()), id, in.OfficeUUIDs)
	write(w, r, map[string]any{"office_uuids": in.OfficeUUIDs}, err, http.StatusOK)
}
func (h *Handler) Sector(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Sector(r.Context(), authctx.MustPrincipal(r.Context()))
	write(w, r, map[string]any{"sector_uuid": out}, err, http.StatusOK)
}
func (h *Handler) SetSector(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SectorUUID *uuid.UUID `json:"sector_uuid"`
	}
	if !decode(w, r, &in) {
		return
	}
	err := h.svc.SetSector(r.Context(), authctx.MustPrincipal(r.Context()), in.SectorUUID)
	write(w, r, in, err, http.StatusOK)
}
func (h *Handler) Sectors(w http.ResponseWriter, r *http.Request) {
	q, ok := listQuery(w, r, "slug")
	if !ok {
		return
	}
	items, total, err := h.catalog.List(r.Context(), authctx.MustPrincipal(r.Context()), q.Q, q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(items, total, q.Limit, q.Offset), err, http.StatusOK)
}
func (h *Handler) GetSector(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.catalog.GetAdmin(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) CreateSector(w http.ResponseWriter, r *http.Request) {
	var in model.SectorInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.catalog.SaveAdmin(r.Context(), authctx.MustPrincipal(r.Context()), uuid.Nil, in)
	write(w, r, out, err, http.StatusCreated)
}
func (h *Handler) UpdateSector(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	var in model.SectorInput
	if !decode(w, r, &in) {
		return
	}
	out, err := h.catalog.SaveAdmin(r.Context(), authctx.MustPrincipal(r.Context()), id, in)
	write(w, r, out, err, http.StatusOK)
}
func (h *Handler) DeleteSector(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	err := h.catalog.DeleteAdmin(r.Context(), authctx.MustPrincipal(r.Context()), id)
	write(w, r, map[string]string{"status": "deleted"}, err, http.StatusOK)
}
func (h *Handler) SectorsMeta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.ResourceMeta{
		Resource: "platform.sectors", DefaultSort: "slug",
		DefaultFields:    []string{"uuid", "slug", "name_tr", "name_en"},
		Capabilities:     resourcemeta.Capabilities{Create: true, Read: true, Update: true, Delete: true, Search: true},
		SearchableFields: []string{"slug", "name_tr", "name_en"},
		SortableFields:   []string{"slug"}, FilterableFields: []string{},
		Columns: []resourcemeta.Column{
			{Key: "uuid", LabelKey: "sectors.fields.uuid", Type: resourcemeta.ColumnTypeUUID, DefaultVisible: true},
			{Key: "slug", LabelKey: "sectors.fields.slug", Type: resourcemeta.ColumnTypeString, DefaultVisible: true},
			{Key: "name_tr", LabelKey: "sectors.fields.name_tr", Type: resourcemeta.ColumnTypeString, DefaultVisible: true},
			{Key: "name_en", LabelKey: "sectors.fields.name_en", Type: resourcemeta.ColumnTypeString, DefaultVisible: true},
			{Key: "created_at", LabelKey: "sectors.fields.created_at", Type: resourcemeta.ColumnTypeDatetime},
			{Key: "updated_at", LabelKey: "sectors.fields.updated_at", Type: resourcemeta.ColumnTypeDatetime},
		},
		Filters: []resourcemeta.Filter{}, Includes: []string{},
	})
}

func (h *Handler) AdminOffices(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	q, ok := listQuery(w, r, "sort_order,name")
	if !ok {
		return
	}
	out, total, err := h.admin.Offices(r.Context(), authctx.MustPrincipal(r.Context()), id, q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(out, total, q.Limit, q.Offset), err, http.StatusOK)
}
func (h *Handler) AdminGroups(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "uuid")
	if !ok {
		return
	}
	q, ok := listQuery(w, r, "name")
	if !ok {
		return
	}
	out, total, err := h.admin.Groups(r.Context(), authctx.MustPrincipal(r.Context()), id, q.Limit, q.Offset)
	write(w, r, apiquery.NewPage(out, total, q.Limit, q.Offset), err, http.StatusOK)
}
