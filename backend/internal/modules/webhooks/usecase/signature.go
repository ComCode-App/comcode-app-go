package usecase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signature header carried by every delivery: "t=<unix seconds>,v1=<hex HMAC-SHA256>" where the
// MAC covers "<t>.<raw body>". Receivers verify with their endpoint secret and reject old
// timestamps to stop replays (see docs/WEBHOOKS.md).
const (
	HeaderSignature = "X-ComCode-Signature"
	HeaderEvent     = "X-ComCode-Event"
	HeaderDelivery  = "X-ComCode-Delivery"
	HeaderEventID   = "X-ComCode-Event-Id"
	UserAgent       = "ComCode-Webhooks/1.0"
)

// Sign returns the signature header value of body at timestamp ts.
func Sign(secret string, ts time.Time, body []byte) string {
	t := strconv.FormatInt(ts.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(t))
	mac.Write([]byte("."))
	mac.Write(body)
	return "t=" + t + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a signature header against body with secret, rejecting timestamps older than
// tolerance. It is what a receiver does; kept here so the documented algorithm has a test.
func Verify(secret, header string, body []byte, now time.Time, tolerance time.Duration) error {
	var ts int64
	var sig string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("webhook signature: bad timestamp")
			}
			ts = n
		case "v1":
			sig = v
		}
	}
	if ts == 0 || sig == "" {
		return fmt.Errorf("webhook signature: missing t or v1")
	}
	at := time.Unix(ts, 0)
	if d := now.Sub(at); d > tolerance || d < -tolerance {
		return fmt.Errorf("webhook signature: timestamp outside tolerance")
	}
	expected := Sign(secret, at, body)
	if !hmac.Equal([]byte(expected), []byte("t="+strconv.FormatInt(ts, 10)+",v1="+sig)) {
		return fmt.Errorf("webhook signature: mismatch")
	}
	return nil
}
