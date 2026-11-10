# API errors

Failed `/v1/*` responses use the standard envelope:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": [
      { "field": "sort", "message": "unknown sort field \"foo\"", "code": "unknown" }
    ]
  },
  "meta": { "request_id": "..." }
}
```

Branch on `error.code`, not on message text. `details` is optional (field-level validation).

| Code | HTTP | When |
|------|------|------|
| `VALIDATION_ERROR` | 400 | Decode/invalid JSON, field validation, password policy, unknown sort |
| `INVALID_RESET_CODE` | 400 | Password-reset OTP wrong/expired/consumed |
| `INVALID_VERIFICATION_CODE` | 400 | Email-verification OTP wrong/expired/consumed |
| `UNAUTHENTICATED` | 401 | Missing/invalid access token, invalid refresh |
| `INVALID_CREDENTIALS` | 401 | Login or change-password: wrong password |
| `FORBIDDEN` | 403 | Missing permission or disabled user |
| `NOT_FOUND` | 404 | Resource does not exist or is not visible |
| `QR_LOGIN_INVALID` | 400 | QR sign-in state / exchange: wrong browser secret or exchange token, or not approved yet (5 wrong attempts delete the session) (TEC-1184) |
| `QR_SESSION_NOT_FOUND` | 404 | QR sign-in session unknown, expired, burned or already exchanged (TEC-1184) |
| `CONFLICT` | 409 | Unique constraint / duplicate (e.g. email already registered) |
| `QR_SESSION_CLAIMED` | 409 | QR sign-in session was opened by another user (TEC-1184) |
| `QR_SESSION_RESOLVED` | 409 | QR sign-in session was already approved or rejected (TEC-1184) |
| `APP_UPDATE_REQUIRED` | 426 | Mobile app (`X-Client: comcode-mobile`) older than `mobile.min_version`; every `/v1` route except `GET /v1/app/config` (TEC-1178) |
| `RATE_LIMITED` | 429 | Auth endpoints: login, register, forgot-password, reset-password |
| `PASSWORD_RESET_FAILED` | 500 | Forgot/reset persist or enqueue failure |
| `INTERNAL_ERROR` | 500 | Unexpected server failure |

Probes (`/healthz`, `/readyz`) are **not** enveloped.

See also [`docs/auth.md`](../auth.md) and [`API_CONVENTIONS.md`](../../API_CONVENTIONS.md).
