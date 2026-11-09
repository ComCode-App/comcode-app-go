package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SavedCardView struct {
	UUID      uuid.UUID `json:"uuid"`
	Provider  string    `json:"provider"`
	Last4     string    `json:"last4"`
	Brand     string    `json:"brand"`
	ExpMonth  *int32    `json:"exp_month,omitempty"`
	ExpYear   *int32    `json:"exp_year,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt string    `json:"created_at"`
}

type CardSessionView struct {
	UUID          uuid.UUID `json:"uuid"`
	Status        string    `json:"status"`
	Provider      string    `json:"provider"`
	IframeToken   *string   `json:"iframe_token,omitempty"`
	CheckoutURL   *string   `json:"checkout_url,omitempty"`
	ProviderToken *string   `json:"provider_token,omitempty"`
	ExpiresAt     string    `json:"expires_at"`
}

type UserSubscriptionView struct {
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	GraceUntil        *string    `json:"grace_until,omitempty"`
	PeriodEnd         *string    `json:"period_end,omitempty"`
	PendingPlanUUID   *uuid.UUID `json:"pending_plan_uuid,omitempty"`
	UUID              uuid.UUID  `json:"uuid"`
	Status            string     `json:"status"`
	PlanUUID          uuid.UUID  `json:"plan_uuid"`
	PlanSlug          string     `json:"plan_slug"`
	PlanName          string     `json:"plan_name"`
	BillingInterval   string     `json:"billing_interval"`
	PriceMinor        int64      `json:"price_minor"`
	Currency          string     `json:"currency"`
	Provider          *string    `json:"provider,omitempty"`
	SavedCardUUID     *uuid.UUID `json:"saved_card_uuid,omitempty"`
	StartsAt          string     `json:"starts_at"`
	NextBillingAt     *string    `json:"next_billing_at,omitempty"`
	CanceledAt        *string    `json:"canceled_at,omitempty"`
}

// GetSubscription returns the account's active paid subscription; visible to every member.
func (s *Service) GetSubscription(ctx context.Context, accountID int64) (*UserSubscriptionView, error) {
	row, err := s.q.GetAccountActivePaymentSubscription(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return mapUserSubscription(row), nil
}

func (s *Service) CancelSubscription(ctx context.Context, payer Payer) error {
	return s.ScheduleCancellation(ctx, payer, true)
}

func (s *Service) ListCards(ctx context.Context, payer Payer) ([]SavedCardView, error) {
	if err := billingWriter(ctx, payer); err != nil {
		return nil, err
	}
	rows, err := s.q.ListPaymentSavedCardsByAccount(ctx, payer.Account.ID)
	if err != nil {
		return nil, err
	}
	out := make([]SavedCardView, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapSavedCard(row))
	}
	return out, nil
}

func (s *Service) StartAddCard(ctx context.Context, payer Payer, clientIP string) (CardSessionView, error) {
	if err := billingWriter(ctx, payer); err != nil {
		return CardSessionView{}, err
	}
	userID, userEmail, userName := payer.UserID, payer.Email, payer.Name
	s.log.Info("card_add_start", "user_id", userID, "client_ip", clientIP)
	provider, err := s.registry.ActiveProvider(ctx)
	if err != nil {
		s.log.Error("card_add_provider_unavailable", "user_id", userID, "error", err)
		return CardSessionView{}, err
	}
	testMode, err := s.registry.TestMode(ctx)
	if err != nil {
		s.log.Error("card_add_test_mode_failed", "user_id", userID, "error", err)
		return CardSessionView{}, err
	}
	merchantOID := fmt.Sprintf("card%s%d", strings.ReplaceAll(uuid.NewString(), "-", ""), time.Now().UnixMilli())
	if len(merchantOID) > 64 {
		merchantOID = merchantOID[:64]
	}
	session, err := s.q.CreatePaymentCardSession(ctx, db.CreatePaymentCardSessionParams{
		AccountID:   payer.Account.ID,
		UserID:      userID,
		Provider:    db.PaymentProvider(provider.Slug()),
		MerchantOid: merchantOID,
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().UTC().Add(30 * time.Minute), Valid: true},
	})
	if err != nil {
		return CardSessionView{}, err
	}
	returnPath := fmt.Sprintf("%s/billing/cards/add/%s", s.frontendURL, session.Uuid)
	callbackBase := s.apiBaseURL + "/v1/public/payment"
	callbackURL := fmt.Sprintf("%s/%s/callback", callbackBase, provider.Slug())
	s.log.Info("card_add_session_created",
		"session_uuid", session.Uuid,
		"user_id", userID,
		"merchant_oid", merchantOID,
		"provider", provider.Slug(),
		"test_mode", testMode,
		"callback_url", callbackURL,
	)
	init, err := provider.InitCardAdd(ctx, payment.CardAddInput{
		SessionUUID: session.Uuid.String(),
		MerchantOID: merchantOID,
		UserEmail:   userEmail,
		UserName:    userName,
		ClientIP:    clientIP,
		SuccessURL:  returnPath + "?result=success",
		FailURL:     returnPath + "?result=fail",
		CallbackURL: callbackURL,
		TestMode:    testMode,
	})
	if err != nil {
		s.log.Error("card_add_provider_init_failed",
			"session_uuid", session.Uuid,
			"merchant_oid", merchantOID,
			"provider", provider.Slug(),
			"error", err,
		)
		_, _ = s.q.FailPaymentCardSession(ctx, session.Uuid)
		return CardSessionView{}, err
	}
	token := init.IframeToken
	if token == "" {
		token = init.ProviderToken
	}
	updated, err := s.q.UpdatePaymentCardSessionInit(ctx, db.UpdatePaymentCardSessionInitParams{
		Uuid:          session.Uuid,
		ProviderToken: textOpt(token),
		CheckoutUrl:   textOpt(init.CheckoutURL),
	})
	if err != nil {
		return CardSessionView{}, err
	}
	view := mapCardSession(updated)
	if init.IframeToken != "" {
		view.IframeToken = &init.IframeToken
	}
	if init.CheckoutURL != "" {
		view.CheckoutURL = &init.CheckoutURL
	}
	if init.ProviderToken != "" {
		view.ProviderToken = &init.ProviderToken
	}
	s.log.Info("card_add_init_ok",
		"session_uuid", session.Uuid,
		"merchant_oid", merchantOID,
		"provider", provider.Slug(),
		"has_checkout_url", init.CheckoutURL != "",
		"has_iframe_token", init.IframeToken != "",
		"provider_token", payment.MaskSecret(init.ProviderToken),
	)
	return view, nil
}

func (s *Service) GetAddCardSession(ctx context.Context, payer Payer, sessionUUID uuid.UUID) (CardSessionView, error) {
	row, err := s.q.GetPaymentCardSessionByUUID(ctx, sessionUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CardSessionView{}, ErrNotFound
		}
		return CardSessionView{}, err
	}
	if row.AccountID != payer.Account.ID {
		return CardSessionView{}, ErrNotFound
	}
	view := mapCardSession(row)
	if row.Status == db.PaymentCardSessionStatusPending && row.ExpiresAt.Time.Before(time.Now().UTC()) {
		view.Status = "expired"
	}
	return view, nil
}

func (s *Service) DeleteCard(ctx context.Context, payer Payer, cardUUID uuid.UUID) error {
	if err := billingWriter(ctx, payer); err != nil {
		return err
	}
	card, err := s.q.GetPaymentSavedCardByUUID(ctx, db.GetPaymentSavedCardByUUIDParams{Uuid: cardUUID, AccountID: payer.Account.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	provider, err := s.registry.ProviderBySlug(ctx, string(card.Provider))
	if err != nil {
		return err
	}
	if err := provider.DeleteSavedCard(ctx, card.ProviderCustomerRef, card.ProviderCardRef); err != nil {
		return err
	}
	return s.q.DeletePaymentSavedCard(ctx, db.DeletePaymentSavedCardParams{
		Uuid: cardUUID, AccountID: payer.Account.ID,
	})
}

func (s *Service) SetDefaultCard(ctx context.Context, payer Payer, cardUUID uuid.UUID) (SavedCardView, error) {
	if err := billingWriter(ctx, payer); err != nil {
		return SavedCardView{}, err
	}
	if _, err := s.q.GetPaymentSavedCardByUUID(ctx, db.GetPaymentSavedCardByUUIDParams{Uuid: cardUUID, AccountID: payer.Account.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SavedCardView{}, ErrNotFound
		}
		return SavedCardView{}, err
	}
	if err := s.q.ClearDefaultPaymentSavedCards(ctx, payer.Account.ID); err != nil {
		return SavedCardView{}, err
	}
	updated, err := s.q.SetDefaultPaymentSavedCard(ctx, db.SetDefaultPaymentSavedCardParams{
		Uuid: cardUUID, AccountID: payer.Account.ID,
	})
	if err != nil {
		return SavedCardView{}, err
	}
	return mapSavedCard(updated), nil
}

func (s *Service) SubscribeWithCard(ctx context.Context, payer Payer, clientIP string, planUUID, cardUUID uuid.UUID) (*UserSubscriptionView, error) {
	if err := billingWriter(ctx, payer); err != nil {
		return nil, err
	}
	existing, lookupErr := s.GetSubscription(ctx, payer.Account.ID)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: use subscription plan change", ErrInvalidRequest)
	}

	userID, userEmail, userName := payer.UserID, payer.Email, payer.Name
	s.log.Info("subscribe_with_card_start",
		"user_id", userID,
		"plan_uuid", planUUID,
		"card_uuid", cardUUID,
		"client_ip", clientIP,
	)
	plan, err := s.q.GetPlanByUUID(ctx, planUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !plan.IsActive || plan.BillingInterval == db.BillingIntervalFree || plan.PriceMinor <= 0 {
		return nil, fmt.Errorf("%w: plan is not available for subscription", ErrInvalidRequest)
	}
	card, err := s.q.GetPaymentSavedCardByUUID(ctx, db.GetPaymentSavedCardByUUIDParams{Uuid: cardUUID, AccountID: payer.Account.ID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: saved card not found", ErrNotFound)
		}
		return nil, err
	}
	provider, err := s.registry.ProviderBySlug(ctx, string(card.Provider))
	if err != nil {
		return nil, err
	}
	testMode, err := s.registry.TestMode(ctx)
	if err != nil {
		return nil, err
	}
	merchantOID := fmt.Sprintf("sub%s%d", strings.ReplaceAll(uuid.NewString(), "-", ""), time.Now().UnixMilli())
	if len(merchantOID) > 64 {
		merchantOID = merchantOID[:64]
	}
	charge, err := provider.ChargeSavedCard(ctx, payment.SavedCardChargeInput{
		MerchantOID: merchantOID,
		AmountMinor: plan.PriceMinor,
		Currency:    plan.Currency,
		UserEmail:   userEmail,
		UserName:    userName,
		ClientIP:    clientIP,
		TestMode:    testMode,
		UToken:      card.ProviderCustomerRef,
		CToken:      card.ProviderCardRef,
		PlanName:    plan.Name,
	})
	if err != nil {
		s.log.Error("subscribe_with_card_charge_failed",
			"user_id", userID,
			"merchant_oid", merchantOID,
			"provider", provider.Slug(),
			"error", err,
		)
		return nil, err
	}
	if !charge.Success {
		s.log.Warn("subscribe_with_card_declined",
			"user_id", userID,
			"merchant_oid", merchantOID,
			"provider", provider.Slug(),
		)
		return nil, fmt.Errorf("%w: payment declined", ErrInvalidRequest)
	}
	s.log.Info("subscribe_with_card_charge_ok",
		"user_id", userID,
		"merchant_oid", merchantOID,
		"provider_ref", charge.ProviderRef,
	)
	if err := s.q.CancelActiveAccountSubscriptions(ctx, payer.Account.ID); err != nil {
		return nil, err
	}
	status := db.SubscriptionStatusActive
	if plan.TrialDays > 0 {
		status = db.SubscriptionStatusTrialing
	}
	_, err = s.q.CreatePaymentSubscription(ctx, db.CreatePaymentSubscriptionParams{
		PlanID:                  plan.ID,
		Status:                  status,
		AccountID:               payer.Account.ID,
		StartsAt:                pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Provider:                db.NullPaymentProvider{PaymentProvider: card.Provider, Valid: true},
		ProviderSubscriptionRef: textOpt(charge.ProviderRef),
		NextBillingAt:           computeNextBilling(plan.BillingInterval, plan.TrialDays),
		SavedCardID:             pgtype.Int8{Int64: card.ID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return s.GetSubscription(ctx, payer.Account.ID)
}

// persistSavedCard stores a card for accountID; userID is the member who added it.
func (s *Service) persistSavedCard(ctx context.Context, accountID, userID int64, providerSlug string, customerRef, cardRef, last4, brand string, isDefault bool) error {
	if customerRef == "" || cardRef == "" {
		s.log.Warn("persist_saved_card_skipped",
			"user_id", userID,
			"provider", providerSlug,
			"has_customer_ref", customerRef != "",
			"has_card_ref", cardRef != "",
		)
		return nil
	}
	s.log.Info("persist_saved_card_start",
		"user_id", userID,
		"provider", providerSlug,
		"last4", last4,
		"brand", brand,
		"is_default", isDefault,
		"customer_ref", payment.MaskSecret(customerRef),
		"card_ref", payment.MaskSecret(cardRef),
	)
	_, err := s.q.UpsertPaymentProviderCustomer(ctx, db.UpsertPaymentProviderCustomerParams{
		AccountID:           accountID,
		UserID:              userID,
		Provider:            db.PaymentProvider(providerSlug),
		ProviderCustomerRef: customerRef,
	})
	if err != nil {
		return err
	}
	count, err := s.q.ListPaymentSavedCardsByAccount(ctx, accountID)
	if err != nil {
		return err
	}
	if isDefault || len(count) == 0 {
		_ = s.q.ClearDefaultPaymentSavedCards(ctx, accountID)
		isDefault = true
	}
	_, err = s.q.UpsertPaymentSavedCard(ctx, db.UpsertPaymentSavedCardParams{
		AccountID:           accountID,
		UserID:              userID,
		Provider:            db.PaymentProvider(providerSlug),
		ProviderCardRef:     cardRef,
		ProviderCustomerRef: customerRef,
		Last4:               last4,
		Brand:               brand,
		IsDefault:           isDefault,
	})
	if err != nil {
		s.log.Error("persist_saved_card_failed", "user_id", userID, "provider", providerSlug, "error", err)
		return err
	}
	s.log.Info("persist_saved_card_ok", "user_id", userID, "provider", providerSlug, "last4", last4)
	return err
}

func mapSavedCard(row db.PaymentSavedCard) SavedCardView {
	view := SavedCardView{
		UUID: row.Uuid, Provider: string(row.Provider),
		Last4: row.Last4, Brand: row.Brand, IsDefault: row.IsDefault,
		CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
	}
	if row.ExpMonth.Valid {
		v := int32(row.ExpMonth.Int16)
		view.ExpMonth = &v
	}
	if row.ExpYear.Valid {
		v := int32(row.ExpYear.Int16)
		view.ExpYear = &v
	}
	return view
}

func mapCardSession(row db.PaymentCardSession) CardSessionView {
	view := CardSessionView{
		UUID: row.Uuid, Status: string(row.Status), Provider: string(row.Provider),
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
	return view
}

func mapUserSubscription(row db.GetAccountActivePaymentSubscriptionRow) *UserSubscriptionView {
	view := &UserSubscriptionView{
		CancelAtPeriodEnd: row.CancelAtPeriodEnd,
		UUID:              row.Uuid, Status: string(row.Status),
		PlanUUID: row.PlanUuid, PlanSlug: row.PlanSlug, PlanName: row.PlanName,
		BillingInterval: string(row.PlanBillingInterval),
		PriceMinor:      row.PlanPriceMinor, Currency: row.PlanCurrency,
		StartsAt: row.StartsAt.Time.Format(time.RFC3339),
	}
	if row.Provider.Valid {
		v := string(row.Provider.PaymentProvider)
		view.Provider = &v
	}
	if row.SavedCardUuid.Valid {
		id, err := uuid.FromBytes(row.SavedCardUuid.Bytes[:])
		if err == nil {
			view.SavedCardUUID = &id
		}
	}
	if row.NextBillingAt.Valid {
		v := row.NextBillingAt.Time.Format(time.RFC3339)
		view.NextBillingAt = &v
	}
	if row.CanceledAt.Valid {
		v := row.CanceledAt.Time.Format(time.RFC3339)
		view.CanceledAt = &v
	}
	if row.GraceUntil.Valid {
		v := row.GraceUntil.Time.Format(time.RFC3339)
		view.GraceUntil = &v
	}
	if row.NextBillingAt.Valid {
		v := row.NextBillingAt.Time.Format(time.RFC3339)
		view.PeriodEnd = &v
	}
	if row.PendingPlanUuid.Valid {
		v := uuid.UUID(row.PendingPlanUuid.Bytes)
		view.PendingPlanUUID = &v
	}

	return view
}
