---
name: storage-media
description: >-
  Object storage and media URL rules for this boilerplate: S3 object keys,
  API-owned public URLs, platform storage explorer, shares and versions.
  Use when adding uploads, logos, downloads, or the storage browser.
---

# Storage & media

**Hard rule:** the browser never builds S3 (SeaweedFS) URLs from object keys. Only API-returned `public_url` / `logo_url` / stream paths.

## Packages

| Piece | Path |
|-------|------|
| Driver | `backend/internal/platform/storage` |
| Key helpers | `backend/internal/platform/storage/keys.go` |
| Explorer module | `backend/internal/modules/storage` |
| Frontend explorer | `frontend/src/features/storage` |
| URL guard | `frontend/src/lib/media/urls.ts` → `assertServiceMediaURL` |

## Keys

Use helpers; do not invent prefixes:

- Branding logo: `app/branding/logo.{ext}`
- Export artifact: `exports/{job_uuid}/export.{ext}`
- Import source: `imports/{job_uuid}/source.{ext}`
- Company media library (TEC-946): `accounts/{account_uuid}/media/{media_uuid}.{ext}` (`storage.CompanyMediaObjectKey`); metadata in `company_media`, see `docs/COMPANIES.md`

Object **bytes** live in S3 (SeaweedFS). **Metadata** (name, size, versions, shares) stays in Postgres.

Env: `STORAGE_DRIVER` (`s3`), `STORAGE_PUBLIC_BASE_URL`, `S3_*`. `STORAGE_PUBLIC_BASE_URL` is for the API, not the browser.

The store is SeaweedFS (S3 gateway on 8333) in both compose files; the Go driver is the generic S3 driver. Anonymous access is GET-only on the bucket. Setup and security notes: `docs/DEVELOPMENT.md` → Object storage.

## Frontend

```ts
import { assertServiceMediaURL } from "@/lib/media/urls";

const src = assertServiceMediaURL(apiObject.public_url);
```

- Relative `/...` and absolute `http(s)://...` returned by the API are allowed.
- Local file preview may use `URL.createObjectURL`; revoke after use.
- Do not concatenate `STORAGE_PUBLIC_BASE_URL` or bucket + key in React.

## Explorer API

Permission: `platform.storage.read` / `platform.storage.write`.

Mounted from `internal/modules/storage/routes.go`:

- List / get / usage / download / preview / versions / activity / shares / links
- Write: folders, uploads (+ session/complete), copy, move, rename, delete, restore, purge

`GET /v1/platform/storage/meta` is registered. This resource is **not** io/bulk-capable.

## Logo / letterhead

Settings (`platform.settings.write`) stores letterhead fields. Logo upload goes through storage; settings responses expose `logo_url` from the API. PDF/XLSX export reads logo bytes server-side.
