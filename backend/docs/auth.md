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

## Linked identities

| Method | Path | Auth |
|--------|------|------|
| GET | `/v1/auth/identities` | Bearer → `{items, total, has_password, has_passkey}` |
| DELETE | `/v1/auth/identities/{provider}` | Bearer (`github`, `google`, `facebook`, `apple`) |

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
| Card activation | user, and client IP | 10, 30 |
| Public analytics events (`POST /v1/events`) | client IP | 600 |

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

Email verification and password reset codes (`SecurityEmail`) and account invitation links (`SecretVars: invite_url`) never sit readable on `notifications` rows (TEC-1099). The row keeps a masked rendering (`••••••` in place of the code or link, `action_url` dropped), so `GET /v1/platform/notifications`, the platform detail `GET /v1/platform/notifications/{uuid}` (any channel, email included; TEC-1120), its search, the export and backups never see it. The real subject and body are AES-GCM encrypted with `APP_ENCRYPTION_KEY` in `notifications.sealed_content`; the delivery worker decrypts them in memory, so retries and stuck-row reclaims send the real code, and a successful send sets the column to NULL. Without a usable key (none, or the built-in development key outside development/test) such notifications are refused instead of stored in plaintext. `GET /v1/notifications` and `GET /v1/notifications/{uuid}` serve `inapp` rows only, so an account cannot read its own verification code back. Migration `000072` masked older rows and cancelled unsent ones.

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
