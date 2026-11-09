package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	paymentusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/payment/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc      *paymentusecase.Service
	activity *activity.Recorder
}

func New(svc *paymentusecase.Service, rec *activity.Recorder) *Handler {
	return &Handler{svc: svc, activity: rec}
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetSettings(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load payment settings")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) PatchSettings(w http.ResponseWriter, r *http.Request) {
	var in paymentusecase.PatchSettingsInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.PatchSettings(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.record(r, "integrations.payment.settings.updated", map[string]any{"active_provider": out.ActiveProvider})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) GetPayTR(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetPayTR(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load paytr settings")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) PatchPayTR(w http.ResponseWriter, r *http.Request) {
	var in paymentusecase.PatchPayTRInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.PatchPayTR(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.record(r, "integrations.payment.paytr.updated", map[string]any{"enabled": out.Enabled})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) GetIyzico(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetIyzico(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to load iyzico settings")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) PatchIyzico(w http.ResponseWriter, r *http.Request) {
	var in paymentusecase.PatchIyzicoInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.PatchIyzico(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.record(r, "integrations.payment.iyzico.updated", map[string]any{"enabled": out.Enabled})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) record(r *http.Request, action string, payload map[string]any) {
	if h.activity == nil {
		return
	}
	p, ok := authctx.PrincipalFrom(r.Context())
	if !ok {
		return
	}
	id := p.UserInternal
	h.activity.Record(r.Context(), &id, action, "platform.payment", nil, payload, r)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, paymentusecase.ErrInvalidRequest) {
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return
	}
	response.InternalErr(w, r, err, "failed to update payment settings")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
