---
name: search-engine
description: >-
  Cmd+K command palette and Meilisearch-backed record search: backend
  searchengine adapters, compose service, frontend catalog, and RBAC-filtered
  specs. Use when adding searchable resources or extending the command palette.
---

# Search engine

Use when adding **record search** to the Cmd+K palette or wiring a new resource into Meilisearch.

## Backend adapter

1. Implement `searchengine.Adapter` in `backend/internal/platform/searchengine/adapters/`.
   - `Spec()` — `id`, `label_key` (`search.specs_<id>`), `permission`, `icon`, `searchable_fields`
   - `ListAll` — full reindex source
   - `Document` — single entity by UUID
   - Never index secrets (passwords, tokens, SMTP, VAPID).
   - Account-owned data: set `AccountScoped: true` in the spec and fill `Document.AccountID` (see below).
2. Add the adapter to `adapters.All(...)` (`adapters/app.go`); the API, the worker and `search-reindex` all register that list.
3. After create/update/delete in the owning use case, call `Indexer.EnqueueUpsert` / `EnqueueDelete` (fail-soft).
4. OpenAPI: generic `/v1/search` + `/v1/search/specs` already cover new specs; no new routes per resource unless you add custom behavior.

## Frontend spec (optional)

Remote specs are listed by `GET /v1/search/specs`. For local metadata/icons, export from the feature:

```ts
// frontend/src/features/users/search/index.tsx
export const usersSearchSpec = defineSearchSpec({
  id: "users",
  labelKey: "search.specs_users",
  icon: Users,
});
```

Pages/navigation items are built from `config/nav.ts` automatically (permission-filtered).

## Compose + env

- Local: `compose.local.yml` → Meilisearch on `${MEILI_PORT:-7700}`
- Prod: private `app` network only (not `dokploy-network`)
- Root `.env`: `SEARCH_ENABLED`, `MEILI_HOST`, `MEILI_MASTER_KEY`, `MEILI_INDEX_PREFIX`

## Ops

```sh
make search-reindex          # full rebuild of every spec in adapters.All (run after deploying a new spec or document field)
```

Boot: worker/API call `Indexer.Bootstrap` when an index is empty.

## Current specs

| Spec | Permission | Hit opens |
|------|------------|-----------|
| users | `platform.users.read` | `/platform/users/{uuid}` |
| roles | `platform.roles.read` | `/platform/settings/roles/{uuid}` |
| orders | `platform.orders.read` | `/platform/orders?open={uuid}` (detail dialog) |
| products | `platform.products.read` | `/platform/products?open={uuid}` (edit dialog) |
| companies | `platform.accounts.read` | `/platform/companies/{uuid}` |
| profiles | `app.profiles.read` | `/profiles/{uuid}` (account-scoped) |
| cards | `app.cards.read` | `/cards?open={uuid}` (settings dialog, account-scoped) |

## Account-scoped specs (TEC-951)

- `Spec.AccountScoped` makes `account_id` a filterable attribute; documents carry `account_id`.
- `Client.Search` adds `account_id = <active account>` (from `authctx.Principal.Account`) to every
  scoped query and drops any hit of another account (defense in depth). No active account → the
  scoped specs are not queried at all (fail closed).
- Platform specs stay unscoped and are gated only by their `platform.*` permission.
- Profiles/cards services enqueue upserts on create/update/activate and deletes on delete; card
  activation moves a card to another account, so it must reindex.
- Tests: `searchengine/searchtest` is an in-memory Meilisearch; see
  `modules/search/usecase/integration_test.go` for the cross-account check.

## RBAC

- Palette open: any session (`auth.session`)
- Spec chips + hits: filtered by each adapter's `permission` (e.g. `platform.users.read`)
- Super admin sees all registered specs

## Do not

- Expose Meilisearch to the browser
- Use `search` query param (use `q`)
- Index logs/activity in v1 (noisy + sensitive)
