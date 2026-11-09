# Public profiles (`/{username}`)

Status: implemented (2026-10-12, TEC-548).

Every profile has a public page at `comcode.../{username}`. Cards (`/c/{code}`) and smart links that target a profile open that page.

## Usernames

- A username is a `canonical_profile` row in the global `routes` registry. It therefore shares one namespace, and one case-insensitive uniqueness index, with card routes (`c/...`), smart links (`s/...`) and system paths.
- Format and reserved words: `internal/platform/publicpath`.
  - Format: 3–40 characters, lowercase letters and digits, with single `.`, `_` or `-` between them.
  - Reserved words include the app's own first-level pages (`account`, `profile`, `login`, `platform`, ...).
  - Routes and usernames use the same list. `TestFrontendTopLevelRoutesAreReserved` scans `frontend/src/app` directly (including nested route groups, API handlers and metadata files) and fails when a served top-level segment is missing; it skips only when frontend source is absent. Add a word to `publicpath` when adding a top-level route.
  - Current served segments: `account`, `analytics`, `api`, `apple-icon`, `background-studio`, `bank-accounts`, `billing`, `c`, `cards`, `forbidden`, `forgot-password`, `health`, `icon.svg`, `kvkk`, `leads`, `login`, `orders`, `platform`, `profile`, `profiles`, `reset-password`, `s`, `share`, `smart-links`, `unauthorized`, `verify-email`. Route groups and `[username]` are not reserved segments.
  - Legacy-bound words (`publicpath.IsLegacyBound`, today only `comcode`) stay reserved, but a printed legacy card owns that address and `/comcode` opens its profile (TEC-955). The drift test fails if the frontend ever serves one.
  - A legacy card whose username is its own printed hash keeps that hash as canonical username; its legacy route sits at the alias `r/legacy/{hash}` and the resolver renders `/{hash}` in place (TEC-956 option E, see `backend/cmd/migrate-legacy/README.md`).
  - Reserved words block new registrations and renames only. Existing canonical usernames still resolve through the visitor API, and owners may retain them when updating settings. Static Next.js routes still take precedence at the corresponding browser URL; reserving a word does not remove that existing collision.
- New profiles get a username on create, derived from the name: Turkish letters are folded, and `-2`, `-3` are appended on clashes. Migration 000040 did the same for existing profiles.
- Each profile has at most one live (`active` or `locked`) canonical route (`uq_routes_canonical_profile_live`).
- Renaming sets the old row to `retired`, so the old path keeps redirecting (301/308) to the new one and no one else can take it. The owner can rename back to one of their own retired usernames.

## Owner settings

`GET/PUT /v1/profiles/{uuid}/public-page` (UI: profile detail → "Public page"):

| Field | Meaning |
|-------|---------|
| `username` | Public path |
| `enabled` | `false` stores the route as `locked`: the page answers 404 and cards stop resolving to it |
| `visible_modules` | `bio`, `contacts`, `social_links`, `addresses`, `bank_accounts`; stored in the route's `presentation_config` |
| `bank_account_uuids` | Bank accounts of the active account to publish (`profile_bank_accounts`). None by default |
| `theme`, `layout_mode` | Theme and layout override, see [Themes](#themes-tec-601). `""` clears |

Writes need an owner or admin role in the active account (docs/TENANCY_AND_ACCESS.md).

## Visitor API (no auth)

- `GET /v1/public/p/{username}`:
  - Hidden modules come back empty.
  - `note`, `birthday` and `gender` are never sent.
  - A renamed username returns only `redirect_to`.
  - A disabled page, a suspended account or an unknown name is 404.
- `GET /v1/public/p/{username}/vcard?locale=` honours the same modules. The bio is the vCard `NOTE` line, so it is dropped when `bio` is hidden. A renamed username answers 301 with a relative Location.

## Frontend

- `app/[username]/page.tsx` is a server component.
  - It loads through `fetchUpstream` (React `cache` shares the load with `generateMetadata`).
  - It sets OG/Twitter metadata and a canonical URL, and redirects renamed usernames.
  - Static app routes win over the dynamic segment.
- `app/c/[code]/page.tsx` redirects open cards on the server. Locked, passcode and inactive cards fall back to the client page.
- Visitor events go to `POST /v1/events` (via the BFF). The subject is `{type: "profile", id: profile_uuid}`. Events:
  - `card.view`, `vcard.download`
  - `contact.{phone,email,website,whatsapp,address}.click`
  - `social.link.click` (`platform` property)
  - `bank.iban.copy`
- No embedding (TEC-1119): visitor pages (`/{username}`, `/c/*`, `/s/*`, `/share/*`) cannot be shown in an iframe on another site. Every route sends `X-Frame-Options: DENY` and `Content-Security-Policy: frame-ancestors 'none'` (`config/security-headers.ts`, applied from `next.config.ts`). Only the storage preview responses the app frames itself (`/api/v1/platform/storage/objects/preview`, `/api/v1/public/storage/*`) allow `'self'`. Embedding on partner sites would need an explicit `frame-ancestors` allowlist, an owner decision.
- The internal Linear feedback widget is not loaded on visitor pages (`components/common/feedback-widget.tsx`).
- Bundle budget (TEC-858): `[username]`, `c` and `s` sit outside `app/(shell)`, so they get only the root `PublicProviders` (locale, theme, toasts) and the public i18n namespaces in `lib/i18n/messages.ts`.
  - No session, React Query, step-up, realtime or dialog providers there. Do not call `useAuth`, `usePermission`, `useDialogs` or React Query hooks from visitor components.
  - A `t()` key used on a visitor page must be in a public namespace; panel namespaces (`lib/i18n/app-catalogs.ts`) are not loaded there.
  - Import feature services directly (`@/features/cards/services/cards.service`), not the feature barrel, which pulls panel components.

## Backgrounds (TEC-599)

- `backgrounds` table:
  - Rows with `account_id` set are the account's Background Studio designs (max 100 per account).
  - Rows with `account_id` NULL are platform gallery presets.
- App API is `/v1/backgrounds` (permissions `app.profiles.*`; writes need owner/admin).
- Presets API is `/v1/platform/backgrounds` (permissions `platform.settings.*`).
- `model.ValidateDocument` validates every document:
  - kind is one of a known set, and the shader type is in a whitelist (mirrors `shader-types.ts`)
  - documents are at most 32 KB
  - `params` strings must be safe tokens (colors, enums) or https URLs, so nothing can break out of CSS, `src` or uniforms; labels outside `params` are plain text without markup
  - video `src` must be https
- The public page setting `background_uuid` is stored as `presentation_config.background_uuid`.
  - It may point at the account's own design or a preset.
  - The public API inlines the document as `background`.
  - A deleted design falls back to the default background.
- Premium designs (TEC-947): animated shaders and videos need `design.premium` to be saved or picked. Without it the
  public page renders the theme's backdrop and keeps the stored choice. See [ENTITLEMENTS.md](./ENTITLEMENTS.md#premium-design-tec-947).
- Static thumbnail (TEC-599 remainder, `backgrounds/thumbnail`):
  - Every save of a solid, gradient or shader design renders a 360×640 JPEG. Solid and gradient designs are drawn exactly;
    a shader is drawn from its palette as a soft mesh blend. Videos have none, since their poster is the stand-in.
  - Stored at `backgrounds/{uuid}/thumb-{hash}.jpg` (`backgrounds.thumbnail_key`, migration 000068). The content hash
    changes the URL on every edit, and the old object is deleted. Deleting the design deletes it too.
  - Rendering is best effort: a failure keeps the save and the page keeps the CSS layer. Designs saved before 000068 get
    one from `BackfillThumbnails` at server start.
  - The API returns `thumbnail_url` on `/v1/backgrounds` and `background_thumbnail_url` on the visitor API.
- Render budget on the public page (TEC-600, `public-profile-background.tsx`):
  - The static layer is server-rendered: solid/gradient CSS, or for shaders a palette derived from the params (`lib/shader-fallback.ts`).
  - The WebGL shader mounts after the browser is idle, and only on capable devices (`lib/render-budget.ts`): hardware WebGL2, no data saver, more than 2 cores and more than 2 GB memory. Otherwise the design's thumbnail is laid over the static layer (the static layer alone when there is no thumbnail).
  - Only the selected shader's chunk is downloaded (`lib/shader-loaders.ts` → `data/shader-chunks/*`). Do not import `shaders-registry` or `lib/json` on the public page; they pull all 30 shaders.
  - `prefers-reduced-motion` freezes the shader on its configured `frame` and stops video autoplay. Rendering pauses off-screen and in hidden tabs.
  - Constrained devices get the video poster instead of the video.
- Do not re-export non-component helpers (`lib/json`, `services/*`) from `features/background-studio/index.ts`. Server components import that index, and the shader registry breaks in the RSC graph (`*Presets` are undefined there).

## Themes (TEC-601)

- Stored on the route: `presentation_config.theme` and `presentation_config.layout_mode`.
  - Set through `PUT /v1/profiles/{uuid}/public-page` (`theme`, `layout_mode`). Omit a field to keep it; `""` clears it.
  - The backend only checks ids against allow-lists: `model.PublicThemes` and `model.PublicLayoutModes`. An unknown stored value reads as unset.
  - The visitor API returns the resolved `theme` and the `layout_mode` override.
- Catalog: `features/public-profile/lib/themes.json` (tokens) and `themes.ts` (types, CSS variables).
  - Themes: `classic_light`, `dark_minimal`, `compact_card`, `bold_gradient`, `elegant_serif`, `glass`.
  - `bold_gradient`, `elegant_serif` and `glass` are premium (`premium`, `fallback` in `themes.json`; `model.PremiumThemes`). They need `design.premium`; without it visitors see the fallback theme (TEC-947).
  - A theme is a layout (`standard`, `compact`, `minimal`), color tokens, fonts (`sans` or `serif`), a heading weight, a cover gradient and a default backdrop.
  - To add a theme: add it to `themes.json`, `model.PublicThemes`, the `PublicTheme` enum in `openapi.yaml` and the `theme_name.*` strings. Then run `pnpm api:generate` and `pnpm test`.
- Rendering (`public-profile-view.tsx`):
  - The tokens are set as CSS variables on the page root (`--card`, `--primary`, `--muted-foreground`, `--comcode-gradient`, ...). Existing components pick them up without theme-specific code.
  - A profile with no theme (and no company theme) keeps the app's own look and follows light/dark.
  - Layout: `layout_mode` if set, else the theme's layout.
  - Backdrop: a Background Studio design (`background_uuid`) replaces the theme's backdrop. Cards, text and buttons keep the theme tokens.
  - Text never sits directly on the backdrop. Every text is on a themed surface (cards, language switch, footer pill), so contrast does not depend on the background.
- Budget and accessibility:
  - Themes add no motion. The shader budget and `prefers-reduced-motion` handling above apply unchanged.
  - The `elegant_serif` heading font (Playfair Display, `lib/theme-fonts.ts`) has `preload: false`. It downloads only when a heading uses it.
  - The `glass` backdrop blur is skipped on constrained devices. Its cards are opaque enough without it.
  - `themes.test.mjs` (`pnpm test`, in CI) checks WCAG AA, at least 4.5:1, for every text and surface pair of every theme. It checks translucent surfaces over black, white and the theme backdrop. It also checks that the catalog matches the backend allow-list.
- Company inheritance (TEC-946): the source is the company branding (`account_branding`, see [COMPANIES.md](./COMPANIES.md#branding-and-media-library-tec-946)), wired with `SetAccountThemes` in the API and the worker. It applies to every profile of the company account.
  - Unlocked: each field the profile set wins (`theme`, `layout_mode`, cover image); the company fills the rest. The company look is the employee's default.
  - Locked: each field the company set wins; the profile's own value only fills fields the company left empty.
  - The visitor API also returns `primary_color` (it replaces the theme's primary color, with white or near-black text on it, `lib/brand.ts`) and `company_logo_url` (shown on a pill over the cover).
  - The owner settings return `inherited_theme`, `inherited_layout_mode` and `branding_locked`. The picker shows the company theme as the "Company theme" option and, when locked, a note that the company look overrides the page.
- UI: profile detail → "Public page" → Theme / Layout (`features/public-profile/components/theme-picker.tsx`). Preview tiles are drawn with each theme's own tokens.

## Lead capture (TEC-552)

- Switch: `presentation_config.lead_capture.enabled` on the route.
  - Profiles: `lead_capture_enabled` in `PUT /v1/profiles/{uuid}/public-page`.
  - Smart links: `presentation_config` via `PUT /v1/routes/{uuid}`.
  - Cards: the card's own `lead_capture_enabled` (card settings) also turns it on.
- `/c/{code}` and `/s/{slug}` always open a profile target with `?via=c/{code}` (or `s/{slug}`), also when their own form is off (TEC-940). The page shows the card's form when it is on, else the profile's own form; either way the lead is stored against the card or link (`via` in `POST /v1/public/leads`), as long as it is live and opens the same profile. Visitor events carry `via` as a property. Direct URL / vCard targets have no form.
- Visitor API (no auth):
  - `GET /v1/public/leads/form?path=` says whether a path's form is on.
  - `POST /v1/public/leads` stores the lead. It is limited per client IP, and needs a name, an email or a phone, and `consent: true` with a known `consent_version` (`kvkk-lead-v1`; bump it with the text in `leads.form.consent_text`).
- A lead belongs to the route's account. It stores the consent time, the consent text version, the IP and the user agent. Owners and admins get a `leads.received` notification.
- App: `/v1/leads` (list / get / `POST {uuid}/read` / delete) with `app.leads.read|write`; writes need owner/admin. Deleting a lead removes it for good.
- Read scope (TEC-940): owners and admins of the account read every lead and exchange. Any other member (office managers included) reads only those that came to their own employee profile (`company_member_profiles`) or to a card that now points at it; the rest are 404 and not listed. The role is read live from `account_members` in the query, always together with the active account.
- Export: `POST /v1/leads/export` (io-engine resource `app.leads`, step-up), with the same read scope. The requester downloads it from `/v1/exports/{uuid}/download`.
- Monthly quotas (TEC-941): see [Locked leads](#locked-leads-tec-941).
- UI: the lead form on `/{username}`, and the inbox at `/leads`.

## Locked leads (TEC-941)

Lead forms, contact exchanges and lead exports each have a monthly plan quota (`leads.monthly_max`, `exchange.monthly_max`, `leads.export.monthly_max`; see [ENTITLEMENTS.md](./ENTITLEMENTS.md#monthly-quotas-tec-941)).

- Past the lead or exchange quota the visitor still gets `201`, and the row is stored with `leads.locked = true`. A quota of 0 turns the form or the exchange off (`GET /v1/public/leads/form` reports it off; submissions are 404).
- Locked rows are masked wherever the owner sees them: list, detail, export and the new-lead notification. Name `A*** Y***`, email `a***@***`, phone `*** 33`; message and sender profile are empty; company and job title stay. `q` does not match a locked row's name, email or phone. The stored data is untouched and follows the same KVKK retention and hard delete.
- Unlocking runs before every read and export (`usecase.unlock`) and is one-way:
  - a new month unlocks every lead locked in an earlier month;
  - a plan upgrade unlocks the current month's oldest locked leads, up to the new quota minus what is already unlocked this month (`account_monthly_usage.unlocked`, so deleting a visible lead does not free a slot);
  - an unlocked lead never locks again, also after a downgrade.
- Each `POST /v1/leads/export` takes one export from the month's quota; past it the answer is `403 PLAN_LIMIT_EXCEEDED` and the inbox disables the export button with the reset date. Super admins bypass it.
- Owners and admins get a panel, realtime and email notification at 80% and 100% of each quota (`entitlements.quota_warning` / `entitlements.quota_reached`, `notifications.QuotaAlerter`).
- UI: `/leads` shows "Bu ay 18/20 lead" meters (`LeadsQuotaMeter`), a lock badge and an upgrade hint on locked rows, and the export quota left under the export button.

## Contact exchange (TEC-567)

A visitor of a profile page shares their own details back with the profile's owner, like swapping paper business cards.

- Switch: `presentation_config.contact_exchange.enabled` on the profile's canonical route, set with `contact_exchange_enabled` in `PUT /v1/profiles/{uuid}/public-page`. Off by default, and independent of the lead form.
- Only profile pages offer it. `GET /v1/public/leads/form?path={username}` returns `exchange_enabled` and `exchange_consent_version`.
- An exchange is a lead with `kind: exchange`. It lands in the same `/leads` inbox (filter `kind=exchange`) and export, follows the same consent, rate limit and hard-delete rules, and notifies owners and admins with `leads.exchange_received`.
- Two ways to share:
  - Anyone: `POST /v1/public/exchanges` with a name, an email or a phone, and an optional company and job title.
  - A signed-in visitor: `POST /v1/exchanges` with `profile_uuid`, one of their active account's profiles. The lead stores a snapshot of that profile: name, company and title from its primary translation, and its first email and phone. It also keeps `sender_profile_id`, so the inbox links to the sender's public page. A plain member may share only a profile they own; owners and admins may share any profile of the account. Sharing with a page of your own active account is `409 SELF_EXCHANGE`. Sharing while impersonating is refused.
- Both need `consent: true` with `kvkk-exchange-v1`. Bump that version together with the text in `leads.exchange.consent_text`.
- `via` (`c/{code}` / `s/{slug}`) attributes the exchange to the card or smart link the visitor came through, when it is live and opens the same profile. Cards and smart links always add `via`, whatever their lead form switch (TEC-940).
- Visitor copy (TEC-940): when the exchange has an email (a signed-in share: the caller's account email), the visitor gets an email through the notifications queue (template `leads.exchange_copy`, `source_event = leads.exchange_copy:{lead uuid}`). It lists what they shared, attaches the owner's public vCard (rendered when the email is sent, `CopyAttachments`; the row stores no file) and links to `/kvkk/exchange?token=…`.
  - The token is `{lead uuid}.{exp}.{hmac}`, signed with a key derived from `JWT_ACCESS_SECRET`, valid for a year.
  - The page deletes only when the visitor confirms (`POST /v1/public/exchanges/delete`), so mail scanners that open links do not use it up. The exchange and its copy email row are deleted; a second use is 404, a forged or expired token 400 `INVALID_DELETE_TOKEN`.
  - Deleting the exchange from the inbox also deletes its copy email row.
- After sharing, the visitor is offered the owner's vCard (`/api/v1/public/p/{username}/vcard`).
- The signed-in visitor is resolved lazily when the form opens (`features/leads/hooks/use-visitor-user.ts`), since visitor pages have no `AuthProvider`.
- UI: the "Share your details" section on `/{username}`. The switch is in the profile's public page settings.

## Verified badge (TEC-549)

- The visitor API returns `verified: true` when a platform admin approved the profile and the
  approval was not revoked. The flag is fail-soft: a lookup error hides the badge, never the page.
- The page shows `VerifiedBadge` (`features/profiles/components/verified-badge.tsx`) after the
  name: a `BadgeCheck` icon with `role="img"`, the accessible label and tooltip
  "Doğrulanmış profil" / "Verified profile" (`public_profile.verified`), focusable for keyboard users.
- It is drawn with the `primary` token. `themes.test.mjs` already requires primary on card to reach
  4.5:1 in every theme (glass included over black, white and its backdrop), so the badge is
  legible on all TEC-601 themes without per-theme code.
- Approve, reject and revoke invalidate the profile's resolver cache entries.
- How a badge is requested, reviewed and lost again: [PROFILES.md](./PROFILES.md#verified-badge-tec-549).

## Follow-ups

- Profile modules (files, catalog, ...): TEC-551.

