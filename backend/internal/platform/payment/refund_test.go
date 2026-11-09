package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Fake provider servers only; the refund calls never reach PayTR or iyzico (TEC-935).

func TestPayTRRefund(t *testing.T) {
	cfg := ProviderConfig{Provider: "paytr", MerchantID: "m-1", MerchantKey: "fixture-key", MerchantSalt: "fixture-salt"}
	reply := `{"status":"success","merchant_oid":"ord1","return_amount":"50.00","reference_no":"rfd1"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/odeme/iade" || r.ParseForm() != nil {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		f := r.PostForm
		mac := hmac.New(sha256.New, []byte(cfg.MerchantKey))
		mac.Write([]byte("m-1" + "ord1" + "50.00" + "fixture-salt"))
		if f.Get("paytr_token") != base64.StdEncoding.EncodeToString(mac.Sum(nil)) {
			t.Error("bad refund token")
		}
		if f.Get("merchant_id") != "m-1" || f.Get("merchant_oid") != "ord1" || f.Get("return_amount") != "50.00" || f.Get("reference_no") != "rfd1" {
			t.Errorf("refund form: %v", f)
		}
		_, _ = w.Write([]byte(reply))
	}))
	defer srv.Close()
	cfg.BaseURL = srv.URL
	p := NewPayTRProvider(cfg)
	in := RefundInput{MerchantOID: "ord1", ReferenceNo: "rfd1", AmountMinor: 5000, Currency: "TRY"}

	res, err := p.Refund(context.Background(), in)
	if err != nil || res.ProviderRef != "rfd1" {
		t.Fatalf("refund: %+v %v", res, err)
	}
	reply = `{"status":"error","err_no":"005","err_msg":"declined"}`
	if _, err := p.Refund(context.Background(), in); !errors.Is(err, ErrRefundRejected) {
		t.Fatalf("declined refund: %v", err)
	}
}

func TestIyzicoRefund(t *testing.T) {
	var refundBody map[string]any
	transactions := `[{"paymentTransactionId":"tx-1","paidPrice":"120.00"}]`
	refundStatus := `{"status":"success","paymentId":"pay-1","paymentTransactionId":"tx-1","price":"50.00"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/payment/detail":
			_, _ = w.Write([]byte(`{"status":"success","paymentId":"pay-1","itemTransactions":` + transactions + `}`))
		case "/payment/refund":
			_ = json.NewDecoder(r.Body).Decode(&refundBody)
			_, _ = w.Write([]byte(refundStatus))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	p := NewIyzicoProvider(ProviderConfig{Provider: "iyzico", APIKey: "fixture", SecretKey: "fixture", BaseURL: srv.URL})
	in := RefundInput{PaymentRef: "pay-1", ReferenceNo: "rfd1", AmountMinor: 5000, Currency: "TRY", ClientIP: "192.0.2.1"}

	res, err := p.Refund(context.Background(), in)
	if err != nil || res.ProviderRef != "tx-1" {
		t.Fatalf("refund: %+v %v", res, err)
	}
	if refundBody["paymentTransactionId"] != "tx-1" || refundBody["price"] != "50.00" || refundBody["conversationId"] != "rfd1" {
		t.Fatalf("refund request: %v", refundBody)
	}
	refundStatus = `{"status":"failure","errorCode":"5091","errorMessage":"declined"}`
	if _, err := p.Refund(context.Background(), in); !errors.Is(err, ErrRefundRejected) {
		t.Fatalf("declined refund: %v", err)
	}
	transactions = `[]`
	if _, err := p.Refund(context.Background(), in); !errors.Is(err, ErrRefundRejected) {
		t.Fatalf("payment without transactions: %v", err)
	}
	if _, err := p.Refund(context.Background(), RefundInput{AmountMinor: 1}); !errors.Is(err, ErrRefundRejected) {
		t.Fatalf("missing payment id: %v", err)
	}
}
