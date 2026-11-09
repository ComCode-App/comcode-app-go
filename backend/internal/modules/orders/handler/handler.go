package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct {
	svc      *usecase.Service
	activity activityRecorder
	stepUp   stepUpChecker
}

// stepUpChecker reports whether the user re-authenticated recently (platform/stepup.Service).
type stepUpChecker interface {
	HasValidGrant(ctx context.Context, userUUID uuid.UUID) (bool, error)
}

// SetStepUp requires a step-up grant for moving an order to refunded, which refunds an online
// payment at the provider (TEC-935).
func (h *Handler) SetStepUp(s stepUpChecker) { h.stepUp = s }

// requireRefundStepUp writes STEP_UP_REQUIRED and returns false when a change to refunded is
// made without a recent step-up grant.
func (h *Handler) requireRefundStepUp(w http.ResponseWriter, r *http.Request) bool {
	if h.stepUp == nil {
		return true
	}
	p, ok := authctx.PrincipalFrom(r.Context())
	if !ok {
		response.Unauthorized(w, r, "Authentication is required")
		return false
	}
	valid, err := h.stepUp.HasValidGrant(r.Context(), p.UserID)
	if err != nil {
		response.Internal(w, r, "Failed to verify step-up grant")
		return false
	}
	if !valid {
		response.StepUpRequired(w, r)
		return false
	}
	return true
}

// writeRefundError maps a provider refund that blocked a change to refunded (same codes as the
// /refunds routes); it returns false for any other error.
func writeRefundError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, usecase.ErrRefundFailed):
		response.Error(w, r, http.StatusBadGateway, "REFUND_FAILED", err.Error())
	case errors.Is(err, usecase.ErrRefundInProgress):
		response.Conflict(w, r, "REFUND_IN_PROGRESS", err.Error())
	default:
		return false
	}
	return true
}

// activityRecorder writes the platform activity log (platform/activity.Recorder).
type activityRecorder interface {
	Record(ctx context.Context, actorID *int64, action, resource string, resourceUUID *uuid.UUID, payload map[string]any, r *http.Request)
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

// SetActivity records manual payment changes made from the platform panel.
func (h *Handler) SetActivity(a activityRecorder) { h.activity = a }

// recordManualPayment logs a platform admin marking an order's payment by hand (TEC-943): online
// checkout pays orders automatically, the manual action stays for exceptions.
func (h *Handler) recordManualPayment(r *http.Request, id uuid.UUID, before *model.OrderDetail, after *model.OrderDetail, action string) {
	if h.activity == nil || after == nil {
		return
	}
	payload := map[string]any{
		"order_number":      after.OrderNumber,
		"status":            after.Status,
		"payment_status":    after.PaymentStatus,
		"payment_provider":  after.PaymentProvider,
		"payment_reference": after.PaymentReference,
	}
	if before != nil {
		payload["previous_status"] = before.Status
		payload["previous_payment_status"] = before.PaymentStatus
	}
	var actor *int64
	if p, ok := authctx.PrincipalFrom(r.Context()); ok && p.UserInternal > 0 {
		v := p.UserInternal
		actor = &v
	}
	h.activity.Record(r.Context(), actor, action, "orders", &id, payload, r)
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.PlatformOrders()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) KPIs(w http.ResponseWriter, r *http.Request) {
	kpis, err := h.svc.GetOverviewKPIs(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to get order kpis")
		return
	}
	response.JSON(w, r, http.StatusOK, kpis)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())

	var status *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}

	var paymentStatus *string
	if ps := r.URL.Query().Get("payment_status"); ps != "" {
		paymentStatus = &ps
	}

	var fulfillmentStatus *string
	if fs := r.URL.Query().Get("fulfillment_status"); fs != "" {
		fulfillmentStatus = &fs
	}

	var orderType *string
	if ot := r.URL.Query().Get("order_type"); ot != "" {
		orderType = &ot
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.ListAll(r.Context(), qStr, nil, status, paymentStatus, fulfillmentStatus, orderType, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list orders")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) ListMyOrders(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	q := apiquery.Parse(r.URL.Query())
	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	var status *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}

	var paymentStatus *string
	if ps := r.URL.Query().Get("payment_status"); ps != "" {
		paymentStatus = &ps
	}

	var fulfillmentStatus *string
	if fs := r.URL.Query().Get("fulfillment_status"); fs != "" {
		fulfillmentStatus = &fs
	}

	var orderType *string
	if ot := r.URL.Query().Get("order_type"); ot != "" {
		orderType = &ot
	}

	items, total, err := h.svc.ListForAccount(r.Context(), p.Account, qStr, status, paymentStatus, fulfillmentStatus, orderType, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list user orders")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) GetMyOrder(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	ord, err := h.svc.GetForAccount(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get order")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	ord, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get order")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

// Create places an order from the platform panel for the account named in the body
// (account_uuid, company_uuid or user_uuid), defaulting to the admin's active account.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	var input model.CreateOrderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	ord, err := h.svc.CreateOnBehalf(r.Context(), input, p.UserInternal, p.Account.ID)
	writeCreateResult(w, r, ord, err)
}

// CreateMine places an order for the caller's active account.
func (h *Handler) CreateMine(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	var input model.CreateOrderInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	// The target account is always the session's active account on app routes.
	input.AccountUUID, input.CompanyUUID, input.UserUUID = nil, nil, nil

	ord, err := h.svc.CreateForAccount(r.Context(), p.Account, p.UserInternal, input)
	writeCreateResult(w, r, ord, err)
}

func writeCreateResult(w http.ResponseWriter, r *http.Request, ord *model.OrderDetail, err error) {
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrEmptyItems), errors.Is(err, usecase.ErrInvalidProduct), errors.Is(err, usecase.ErrInvalidAccount):
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
		case errors.Is(err, usecase.ErrForbidden):
			response.Forbidden(w, r, "your account role cannot place orders")
		default:
			response.InternalErr(w, r, err, "failed to create order")
		}
		return
	}
	response.JSON(w, r, http.StatusCreated, ord)
}

func (h *Handler) ProvisionOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	ord, err := h.svc.ProvisionOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		if errors.Is(err, usecase.ErrOrderClosed) {
			response.Conflict(w, r, response.CodeConflict, "order is cancelled or refunded")
			return
		}
		response.InternalErr(w, r, err, "failed to provision order cards and routes")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) ProvisionItem(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid order uuid")
		return
	}

	itemID, err := uuid.Parse(r.PathValue("itemUuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid item uuid")
		return
	}

	item, err := h.svc.ProvisionSingleItem(r.Context(), orderID, itemID)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) || errors.Is(err, usecase.ErrItemNotFound) {
			response.NotFound(w, r, "item not found")
			return
		}
		if errors.Is(err, usecase.ErrOrderClosed) {
			response.Conflict(w, r, response.CodeConflict, "order is cancelled or refunded")
			return
		}
		response.InternalErr(w, r, err, "failed to provision item")
		return
	}

	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) BulkUpdateItems(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid order uuid")
		return
	}

	var input model.BulkUpdateOrderItemsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	ord, err := h.svc.BulkUpdateItems(r.Context(), orderID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to bulk update items")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateOrderStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	if input.Status == "refunded" && !h.requireRefundStepUp(w, r) {
		return
	}
	before, _ := h.svc.Get(r.Context(), id)
	ord, err := h.svc.UpdateStatus(r.Context(), id, input)
	if err == nil && input.Status == "paid" && (before == nil || before.Status != "paid") {
		h.recordManualPayment(r, id, before, ord, "order.status_marked_paid")
	}
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		if writeRefundError(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to update order status")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) UpdatePaymentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateOrderPaymentStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	if input.PaymentStatus == model.PaymentStatusRefunded && !h.requireRefundStepUp(w, r) {
		return
	}
	before, _ := h.svc.Get(r.Context(), id)
	ord, err := h.svc.UpdatePaymentStatus(r.Context(), id, input)
	if err == nil && (before == nil || before.PaymentStatus != input.PaymentStatus) {
		h.recordManualPayment(r, id, before, ord, "order.payment_status_changed")
	}
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		if writeRefundError(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to update payment status")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) UpdateFulfillmentStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateOrderFulfillmentStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	ord, err := h.svc.UpdateFulfillmentStatus(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to update fulfillment status")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) UpdateShipping(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateOrderShippingInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	ord, err := h.svc.UpdateShipping(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to update shipping")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) UpdateNotes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateOrderNotesInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	ord, err := h.svc.UpdateNotes(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to update notes")
		return
	}

	response.JSON(w, r, http.StatusOK, ord)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.InternalErr(w, r, err, "failed to delete order")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// Asset Requests
func (h *Handler) CreateAssetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.CreateAssetRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	var actorID *int64
	if p, ok := authctx.PrincipalFrom(r.Context()); ok {
		actorID = &p.UserInternal
	}

	req, err := h.svc.CreateAssetRequest(r.Context(), id, input, actorID)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to create asset request")
		return
	}

	response.JSON(w, r, http.StatusCreated, req)
}

func (h *Handler) ReviewAsset(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid order uuid")
		return
	}

	assetID, err := uuid.Parse(r.PathValue("assetUuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid asset uuid")
		return
	}

	var input model.ReviewAssetRequestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	req, err := h.svc.ReviewAsset(r.Context(), orderID, assetID, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "order not found")
			return
		}
		response.InternalErr(w, r, err, "failed to review asset")
		return
	}

	response.JSON(w, r, http.StatusOK, req)
}

func (h *Handler) GetAssetByToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		response.BadRequest(w, r, response.CodeValidationError, "token is required")
		return
	}

	req, err := h.svc.GetAssetByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidToken) {
			response.NotFound(w, r, "asset request not found or token expired")
			return
		}
		response.InternalErr(w, r, err, "failed to get asset request")
		return
	}

	response.JSON(w, r, http.StatusOK, req)
}

func (h *Handler) UploadAsset(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		response.BadRequest(w, r, response.CodeValidationError, "token is required")
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	req, err := h.svc.UploadAsset(r.Context(), token, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidToken) {
			response.NotFound(w, r, "asset request not found or token expired")
			return
		}
		response.InternalErr(w, r, err, "failed to upload asset")
		return
	}

	response.JSON(w, r, http.StatusOK, req)
}

// ManifestQRCodes streams a ZIP of QR PNGs, one per provisioned card, for printing (TEC-581).
func (h *Handler) ManifestQRCodes(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	data, filename, err := h.svc.ManifestQRZip(r.Context(), id)
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "order not found")
		return
	case errors.Is(err, usecase.ErrNoProvisionedCards):
		response.Conflict(w, r, response.CodeConflict, "order has no provisioned cards")
		return
	case err != nil:
		response.InternalErr(w, r, err, "failed to build qr codes")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}
