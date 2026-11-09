package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidRequest = errors.New("invalid request")
	ErrForbidden      = errors.New("forbidden")
)

// Payer is who a billing operation acts for: the account that owns billing (docs/TENANCY_AND_ACCESS.md)
// and the member performing it, whose email and name go to the payment provider.
type Payer struct {
	Account authctx.Account
	UserID  int64
	Email   string
	Name    string
}

type TransactionRunner interface {
	WithTx(context.Context, func(*db.Queries) error) error
}

type Service struct {
	transactions TransactionRunner
	recurring    payment.RecurringCharger
	q            *db.Queries
	registry     *payment.Registry
	orders       OrderPayments
	activity     activityRecorder
	frontendURL  string
	apiBaseURL   string
	log          *slog.Logger
	storage      storage.Driver
}

func New(q *db.Queries, registry *payment.Registry, frontendURL, apiBaseURL string, log *slog.Logger, transactions TransactionRunner) *Service {
	return &Service{
		q:            q,
		transactions: transactions,
		recurring:    registry,
		registry:     registry,
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		apiBaseURL:   strings.TrimRight(apiBaseURL, "/"),
		log:          billingLog(log),
	}
}

type BillingPlanLimit struct {
	FeatureKey string `json:"feature_key"`
	ValueType  string `json:"value_type"`
	IntValue   *int32 `json:"int_value,omitempty"`
	BoolValue  *bool  `json:"bool_value,omitempty"`
}

type BillingPlan struct {
	UUID            uuid.UUID          `json:"uuid"`
	Slug            string             `json:"slug"`
	Name            string             `json:"name"`
	Description     *string            `json:"description,omitempty"`
	BillingInterval string             `json:"billing_interval"`
	PriceMinor      int64              `json:"price_minor"`
	Currency        string             `json:"currency"`
	TrialDays       int32              `json:"trial_days"`
	Limits          []BillingPlanLimit `json:"limits"`
}

type CheckoutSessionView struct {
	UUID          uuid.UUID `json:"uuid"`
	Status        string    `json:"status"`
	Provider      string    `json:"provider"`
	PlanUUID      uuid.UUID `json:"plan_uuid"`
	PlanName      string    `json:"plan_name"`
	AmountMinor   int64     `json:"amount_minor"`
	Currency      string    `json:"currency"`
	IframeToken   *string   `json:"iframe_token,omitempty"`
	CheckoutURL   *string   `json:"checkout_url,omitempty"`
	ProviderToken *string   `json:"provider_token,omitempty"`
	ExpiresAt     string    `json:"expires_at"`
	CompletedAt   *string   `json:"completed_at,omitempty"`
}

type StartCheckoutResult struct {
	Session CheckoutSessionView `json:"session"`
}

func (s *Service) ListPlans(ctx context.Context) ([]BillingPlan, error) {
	rows, err := s.q.ListActivePaidPlans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BillingPlan, 0, len(rows))
	for _, row := range rows {
		limits, err := s.q.ListPlanLimitsForPlanID(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, mapBillingPlan(row, limits))
	}
	return out, nil
}

func (s *Service) StartCheckout(ctx context.Context, payer Payer, planUUID uuid.UUID, clientIP string) (StartCheckoutResult, error) {
	if err := billingWriter(ctx, payer); err != nil {
		return StartCheckoutResult{}, err
	}
	existing, lookupErr := s.GetSubscription(ctx, payer.Account.ID)
	if lookupErr != nil {
		return StartCheckoutResult{}, lookupErr
	}
	if existing != nil {
		return StartCheckoutResult{}, fmt.Errorf("%w: use subscription plan change", ErrInvalidRequest)
	}

	userID, userEmail, userName := payer.UserID, payer.Email, payer.Name
	s.log.Info("checkout_start",
		"user_id", userID,
		"plan_uuid", planUUID,
		"client_ip", clientIP,
	)
	plan, err := s.q.GetPlanByUUID(ctx, planUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("checkout_plan_not_found", "plan_uuid", planUUID)
			return StartCheckoutResult{}, ErrNotFound
		}
		s.log.Error("checkout_plan_load_failed", "plan_uuid", planUUID, "error", err)
		return StartCheckoutResult{}, err
	}
	if !plan.IsActive || plan.BillingInterval == db.BillingIntervalFree || plan.PriceMinor <= 0 {
		s.log.Warn("checkout_plan_unavailable",
			"plan_uuid", planUUID,
			"is_active", plan.IsActive,
			"billing_interval", plan.BillingInterval,
			"price_minor", plan.PriceMinor,
		)
		return StartCheckoutResult{}, fmt.Errorf("%w: plan is not available for checkout", ErrInvalidRequest)
	}
	provider, err := s.registry.ActiveProvider(ctx)
	if err != nil {
		s.log.Error("checkout_provider_unavailable", "error", err)
		return StartCheckoutResult{}, err
	}
	testMode, err := s.registry.TestMode(ctx)
	if err != nil {
		s.log.Error("checkout_test_mode_failed", "error", err)
		return StartCheckoutResult{}, err
	}

	merchantOID := fmt.Sprintf("chk%s%d", strings.ReplaceAll(uuid.NewString(), "-", ""), time.Now().UnixMilli())
	if len(merchantOID) > 64 {
		merchantOID = merchantOID[:64]
	}
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	session, err := s.q.CreateCheckoutSession(ctx, db.CreateCheckoutSessionParams{
		AccountID:   payer.Account.ID,
		UserID:      userID,
		PlanID:      plan.ID,
		Provider:    db.PaymentProvider(provider.Slug()),
		Status:      db.CheckoutSessionStatusPending,
		MerchantOid: merchantOID,
		AmountMinor: plan.PriceMinor,
		Currency:    plan.Currency,
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Metadata:    []byte("{}"),
	})
	if err != nil {
		return StartCheckoutResult{}, err
	}

	checkoutPath := fmt.Sprintf("%s/billing/checkout/%s", s.frontendURL, session.Uuid)
	callbackBase := s.apiBaseURL + "/v1/public/payment"
	callbackURL := fmt.Sprintf("%s/%s/callback", callbackBase, provider.Slug())
	s.log.Info("checkout_session_created",
		"session_uuid", session.Uuid,
		"merchant_oid", merchantOID,
		"provider", provider.Slug(),
		"amount_minor", plan.PriceMinor,
		"currency", plan.Currency,
		"test_mode", testMode,
		"callback_url", callbackURL,
	)
	init, err := provider.InitCheckout(ctx, payment.CheckoutInput{
		SessionUUID: session.Uuid.String(),
		MerchantOID: merchantOID,
		AmountMinor: plan.PriceMinor,
		Currency:    plan.Currency,
		PlanName:    plan.Name,
		UserEmail:   userEmail,
		UserName:    userName,
		ClientIP:    clientIP,
		SuccessURL:  checkoutPath + "?result=success",
		FailURL:     checkoutPath + "?result=fail",
		CallbackURL: callbackURL,
		TestMode:    testMode,
		StoreCard:   true,
	})
	if err != nil {
		s.log.Error("checkout_provider_init_failed",
			"session_uuid", session.Uuid,
			"merchant_oid", merchantOID,
			"provider", provider.Slug(),
			"error", err,
		)
		_, _ = s.q.FailCheckoutSession(ctx, db.FailCheckoutSessionParams{Uuid: session.Uuid})
		return StartCheckoutResult{}, err
	}

	updated, err := s.q.UpdateCheckoutSessionInit(ctx, db.UpdateCheckoutSessionInitParams{
		Uuid: session.Uuid,
		ProviderToken: textOpt(func() string {
			if init.IframeToken != "" {
				return init.IframeToken
			}
			return init.ProviderToken
		}()),
		CheckoutUrl: textOpt(init.CheckoutURL),
	})
	if err != nil {
		return StartCheckoutResult{}, err
	}

	view := mapCheckoutSession(updated, plan.Name, plan.Uuid)
	if init.IframeToken != "" {
		view.IframeToken = &init.IframeToken
	}
	if init.CheckoutURL != "" {
		view.CheckoutURL = &init.CheckoutURL
	}
	if init.ProviderToken != "" {
		view.ProviderToken = &init.ProviderToken
	}
	s.log.Info("checkout_init_ok",
		"session_uuid", session.Uuid,
		"merchant_oid", merchantOID,
		"provider", provider.Slug(),
		"has_checkout_url", init.CheckoutURL != "",
		"has_iframe_token", init.IframeToken != "",
		"provider_token", payment.MaskSecret(init.ProviderToken),
	)
	return StartCheckoutResult{Session: view}, nil
}

func (s *Service) GetCheckout(ctx context.Context, payer Payer, sessionUUID uuid.UUID) (CheckoutSessionView, error) {
	row, err := s.q.GetCheckoutSessionByUUID(ctx, sessionUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CheckoutSessionView{}, ErrNotFound
		}
		return CheckoutSessionView{}, err
	}
	if row.AccountID != payer.Account.ID {
		return CheckoutSessionView{}, ErrNotFound
	}
	view := mapCheckoutSessionRow(row)
	if row.Status == db.CheckoutSessionStatusPending && row.ExpiresAt.Time.Before(time.Now().UTC()) {
		view.Status = "expired"
	}
	return view, nil
}

func (s *Service) HandlePayTRCallback(ctx context.Context, payload map[string]string) error {
	s.log.Info("paytr_callback_received", "payload", safePayload(payload))
	provider, err := s.registry.ProviderBySlug(ctx, "paytr")
	if err != nil {
		s.log.Error("paytr_callback_provider_failed", "error", err)
		return err
	}
	result, err := provider.VerifyCallback(ctx, payload)
	if err != nil {
		s.log.Error("paytr_callback_verify_failed", "payload", safePayload(payload), "error", err)
		return err
	}
	s.log.Info("paytr_callback_verified",
		"merchant_oid", result.MerchantOID,
		"success", result.Success,
		"is_card_session", result.IsCardSession,
		"provider_ref", result.ProviderRef,
		"has_card_tokens", result.UToken != "" && result.CToken != "",
	)
	if isOrderPayment(result.MerchantOID) {
		if _, err := s.finalizeOrderPayment(ctx, result, payload, "paytr"); err != nil {
			s.log.Error("paytr_callback_finalize_order_failed", "merchant_oid", result.MerchantOID, "error", err)
			return err
		}
		s.log.Info("paytr_callback_done", "merchant_oid", result.MerchantOID, "success", result.Success)
		return nil
	}
	if result.IsCardSession {
		if err := s.finalizeCardSession(ctx, result, "paytr"); err != nil {
			s.log.Error("paytr_callback_finalize_card_failed", "merchant_oid", result.MerchantOID, "error", err)
			return err
		}
		s.log.Info("paytr_callback_card_finalized", "merchant_oid", result.MerchantOID, "success", result.Success)
		return nil
	}
	if err := s.finalizePayTRCheckout(ctx, result, payload); err != nil {
		return err
	}

	s.log.Info("paytr_callback_done", "merchant_oid", result.MerchantOID, "success", result.Success)
	return nil
}

type CallbackOutcome struct {
	Success       bool
	IsCardSession bool
	// OrderSession is the order checkout session (TEC-943) the callback belongs to, when known;
	// the browser returns to its result page.
	OrderSession uuid.UUID
}

func (s *Service) resolveIyzicoConversationID(ctx context.Context, payload map[string]string) {
	if payload["merchant_oid"] != "" || payload["conversationId"] != "" {
		s.log.Debug("iyzico_conversation_from_payload",
			"merchant_oid", payload["merchant_oid"],
			"conversation_id", payload["conversationId"],
		)
		return
	}
	token := strings.TrimSpace(payload["token"])
	if token == "" {
		s.log.Warn("iyzico_conversation_unresolved", "reason", "missing token and conversation id")
		return
	}
	if row, err := s.q.GetPaymentCardSessionByProviderToken(ctx, pgtype.Text{String: token, Valid: true}); err == nil {
		payload["merchant_oid"] = row.MerchantOid
		s.log.Info("iyzico_conversation_resolved",
			"source", "card_session",
			"session_uuid", row.Uuid,
			"merchant_oid", row.MerchantOid,
			"token", payment.MaskSecret(token),
		)
		return
	}
	if row, err := s.q.GetOrderPaymentSessionByProviderToken(ctx, pgtype.Text{String: token, Valid: true}); err == nil {
		payload["merchant_oid"] = row.MerchantOid
		s.log.Info("iyzico_conversation_resolved",
			"source", "order_payment_session",
			"session_uuid", row.Uuid,
			"merchant_oid", row.MerchantOid,
			"token", payment.MaskSecret(token),
		)
		return
	}
	if row, err := s.q.GetCheckoutSessionByProviderToken(ctx, pgtype.Text{String: token, Valid: true}); err == nil {
		payload["merchant_oid"] = row.MerchantOid
		s.log.Info("iyzico_conversation_resolved",
			"source", "checkout_session",
			"session_uuid", row.Uuid,
			"merchant_oid", row.MerchantOid,
			"token", payment.MaskSecret(token),
		)
		return
	}
	s.log.Warn("iyzico_conversation_unresolved",
		"token", payment.MaskSecret(token),
		"reason", "no session matched provider_token",
	)
}

func (s *Service) HandleIyzicoCallback(ctx context.Context, payload map[string]string) (CallbackOutcome, error) {
	s.log.Info("iyzico_callback_received", "payload", safePayload(payload))
	s.resolveIyzicoConversationID(ctx, payload)
	var failed CallbackOutcome
	if oid := payload["merchant_oid"]; isOrderPayment(oid) {
		if row, err := s.q.GetOrderPaymentSessionByMerchantOID(ctx, oid); err == nil {
			failed.OrderSession = row.Uuid
		}
	}
	provider, err := s.registry.ProviderBySlug(ctx, "iyzico")
	if err != nil {
		s.log.Error("iyzico_callback_provider_failed", "error", err)
		return failed, err
	}
	result, err := provider.VerifyCallback(ctx, payload)
	if err != nil {
		s.log.Error("iyzico_callback_verify_failed",
			"payload", safePayload(payload),
			"merchant_oid", payload["merchant_oid"],
			"error", err,
		)
		return failed, err
	}
	s.log.Info("iyzico_callback_verified",
		"merchant_oid", result.MerchantOID,
		"success", result.Success,
		"is_card_session", result.IsCardSession,
		"provider_ref", result.ProviderRef,
		"has_card_tokens", result.UToken != "" && result.CToken != "",
		"card_last4", result.CardLast4,
		"card_brand", result.CardBrand,
	)
	outcome := CallbackOutcome{Success: result.Success, IsCardSession: result.IsCardSession}
	if isOrderPayment(result.MerchantOID) {
		session, err := s.finalizeOrderPayment(ctx, result, nil, "iyzico")
		outcome.OrderSession = session
		if err != nil {
			s.log.Error("iyzico_callback_finalize_order_failed", "merchant_oid", result.MerchantOID, "error", err)
			return outcome, err
		}
		s.log.Info("iyzico_callback_done", "merchant_oid", result.MerchantOID, "success", result.Success)
		return outcome, nil
	}
	if result.IsCardSession {
		if err := s.finalizeCardSession(ctx, result, "iyzico"); err != nil {
			s.log.Error("iyzico_callback_finalize_card_failed", "merchant_oid", result.MerchantOID, "error", err)
			return CallbackOutcome{}, err
		}
		s.log.Info("iyzico_callback_card_finalized", "merchant_oid", result.MerchantOID, "success", result.Success)
		return outcome, nil
	}
	if err := s.finalizeCheckout(ctx, result.MerchantOID, result.Success, result.ProviderRef, "iyzico"); err != nil {
		s.log.Error("iyzico_callback_finalize_checkout_failed", "merchant_oid", result.MerchantOID, "error", err)
		return CallbackOutcome{}, err
	}
	if result.Success && result.UToken != "" && result.CToken != "" {
		row, err := s.q.GetCheckoutSessionByMerchantOID(ctx, result.MerchantOID)
		if err == nil {
			if err := s.persistSavedCard(ctx, row.AccountID, row.UserID, "iyzico", result.UToken, result.CToken, result.CardLast4, result.CardBrand, true); err != nil {
				s.log.Error("iyzico_callback_persist_card_failed", "merchant_oid", result.MerchantOID, "user_id", row.UserID, "error", err)
			} else {
				s.log.Info("iyzico_callback_card_persisted", "merchant_oid", result.MerchantOID, "user_id", row.UserID)
			}
		}
	}
	s.log.Info("iyzico_callback_done", "merchant_oid", result.MerchantOID, "success", result.Success)
	return outcome, nil
}

func (s *Service) finalizeCardSession(ctx context.Context, result payment.CallbackResult, providerSlug string) error {
	s.log.Info("finalize_card_session_start",
		"merchant_oid", result.MerchantOID,
		"provider", providerSlug,
		"success", result.Success,
	)
	row, err := s.q.GetPaymentCardSessionByMerchantOID(ctx, result.MerchantOID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("finalize_card_session_not_found", "merchant_oid", result.MerchantOID)
			return ErrNotFound
		}
		s.log.Error("finalize_card_session_load_failed", "merchant_oid", result.MerchantOID, "error", err)
		return err
	}
	if row.Status == db.PaymentCardSessionStatusCompleted {
		s.log.Info("finalize_card_session_already_completed", "session_uuid", row.Uuid)
		return nil
	}
	if !result.Success {
		s.log.Warn("finalize_card_session_payment_failed", "session_uuid", row.Uuid, "merchant_oid", result.MerchantOID)
		_, err := s.q.FailPaymentCardSession(ctx, row.Uuid)
		return err
	}
	if _, err := s.q.CompletePaymentCardSession(ctx, row.Uuid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("finalize_card_session_complete_noop", "session_uuid", row.Uuid)
			return nil
		}
		s.log.Error("finalize_card_session_complete_failed", "session_uuid", row.Uuid, "error", err)
		return err
	}
	if result.UToken == "" || result.CToken == "" {
		s.log.Warn("finalize_card_session_missing_tokens",
			"session_uuid", row.Uuid,
			"merchant_oid", result.MerchantOID,
			"has_utoken", result.UToken != "",
			"has_ctoken", result.CToken != "",
		)
	}
	if err := s.persistSavedCard(ctx, row.AccountID, row.UserID, providerSlug, result.UToken, result.CToken, result.CardLast4, result.CardBrand, true); err != nil {
		s.log.Error("finalize_card_session_persist_failed", "session_uuid", row.Uuid, "user_id", row.UserID, "error", err)
		return err
	}
	s.log.Info("finalize_card_session_ok", "session_uuid", row.Uuid, "user_id", row.UserID)
	return nil
}

func (s *Service) finalizeCheckout(ctx context.Context, merchantOID string, success bool, providerRef, providerSlug string) error {
	s.log.Info("finalize_checkout_start",
		"merchant_oid", merchantOID,
		"provider", providerSlug,
		"success", success,
		"provider_ref", providerRef,
	)
	row, err := s.q.GetCheckoutSessionByMerchantOID(ctx, merchantOID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Error("finalize_checkout_not_found", "merchant_oid", merchantOID)
			return ErrNotFound
		}
		s.log.Error("finalize_checkout_load_failed", "merchant_oid", merchantOID, "error", err)
		return err
	}
	if row.Status == db.CheckoutSessionStatusCompleted {
		s.log.Info("finalize_checkout_already_completed", "session_uuid", row.Uuid)
		return nil
	}
	if !success {
		s.log.Warn("finalize_checkout_payment_failed", "session_uuid", row.Uuid, "merchant_oid", merchantOID)
		_, err := s.q.FailCheckoutSession(ctx, db.FailCheckoutSessionParams{Uuid: row.Uuid})
		return err
	}
	if _, err := s.q.CompleteCheckoutSession(ctx, db.CompleteCheckoutSessionParams{
		Uuid:        row.Uuid,
		ProviderRef: textOpt(providerRef),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.log.Warn("finalize_checkout_complete_noop", "session_uuid", row.Uuid)
			return nil
		}
		s.log.Error("finalize_checkout_complete_failed", "session_uuid", row.Uuid, "error", err)
		return err
	}
	if err := s.q.CancelActiveAccountSubscriptions(ctx, row.AccountID); err != nil {
		s.log.Error("finalize_checkout_cancel_subs_failed", "session_uuid", row.Uuid, "user_id", row.UserID, "error", err)
		return err
	}
	plan, err := s.q.GetPlanByID(ctx, row.PlanID)
	if err != nil {
		s.log.Error("finalize_checkout_plan_failed", "session_uuid", row.Uuid, "plan_id", row.PlanID, "error", err)
		return err
	}
	status := db.SubscriptionStatusActive
	if plan.TrialDays > 0 {
		status = db.SubscriptionStatusTrialing
	}
	nextBilling := computeNextBilling(plan.BillingInterval, plan.TrialDays)
	sub, err := s.q.CreatePaymentSubscription(ctx, db.CreatePaymentSubscriptionParams{
		PlanID:                  plan.ID,
		Status:                  status,
		AccountID:               row.AccountID,
		StartsAt:                pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Provider:                db.NullPaymentProvider{PaymentProvider: db.PaymentProvider(providerSlug), Valid: true},
		ProviderSubscriptionRef: textOpt(providerRef),
		CheckoutSessionID:       pgtype.Int8{Int64: row.ID, Valid: true},
		NextBillingAt:           nextBilling,
		SavedCardID:             pgtype.Int8{},
	})
	if err != nil {
		s.log.Error("finalize_checkout_create_subscription_failed", "session_uuid", row.Uuid, "user_id", row.UserID, "error", err)
		return err
	}
	s.log.Info("finalize_checkout_ok",
		"session_uuid", row.Uuid,
		"user_id", row.UserID,
		"subscription_uuid", sub.Uuid,
		"plan_id", plan.ID,
		"status", status,
	)
	return nil
}

func computeNextBilling(interval db.BillingInterval, trialDays int32) pgtype.Timestamptz {
	now := time.Now().UTC()
	if trialDays > 0 {
		return pgtype.Timestamptz{Time: now.AddDate(0, 0, int(trialDays)), Valid: true}
	}
	switch interval {
	case db.BillingIntervalMonth:
		return pgtype.Timestamptz{Time: now.AddDate(0, 1, 0), Valid: true}
	case db.BillingIntervalYear:
		return pgtype.Timestamptz{Time: now.AddDate(1, 0, 0), Valid: true}
	default:
		return pgtype.Timestamptz{}
	}
}

func mapBillingPlan(row db.Plan, limits []db.ListPlanLimitsForPlanIDRow) BillingPlan {
	var desc *string
	if row.Description.Valid {
		v := row.Description.String
		desc = &v
	}
	out := BillingPlan{
		UUID: row.Uuid, Slug: row.Slug, Name: row.Name, Description: desc,
		BillingInterval: string(row.BillingInterval),
		PriceMinor:      row.PriceMinor, Currency: row.Currency, TrialDays: row.TrialDays,
		Limits: make([]BillingPlanLimit, 0, len(limits)),
	}
	for _, lim := range limits {
		item := BillingPlanLimit{FeatureKey: lim.FeatureKey, ValueType: string(lim.ValueType)}
		if lim.ValueInt.Valid {
			v := lim.ValueInt.Int32
			item.IntValue = &v
		}
		if lim.ValueBool.Valid {
			v := lim.ValueBool.Bool
			item.BoolValue = &v
		}
		out.Limits = append(out.Limits, item)
	}
	return out
}

func mapCheckoutSession(row db.PaymentCheckoutSession, planName string, planUUID uuid.UUID) CheckoutSessionView {
	view := CheckoutSessionView{
		UUID: row.Uuid, Status: string(row.Status), Provider: string(row.Provider),
		PlanUUID: planUUID, PlanName: planName, AmountMinor: row.AmountMinor, Currency: row.Currency,
		ExpiresAt: row.ExpiresAt.Time.Format(time.RFC3339),
	}
	if row.ProviderToken.Valid && row.ProviderToken.String != "" {
		v := row.ProviderToken.String
		view.ProviderToken = &v
		if row.Provider == db.PaymentProviderPaytr {
			view.IframeToken = &v
		}
	}
	if row.CheckoutUrl.Valid && row.CheckoutUrl.String != "" {
		v := row.CheckoutUrl.String
		view.CheckoutURL = &v
	}
	if row.CompletedAt.Valid {
		v := row.CompletedAt.Time.Format(time.RFC3339)
		view.CompletedAt = &v
	}
	return view
}

func mapCheckoutSessionRow(row db.GetCheckoutSessionByUUIDRow) CheckoutSessionView {
	view := CheckoutSessionView{
		UUID: row.Uuid, Status: string(row.Status), Provider: string(row.Provider),
		PlanUUID: row.PlanUuid, PlanName: row.PlanName, AmountMinor: row.AmountMinor, Currency: row.Currency,
		ExpiresAt: row.ExpiresAt.Time.Format(time.RFC3339),
	}
	if row.ProviderToken.Valid && row.ProviderToken.String != "" {
		v := row.ProviderToken.String
		view.ProviderToken = &v
		if row.Provider == db.PaymentProviderPaytr {
			view.IframeToken = &v
		}
	}
	if row.CheckoutUrl.Valid && row.CheckoutUrl.String != "" {
		v := row.CheckoutUrl.String
		view.CheckoutURL = &v
	}
	if row.CompletedAt.Valid {
		v := row.CompletedAt.Time.Format(time.RFC3339)
		view.CompletedAt = &v
	}
	return view
}

func textOpt(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}
