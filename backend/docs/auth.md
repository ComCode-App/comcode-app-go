# Auth & RBAC

Users are global; permissions come from assigned roles (union). App data belongs to **accounts** (personal + company) and is scoped to the session's active account; account roles (owner/admin/member) gate writes. Design: [`docs/TENANCY_AND_ACCESS.md`](../../docs/TENANCY_AND_ACCESS.md).

## Roles

| Role | Type | Notes |
|------|------|-------|
| `super_admin` | system (`is_system=true`) | All permissions; cannot delete slug or demote last member |
| `subscriber` | system role | `auth.session`, `notifications.read`, `app.*` feature permissions |
| Custom roles | platform-managed | Created via `/v1/platform/roles`; permissions from DB catalog only |

`users.is_super_admin` column is removed. JWT and `/me` still expose `is_super_admin` when the user has the `super_admin` role.

## Permissions (seed)

| Permission | Purpose |
|------------|---------|
| `platform.users.read` / `.write` / `.export` / `.import` | Platform user CRUD + I/O |
| `platform.users.impersonate` | Impersonate another user from platform admin (step-up) |
| `platform.users.bulk.disable` / `.bulk.enable` | User bulk actions |
| `platform.roles.read` / `.write` / `.export` / `.import` | Role CRUD + I/O |
| `platform.roles.bulk.delete` | Role bulk delete |
| `platform.bulk.read` | Bulk job list / rollback |
| `platform.notifications.read` / `.read_all` / `.export` | Platform notification log (`.read_all` unlocks other users' rows) |
| `platform.settings.read` / `.write` | Letterhead / export branding |
| `platform.activity.read` | Audit log |
| `platform.imports.read` / `platform.exports.read` | I/O job lists |
| `platform.storage.read` / `.write` | S3/SeaweedFS file browser |
| `platform.logs.read` / `.write` | Application logs + purge |
| `platform.access.read` / `.write` | Step-up policy |
| `platform.auth.settings.read` / `.write` | Registration policy (also requires super admin) |
| `platform.integrations.github.read` / `.write` | GitHub App settings (also requires super admin) |
| `platform.integrations.google.read` / `.write` | Google OAuth settings (also requires super admin) |
| `platform.integrations.facebook.read` / `.write` | Facebook OAuth settings (also requires super admin) |
| `platform.integrations.apple.read` / `.write` | Apple OAuth settings (also requires super admin) |
| `auth.session` | Sign-in / CMS access |
| `notifications.read` / `notifications.manage` | User notifications |

Super admin bypasses permission checks in `HasPermission`. Most platform HTTP routes use `RequirePermission` only. Auth settings and OAuth integration routes also use `RequireSuperAdmin`.

## Login & registration

- Password / passkey / OAuth availability is controlled by `auth_settings` + per-provider flags (`GET /v1/app/config` → `auth_methods`).
- Global `registration_enabled` must be on, plus the method’s `register` flag, for self-registration.
- Password register assigns the configured **default role** when set; otherwise no roles.
- **OAuth**: linked accounts can sign in when `login` is enabled. Unlinked accounts may self-register only when register is allowed for that provider.
- Secrets for OAuth apps are encrypted with `APP_ENCRYPTION_KEY`.

### Sign in with Apple (web, TEC-1123)

- **Signing key**: the platform admin uploads the `.p8` key with its Team ID and Key ID on the Apple integration page (`PATCH /v1/platform/integrations/apple`: `team_id`, `key_id`, `private_key`, `remove_private_key`). The key is validated (PEM, PKCS#8, EC P-256), stored encrypted with `APP_ENCRYPTION_KEY` in `oauth_provider_settings.apple_private_key_enc` (migration 000076) and is write-only: responses carry only `private_key_configured`, `private_key_fingerprint` (SHA-256 of the public key) and `client_secret_source`. Activity log meta has the Key ID and fingerprint, never the key; `logredact` / `redact.ts` replace any PEM private key in log text with `[private-key]`.
- **Client secret precedence**: (1) a stored signing key: the adapter route `/v1/internal/auth/oauth/apple` returns a freshly generated ES256 `client_secret` (sub = Services ID, valid 24 h) with `client_secret_expires_at`; (2) otherwise the pasted client secret JWT (`client_secret`); (3) otherwise Apple sign-in stays off. There is no env fallback: ComCode never read Apple settings from env. The frontend provider cache (TEC-1113) never outlives `client_secret_expires_at` minus 10 minutes.
- **form_post callback**: Apple returns with a cross-site POST to `/api/auth/callback/apple`. On `signin/apple` and `callback/apple` only, and only over https, the Auth.js state / PKCE / nonce cookies (and a dedicated callback-url cookie) are `SameSite=None; Secure` (`frontend/src/lib/auth/apple-form-post.ts`); session and CSRF cookies are unchanged. The Auth.js route refuses every other cross-site POST to `/api/auth/*` with the BFF's origin check.

### Native Apple / Google sign-in (mobile, TEC-1180)

| Method | Path | Auth |
|--------|------|------|
| POST | `/v1/auth/oauth/{apple\|google}/native` | Public → token pair (like `/auth/login`) |
| POST | `/v1/auth/identities/{apple\|google}/native` | Bearer → identity list (links to the signed-in user) |

- **Mobile only**: the app calls it through the mobile gateway (`/api/mobile/v1/*`). The cookie BFF refuses `auth/oauth/*/native` with `404` (`isCookieBffRefusedPath`), so the token pair is never handed to browser JavaScript.
- **Flow**: the app generates a random `nonce`, gives its SHA-256 to the Apple / Google SDK and sends `{id_token, nonce, authorization_code?, given_name?, family_name?, totp_code?}`. Apple's `authorization_code` is exchanged for a refresh token (see Apple tokens below); Google ignores it.
- **Verification** (`internal/platform/oidc`): RS256 signature against the provider JWKS (`appleid.apple.com/auth/keys`, `googleapis.com/oauth2/v3/certs`), cached for the response `max-age` (default 1 h, at most 24 h). A token with an unknown `kid` re-fetches the JWKS at most once a minute per provider (failed fetches count), so forged key ids cannot hammer the provider; a rotated key is picked up within a minute. Then `iss`, `exp` (1 minute leeway), `aud` (one of the configured audiences), a non-empty `sub` and `nonce` (equal to the raw nonce or its hex SHA-256; required). Any failure answers `401 OAUTH_INVALID_TOKEN`.
- **Audiences** (`oauth_provider_settings.native_client_ids`, migration 000079): set on Platform → Integrations → Apple (iOS bundle id) / Google (iOS and Android client ids; add the web client id if Android Credential Manager puts it in `aud`). `PATCH /v1/platform/integrations/{apple|google}` `native_client_ids` replaces the list. Native sign-in is on when `login_enabled` is on and the list is not empty, otherwise `403 OAUTH_PROVIDER_DISABLED`. It needs no client secret, so a provider may be switched on with native ids alone (the web provider then stays off until it has its credentials).
- **Outcomes**: the identity is looked up by provider + `sub` in `oauth_accounts` (the same rows the web Auth.js flow writes).
  - Linked: sign in. A user with an authenticator gets `403 MFA_REQUIRED` and resends the same token and nonce with `totp_code`. The `password_login_totp_required` policy does not apply, as for web OAuth. Disabled user: `403`.
  - Not linked, verified email of an existing account: `409 OAUTH_ACCOUNT_NOT_LINKED`. No automatic linking by email: the user signs in with the existing method and links the provider from the profile.
  - Not linked, `email_verified` false or no email: `403 OAUTH_EMAIL_UNVERIFIED`. Such a token never matches, creates or links an account (the link endpoint refuses it too).
  - Not linked, new verified email: with `registration_enabled` and the provider's `register_enabled` on, a passwordless user is created (email verified, `name` / `surname` from the token or the request, else the email local part; default role; personal account) and the identity linked. Otherwise `403 REGISTRATION_DISABLED`.
- **Session**: always starts on the personal account. With `X-Client: comcode-mobile` it is a mobile session (mobile refresh lifetime, device label; see Client type below).
- **Linking** needs a signed-in, non-impersonation session; `409 CONFLICT` when the identity is linked to another user or the user already has another identity of that provider. Activity: `auth.identity.linked`.

### Apple tokens: exchange and revocation (TEC-1181)

App Store review expects an app with Sign in with Apple to revoke the user's Apple tokens when the account is deleted. ComCode keeps one Apple refresh token per Apple identity for that only.

- **Exchange**: when a native Apple sign-in (`/auth/oauth/apple/native`) or link (`/auth/identities/apple/native`) succeeds and the body has `authorization_code`, the API posts it to `https://appleid.apple.com/auth/token` with `client_id` = the bundle id the id_token's `aud` matched (not the web Services ID) and a `client_secret` JWT signed with the panel `.p8` (ES256, `kid` = Key ID, `iss` = Team ID, `sub` = client id, `aud` = `https://appleid.apple.com`, valid 5 minutes; `appleauth.TokenClient`). The refresh token is stored encrypted with `APP_ENCRYPTION_KEY` in `oauth_accounts.refresh_token_enc` and the client id in `oauth_accounts.client_id` (migration 000082). The access and id tokens are discarded. A later sign-in with a new code replaces the stored token. The code is used only once the sign-in succeeds (an `MFA_REQUIRED` round resends it).
- **Best effort**: no `.p8` key stored, Apple refusing or unreachable (10 s timeout): the sign-in or link still succeeds, and the failure is logged (`auth_apple_code_exchange_failed` / `_skipped`).
- **Revocation**: `DELETE /v1/auth/identities/apple` reads the stored token, deletes the identity, then posts it to `https://appleid.apple.com/auth/revoke` (`token_type_hint=refresh_token`) with the stored `client_id`. A row without a token makes no call. A token stored by the web (Auth.js) flow has no `client_id` and is revoked with the web Services ID (secret generated from the `.p8`, else the pasted web client secret). A failed revoke is logged (`auth_apple_token_revoke_failed`) and never blocks the unlink.
- **Account deletion hook**: `usecase.AppleTokens.RevokeForUser(ctx, userID)` revokes and clears the stored token and returns the error for the caller to log. The account deletion cleanup (TEC-1122 §5.3) calls it; `AuthUseCase.AppleTokens()` hands it out.
- **No secrets in logs**: errors carry the endpoint, the HTTP status and Apple's error code (`invalid_grant`, ...) only, and pass through `logredact`. Codes, tokens, client secrets and the key are never logged.
- The Apple host is a constructor argument (`appleauth.NewTokenClient(baseURL, ...)`, production `appleauth.DefaultBaseURL`), so tests use an `httptest` server. There is no env setting.

## Email code sign-in (TEC-1121)

| Method | Path | Auth |
|--------|------|------|
| POST | `/v1/auth/email-code/request` | Public → `200 {status: accepted}` |
| POST | `/v1/auth/email-code/verify` | Public → token pair (like `/auth/login`) |

- 6-digit code, 10 minutes, 5 attempts, single use (`ConsumeOTP` reports a lost race), stored as SHA-256 in `otp_codes` (`type = login_code`, migration `000075`). A new request replaces the previous code.
- The email (`auth.login_code`, tr/en, user locale) is a `SecurityEmail`: the code is only stored sealed (see Security emails) and the notification preference does not apply.
- `request` always answers `accepted` for a well-formed email: known (any status), unknown with registration open (a sign-up code is sent) and unknown with registration closed (nothing is sent). A failure while issuing or enqueuing is logged (`login_code_request_failed`) and reported, like forgot password.
- `verify` checks the code first: wrong / expired / used / exhausted codes and unknown emails all answer `400 INVALID_EMAIL_CODE`. A disabled account is reported (`403`) only after a valid code. A valid code marks the email verified.
- 2FA: a user with an authenticator gets `403 MFA_REQUIRED` without `totp_code`; the email code stays valid, so the client resends the same code with `totp_code`. A wrong `totp_code` (`401 INVALID_MFA_CODE`) burns one of the code's attempts. The access policy "2FA required for sign-in without passkey/OAuth" (`password_login_totp_required`) applies to email codes too: a user without an authenticator gets `403 MFA_NOT_ENROLLED`.
- **Sign-up by code**: for an unknown email, while the global `registration_enabled` switch is on, a valid code creates the user (email verified, no password, `name` = email local part, empty `surname`, default role, welcome notification, personal account via the users trigger) and signs in on the personal account. With registration closed no account is created. Users who never set a password set one with the password reset flow.
- Email-code login is not gated by the password login switch; it is always offered on the login page.

### App Store / Play review accounts (TEC-1182)

Store reviewers cannot receive the emailed code, so the backend can give chosen accounts a fixed code.

- **Configure**: `AUTH_REVIEW_ACCOUNTS=review@example.com:246810[,other@example.com:913572]` (production: `.env.server` / Dokploy env, delivered by `compose.prod.yml`; blank or unset = off). Create the review account first (a normal active user, no authenticator unless you hand the reviewer one); give the reviewer only that email and its code.
- **Validation at boot** (`internal/config/review_accounts.go`): each entry is `email:code`, the code exactly 6 digits and not trivially guessable (repeated digits like `000000`, runs like `123456` / `654321`, patterns like `121212` / `123123`, and a few common codes are refused). Any malformed entry or duplicate email stops the API in every environment, so a typo never half-enables the bypass. Errors name the entry number, never the code.
- **Behaviour**: for a listed email that belongs to an **existing active** account, `request` sends no email and stores no code (same `accepted` response), and `verify` accepts the fixed code for that email only. Emailed codes keep working. The fixed code never works for another email, for an unknown email (it never creates an account, even with registration open) or for a disabled account, and does not mark the email verified. MFA rules are unchanged (authenticator `MFA_REQUIRED` round-trip, `MFA_NOT_ENROLLED` policy), and the per-IP / per-email rate limits below apply as for any code.
- **Audit**: each sign-in with a fixed code logs INFO `auth_review_code_used` with `user_id`, a truncated SHA-256 of the email (`email_sha256`) and `request_id`; never the code or the email in clear. A suppressed request logs `auth_review_account_code_request` (`user_id`), and boot logs `auth_review_accounts_enabled` with the count only.
- **Rotate** the code after each store review (new random 6 digits, redeploy): a reviewer's code must not stay valid for the next submission. **Remove** the variable (blank) when no review is pending; the account itself can stay.

## QR sign-in (TEC-1184)

Web shows a QR, the signed-in mobile app approves it (one direction only, [mobile.md §3](../../docs/design/mobile.md#3-qr-ile-giriş), KARAR-6). Module: `internal/modules/qrlogin`. State is Redis only: `app:<env>:qrlogin:<sha256(id)>` (the raw id is never a key), TTL 120 s; approval resets it to a 60 s exchange window. No table. Every transition is one Lua script: `pending → claimed → approved | rejected`, and a successful exchange deletes the key ("exchanged"; a replay gets 404).

| Method | Path | Auth |
|---|---|---|
| POST | `/v1/auth/qr/sessions` | Anonymous (web login tab) |
| POST | `/v1/auth/qr/sessions/{id}/state` | `browser_secret` |
| POST | `/v1/auth/qr/exchange` | `browser_secret` + `exchange_token`; web server only |
| GET | `/v1/auth/qr/sessions/{id}` | Bearer (phone) |
| POST | `/v1/auth/qr/sessions/{id}/approve` | Bearer |
| POST | `/v1/auth/qr/sessions/{id}/reject` | Bearer |

1. **Create** returns `session_id` (QR: `{PUBLIC_FRONTEND_URL}/login/qr/{id}`), a `browser_secret` that stays in the tab (only its SHA-256 is stored) and an anonymous Centrifugo grant: a connection JWT with `sub: ""` and a subscription JWT for one channel `qrlogin:{random}` (not derived from the session id), both valid until 30 s after the session. The request's User-Agent, client IP (`clientip`) and approximate location (GeoIP engine, `internal/platform/geoip`; empty when unknown) are stored for the approval screen. `503 REALTIME_DISABLED` without Centrifugo.
2. **Get** (phone) shows browser / OS / device parsed from the User-Agent (display hint only), IP, location, created / expires, and **claims** the session for the caller: another user then gets `409 QR_SESSION_CLAIMED`. The first claim publishes `{type:"scanned"}`.
3. **Approve / reject**: exactly one wins (`409 QR_SESSION_RESOLVED` for the rest). Approval publishes `{type:"approved", exchange_token}`, rejection `{type:"rejected"}`. After an approval the user gets the `auth.qr_login_approved` security email (sent even with email notifications off) and an in-app notice with browser, IP and location (migration `000081`).
   - Only an **active** user can approve (`403 FORBIDDEN` otherwise; disabled users are already refused by the auth middleware).
   - **Impersonation sessions** cannot view, approve or reject (`403 FORBIDDEN`), so a support session never claims a user's QR.
   - **Step-up (KARAR-15)**: a super admin or a user with any `platform.*` permission needs a fresh step-up grant (`/v1/auth/step-up/*`) to approve, else `403 STEP_UP_REQUIRED`. Other users approve without it (the app asks for the device's local check).
4. **State** is read once after the tab (re)subscribes, never polled: `pending | scanned | approved (+ exchange_token) | rejected`.
5. **Exchange** (NextAuth `qr-login` provider on the web server, mobile.md B8) → the same token pair as `/v1/auth/login` plus `user {uuid,email,name}`. The session is issued like every sign-in: **personal account** active, the approving phone's account is not carried; the session row records the QR browser's User-Agent / IP. No TOTP prompt: the phone already holds a signed-in session. The user is re-checked (active) at exchange.

Wrong `browser_secret` / `exchange_token` (`400 QR_LOGIN_INVALID`) burn attempts on `state` and `exchange`; the 5th deletes the session. The mobile gateway refuses `/api/mobile/v1/auth/qr/exchange`; the browser BFF refusal comes with the web panel (mobile.md B8).

Centrifugo: namespace `qrlogin` in `deploy/centrifugo/config*.json` with every client permission off (subscribe, publish, history, presence). The tab can join its channel only with the subscription token; `allow_anonymous_connect_without_token` stays off. Restart Centrifugo after deploying the config.

Phishing: an attacker can show a victim their own QR. The approval screen shows browser, IP and location; the app warns to approve only a code on a screen in front of you.

## Linked identities

| Method | Path | Auth |
|--------|------|------|
| GET | `/v1/auth/identities` | Bearer → `{items, total, has_password, has_passkey}` |
| DELETE | `/v1/auth/identities/{provider}` | Bearer (`github`, `google`, `facebook`, `apple`) |
| POST | `/v1/auth/identities/{apple\|google}/native` | Bearer, native id_token (see Native Apple / Google sign-in) |

Last sign-in method guard (TEC-1105):

- `has_password` comes from `users.password_set` (migration 000073). OAuth sign-up and user imports store a random hash and set it `false`; every password update (reset, change, admin set) sets it `true`. An OAuth-only user sets a password through the password reset flow.
- `DELETE /v1/auth/identities/{provider}` and `DELETE /v1/auth/passkeys/{uuid}` answer `409 LAST_SIGN_IN_METHOD` when the user has no password and the identity / passkey is their last sign-in method (no other identity, no other passkey).

GitHub: `GET/PATCH /v1/platform/integrations/github`. Google / Facebook / Apple: `/v1/platform/integrations/{provider}`. Auth policy: `GET/PATCH /v1/platform/auth/settings`.

Internal NextAuth adapter routes (not in OpenAPI): `/v1/internal/auth/oauth/{provider}`, `/v1/internal/auth/accounts`, `POST /v1/internal/auth/users`.

## JWT

HS256 claims: `sub`, `roles`, `is_super_admin`, `exp`, `sid` (session family), `acc` (active account UUID), optional `imp` (impersonator user UUID).

On every request the principal is reloaded from the database: roles and super-admin status (revocations apply immediately), the session family (revoked or expired → `401`) and the `acc` membership (lost membership → falls back to the personal account).

## Sessions

Each device session is a refresh-token **family**: refresh rotates the token atomically inside the family, so `sid` stays stable and in-flight access tokens keep working. Revoking a session (or logout) revokes the whole family. `sid` on the access token marks the current session.

| Method | Path | Auth |
|--------|------|------|
| GET | `/v1/auth/sessions` | Bearer |
| DELETE | `/v1/auth/sessions/{uuid}` | Bearer (own session) |
| POST | `/v1/auth/sessions/revoke-others` | Bearer (keeps current `sid`) |

`GET /v1/auth/sessions` items carry `client_type` (`web` | `mobile`) and, for mobile sessions, `device_name` and the latest `app_version`; the account page's sessions list shows them.

### Client type and refresh lifetime (TEC-1179)

- The mobile app sends `X-Client: mobile` (`comcode-mobile` is accepted too), `X-Device-Name` and `X-App-Version` on every request. Every handler that starts a session (login, email code verify, the adapter session used by OAuth / passkey) reads them through `sessionMeta`; other `X-Client` values, or none, mean `web`. Labels are cut to printable characters (device name max 100, app version max 32) and are display-only.
- The web BFF builds its upstream headers from an allowlist (`frontend/src/lib/server/upstream.ts`, `bff-proxy.ts`) and never forwards these headers, so browser sessions are always `web`. Impersonation sessions are always `web`.
- `refresh_tokens.client_type` is fixed when the family starts. Web sessions live `JWT_REFRESH_TTL` (default 7 days), mobile sessions `JWT_REFRESH_TTL_MOBILE` (default 60 days). Both slide: every refresh (and account switch) issues a token with a fresh lifetime of the family's type. A refresh request cannot change the type, so a stolen web token cannot be turned into a 60-day one; `X-App-Version` on a refresh updates the session's app version, the device name stays as set at sign-in.

### Refresh token reuse detection (TEC-1179)

Rotation marks the old token `rotated_at` (in addition to `revoked_at`). When `POST /v1/auth/refresh` gets a token it cannot rotate:

| Token state | Answer | Effect |
|-------------|--------|--------|
| Unknown, expired, or revoked by logout / session revocation / password change | `401` | none |
| Rotated less than 30 s ago (database clock) | `409 REFRESH_RACE` | none: a concurrent refresh of the same client won; use its token |
| Rotated earlier | `401` | the whole family is revoked, `auth_refresh_reuse_detected` is logged (WARN) and the owner gets the `auth.refresh_reuse` security email |

The email is only sent when the family still had a live token, so replaying a token of an already revoked session sends nothing. Clients refresh single-flight (web: `refresh-single-flight.ts`); the browser's refresh flow treats `409` as "renewed elsewhere" and retries the request.

### Push devices follow the session (TEC-1186)

A mobile push device (`push_devices`, `/v1/notifications/push-devices`) stores the `sid` it registered from in `session_family_uuid`. The revocation queries in `queries/auth_tokens.sql` disable the family's devices in the same statement as its refresh tokens:

| Revocation | Query | `disabled_reason` |
|------------|-------|-------------------|
| Logout (also a disabled user refreshing) | `RevokeRefreshFamilyByHash` | `signed_out` |
| One session (sessions list, refresh reuse, end of impersonation) | `RevokeRefreshFamilyForUser` | `session_revoked` |
| Sign out other sessions (devices of the kept session stay) | `RevokeOtherRefreshFamiliesForUser` | `session_revoked` |
| Password change / reset (every session) | `RevokeAllRefreshTokensForUser` | `all_sessions_revoked` |

Registration only succeeds while the family is live (`409 PUSH_SESSION_INACTIVE` otherwise), so an access token that outlives its session cannot re-enable the device; signing in again and registering moves the device to the new session. A family that simply expires is not marked; the sender (B10) must skip devices whose family has no live refresh token. A new revocation path must disable push devices the same way.
## Mobile clients (TEC-1178)

The mobile app does not use cookies. It reaches the private Go API through the Next.js **mobile
gateway** `/api/mobile/v1/*` → `/v1/*` (`frontend/src/lib/server/mobile-gateway.ts`, design:
[docs/design/mobile.md](../../docs/design/mobile.md) §2.1):

- Auth is the app's own `Authorization: Bearer <access>`, forwarded as is. The gateway never reads
  or sets a cookie: a request carrying only the web session cookie reaches Go unauthenticated, and
  upstream `Set-Cookie` is dropped.
- Token responses (`/v1/auth/login`, `/refresh`, `/email-code/verify`, `/accounts/switch`) keep
  `access_token` / `refresh_token` in the JSON; the app stores them in secure storage and refreshes
  itself (`POST /v1/auth/refresh` with the refresh token in the body). The cookie BFF
  (`/api/v1/*`) still strips them from browser responses.
- Refused with `404`: everything the BFF refuses (`/v1/internal/*`, traversal), the payment
  callbacks (`/v1/public/payment/*`) and `/v1/auth/qr/exchange` (called by the web server only).
- Forwarded: `X-Forwarded-For` / `User-Agent` (Go resolves the client IP through
  `TRUSTED_PROXIES`, so IP rate limits see the phone, not Next.js) and the app headers
  `X-Client: comcode-mobile`, `X-App-Version`, `X-Platform: ios|android`.
- **Version gate**: `GET /v1/app/config` returns `mobile: {min_version, latest_version,
  store_urls: {ios, android}}` from the platform settings (Settings → General → Mobile app,
  `PATCH /v1/platform/settings` `mobile_*` fields, `platform.settings.write`). A `/v1` request with
  `X-Client: comcode-mobile` and an `X-App-Version` below `min_version` is answered
  `426 APP_UPDATE_REQUIRED` (`error.details[0].message` = the minimum); `GET /v1/app/config`
  stays reachable so the app can open the store. The headers are never used for a security
  decision; the gate reads the setting through a 30-second cache and fails open if it cannot.

## Rate limits

Redis INCR, 15-minute window, fail-open if Redis is down. `429 RATE_LIMITED` + `Retry-After`.

| Endpoint | Key | Max |
|----------|-----|-----|
| Login | IP + email, IP, email | 10, 50, 30 |
| Register | IP | 5 |
| Forgot password | IP + email, IP, email | 5, 20, 5 |
| Reset password | IP, email | 10, 10 |
| Verification email request (`/auth/email/verify-request`) | user, IP | 5, 20 |
| Verify email (`/auth/email/verify`) | IP, email | 10, 10 |
| Email code request (`/auth/email-code/request`) | IP, email | 10, 5 |
| Email code verify (`/auth/email-code/verify`) | IP, email | 30, 10 |
| Native OAuth sign-in (`/auth/oauth/{provider}/native`) | IP; provider subject (after verification) | 30; 10 |
| Native identity link (`/auth/identities/{provider}/native`) | user, IP | 10, 30 |
| Card activation | user, and client IP | 10, 30 |
| Public analytics events (`POST /v1/events`) | client IP | 600 |
| QR sign-in create (`/auth/qr/sessions`) | client IP | 60 |
| QR sign-in state | client IP (+ 5 wrong secrets delete the session) | 60 |
| QR sign-in exchange | client IP, session | 60, 10 |
| QR sign-in view / approve + reject | user | 30, 30 |

Login and forgot-password keep a per-email cap across IPs so a botnet cannot guess one account's password from many addresses; it is higher than the IP + email cap, so locking a person out takes many requests and only lasts until the window ends (password reset and passkey sign-in are not affected).

"IP" is the end user's address, resolved once per request by `internal/platform/clientip`:

- The BFF forwards the incoming `X-Forwarded-For` chain and `User-Agent` on every upstream call (`frontend/src/lib/server/upstream.ts`).
- Go honours `X-Forwarded-For` only when the direct peer is in `TRUSTED_PROXIES` (default: loopback + private ranges). It walks the chain right to left and takes the first address that is not a trusted proxy; the client-supplied prefix is ignored.
- From any other peer the socket address is used, so a spoofed header cannot bypass the limits.
- The middleware rewrites `RemoteAddr`; use `clientip.String(r)` / `clientip.IP(r)` and never read `X-Forwarded-For` in handlers.

## Account enumeration (TEC-1101)

- Login verifies the password first. An unknown email runs an Argon2 check against a dummy hash (computed once, same parameters), so it costs as much as a wrong password, and both return the same `401 INVALID_CREDENTIALS` "Email or password is incorrect". A disabled or suspended account is reported (`403`) only after the correct password; 2FA (`MFA_REQUIRED`, `INVALID_MFA_CODE`, `MFA_NOT_ENROLLED`) also follows a correct password.
- `POST /v1/auth/password/forgot` always answers `accepted`, also when issuing the code or enqueuing the email fails for a registered email: that failure is logged at ERROR (`password_reset_request_failed`, with `request_id`, email redacted) and sent to the error tracker instead of returning a 500 (TEC-1116); `POST /v1/auth/password/reset` and `POST /v1/auth/email/verify` answer an unknown email like a wrong code. Email code sign-in follows the same rules (see Email code sign-in).
- `POST /v1/auth/email/verify-request` only mails the caller's own address. An `email` field naming another address is ignored with the same `accepted` response.

## Security emails

Email verification and password reset codes (`SecurityEmail`; the QR sign-in notice `auth.qr_login_approved` uses the same path but carries no secret) and account invitation links (`SecretVars: invite_url`) never sit readable on `notifications` rows (TEC-1099). The row keeps a masked rendering (`••••••` in place of the code or link, `action_url` dropped), so `GET /v1/platform/notifications`, the platform detail `GET /v1/platform/notifications/{uuid}` (any channel, email included; TEC-1120), its search, the export and backups never see it. The real subject and body are AES-GCM encrypted with `APP_ENCRYPTION_KEY` in `notifications.sealed_content`; the delivery worker decrypts them in memory, so retries and stuck-row reclaims send the real code, and a successful send sets the column to NULL. Without a usable key (none, or the built-in development key outside development/test) such notifications are refused instead of stored in plaintext. `GET /v1/notifications` and `GET /v1/notifications/{uuid}` serve `inapp` rows only, so an account cannot read its own verification code back. Migration `000072` masked older rows and cancelled unsent ones. The refresh reuse notice (`auth.refresh_reuse`, see Sessions) is also a `SecurityEmail`, so it ignores email preferences.

## `/v1/auth/me`

Returns `user`, `roles[]`, `permissions[]`, `account` (active), `accounts[]`, `subscription` (plan of the active account), self-service links, and realtime hints.

## Accounts

| Method | Path | Notes |
|--------|------|-------|
| GET | `/v1/auth/accounts` | Memberships with role and active flag |
| POST | `/v1/auth/accounts/switch` | Change the active account; returns a token pair |
| GET/PATCH | `/v1/account` | Active account (rename: owner/admin, company accounts) |
| GET/POST | `/v1/account/members` | List / add by email (owner/admin) |
| PATCH/DELETE | `/v1/account/members/{user_uuid}` | Change role / remove or leave; only owners manage owners; the last owner is protected |
| — | `/v1/platform/accounts/*` | Platform CRUD for company accounts (`platform.accounts.read` / `.write`) |

## Platform users

| Method | Path | Permission |
|--------|------|------------|
| GET | `/v1/platform/users/meta` | `platform.users.read` |
| GET | `/v1/platform/users` | `platform.users.read` — query: `limit`,`offset`,`q`,`sort`,`status`,`role` |
| POST | `/v1/platform/users` | `platform.users.write` — body includes `role_uuids[]` |
| GET | `/v1/platform/users/{uuid}` | `platform.users.read` — includes `roles[]` |
| PATCH | `/v1/platform/users/{uuid}` | `platform.users.write` — optional `role_uuids[]` |
| POST | `/v1/platform/users/{uuid}/password` | `platform.users.write` |
| POST | `/v1/platform/users/{uuid}/impersonate` | `platform.users.impersonate` (step-up) |
| POST | `/v1/auth/impersonation/stop` | Bearer (active impersonation session) |

Last super admin cannot be demoted via role removal or disable.

### Super admin boundary

Permissions alone never reach a super admin. A caller who holds `platform.users.*` but not the
`super_admin` role gets **403 `FORBIDDEN`** ("Only a super admin can change super admin accounts
or the super_admin role", `usecase.ErrSuperAdminRequired`) when they:

- grant `super_admin` (create or patch with its uuid in `role_uuids[]`, also to themselves);
- change a super admin account at all: profile, `role_uuids[]` (so also removing `super_admin`),
  `status` (disable/enable), password (`/password`) or impersonation.

The check reads the request principal, which `LoadPrincipal` reloads from the database on every
request. The other write paths enforce the same rule:

- **Bulk** (`POST /v1/platform/users/bulk`): the bulk service puts the job's actor on the context
  (`bulkengine.WithActor`, also for queued runs and rollbacks); `disable`/`enable` of a super admin
  fails that item unless the actor is a super admin, and so does a rollback that touches a super
  admin or restores a role snapshot holding `super_admin`.
- **Import** (`POST /v1/platform/users/import`): a row whose `role_slugs` (or the default
  `role_slugs`) contain `super_admin` is rejected ("the super_admin role cannot be assigned by
  import"), whoever runs it. An import rollback leaves users who are super admins by then alone.

New super admins come from an existing super admin on the user page, or from `make create-super-admin`.

## Platform roles

| Method | Path | Permission |
|--------|------|------------|
| GET | `/v1/platform/roles/meta` | `platform.roles.read` |
| GET | `/v1/platform/roles` | `platform.roles.read` |
| POST | `/v1/platform/roles` | `platform.roles.write` |
| GET | `/v1/platform/roles/{uuid}` | `platform.roles.read` |
| PATCH | `/v1/platform/roles/{uuid}` | `platform.roles.write` — system role slug immutable |
| DELETE | `/v1/platform/roles/{uuid}` | `platform.roles.write` — system roles → `409` |
| GET | `/v1/platform/permissions` | `platform.roles.read` — read-only catalog |

New permission slugs are added via migrations/seeds only (no permission CRUD API).

## CLI

`make create-super-admin` upserts the user and assigns the `super_admin` role.
