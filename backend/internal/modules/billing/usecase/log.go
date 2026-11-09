package usecase

import (
	"log/slog"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
)

func billingLog(log *slog.Logger) *slog.Logger {
	if log == nil {
		log = slog.Default()
	}
	return log.With("component", "billing")
}

func safePayload(payload map[string]string) map[string]string {
	return payment.SafePayload(payload)
}
