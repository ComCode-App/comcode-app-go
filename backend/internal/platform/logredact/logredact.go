// Package logredact removes personal data and credentials from free-form text before it
// is written to logs or sent to the error tracker (TEC-1113).
//
// It is a best-effort filter for error messages and reasons: callers must still never log
// request bodies, passwords, cookies or tokens on purpose.
package logredact

import "regexp"

const redacted = "[redacted]"

var (
	// PEM private keys (Apple .p8, GitHub App keys), also pasted as one line
	// with literal \n; an unterminated block is cut to the end of the text.
	rePrivateKey = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`)
	reEmail      = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	reJWT        = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*`)
	reBearer     = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=\-]+`)
	// Expo push tokens (TEC-1187): Expo error messages quote the device token.
	reExpoToken = regexp.MustCompile(`(?:ExponentPushToken|ExpoPushToken)\[[^\]]*\]?`)
	// key=value / "key":"value" pairs whose key names a secret.
	reSecretPair = regexp.MustCompile(`(?i)\b((?:access_|refresh_|id_)?token|password|passwd|secret|api[_-]?key|authorization|cookie|code|otp|totp_code|ticket|dsn)(\s*=\s*|"\s*:\s*"?)([^\s"'&,;]+)`)
	rePhoneIntl  = regexp.MustCompile(`\+\d{1,3}[\s\-.]?\(?\d{2,4}\)?(?:[\s\-.]?\d{2,4}){2,4}`)
	rePhoneTR    = regexp.MustCompile(`\b0?5\d{2}[\s\-.]?\d{3}[\s\-.]?\d{2}[\s\-.]?\d{2}\b`)
)

// String returns s with PEM private keys, Expo push tokens, emails, phone numbers, JWTs, bearer/basic credentials and
// secret-looking key=value pairs replaced by placeholders.
func String(s string) string {
	if s == "" {
		return s
	}
	s = rePrivateKey.ReplaceAllString(s, "[private-key]")
	s = reJWT.ReplaceAllString(s, "[token]")
	s = reExpoToken.ReplaceAllString(s, "[push-token]")
	s = reBearer.ReplaceAllString(s, "$1 [token]")
	s = reSecretPair.ReplaceAllString(s, "$1$2"+redacted)
	s = reEmail.ReplaceAllString(s, "[email]")
	s = rePhoneIntl.ReplaceAllString(s, "[phone]")
	s = rePhoneTR.ReplaceAllString(s, "[phone]")
	return s
}

// Error is String(err.Error()); nil yields "".
func Error(err error) string {
	if err == nil {
		return ""
	}
	return String(err.Error())
}
