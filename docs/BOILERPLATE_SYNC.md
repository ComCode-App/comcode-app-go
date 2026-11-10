# Boilerplate senkronizasyon kaydı

ComCode, [nextjs-go-boilerplate](https://github.com/furkanmeclis/nextjs-go-boilerplate) üzerinden başladı ama iki reponun ortak git geçmişi yok. Bu belge, boilerplate'te yapılan değişikliklerin ComCode'a hangi kararla alındığının kalıcı kaydıdır: bir sonraki senkronizasyon nereden başlayacağını ve her commit için neden o kararın verildiğini buradan okur.

## Kapsam ve sonraki senkronizasyon

| Alan | Değer |
|------|-------|
| Boilerplate repo | <https://github.com/furkanmeclis/nextjs-go-boilerplate> (`main`) |
| Taban commit | `3ea585c8` (2026-08-24) |
| Son incelenen boilerplate commit'i | `a3b49617` (2026-11-04) |
| İnceleme tarihi | 2026-11-09 |
| İncelenen aralık | `3ea585c8..a3b49617`, merge hariç 105 commit |

Sonraki senkronizasyon için yöntem:

1. Boilerplate klonunda: `git fetch origin && git log --no-merges --reverse --format='%h %ad %s' --date=short a3b49617..origin/main`.
2. Her commit'i ComCode'daki karşılığıyla karşılaştırıp aşağıdaki kararlardan biriyle sınıflandır:
   - **PORT**: ComCode'da yok veya aynı hata var; alınacak.
   - **PORT (kısmi)**: Commit'in yalnız bir parçası ComCode'a uyuyor (geri kalanı tenant core, mobil vb.).
   - **ZATEN VAR** / **ZATEN VAR (kısmi)**: ComCode'da aynısı (veya daha iyisi) var; kısmi ise kalan farklar gerekçede yazılı.
   - **ATLA**: ComCode'a uymuyor.
   - Port tamamlanınca karar hücresine durum eklenir: `PORT edildi (TEC-xxxx)`, `PORT (kısmi) edildi (TEC-xxxx)`, ZATEN VAR (kısmi) satırlarında `eksik kısım port edildi (TEC-xxxx)`. Issue id'si değişikliği getiren birleşmiş PR'ın commit'inden alınır.
3. Port edilecekleri gruplayıp Linear'da (takım `TEC`) iş maddesi aç, değişiklikleri **elle** taşı. Ortak geçmiş olmadığı için `cherry-pick` / `merge` kullanılmaz.
4. Bu belgeye yeni bir bölüm ekle (aralık, tarih, tablo) ve yukarıdaki "Son incelenen boilerplate commit'i" satırını güncelle.

## Kurallar

- **Tenant core asla port edilmez.** Boilerplate'in organizations, `/t/{slug}`, `X-Organization`, `TENANCY_*` modeli ComCode'da yoktur; ComCode kişisel ve şirket hesaplarını kullanır ([TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md)). Tenant commit'lerinden yalnız modelden bağımsız fikirler (ör. sorgu izolasyon lint'i) uyarlanır.
- **Migration'lar ComCode numaralandırmasını sürdürür.** Boilerplate migration numaraları kopyalanmaz; yeni dosya `backend/migrations` içindeki son numaradan devam eder.
- **Mobil maddeler kapılıdır.** ComCode'da `mobile/` uygulaması yok. Native giriş, Expo push, QR giriş ve mobil CI, bir mobil uygulama kararı verilene kadar uygulanmaz (CI işleri eklense bile `mobile/` yokken atlanır).
- Boilerplate'in brand'e özgü veya ComCode'da zaten farklı çözülmüş parçaları (ör. ComCode'un `errreport` tabanlı hata izleme mimarisi, `clientip` çözümü) korunur; boilerplate sürümüyle değiştirilmez.

## Commit tablosu (3ea585c8..a3b49617)

Kronolojik sıra (boilerplate `git log --reverse` sırası). Issue sütunu, işi takip eden Linear maddesini gösterir; boş olanlar işlem gerektirmeyen ATLA / ZATEN VAR satırlarıdır.

| SHA | Tarih | Konu | Karar | Gerekçe | Issue |
|-----|-------|------|-------|---------|-------|
| `b579135` | 2026-08-29 | Frontend MFA hata sınıfları | ZATEN VAR | `frontend/src/auth.ts` MFA `CredentialsSignin` alt sınıfları ve `login-form.tsx` kod çözümü mevcut. |  |
| `a7f2da9` | 2026-09-16 | Credentials authorize hatalarını logla | PORT edildi (TEC-1113) | `auth.ts` authorize hatasını sessizce yutuyor; e-posta loglanmadan (code/status/request_id) taşınacak. | [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) |
| `d1f6f40` | 2026-09-19 | NextAuth provider cache, paralel OAuth config | PORT edildi (TEC-1113) | `auth.ts` her istekte public config + sıralı 4 `loadOAuthConfig` çağırıyor; TTL cache + `Promise.all`. | [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) |
| `2dc70e5` | 2026-09-19 | BFF route handler'da proxy hatalarını yakala | PORT edildi (TEC-1102) | `app/api/v1/[...path]/route.ts` try/catch yok; upstream düşerse HTML 500 döner, JSON zarfı bozulur. | [TEC-1102](https://linear.app/technowide-furkan/issue/TEC-1102) |
| `9f02f10` | 2026-09-19 | Yenilemede guard loader'ı sessionStorage ile atla | PORT | Opsiyonel. Hesap değiştirme/impersonation ile birlikte temizlenmeli (KARAR). | [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) |
| `94b244f` | 2026-09-24 | BFF upstream Content-Length'i iletme | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1102) | Buffered yolda siliniyor; `bff-proxy.ts` passthrough allowlist'i ve stream yolu hâlâ upstream uzunluğunu iletiyor. | [TEC-1102](https://linear.app/technowide-furkan/issue/TEC-1102) |
| `60110d4` | 2026-09-25 | Prod'da public varsayılan secret'ları reddet | PORT edildi (TEC-1100) | `config.go` `APP_ENCRYPTION_KEY` dev fallback'i, `validate()` yalnız JWT/Meili kontrol ediyor; adapter key sabit-zamanlı karşılaştırılmıyor. | [TEC-1100](https://linear.app/technowide-furkan/issue/TEC-1100) |
| `c6ad6d8` | 2026-09-25 | Refresh tek kullanım, login enumeration, verify abuse | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1101) | Refresh rotasyonu var (`auth_tokens.sql` CTE). Enumeration (timing, disabled sırası) ve `RequestEmailVerification` override + rate limit eksik. | [TEC-1101](https://linear.app/technowide-furkan/issue/TEC-1101) |
| `d08c1c1` | 2026-09-25 | Storage: aktif içeriği inline sunma | PORT edildi (TEC-1097) | `storage/handler` `writeFile`/`StreamMedia` SVG/HTML'i inline, nosniff yok; avatar uzantısı istemciden → stored XSS. | [TEC-1097](https://linear.app/technowide-furkan/issue/TEC-1097) |
| `6e4b0b7` | 2026-09-25 | BFF: CSRF origin, path allowlist, client IP, güvenlik başlıkları | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1102) | Client IP ComCode'da daha iyi (`clientip` + TRUSTED_PROXIES). Sec-Fetch-Site/Origin, `..`/`internal/*` reddi ve `next.config.ts` başlıkları eksik. | [TEC-1102](https://linear.app/technowide-furkan/issue/TEC-1102) |
| `ecfe277` | 2026-09-25 | super_admin'e yetki yükseltmeyi engelle | PORT edildi (TEC-1098) | `auth/usecase/platform.go` aktörün SA olup olmadığına bakmıyor; SetPassword ve import adapter'ı SA'yı korumuyor. | [TEC-1098](https://linear.app/technowide-furkan/issue/TEC-1098) |
| `8c713dc` | 2026-09-25 | Realtime: rastgele conversation kanalı token'ı verme | PORT edildi (TEC-1106) | `CanSubscribeChannel` `conversation:*` için koşulsuz true; tek satırlık fail-closed düzeltme. | [TEC-1106](https://linear.app/technowide-furkan/issue/TEC-1106) |
| `0d4031f` | 2026-09-25 | Login open redirect | PORT edildi (TEC-1103) | `login-form.tsx`/`register-form.tsx` `/\evil.com` kabul ediyor; ortak `safeNextPath`. | [TEC-1103](https://linear.app/technowide-furkan/issue/TEC-1103) |
| `9090bb2` | 2026-09-25 | Logo byte sniff + multipart gövde limiti | PORT edildi (TEC-1097) | `platform/storage/logo.go` beyan edilen Content-Type'a güveniyor; birçok handler'da `MaxBytesReader` yok. | [TEC-1097](https://linear.app/technowide-furkan/issue/TEC-1097) |
| `a3abe18` | 2026-09-25 | SMTP header injection | ZATEN VAR | `platform/mail/mail.go` subject CR/LF temizliği + Q-encode, alıcılar `Rcpt` ile. |  |
| `7399c96` | 2026-09-25 | Auth dokümanı: rate limit + client IP | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1101) | Client IP bölümü `backend/docs/auth.md`'de var; verify limit satırları TEC-1101 ile eklenecek. | [TEC-1101](https://linear.app/technowide-furkan/issue/TEC-1101) |
| `3d08cc5` | 2026-09-26 | Prod: PUBLIC_FRONTEND_URL ve WebAuthn RP ID backend'e | ZATEN VAR | `compose.prod.yml` ikisi de `:?required`. | [TEC-1095](https://linear.app/technowide-furkan/issue/TEC-1095) |
| `670d75f` | 2026-09-27 | Paralel refresh'te çıkışa atma (single-flight) | PORT edildi (TEC-1102) | BFF her 401'de bağımsız refresh yapıyor ve başarısızlıkta cookie siliyor; sunucu tarafı single-flight yok. | [TEC-1102](https://linear.app/technowide-furkan/issue/TEC-1102) |
| `a6f3c1d` | 2026-10-03 | Compose: SMTP kimlik bilgileri + MAIL_DRIVER | ZATEN VAR | `compose.prod.yml` backend-environment anchor'ında SMTP_* var, worker anchor'ı devralıyor. | [TEC-1095](https://linear.app/technowide-furkan/issue/TEC-1095) |
| `f4a228f` | 2026-10-03 | Apple form_post callback'inde OAuth check cookie'leri | PORT edildi (TEC-1123) | Check cookie'leri yalnız `signin/apple` + `callback/apple` isteklerinde ve https'te `SameSite=None; Secure` (istek bazlı Auth.js config); ayrıca Auth.js POST'larına çapraz site kontrolü, yalnız Apple callback muaf. | [TEC-1123](https://linear.app/technowide-furkan/issue/TEC-1123) |
| `1eac3a9` | 2026-10-01 | Şema: e-posta kodu, native OAuth token, hesap pasifleştirme | PORT (kısmi) | `deactivated_at` ve yeni OTP tipleri yok; `oauth_accounts.client_id` yalnız native → atla. E-posta kodu kısmı (`login_code` OTP tipi, `auth.login_code` şablonu, tek kullanımlık `ConsumeOTP`) TEC-1121 ile port edildi (migration 000075). | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `f8b28be` | 2026-10-01 | id_token doğrulayıcı (JWKS) + Apple token client | PORT (kısmi) | Doğrulayıcı TEC-1180 ile elle port edildi (`internal/platform/oidc`; bilinmeyen `kid` yeniden çekmesi başarısız denemeler dahil dakikada bir). Apple token client (takas / iptal) TEC-1181 ile elle port edildi (`appleauth.TokenClient`, `auth/usecase.AppleTokens`): `client_secret` panel `.p8`'iyle üretilir (TEC-1123 `SigningKey`), `client_id` = id_token'ın eşleştiği bundle id, refresh token şifreli + `oauth_accounts.client_id` (000082), erişim token'ı saklanmaz; iptal kimlik kaldırılınca, `RevokeForUser` hesap silme (TEC-1122) için hazır. Boilerplate'in env yedeği alınmadı. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114), [TEC-1181](https://linear.app/technowide-furkan/issue/TEC-1181) |
| `e72f52a` | 2026-10-01 | E-posta kodu girişi, native Apple/Google, pasifleştirme | PORT (kısmi) | E-posta kodu ve self-servis silme yok; native/link mobil → TEC-1180 ile uyarlandı (`/v1/auth/oauth/{apple\|google}/native`, `/v1/auth/identities/{provider}/native`; audience'lar panelden, link ticket akışı yok, KARAR-5). Pasifleştirme hesap modeline uyarlanmalı. E-posta kodu girişi TEC-1121 ile port edildi (`/v1/auth/email-code/*`; inceleme hesapları `AUTH_REVIEW_ACCOUNTS` alınmadı). | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `0ba91ed` | 2026-10-01 | OpenAPI: email code, native OAuth, link, deactivation | PORT (kısmi) | Yalnız port edilen uçların `openapi.yaml` kısmı. E-posta kodu uçları TEC-1121 ile eklendi. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `60ae434` | 2026-10-01 | Web: e-posta kodu ile giriş ve hesap silme UI | PORT | Login'de kod sekmesi, hesap ayarlarında silme diyaloğu yok. Login kod adımı TEC-1121 ile port edildi; hesap silme diyaloğu hâlâ yok. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `15f5db1` | 2026-10-01 | QR ile giriş oturumları (anonim Centrifugo) | ATLA | Onay mobil uygulamadan yapılıyor; mobil uygulama yok. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `a63dd0d` | 2026-10-01 | Web QR giriş paneli + /login/qr | ATLA | 15f5db1'e bağımlı. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `e3d6e54` | 2026-10-14 | QR doküman temizliği sonrası tip üretimi | ATLA | Yalnız QR OpenAPI tiplerinin yeniden üretimi. |  |
| `7860f12` | 2026-10-02 | Native kimlik bağlama, has_password, son giriş yöntemi koruması | PORT (kısmi) edildi (TEC-1105) | Native link atla. `UnlinkOAuthAccountForUser` son kimliğin silinmesine izin veriyor → `password_set` + `LAST_SIGN_IN_METHOD`. Migration `000073`. | [TEC-1105](https://linear.app/technowide-furkan/issue/TEC-1105) |
| `e427fce` | 2026-10-02 | Test: şifre sıfırlama password_set'i işaretler | PORT edildi (TEC-1105) | 7860f12'deki `password_set` ile birlikte. | [TEC-1105](https://linear.app/technowide-furkan/issue/TEC-1105) |
| `76523eb` | 2026-10-03 | Platform admin'de Apple .p8 yükleme | PORT (kısmi) edildi (TEC-1123) | Panelden .p8 + Team/Key ID, şifreli saklama, web client_secret üretimi (24 sa), provider cache süresi. Native exchange/revocation TEC-1181 ile eklendi; `AUTH_APPLE_*` env yedeği atlandı (ComCode env'den Apple okumuyor). | [TEC-1123](https://linear.app/technowide-furkan/issue/TEC-1123) |
| `dcc754d` | 2026-10-03 | Bilinmeyen e-posta için e-posta koduyla kayıt | PORT | E-posta kodu girişiyle birlikte; `authsettings` kayıt kuralına bağlı. TEC-1121 ile port edildi. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `3efcc07` | 2026-10-03 | Hata izleme (Sentry) + istek/auth hata loglama | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1113) | ComCode error-tracking daha gelişmiş (`errortracking`, `errreport`). 4xx/`/v1/auth/*` erişim logu ve 499 ayrımı eksik. | [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) |
| `2bb0096` | 2026-10-03 | Kullanımdan kalkan SendDefaultPII'yi kaldır | ZATEN VAR | `platform/errreport` zaten kullanmıyor. | [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) |
| `fc6264e` | 2026-10-01 | push_devices / push_tickets tabloları | ATLA | Expo mobil push; ComCode'da web push (VAPID) var. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `e15a659` | 2026-10-01 | Expo Push Service ile mobil push | ATLA | Mobil-only. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `065375f` | 2026-10-01 | Mobil push'ları worker'dan kuyrukla | ATLA | Mobil-only. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `8e4e6c3` | 2026-10-03 | Push cihaz kaydına rate limit | ATLA | Mobil-only; register ucu yok. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `fa3f11a` | 2026-10-14 | API hata mesajlarını koda göre yerelleştir | PORT edildi (TEC-1110) | `lib/api/errors.ts` sunucu mesajını aynen gösteriyor; `errors.json` içinde `codes`/`http` yok. | [TEC-1110](https://linear.app/technowide-furkan/issue/TEC-1110) |
| `47c6a1c` | 2026-10-14 | Realtime bildirim olayında payload ilet | PORT edildi (TEC-1110) | `notifications/providers` `RealtimeProvider.Deliver` `n.Payload`'ı yayınlamıyor. | [TEC-1110](https://linear.app/technowide-furkan/issue/TEC-1110) |
| `c6ba13e` | 2026-10-14 | Auth guard ve forbidden/unauthorized sayfalarını çevir | PORT edildi (TEC-1110) | `forbidden`/`unauthorized` sayfaları ve `route-guard.tsx` sabit Türkçe. | [TEC-1110](https://linear.app/technowide-furkan/issue/TEC-1110) |
| `8d7202f` | 2026-10-14 | Port edilen auth/push kodunu marka-nötr yap | ATLA | Düzeltilen dosyalar ComCode'da yok; native/QR port edilirse bu sonraki hal alınır. |  |
| `7f9b992` | 2026-10-14 | CI: yeşil feature branch için auto PR + merge | PORT edildi (TEC-1093) | `auto-pr` işi devre dışı (`if: false`/değişken) eklenecek; pnpm 11 pin'i atla. `auto-pr` `if: false` ile kapalı. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `e9fc3e0` | 2026-10-14 | Deploy: uygulama başına özel ağ | PORT edildi (TEC-1095, TEC-1115) | `app` ağı ve prefix'li isimler var; `stop_grace_period` TEC-1095'te, backend'in `dokploy-network`'ten çıkması ve `/healthz` healthcheck TEC-1115'te port edildi. | [TEC-1095](https://linear.app/technowide-furkan/issue/TEC-1095), [TEC-1115](https://linear.app/technowide-furkan/issue/TEC-1115) |
| `5da990f` | 2026-10-14 | Backend: kalan dosyaları gofmt | PORT edildi (TEC-1112) | `gofmt -l` ~74 dosya listeliyor; format hook'undan önce tek mekanik commit. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `9f687d7` | 2026-10-14 | Repo düzeyi Claude Code kurulumu | PORT edildi (TEC-1112) | `CLAUDE.md`, `.mcp.json`, hooks, reviewer, `verify-changes`/`db-migration` skill'leri yok; `.claude/settings.json` attribution bloğu korunarak. Reviewer `comcode-reviewer` adıyla; hook kaydı (`hooks` bloğu) `.claude/settings.json`'a sahip tarafından eklenir. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `14c7755` | 2026-10-14 | Tenant core tasarım çalışması | ATLA | Tenant core port edilmez; uyarlanabilir fikirler TEC-1107 ve TEC-1114'te. |  |
| `8eb1350` | 2026-10-14 | orgctx sözleşmesi ve tenant yanıt kodları | ATLA | Tenant core; karşılığı `authctx` `Principal.Account`. |  |
| `015d793` | 2026-10-14 | RequireOrganization / RequireTenantPermission | ATLA | Tenant core; üyelik `LoadPrincipal` içinde doğrulanıyor. |  |
| `87ea474` | 2026-10-14 | Redis cache'li orgctx resolver | ATLA | Tenant core. Fikir: `LoadPrincipal` için versiyonlu Redis cache. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `3b7d830` | 2026-10-14 | TENANCY_* config | ATLA | Tenant core bayrakları. |  |
| `cf0dc7e` | 2026-10-14 | GET /v1/tenant/context | ATLA | Karşılığı `/v1/account`, `/v1/auth/me`. |  |
| `423d4a3` | 2026-10-14 | BFF: X-Organization iletimi | ATLA | Tenant core; ComCode BFF zaten header allowlist kullanıyor. |  |
| `704b3a6` | 2026-10-14 | Sistem tanılama API'si | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1112) | Kaynak ComCode (`settings/usecase/system.go`). Küçük fark: sürücü adı sabit `"s3"`, Cmd+K rol linki 404. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `f4ecb7c` | 2026-10-14 | /platform/settings System Settings kabuğu | ZATEN VAR | Kaynak ComCode (`app/(shell)/(platform)/platform/settings/`). Skill ve dokümanlardaki eski `/platform/access`, `/platform/integrations/*` yolları TEC-1112 ile güncellendi. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `52ca5f5` | 2026-10-14 | Frontend: kalan dosyaları prettier | PORT edildi (TEC-1112) | `format:check` CI'da çalışmıyor; hook'tan önce `pnpm format` tek commit. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `4f63d95` | 2026-10-14 | Tenant core şeması ve sqlc | ATLA | Organizations şeması alınmaz; sorgu lint fikri 1a5b5db satırında. |  |
| `2a72917` | 2026-10-14 | Organizations modülü, /me üyelikleri | ATLA | Karşılığı `modules/accounts`, `/v1/auth/me` `accounts[]`. |  |
| `45cc26e` | 2026-10-14 | Platform organizations API dokümanı | ATLA | Tenant core OpenAPI; ComCode'da `/v1/platform/accounts/*`. |  |
| `0b109cf` | 2026-10-14 | Tenant core faz 1 bağlantısı, RBAC seed düzeltmesi | ATLA | Seed düzeltmesi tenant rol scope'una özgü. |  |
| `7a76aec` | 2026-11-03 | Frontend: Node 22.13+ zorunlu | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1112) | pnpm 10 kullanılıyor; yalnız `.nvmrc` (20) ile Dockerfile (22) tutarsızlığı giderilecek. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `7dfb448` | 2026-11-03 | Organizations i18n etiketleri | ATLA | Yalnız organizations anahtarları. |  |
| `62b72ad` | 2026-11-03 | Slug: Türkçe harfleri ASCII'ye katla | PORT edildi (TEC-1111) | `platform/slug/slug.go` Türkçe harfleri siliyor ("Çağrı Şirketi" → "a-r-irketi"); mevcut slug'lar yeniden yazılmaz. | [TEC-1111](https://linear.app/technowide-furkan/issue/TEC-1111) |
| `2c0ed87` | 2026-11-03 | Storage: upload gövdesini MaxBytesReader ile sınırla | PORT edildi (TEC-1097) | `storage/handler` `ParseMultipartForm(512<<20)` öncesi limit yok; kimliksiz `POST /v1/orders/assets/upload` dahil. | [TEC-1097](https://linear.app/technowide-furkan/issue/TEC-1097) |
| `ac23b89` | 2026-11-03 | SMTP timeout ve STARTTLS / implicit TLS | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1106) | Timeout ve `SMTP_TLS` var. Fark: `return c.Quit()` → QUIT hatasında mükerrer e-posta. | [TEC-1106](https://linear.app/technowide-furkan/issue/TEC-1106) |
| `529ed47` | 2026-11-03 | clientip: XFF'ye yalnız güvenilir proxy arkasında güven | ZATEN VAR | `platform/clientip` + `TRUSTED_PROXIES` zaten ComCode'da. |  |
| `2761ffc` | 2026-11-03 | Storage: MinIO → SeaweedFS, anonim erişimi kapat | ZATEN VAR (kısmi) | SeaweedFS var; anonim Read tüm bucket'ta açık, özel nesneler (exports/imports) aynı bucket'ta → public/private ayrımı. | [TEC-1109](https://linear.app/technowide-furkan/issue/TEC-1109) |
| `5478a7d` | 2026-11-03 | Env core tasarım çalışması | PORT edildi (TEC-1092) | `docs/design/` ve env çekirdeği yok. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `c33cb26` | 2026-11-03 | Şema tabanlı prod env üretici (faz 1) | PORT edildi (TEC-1092) | `deploy/env/schema`, `scripts/prod-env.sh`, `make prod-env-*` yok; şema ComCode anahtarlarıyla yazılmalı. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `46ab039` | 2026-11-03 | .env.server.example'ı şemadan üret (faz 2) | PORT edildi (TEC-1092) | `check-env-sync.sh`, `make env-check` yok. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `85ddaec` | 2026-11-03 | CI: env şema sync kontrolü | PORT edildi (TEC-1092) | CI'da `env` işi yok; self-hosted runner'a uyarlanacak. CI `env` işi GitHub-hosted runner'da. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `8c5c2c2` | 2026-11-03 | Backend env okumaları container'a ulaşıyor mu (faz 3) | PORT edildi (TEC-1092) | `check-compose-env.sh` + allowlist yok. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `48c1a22` | 2026-11-03 | CI: compose-env kontrolü | PORT edildi (TEC-1092) | 85ddaec ile aynı CI işi. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `8cd2080` | 2026-11-03 | Şemayla eşleşen secret guard, APP_ENV doğrulama (faz 4) | PORT edildi (TEC-1100, TEC-1092) | `validate()` `APP_ENCRYPTION_KEY`, `DB_PASSWORD`, Redis, S3, Centrifugo sırlarını ve `APP_ENV`'i doğrulamıyor. | [TEC-1100](https://linear.app/technowide-furkan/issue/TEC-1100) |
| `b046965` | 2026-11-03 | Şema tabanlı env akışı dokümanı (faz 5) | PORT edildi (TEC-1092, TEC-1096) | Env core'un geri kalanıyla birlikte gelir. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `ece3989` | 2026-11-03 | Compose: backend operatör ayarlarını ilet | PORT edildi (TEC-1092) | `config` `JWT_*_TTL`, `DB_MAX_CONNS`, `HTTP_*_TIMEOUT` vb. okuyor ama `compose.prod.yml` geçirmiyor. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `b82338c` | 2026-11-03 | Test: guard testlerinde Close hataları (errcheck) | PORT edildi (TEC-1100, TEC-1092) | `config/guard_test.go` env core ile gelince dahil edilir. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `0796eb4` | 2026-11-03 | CI: prod'da prebuilt image çek | PORT edildi (TEC-1095) | `compose.prod.yml` hâlâ `build:` taşıyor, `pull_policy` ve `deploy/images.json` yok. | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094), [TEC-1095](https://linear.app/technowide-furkan/issue/TEC-1095) |
| `26fac85` | 2026-11-03 | CI: path-filtreli pipeline, GHCR publish, v0.N tag | PORT edildi (TEC-1093, TEC-1094) | `ci.yml` yalnız backend/frontend işleri; image-build/publish/release-tag yok. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `59bab48` | 2026-11-03 | CI: mobile CI + EAS release | PORT (kısmi) edildi (TEC-1093) | `mobile/` yok; işler kapılı eklenir, mobil uygulama gelene kadar çalışmaz. İşler `MOBILE_ENABLED` repo değişkenine kapılı. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `bd2e26c` | 2026-11-03 | Doküman: GHCR-pull + v0.* tag deploy modeli | PORT edildi (TEC-1094, TEC-1096) | `docs/DEVELOPMENT.md` prod bölümü build-on-server modelini anlatıyor. | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094), [TEC-1096](https://linear.app/technowide-furkan/issue/TEC-1096) |
| `84840de` | 2026-11-03 | app_releases tablosu | PORT edildi (TEC-1094) | Tablo yok; migration ComCode numarasıyla (000072+). Migration `000074`. | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) |
| `2b9982b` | 2026-11-03 | Versiyon + notları backend'e göm, deploy kaydet | PORT edildi (TEC-1094) | `internal/buildinfo`, `cmd/release-record` yok. Uyarlandı: `internal/platform/release`, kayıt backend açılışında `app_releases` upsert (ayrı `release-record` yok). | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) |
| `2e0a092` | 2026-11-03 | CI: image'lara release provenance | PORT edildi (TEC-1094) | GHCR pipeline'ına bağlı. | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) |
| `647be7a` | 2026-11-03 | CI review düzeltmeleri, mobile-release kapısı | PORT edildi (TEC-1093, TEC-1094) | Path filtresi + mobile-release gating yok. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `b012eb3` | 2026-11-03 | CI: ubuntu-24.04 pin, Node 24 action'lar | PORT (kısmi) edildi (TEC-1093) | Runner pin'i geçersiz (self-hosted); yalnız action major sürümleri. Self-hosted runner kaldırıldı; `ubuntu-24.04` pin'i de alındı. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `24d1b20` | 2026-11-03 | GHCR tag'lerini IMAGE_REPO'dan türet (schema v2) | PORT (kısmi) edildi (TEC-1092, TEC-1094, TEC-1095) | Env şeması ve GHCR modeliyle birlikte. Şema v2 `BACKEND_IMAGE`/`FRONTEND_IMAGE`'i türetmek yerine kaldırdı; compose adı `IMAGE_REPO`/`IMAGE_TAG`'den kuruyor. | [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) |
| `1c94dfd` | 2026-11-03 | CI: test edilmemiş kodu publish/auto-merge etme | PORT edildi (TEC-1093, TEC-1094) | Pipeline ile birlikte; main push'ta tam suite, auto-pr kapısı. | [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) |
| `02e6ee9` | 2026-10-16 | Web push: Notification izni yalnız API varsa | ZATEN VAR | `use-web-push-sync.ts`, `lib/web-push/register.ts` zaten kontrol ediyor. |  |
| `43a0cb4` | 2026-11-03 | Compose: STORAGE_MAX_BYTES varsayılanı config.go'da | PORT edildi (TEC-1092) | Compose'da `STORAGE_MAX_BYTES` hiç yok; `${STORAGE_MAX_BYTES:-}` eklenecek. | [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) |
| `156af6a` | 2026-11-03 | /t/{slug} organizasyon kabuğu | PORT (kısmi) edildi (TEC-1103) | Tenant kabuğu atla; yalnız ortak `next-path` (backslash/kontrol karakteri) ve RouteGuard `?next=` parçası. | [TEC-1103](https://linear.app/technowide-furkan/issue/TEC-1103) |
| `57244d8` | 2026-11-03 | Playwright MCP çıktısını ignore et | PORT edildi (TEC-1112) | 9f687d7 ile `.mcp.json`/Playwright MCP gelince tek satır `.gitignore`. | [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) |
| `8fdc26f` | 2026-11-03 | /me üyelikleri yalnız tenancy açıkken | ATLA | Tenant core'a özgü. |  |
| `917a4f2` | 2026-11-03 | Her API hatası bir kez toast | PORT edildi (TEC-1110) | `unwrap()` ve QueryCache `onError` aynı hatayı iki kez emit ediyor; WeakSet ile tek emit. | [TEC-1110](https://linear.app/technowide-furkan/issue/TEC-1110) |
| `e5856d4` | 2026-11-03 | Organizasyon yönetim UI | ATLA | Tenant core; şirketler `platform.companies.*` ile yönetiliyor. |  |
| `ef377b9` | 2026-11-03 | Davetler, üyeler, ayarlar (tenant faz 3) | ATLA | Tenant core. Davet güvenliği fikirleri (`accounts/usecase/invites.go`) TEC-1104'te. | [TEC-1104](https://linear.app/technowide-furkan/issue/TEC-1104) |
| `1a5b5db` | 2026-11-03 | Org-scoped engine'ler ve izolasyon lint'i | PORT (kısmi) edildi (TEC-1107) | Engine'ler atla; lint `account_id` için uyarlanacak (`queries_test.go` + tablo listesi). | [TEC-1107](https://linear.app/technowide-furkan/issue/TEC-1107) |
| `9339e09` | 2026-11-03 | Tenant core dokümanları, add-tenant-resource skill | ATLA | Tenant core. |  |
| `6dd4c13` | 2026-11-03 | Modül self-registration, adım 1 | PORT | Opsiyonel (KARAR). En değerli parça route↔OpenAPI testi; guard'lar ComCode'a uyarlanmalı. | [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) |
| `4141d63` | 2026-11-04 | Davet hijack'i ve review bulguları | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1104) | E-posta eşleşmesi, tek kullanım var. Eksik: doğrulanmamış e-postayla `AddMember`, üye çıkarınca açık davetlerin iptali. | [TEC-1104](https://linear.app/technowide-furkan/issue/TEC-1104) |
| `8686dc6` | 2026-11-04 | Güvenlik e-posta sırlarını satırdan uzak tut | PORT edildi (TEC-1099) | OTP/davet kodu `notifications.body`'de düz metin ve kullanıcı inbox'ından okunabiliyor; `sealed_content`. Migration `000072`. | [TEC-1099](https://linear.app/technowide-furkan/issue/TEC-1099) |
| `554ad8f` | 2026-11-04 | Kayıt kapalıyken davetli adres kayıt olabilsin | ZATEN VAR (kısmi) | `auth.go` `PendingFor` ile var; e-posta kodu/OAuth ile davetli kayıt yok. | [TEC-1104](https://linear.app/technowide-furkan/issue/TEC-1104) |
| `70e744c` | 2026-11-04 | Platform session yokken search çağırma | ATLA | Sorun tenant-only üyelerden doğuyor; palette yalnız session'lı kabuklarda. |  |
| `f077b5f` | 2026-11-04 | create-super-admin image'da | ZATEN VAR (kısmi), eksik kısım port edildi (TEC-1106) | Binary image'da var; `cmd/create-super-admin` hâlâ `admin@example.com`/`Password1` varsayılanı taşıyor. | [TEC-1106](https://linear.app/technowide-furkan/issue/TEC-1106), [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) |
| `a3b4961` | 2026-11-04 | Auth.js session cookie'sini düz HTTP'de eşle | ZATEN VAR | `lib/auth/secure-cookies.ts` AUTH_URL şemasına bakıyor. |  |

## Linear iş maddeleri

| Issue | Konu |
|-------|------|
| [TEC-1092](https://linear.app/technowide-furkan/issue/TEC-1092) | Env core (şema tabanlı prod env) ve compose operatör ayarları |
| [TEC-1093](https://linear.app/technowide-furkan/issue/TEC-1093) | CI iskeleti (path filtresi, kapılı mobil, devre dışı auto-pr, env kontrolleri) |
| [TEC-1094](https://linear.app/technowide-furkan/issue/TEC-1094) | GHCR image'ları, v0.N tag'leri, release provenance |
| [TEC-1095](https://linear.app/technowide-furkan/issue/TEC-1095) | Prod compose: image pull ve uygulama başına özel ağ |
| [TEC-1096](https://linear.app/technowide-furkan/issue/TEC-1096) | Deploy dokümantasyonu |
| [TEC-1097](https://linear.app/technowide-furkan/issue/TEC-1097) | Upload güvenliği (aktif içerik, byte sniff, gövde limitleri) |
| [TEC-1098](https://linear.app/technowide-furkan/issue/TEC-1098) | super_admin yetki yükseltme |
| [TEC-1099](https://linear.app/technowide-furkan/issue/TEC-1099) | Bildirim satırlarında güvenlik e-postası sırları |
| [TEC-1100](https://linear.app/technowide-furkan/issue/TEC-1100) | Config varsayılan secret'ları ve APP_ENV doğrulaması |
| [TEC-1101](https://linear.app/technowide-furkan/issue/TEC-1101) | Login enumeration, e-posta doğrulama kötüye kullanımı, rate limit |
| [TEC-1102](https://linear.app/technowide-furkan/issue/TEC-1102) | BFF sertleştirme |
| [TEC-1103](https://linear.app/technowide-furkan/issue/TEC-1103) | Open redirect (`?next=`) |
| [TEC-1104](https://linear.app/technowide-furkan/issue/TEC-1104) | Şirket daveti sertleştirme |
| [TEC-1105](https://linear.app/technowide-furkan/issue/TEC-1105) | Son giriş yöntemi koruması (`has_password`) |
| [TEC-1106](https://linear.app/technowide-furkan/issue/TEC-1106) | Küçük güvenlik düzeltmeleri (realtime kanal, SMTP QUIT, CLI varsayılanları) |
| [TEC-1107](https://linear.app/technowide-furkan/issue/TEC-1107) | Hesap izolasyon lint'i (`account_id`) |
| [TEC-1108](https://linear.app/technowide-furkan/issue/TEC-1108) | Bu belge |
| [TEC-1109](https://linear.app/technowide-furkan/issue/TEC-1109) | Storage public/private ayrımı |
| [TEC-1110](https://linear.app/technowide-furkan/issue/TEC-1110) | Frontend hata gösterimi ve i18n |
| [TEC-1111](https://linear.app/technowide-furkan/issue/TEC-1111) | Türkçe slug |
| [TEC-1112](https://linear.app/technowide-furkan/issue/TEC-1112) | Repo hijyeni (format, Claude Code kurulumu, Node sürümü) |
| [TEC-1113](https://linear.app/technowide-furkan/issue/TEC-1113) | Auth loglama ve performans |
| [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) | Karar bekleyen özellikler (e-posta kodu girişi, hesap silme, Apple web, modül registry, principal cache, mobil/native/push) |
| [TEC-1115](https://linear.app/technowide-furkan/issue/TEC-1115) | Backend'i `dokploy-network`'ten çıkar, healthcheck `/healthz` |

## Özet

| Karar | Adet |
|-------|------|
| PORT | 48 |
| PORT (kısmi) | 9 |
| ZATEN VAR | 9 |
| ZATEN VAR (kısmi) | 13 |
| ATLA | 26 |
| **Toplam** | **105** |

Sayılar ilk karara göredir; "edildi" eki bir durumdur, kararı değiştirmez. 2026-11-09 itibarıyla `cc-collective-branch`'e birleşmiş PR'larla 59 satır port edildi olarak işaretli (TEC-1112 dahil).

**ZATEN VAR notu.** ComCode, boilerplate'in bu aralıkta eklediği bazı şeyleri zaten kendisi taşıyordu (bir kısmı boilerplate'e ComCode'dan geçti):

- Tek kullanımlık refresh token rotasyonu (`auth_tokens.sql` atomik CTE).
- `platform/clientip` ve `TRUSTED_PROXIES` ile güvenilir proxy arkasında XFF çözümü; BFF zinciri iletiyor.
- SMTP timeout, `SMTP_TLS` (STARTTLS / 465 implicit TLS) ve header injection koruması.
- `/platform/settings` System Settings kabuğu ve sistem tanılama API'si (ikisi de ComCode kaynaklı).
- Panelden yönetilen Sentry/GlitchTip hata izleme (`errortracking`, `errreport`), `SendDefaultPII` kullanılmıyor.
- SeaweedFS (MinIO yerine), uygulama başına `app` ağı, prod compose'da SMTP ve `PUBLIC_FRONTEND_URL`/WebAuthn RP ID.
- Şirket daveti temel güvenliği (yalnız SHA-256 token, e-posta eşleşmesi, tek kullanım) ve kayıt kapalıyken davetli kayıt.
- MFA hata sınıfları, Auth.js secure cookie eşleşmesi, web push `Notification` kontrolü.

Tenant core commit'lerinin tamamı (organizations, `/t/{slug}`, `X-Organization`, `TENANCY_*`) bilinçli olarak ATLA; mobil/native/push commit'leri mobil uygulama kararına kadar [TEC-1114](https://linear.app/technowide-furkan/issue/TEC-1114) altında bekliyor.
