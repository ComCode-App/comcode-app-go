package payment

import (
	"context"
	"fmt"
	"strings"

	iyzipay "github.com/parevo-lab/iyzipay-go"
)

type IyzicoProvider struct {
	cfg    ProviderConfig
	client *iyzipay.Client
}

func NewIyzicoProvider(cfg ProviderConfig) *IyzicoProvider {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://sandbox-api.iyzipay.com"
	}
	return &IyzicoProvider{
		cfg: cfg,
		client: iyzipay.NewClient(&iyzipay.Config{
			APIKey:    cfg.APIKey,
			SecretKey: cfg.SecretKey,
			BaseURL:   baseURL,
		}),
	}
}

func (p *IyzicoProvider) Slug() string { return "iyzico" }

func (p *IyzicoProvider) InitCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error) {
	return p.initForm(ctx, formParams{
		conversationID: in.MerchantOID,
		sessionUUID:    in.SessionUUID,
		amountMinor:    in.AmountMinor,
		currency:       in.Currency,
		planName:       in.PlanName,
		userEmail:      in.UserEmail,
		userName:       in.UserName,
		userPhone:      in.UserPhone,
		userAddress:    in.UserAddress,
		clientIP:       in.UserIP(),
		callbackURL:    in.CallbackURL,
		cardUserKey:    in.SavedUToken,
		physical:       in.Physical,
	})
}

func (p *IyzicoProvider) InitCardAdd(ctx context.Context, in CardAddInput) (CheckoutResult, error) {
	return p.initForm(ctx, formParams{
		conversationID: in.MerchantOID,
		sessionUUID:    in.SessionUUID,
		amountMinor:    100,
		currency:       "TRY",
		planName:       "Card validation",
		userEmail:      in.UserEmail,
		userName:       in.UserName,
		userPhone:      in.UserPhone,
		userAddress:    in.UserAddress,
		clientIP:       in.UserIP(),
		callbackURL:    in.CallbackURL,
	})
}

type formParams struct {
	conversationID, sessionUUID, currency, planName, userEmail, userName, userPhone, userAddress, clientIP, callbackURL, cardUserKey string
	amountMinor                                                                                                                      int64
	physical                                                                                                                         bool
}

func (p *IyzicoProvider) initForm(ctx context.Context, in formParams) (CheckoutResult, error) {
	price := formatTRY(in.amountMinor)
	names := splitName(in.userName)
	group, category, itemType := iyzipay.PaymentGroupSubscription, "Subscription", iyzipay.BasketItemTypeVirtual
	if in.physical {
		group, category, itemType = iyzipay.PaymentGroupProduct, "Card order", iyzipay.BasketItemTypePhysical
	}
	req := &iyzipay.CheckoutFormInitializeRequest{
		Locale:              iyzipay.LocaleTR,
		ConversationID:      in.conversationID,
		Price:               price,
		PaidPrice:           price,
		Currency:            iyzipay.CurrencyTRY,
		BasketID:            in.sessionUUID,
		PaymentGroup:        group,
		CallbackURL:         in.callbackURL,
		EnabledInstallments: []int{1},
		Buyer: &iyzipay.Buyer{
			ID:                  in.sessionUUID,
			Name:                names.first,
			Surname:             names.last,
			Email:               in.userEmail,
			GsmNumber:           defaultPhone(in.userPhone),
			IdentityNumber:      "11111111111",
			RegistrationAddress: defaultAddress(in.userAddress),
			IP:                  in.clientIP,
			City:                "Istanbul",
			Country:             "Turkey",
			ZipCode:             "34000",
		},
		ShippingAddress: &iyzipay.Address{
			ContactName: in.userName,
			City:        "Istanbul",
			Country:     "Turkey",
			Address:     defaultAddress(in.userAddress),
			ZipCode:     "34000",
		},
		BillingAddress: &iyzipay.Address{
			ContactName: in.userName,
			City:        "Istanbul",
			Country:     "Turkey",
			Address:     defaultAddress(in.userAddress),
			ZipCode:     "34000",
		},
		BasketItems: []iyzipay.BasketItem{{
			ID:        in.sessionUUID,
			Name:      in.planName,
			Category1: category,
			ItemType:  itemType,
			Price:     price,
		}},
		PaymentSource: "API",
		CardUserKey:   in.cardUserKey,
	}
	resp, err := p.client.CheckoutForm.Initialize(ctx, req)
	if err != nil {
		Logger().Error("iyzico_init_form_failed",
			"conversation_id", in.conversationID,
			"session_uuid", in.sessionUUID,
			"amount_minor", in.amountMinor,
			"error", err,
		)
		return CheckoutResult{}, err
	}
	if resp.Status != "success" {
		Logger().Error("iyzico_init_form_rejected",
			"conversation_id", in.conversationID,
			"session_uuid", in.sessionUUID,
			"error_message", resp.ErrorMessage,
			"error_code", resp.ErrorCode,
		)
		return CheckoutResult{}, fmt.Errorf("%w: iyzico: %s", ErrUnavailable, resp.ErrorMessage)
	}
	Logger().Info("iyzico_init_form_ok",
		"conversation_id", in.conversationID,
		"session_uuid", in.sessionUUID,
		"amount_minor", in.amountMinor,
		"callback_url", in.callbackURL,
		"token", MaskSecret(resp.Token),
		"payment_page_url", resp.PaymentPageURL,
	)
	return CheckoutResult{
		ProviderToken: resp.Token,
		CheckoutURL:   resp.PaymentPageURL,
	}, nil
}

func (p *IyzicoProvider) ChargeSavedCard(ctx context.Context, in SavedCardChargeInput) (CallbackResult, error) {
	price := formatTRY(in.AmountMinor)
	names := splitName(in.UserName)
	req := &iyzipay.PaymentRequest{
		Locale:         iyzipay.LocaleTR,
		ConversationID: in.MerchantOID,
		Price:          price,
		PaidPrice:      price,
		Currency:       iyzipay.CurrencyTRY,
		Installment:    1,
		BasketID:       in.MerchantOID,
		PaymentChannel: "WEB",
		PaymentGroup:   iyzipay.PaymentGroupSubscription,
		PaymentCard:    &iyzipay.PaymentCard{CardUserKey: &in.UToken, CardToken: &in.CToken},
		Buyer: &iyzipay.Buyer{
			ID:                  in.MerchantOID,
			Name:                names.first,
			Surname:             names.last,
			Email:               in.UserEmail,
			GsmNumber:           defaultPhone(in.UserPhone),
			IdentityNumber:      "11111111111",
			RegistrationAddress: defaultAddress(in.UserAddress),
			IP:                  in.ClientIP,
			City:                "Istanbul",
			Country:             "Turkey",
			ZipCode:             "34000",
		},
		ShippingAddress: &iyzipay.Address{
			ContactName: in.UserName,
			City:        "Istanbul",
			Country:     "Turkey",
			Address:     defaultAddress(in.UserAddress),
			ZipCode:     "34000",
		},
		BillingAddress: &iyzipay.Address{
			ContactName: in.UserName,
			City:        "Istanbul",
			Country:     "Turkey",
			Address:     defaultAddress(in.UserAddress),
			ZipCode:     "34000",
		},
		BasketItems: []iyzipay.BasketItem{{
			ID:        in.MerchantOID,
			Name:      in.PlanName,
			Category1: "Subscription",
			ItemType:  iyzipay.BasketItemTypeVirtual,
			Price:     price,
		}},
	}
	resp, err := p.client.Payment.Create(ctx, req)
	if err != nil {
		return CallbackResult{}, err
	}
	return CallbackResult{
		Success:     resp.Status == "success",
		MerchantOID: in.MerchantOID,
		ProviderRef: resp.PaymentID,
	}, nil
}

func (p *IyzicoProvider) ListSavedCards(ctx context.Context, customerRef string) ([]ProviderCard, error) {
	resp, err := p.client.Card.List(ctx, &iyzipay.RetrieveCardListRequest{
		Locale:         iyzipay.LocaleTR,
		ConversationID: customerRef,
		CardUserKey:    customerRef,
	})
	if err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("%w: iyzico list cards: %s", ErrUnavailable, resp.ErrorMessage)
	}
	out := make([]ProviderCard, 0, len(resp.CardDetails))
	for _, card := range resp.CardDetails {
		out = append(out, ProviderCard{
			CardRef:     card.CardToken,
			CustomerRef: card.CardUserKey,
			Last4:       card.LastFourDigits,
			Brand:       card.CardAssociation,
		})
	}
	return out, nil
}

func (p *IyzicoProvider) DeleteSavedCard(ctx context.Context, customerRef, cardRef string) error {
	resp, err := p.client.Card.Delete(ctx, &iyzipay.DeleteCardRequest{
		Locale:         iyzipay.LocaleTR,
		ConversationID: customerRef,
		CardUserKey:    customerRef,
		CardToken:      cardRef,
	})
	if err != nil {
		return err
	}
	if resp.Status != "success" {
		return fmt.Errorf("%w: iyzico delete card: %s", ErrUnavailable, resp.ErrorMessage)
	}
	return nil
}

func (p *IyzicoProvider) VerifyCallback(ctx context.Context, payload map[string]string) (CallbackResult, error) {
	token := payload["token"]
	if token == "" {
		Logger().Warn("iyzico_verify_missing_token", "payload", SafePayload(payload))
		return CallbackResult{}, fmt.Errorf("%w: missing iyzico token", ErrInvalidRequest)
	}
	conversationID := payload["merchant_oid"]
	if conversationID == "" {
		conversationID = payload["conversationId"]
	}
	Logger().Info("iyzico_verify_start",
		"conversation_id", conversationID,
		"token", MaskSecret(token),
		"payload", SafePayload(payload),
	)
	resp, err := p.client.CheckoutForm.Retrieve(ctx, &iyzipay.RetrieveCheckoutFormRequest{
		Locale:         iyzipay.LocaleTR,
		ConversationID: conversationID,
		Token:          token,
	})
	if err != nil {
		Logger().Error("iyzico_verify_retrieve_failed",
			"conversation_id", conversationID,
			"token", MaskSecret(token),
			"error", err,
		)
		return CallbackResult{}, err
	}
	if resp.Status != "success" {
		msg := strings.TrimSpace(resp.ErrorMessage)
		if msg == "" {
			msg = "iyzico checkout retrieve failed"
		}
		Logger().Error("iyzico_verify_retrieve_rejected",
			"conversation_id", conversationID,
			"token", MaskSecret(token),
			"api_status", resp.Status,
			"error_message", resp.ErrorMessage,
			"error_code", resp.ErrorCode,
		)
		return CallbackResult{}, fmt.Errorf("%w: %s", ErrInvalidRequest, msg)
	}
	if resp.Signature != "" {
		params := []string{resp.PaymentID, resp.Currency, resp.BasketID, resp.ConversationID, resp.PaidPrice, resp.Price}
		expected := iyzipay.CalculateHMACSignature(params, p.cfg.SecretKey)
		if resp.Signature != expected {
			Logger().Error("iyzico_verify_signature_invalid",
				"conversation_id", resp.ConversationID,
				"payment_id", resp.PaymentID,
			)
			return CallbackResult{}, fmt.Errorf("%w: invalid iyzico signature", ErrInvalidRequest)
		}
	}
	conversationID = resp.ConversationID
	isCard := strings.HasPrefix(conversationID, "card")
	result := CallbackResult{
		Success:       resp.PaymentStatus == "SUCCESS",
		MerchantOID:   conversationID,
		ProviderRef:   resp.PaymentID,
		IsCardSession: isCard,
	}
	Logger().Info("iyzico_verify_retrieved",
		"conversation_id", conversationID,
		"payment_status", resp.PaymentStatus,
		"payment_id", resp.PaymentID,
		"is_card_session", isCard,
		"price", resp.Price,
		"paid_price", resp.PaidPrice,
	)
	if resp.PaymentStatus == "SUCCESS" && resp.PaymentID != "" {
		payResp, err := p.client.Payment.Retrieve(ctx, &iyzipay.RetrievePaymentRequest{
			Locale:         iyzipay.LocaleTR,
			ConversationID: conversationID,
			PaymentID:      resp.PaymentID,
		})
		if err != nil {
			Logger().Warn("iyzico_verify_payment_retrieve_failed",
				"conversation_id", conversationID,
				"payment_id", resp.PaymentID,
				"error", err,
			)
		} else if payResp.Status == "success" {
			result.UToken = payResp.CardUserKey
			result.CToken = payResp.CardToken
			result.CardLast4 = payResp.LastFourDigits
			result.CardBrand = payResp.CardAssociation
			Logger().Info("iyzico_verify_payment_retrieved",
				"conversation_id", conversationID,
				"payment_id", resp.PaymentID,
				"has_card_user_key", payResp.CardUserKey != "",
				"has_card_token", payResp.CardToken != "",
				"last4", payResp.LastFourDigits,
				"brand", payResp.CardAssociation,
			)
		} else {
			Logger().Warn("iyzico_verify_payment_retrieve_rejected",
				"conversation_id", conversationID,
				"payment_id", resp.PaymentID,
				"api_status", payResp.Status,
				"error_message", payResp.ErrorMessage,
			)
		}
	} else if resp.PaymentStatus != "SUCCESS" {
		Logger().Warn("iyzico_verify_payment_not_success",
			"conversation_id", conversationID,
			"payment_status", resp.PaymentStatus,
			"md_status", resp.MdStatus,
		)
	}
	return result, nil
}

type nameParts struct {
	first string
	last  string
}

func splitName(full string) nameParts {
	full = strings.TrimSpace(full)
	if full == "" {
		return nameParts{first: "Subscriber", last: "User"}
	}
	parts := strings.Fields(full)
	if len(parts) == 1 {
		return nameParts{first: parts[0], last: "User"}
	}
	return nameParts{first: parts[0], last: strings.Join(parts[1:], " ")}
}

func defaultPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone != "" {
		return phone
	}
	return "+905000000000"
}

func defaultAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr != "" {
		return addr
	}
	return "Turkey"
}

// Refund refunds through iyzico's refund API, which works on payment transactions (basket
// items). Order payments have exactly one basket item, so the payment is retrieved first and
// its single transaction is refunded.
func (p *IyzicoProvider) Refund(ctx context.Context, in RefundInput) (RefundResult, error) {
	if strings.TrimSpace(in.PaymentRef) == "" {
		return RefundResult{}, fmt.Errorf("%w: iyzico payment id is missing", ErrRefundRejected)
	}
	pay, err := p.client.Payment.Retrieve(ctx, &iyzipay.RetrievePaymentRequest{
		Locale:         iyzipay.LocaleTR,
		ConversationID: in.ReferenceNo,
		PaymentID:      in.PaymentRef,
	})
	// Nothing has been refunded yet, so a failed lookup is safe to retry.
	if err != nil {
		return RefundResult{}, fmt.Errorf("%w: iyzico payment lookup: %v", ErrRefundRejected, err)
	}
	if pay.Status != "success" {
		return RefundResult{}, fmt.Errorf("%w: iyzico payment lookup: %s", ErrRefundRejected, pay.ErrorMessage)
	}
	if len(pay.ItemTransactions) != 1 || pay.ItemTransactions[0].PaymentTransactionID == "" {
		return RefundResult{}, fmt.Errorf("%w: iyzico payment has %d transactions, expected 1", ErrRefundRejected, len(pay.ItemTransactions))
	}
	ip := strings.TrimSpace(in.ClientIP)
	if ip == "" {
		ip = "127.0.0.1"
	}
	resp, err := p.client.Refund.Create(ctx, &iyzipay.RefundRequest{
		Locale:               iyzipay.LocaleTR,
		ConversationID:       in.ReferenceNo,
		PaymentTransactionID: pay.ItemTransactions[0].PaymentTransactionID,
		Price:                formatTRY(in.AmountMinor),
		Currency:             iyzipay.CurrencyTRY,
		IP:                   ip,
		Reason:               iyzipay.RefundReasonOther,
		Description:          in.Reason,
	})
	if err != nil {
		return RefundResult{}, fmt.Errorf("%w: iyzico refund: %v", ErrUnavailable, err)
	}
	if resp.Status != "success" {
		Logger().Error("iyzico_refund_rejected",
			"payment_id", in.PaymentRef,
			"reference_no", in.ReferenceNo,
			"error_code", resp.ErrorCode,
			"error_message", resp.ErrorMessage,
		)
		msg := strings.TrimSpace(resp.ErrorMessage)
		if msg == "" {
			msg = "refund declined"
		}
		if resp.ErrorCode != "" {
			msg = resp.ErrorCode + ": " + msg
		}
		return RefundResult{}, fmt.Errorf("%w: iyzico: %s", ErrRefundRejected, msg)
	}
	Logger().Info("iyzico_refund_ok", "payment_id", in.PaymentRef, "reference_no", in.ReferenceNo, "amount_minor", in.AmountMinor)
	ref := resp.PaymentTransactionID
	if ref == "" {
		ref = resp.PaymentID
	}
	return RefundResult{ProviderRef: ref}, nil
}
