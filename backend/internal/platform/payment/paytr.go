package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paytrconfig "github.com/streamerd/paytr-go/config"
	paytrpayment "github.com/streamerd/paytr-go/payment"
	paytrdomain "github.com/streamerd/paytr-go/types"
)

const paytrTokenURL = "https://www.paytr.com/odeme/api/get-token"

type PayTRProvider struct {
	cfg ProviderConfig
	svc paytrpayment.Service
}

func NewPayTRProvider(cfg ProviderConfig) *PayTRProvider {
	return &PayTRProvider{
		cfg: cfg,
		svc: paytrpayment.NewService(paytrconfig.PayTRConfig{
			MerchantID:   cfg.MerchantID,
			MerchantKey:  cfg.MerchantKey,
			MerchantSalt: cfg.MerchantSalt,
		}),
	}
}

func (p *PayTRProvider) Slug() string { return "paytr" }

func (p *PayTRProvider) InitCheckout(ctx context.Context, in CheckoutInput) (CheckoutResult, error) {
	return p.initIframe(ctx, iframeParams{
		userIP:      in.UserIP(),
		merchantOID: in.MerchantOID,
		email:       in.UserEmail,
		amountMinor: in.AmountMinor,
		currency:    in.Currency,
		planName:    in.PlanName,
		userName:    in.UserName,
		userPhone:   in.UserPhone,
		userAddress: in.UserAddress,
		successURL:  in.SuccessURL,
		failURL:     in.FailURL,
		testMode:    in.TestMode,
		storeCard:   in.StoreCard,
	})
}

func (p *PayTRProvider) InitCardAdd(ctx context.Context, in CardAddInput) (CheckoutResult, error) {
	return p.initIframe(ctx, iframeParams{
		userIP:      in.UserIP(),
		merchantOID: in.MerchantOID,
		email:       in.UserEmail,
		amountMinor: 100,
		currency:    "TRY",
		planName:    "Card validation",
		userName:    in.UserName,
		userPhone:   in.UserPhone,
		userAddress: in.UserAddress,
		successURL:  in.SuccessURL,
		failURL:     in.FailURL,
		testMode:    in.TestMode,
		storeCard:   true,
	})
}

type iframeParams struct {
	userIP, merchantOID, email, currency, planName, userName, userPhone, userAddress string
	successURL, failURL                                                              string
	amountMinor                                                                      int64
	testMode, storeCard                                                              bool
}

func (p *PayTRProvider) initIframe(ctx context.Context, in iframeParams) (CheckoutResult, error) {
	amountStr := strconv.FormatInt(in.amountMinor, 10)
	testMode := "0"
	if in.testMode {
		testMode = "1"
	}
	basket, err := json.Marshal([][]any{{in.planName, formatTRY(in.amountMinor), 1}})
	if err != nil {
		return CheckoutResult{}, err
	}
	userBasket := base64.StdEncoding.EncodeToString(basket)
	token := p.generateIframeToken(
		in.userIP, in.merchantOID, in.email, amountStr,
		userBasket, "0", "0", in.currency, testMode,
	)

	form := url.Values{}
	form.Set("merchant_id", p.cfg.MerchantID)
	form.Set("user_ip", in.userIP)
	form.Set("merchant_oid", in.merchantOID)
	form.Set("email", in.email)
	form.Set("payment_amount", amountStr)
	form.Set("paytr_token", token)
	form.Set("user_basket", userBasket)
	form.Set("debug_on", testMode)
	form.Set("no_installment", "0")
	form.Set("max_installment", "0")
	form.Set("user_name", in.userName)
	form.Set("user_address", defaultAddress(in.userAddress))
	form.Set("user_phone", defaultPhone(in.userPhone))
	form.Set("merchant_ok_url", in.successURL)
	form.Set("merchant_fail_url", in.failURL)
	form.Set("timeout_limit", "30")
	form.Set("currency", in.currency)
	form.Set("test_mode", testMode)
	form.Set("lang", "tr")
	if in.storeCard {
		form.Set("store_card", "1")
	}

	tokenURL := paytrTokenURL
	if p.cfg.BaseURL != "" {
		tokenURL = strings.TrimRight(p.cfg.BaseURL, "/") + "/odeme/api/get-token"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return CheckoutResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return CheckoutResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckoutResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return CheckoutResult{}, fmt.Errorf("%w: paytr token HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	var out struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return CheckoutResult{}, fmt.Errorf("paytr token decode: %w", err)
	}
	if out.Status != "success" || out.Token == "" {
		Logger().Error("paytr_init_iframe_rejected",
			"merchant_oid", in.merchantOID,
			"reason", out.Reason,
			"status", out.Status,
		)
		return CheckoutResult{}, fmt.Errorf("%w: paytr: %s", ErrUnavailable, out.Reason)
	}
	Logger().Info("paytr_init_iframe_ok",
		"merchant_oid", in.merchantOID,
		"amount_minor", in.amountMinor,
		"test_mode", in.testMode,
		"store_card", in.storeCard,
		"token", MaskSecret(out.Token),
	)
	return CheckoutResult{IframeToken: out.Token}, nil
}

func (p *PayTRProvider) ChargeSavedCard(_ context.Context, in SavedCardChargeInput) (CallbackResult, error) {
	testMode := "0"
	if in.TestMode {
		testMode = "1"
	}
	basket := fmt.Sprintf(`[["%s","%.2f",1]]`, in.PlanName, float64(in.AmountMinor)/100)
	req := paytrdomain.SavedCardPaymentRequest{
		CommonPaymentRequest: paytrdomain.CommonPaymentRequest{
			UserIP:           in.ClientIP,
			MerchantOid:      in.MerchantOID,
			Email:            in.UserEmail,
			PaymentAmount:    float64(in.AmountMinor) / 100,
			PaymentType:      "card",
			Currency:         in.Currency,
			TestMode:         testMode,
			NonThreeD:        "1",
			UserName:         in.UserName,
			UserAddress:      defaultAddress(in.UserAddress),
			UserPhone:        defaultPhone(in.UserPhone),
			UserBasket:       basket,
			DebugOn:          testMode,
			ClientLang:       "tr",
			InstallmentCount: "0",
		},
		UToken:           in.UToken,
		CToken:           in.CToken,
		RecurringPayment: "1",
	}
	resp, err := p.svc.SavedCardPayment(req)
	if err != nil {
		return CallbackResult{}, err
	}
	success := resp.Status == "success"
	return CallbackResult{
		Success:     success,
		MerchantOID: in.MerchantOID,
		ProviderRef: stringField(resp.Data, "payment_id"),
	}, nil
}

func (p *PayTRProvider) ListSavedCards(_ context.Context, customerRef string) ([]ProviderCard, error) {
	resp, err := p.svc.GetSavedCards(customerRef)
	if err != nil {
		return nil, err
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("%w: paytr list cards: %s", ErrUnavailable, resp.Message)
	}
	raw, ok := resp.Data["cards"]
	if !ok {
		return []ProviderCard{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return []ProviderCard{}, nil
	}
	out := make([]ProviderCard, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ProviderCard{
			CardRef:     fmt.Sprint(m["ctoken"]),
			CustomerRef: customerRef,
			Last4:       fmt.Sprint(m["last_4"]),
			Brand:       fmt.Sprint(m["c_brand"]),
		})
	}
	return out, nil
}

func (p *PayTRProvider) DeleteSavedCard(_ context.Context, customerRef, cardRef string) error {
	resp, err := p.svc.DeleteSavedCard(customerRef, cardRef)
	if err != nil {
		return err
	}
	if resp.Status != "success" {
		return fmt.Errorf("%w: paytr delete card: %s", ErrUnavailable, resp.Message)
	}
	return nil
}

func (p *PayTRProvider) VerifyCallback(_ context.Context, payload map[string]string) (CallbackResult, error) {
	merchantOID := payload["merchant_oid"]
	status := payload["status"]
	totalAmount := payload["total_amount"]
	hash := payload["hash"]
	Logger().Info("paytr_verify_start",
		"merchant_oid", merchantOID,
		"status", status,
		"total_amount", totalAmount,
		"payload", SafePayload(payload),
	)
	if merchantOID == "" || hash == "" || (status != "success" && status != "failed") {
		Logger().Warn("paytr_verify_missing_fields", "payload", SafePayload(payload))
		return CallbackResult{}, fmt.Errorf("%w: missing paytr callback fields", ErrInvalidRequest)
	}
	expected := p.callbackHash(merchantOID, status, totalAmount)
	if !hmac.Equal([]byte(expected), []byte(hash)) {
		Logger().Error("paytr_verify_hash_invalid", "merchant_oid", merchantOID, "status", status)
		return CallbackResult{}, fmt.Errorf("%w: invalid paytr callback hash", ErrInvalidRequest)
	}
	amount, err := strconv.ParseInt(totalAmount, 10, 64)
	if err != nil || amount < 0 || (status == "success" && amount == 0) {
		return CallbackResult{}, fmt.Errorf("%w: invalid paytr total_amount", ErrInvalidRequest)
	}
	isCard := strings.HasPrefix(merchantOID, "card")
	result := CallbackResult{
		Success:       status == "success",
		MerchantOID:   merchantOID,
		ProviderRef:   payload["payment_id"],
		UToken:        payload["utoken"],
		CToken:        payload["ctoken"],
		CardLast4:     payload["last_4"],
		CardBrand:     payload["c_brand"],
		IsCardSession: isCard,
	}
	Logger().Info("paytr_verify_ok",
		"merchant_oid", merchantOID,
		"success", result.Success,
		"is_card_session", isCard,
		"has_card_tokens", result.UToken != "" && result.CToken != "",
	)
	return result, nil
}

func (p *PayTRProvider) generateIframeToken(
	userIP, merchantOID, email, amount, basket, noInstallment, maxInstallment, currency, testMode string,
) string {
	hashStr := p.cfg.MerchantID + userIP + merchantOID + email + amount +
		basket + noInstallment + maxInstallment + currency + testMode + p.cfg.MerchantSalt
	mac := hmac.New(sha256.New, []byte(p.cfg.MerchantKey))
	mac.Write([]byte(hashStr))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (p *PayTRProvider) callbackHash(merchantOID, status, totalAmount string) string {
	hashStr := merchantOID + p.cfg.MerchantSalt + status + totalAmount
	mac := hmac.New(sha256.New, []byte(p.cfg.MerchantKey))
	mac.Write([]byte(hashStr))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func stringField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

func formatTRY(minor int64) string {
	return fmt.Sprintf("%.2f", float64(minor)/100)
}

func (in CheckoutInput) UserIP() string {
	if ip := strings.TrimSpace(in.ClientIP); ip != "" {
		return ip
	}
	return "127.0.0.1"
}

func (in CardAddInput) UserIP() string {
	if ip := strings.TrimSpace(in.ClientIP); ip != "" {
		return ip
	}
	return "127.0.0.1"
}

const paytrRefundURL = "https://www.paytr.com/odeme/iade"

// Refund calls PayTR's refund API (iade). The token signs merchant_id + merchant_oid +
// return_amount + merchant_salt; reference_no makes a retried refund idempotent at PayTR.
func (p *PayTRProvider) Refund(ctx context.Context, in RefundInput) (RefundResult, error) {
	amount := formatTRY(in.AmountMinor)
	mac := hmac.New(sha256.New, []byte(p.cfg.MerchantKey))
	mac.Write([]byte(p.cfg.MerchantID + in.MerchantOID + amount + p.cfg.MerchantSalt))
	form := url.Values{}
	form.Set("merchant_id", p.cfg.MerchantID)
	form.Set("merchant_oid", in.MerchantOID)
	form.Set("return_amount", amount)
	form.Set("paytr_token", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	form.Set("reference_no", in.ReferenceNo)

	refundURL := paytrRefundURL
	if p.cfg.BaseURL != "" {
		refundURL = strings.TrimRight(p.cfg.BaseURL, "/") + "/odeme/iade"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refundURL, strings.NewReader(form.Encode()))
	if err != nil {
		return RefundResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return RefundResult{}, fmt.Errorf("%w: paytr refund: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return RefundResult{}, fmt.Errorf("%w: paytr refund: %v", ErrUnavailable, err)
	}
	if resp.StatusCode >= 500 {
		return RefundResult{}, fmt.Errorf("%w: paytr refund HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	var out struct {
		Status      string `json:"status"`
		ErrNo       any    `json:"err_no"`
		ErrMsg      string `json:"err_msg"`
		ReferenceNo string `json:"reference_no"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			return RefundResult{}, fmt.Errorf("%w: paytr refund HTTP %d", ErrRefundRejected, resp.StatusCode)
		}
		return RefundResult{}, fmt.Errorf("%w: paytr refund decode: %v", ErrUnavailable, err)
	}
	if out.Status != "success" {
		Logger().Error("paytr_refund_rejected",
			"merchant_oid", in.MerchantOID,
			"reference_no", in.ReferenceNo,
			"err_no", out.ErrNo,
			"err_msg", out.ErrMsg,
		)
		msg := strings.TrimSpace(out.ErrMsg)
		if msg == "" {
			msg = "refund declined"
		}
		if out.ErrNo != nil {
			msg = fmt.Sprintf("%v: %s", out.ErrNo, msg)
		}
		return RefundResult{}, fmt.Errorf("%w: paytr: %s", ErrRefundRejected, msg)
	}
	Logger().Info("paytr_refund_ok", "merchant_oid", in.MerchantOID, "reference_no", in.ReferenceNo, "amount_minor", in.AmountMinor)
	ref := out.ReferenceNo
	if ref == "" {
		ref = in.ReferenceNo
	}
	return RefundResult{ProviderRef: ref}, nil
}
