package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	plansusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/plans/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *plansusecase.Service
}

func New(svc *plansusecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.PlatformPlans())
}

func (h *Handler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListFeatures(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to list feature definitions")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.List(r.Context(), q.Limit, q.Offset, q.Q)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list plans")
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
	plan, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, plansusecase.ErrNotFound) {
			response.NotFound(w, r, "plan not found")
			return
		}
		response.InternalErr(w, r, err, "failed to load plan")
		return
	}
	response.JSON(w, r, http.StatusOK, plan)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Slug            string                        `json:"slug"`
		Name            string                        `json:"name"`
		Description     *string                       `json:"description"`
		BillingInterval string                        `json:"billing_interval"`
		PriceMinor      int64                         `json:"price_minor"`
		Currency        string                        `json:"currency"`
		TrialDays       int32                         `json:"trial_days"`
		IsActive        bool                          `json:"is_active"`
		SortOrder       int32                         `json:"sort_order"`
		Limits          []plansusecase.PlanLimitInput `json:"limits"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	plan, err := h.svc.Create(r.Context(), plansusecase.CreatePlanInput{
		Slug: in.Slug, Name: in.Name, Description: in.Description,
		BillingInterval: in.BillingInterval, PriceMinor: in.PriceMinor,
		Currency: in.Currency, TrialDays: in.TrialDays,
		IsActive: in.IsActive, SortOrder: in.SortOrder, Limits: in.Limits,
	})
	if err != nil {
		if writeValidation(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to create plan")
		return
	}
	response.JSON(w, r, http.StatusCreated, plan)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	var in struct {
		Name            *string                        `json:"name"`
		Description     *string                        `json:"description"`
		BillingInterval *string                        `json:"billing_interval"`
		PriceMinor      *int64                         `json:"price_minor"`
		Currency        *string                        `json:"currency"`
		TrialDays       *int32                         `json:"trial_days"`
		IsActive        *bool                          `json:"is_active"`
		SortOrder       *int32                         `json:"sort_order"`
		Limits          *[]plansusecase.PlanLimitInput `json:"limits"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	plan, err := h.svc.Patch(r.Context(), id, plansusecase.PatchPlanInput{
		Name: in.Name, Description: in.Description, BillingInterval: in.BillingInterval,
		PriceMinor: in.PriceMinor, Currency: in.Currency, TrialDays: in.TrialDays,
		IsActive: in.IsActive, SortOrder: in.SortOrder, Limits: in.Limits,
	})
	if err != nil {
		if errors.Is(err, plansusecase.ErrNotFound) {
			response.NotFound(w, r, "plan not found")
			return
		}
		if writeValidation(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to update plan")
		return
	}
	response.JSON(w, r, http.StatusOK, plan)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, plansusecase.ErrNotFound) {
			response.NotFound(w, r, "plan not found")
			return
		}
		if errors.Is(err, plansusecase.ErrConflict) {
			response.Conflict(w, r, response.CodeConflict, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to delete plan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}

// writeValidation answers 400 VALIDATION_ERROR for ErrInvalidRequest, with field details when
// the use case returned a *ValidationError (plan limits). It reports whether it wrote.
func writeValidation(w http.ResponseWriter, r *http.Request, err error) bool {
	var verr *plansusecase.ValidationError
	if errors.As(err, &verr) {
		details := make([]response.Detail, 0, len(verr.Fields))
		for _, f := range verr.Fields {
			details = append(details, response.Detail{Field: f.Field, Message: f.Message, Code: f.Code})
		}
		response.ValidationError(w, r, details)
		return true
	}
	if errors.Is(err, plansusecase.ErrInvalidRequest) {
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return true
	}
	return false
}
