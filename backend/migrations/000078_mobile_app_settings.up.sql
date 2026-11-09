-- Mobile app version gate (TEC-1178, docs/design/mobile.md §2.1). Managed from the platform
-- panel (Settings → General) and published in GET /v1/app/config as `mobile`. A request with
-- `X-Client: comcode-mobile` and an `X-App-Version` below mobile_min_version is answered
-- 426 APP_UPDATE_REQUIRED. Empty strings mean "not set" (no gate, no store link).
ALTER TABLE app_settings
    ADD COLUMN mobile_min_version       VARCHAR(32)  NOT NULL DEFAULT '',
    ADD COLUMN mobile_latest_version    VARCHAR(32)  NOT NULL DEFAULT '',
    ADD COLUMN mobile_ios_store_url     VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN mobile_android_store_url VARCHAR(512) NOT NULL DEFAULT '';
