package payment

import (
	"log/slog"
	"strings"
)

// Logger returns the structured logger for payment provider operations.
func Logger() *slog.Logger {
	return slog.Default().With("component", "payment")
}

var sensitivePayloadKeys = map[string]struct{}{
	"hash":        {},
	"paytr_token": {},
	"token":       {},
	"utoken":      {},
	"ctoken":      {},
	"card_token":  {},
	"cardToken":   {},
	"cardUserKey": {},
}

// SafePayload returns a copy of callback/form payload with sensitive values masked.
func SafePayload(payload map[string]string) map[string]string {
	if len(payload) == 0 {
		return nil
	}
	out := make(map[string]string, len(payload))
	for k, v := range payload {
		key := strings.ToLower(k)
		if _, ok := sensitivePayloadKeys[key]; ok {
			out[k] = MaskSecret(v)
			continue
		}
		out[k] = v
	}
	return out
}

// MaskSecret masks a secret/token for logs.
func MaskSecret(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "…" + v[len(v)-4:]
}
