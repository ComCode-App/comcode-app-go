package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recurringSettings struct{ config ProviderConfig }

func (r recurringSettings) ActiveProvider(context.Context) (string, error) { return "none", nil }
func (r recurringSettings) ProviderConfig(context.Context, string) (ProviderConfig, error) {
	return r.config, nil
}
func (r recurringSettings) TestMode(context.Context) (bool, error) { return true, nil }

func TestRecurringPayTRUnsupportedWithoutNetwork(t *testing.T) {
	registry := NewRegistry(nil)
	_, err := registry.ChargeRecurring(context.Background(), "paytr", SavedCardChargeInput{})
	if !errors.Is(err, ErrRecurringUnsupported) {
		t.Fatal(err)
	}
}
func TestRecurringIyzicoStoredCard(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(map[bool]string{true: "captured", false: "declined"}[success], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/payment/auth" || r.Method != "POST" {
					t.Errorf("unexpected endpoint %s %s", r.Method, r.URL)
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "IYZWSv2 ") {
					t.Error("unsigned request")
				}
				var body struct {
					ConversationID string `json:"conversationId"`
					Price          string `json:"price"`
					PaymentGroup   string `json:"paymentGroup"`
					Card           struct {
						User  string `json:"cardUserKey"`
						Token string `json:"cardToken"`
					} `json:"paymentCard"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.ConversationID != "renewal-fixture" || body.Price != "123.45" || body.Card.User != "fixture-user" || body.Card.Token != "fixture-card" || body.PaymentGroup != "SUBSCRIPTION" {
					t.Errorf("incorrect stored-card request %+v", body)
				}
				status := "failure"
				if success {
					status = "success"
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "paymentId": "fixture-payment"})
			}))
			defer server.Close()
			registry := NewRegistry(recurringSettings{config: ProviderConfig{Provider: "iyzico", Enabled: true, BaseURL: server.URL, APIKey: "fixture-key", SecretKey: "fixture-secret"}})
			result, err := registry.ChargeRecurring(context.Background(), "iyzico", SavedCardChargeInput{
				MerchantOID: "renewal-fixture", AmountMinor: 12345, Currency: "TRY", UToken: "fixture-user", CToken: "fixture-card", UserName: "Test Payer", UserEmail: "payer@example.test",
			})
			if err != nil || result.Success != success || result.ProviderRef != "fixture-payment" {
				t.Fatalf("result %+v %v", result, err)
			}
		})
	}
}
