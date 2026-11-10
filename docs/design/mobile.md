# Mobil uygulama: native giriş, QR ile giriş ve Expo push

Durum: **tasarım önerisi, sahibin onayını bekliyor** (TEC-1124). Bu belgede kod yok. Kararlar
onaylanınca §7'deki alt işler Linear'da açılır. `KARAR-n` ile işaretli maddeler sahibin vermesi
gereken kararlardır; her birinde önerilen seçenek ilk sıradadır (§8).

Sahip bir mobil uygulama yapılacağını onayladı. Bu belge, [BOILERPLATE_SYNC.md](../BOILERPLATE_SYNC.md)
içinde "mobil karar verilene kadar ATLA" diye bekletilen boilerplate işlerini (native giriş, QR ile
giriş, Expo push, mobil CI) ComCode'un hesap modeline göre yeniden ele alır.

## 0. Kısa özet

- Uygulama **Expo (React Native) + TypeScript**, depoda `mobile/` altında. API tipleri
  `backend/docs/openapi.yaml`'dan üretilir, **commit edilir ve sürümü pin'lenir**; CI'daki hazır
  `mobile` / `mobile-release` işleri `MOBILE_ENABLED` ile açılır.
- Go API bugün **özel** (Dokploy domain'i yok, yalnızca BFF erişiyor) ve BFF token'ları HttpOnly
  çerezde tutup yanıtlardan siler. Mobil uygulama bu yüzden bugünkü `/api/v1/*` yolunu kullanamaz.
  Önerilen çözüm: Next.js'te çerez kullanmayan, yalnızca `Authorization: Bearer` ileten ayrı bir
  **mobil geçit** (`/api/mobile/v1/*`) (KARAR-1).
- Giriş: e-posta kodu ve parola (+TOTP) bugünkü uçlarla **değişmeden** çalışır. Yeni olan:
  native **Apple / Google** `id_token` girişi, oturuma cihaz bilgisi, mobil için daha uzun ve
  yeniden kullanım tespitli refresh politikası, App Store inceleme hesabı.
- **QR ile giriş**: web giriş sayfası QR gösterir, telefondaki oturum açık uygulama okutup onaylar
  (boilerplate `15f5db1` modeli). Ters yön (webden telefona giriş) v1'de yok (KARAR-6).
- **Expo push**: `push_devices` + `push_tickets` tabloları, `POST/DELETE` cihaz uçları, worker'da
  Expo gönderimi, makbuz kontrolü ve ölü token temizliği. Kilit ekranı metinleri kişisel veri
  içermez.
- **v1 kapsamı**: giriş + hesap değiştirme, profillerim (görüntüle / temel düzenleme), kartlarım
  (görüntüle / kilitle), paylaş (QR), gelen kutusu (lead'ler), bildirimler, QR ile web girişi onayı,
  analitik özet, oturumlar ve hesap silme. Satın alma, sipariş, şirket yönetimi ve sayfa tasarımı
  webde kalır.
- Backend işleri uygulama olmadan yapılıp test edilebilir; sıralama §7'de.

## 1. Boilerplate'teki mobil yığın ve ComCode'un aldıkları

### 1.1 Boilerplate'te ne var

Boilerplate'te **`mobile/` uygulama ağacı yok**. Olanlar:

| Parça | Boilerplate | Not |
|---|---|---|
| Mobil CI sözleşmesi | `.agents/skills/mobile-ci`, `ci.yml` `mobile` / `mobile-release` | Expo uygulamasının uyması gereken kurallar: `npm ci`, `lint`, `typecheck`, `test`, `api:generate` |
| API tipleri | `openapi-typescript ../backend/docs/openapi.yaml -o src/core/api/schema.d.ts` | Üretici **tam sürüme pin'li** (`^`/`~` yok), çıktı commit edilir; CI `api:generate` sonrası `git diff --exit-code` çalıştırır |
| Sürüm tetikleyici | `mobile_release` yalnızca gerçek (doküman olmayan) `mobile/**` değişikliğinde | `openapi.yaml` değişikliği tek başına uygulama yayını başlatmaz; tipler yeniden üretilip commit edilince `mobile/**` değişir ve yayın tetiklenir |
| EAS | `eas build --platform all --profile production --non-interactive --no-wait` | `EXPO_TOKEN` secret'ı ve `app.json` içinde `expo.extra.eas.projectId` yoksa iş yeşil atlanır |
| id_token doğrulayıcı (`f8b28be`) | `internal/platform/oidc` (JWKS önbelleği, RS256, `iss`/`aud`/`exp`/`nonce`), `internal/platform/appleauth` istemcisi | Native Apple / Google girişi için |
| Native OAuth (`e72f52a`) | `POST /v1/auth/oauth/{apple\|google}/native`, link ticket akışı, Apple token iptali | |
| QR ile giriş (`15f5db1`, `a63dd0d`) | `internal/modules/qrlogin` (yalnız Redis, 120 sn), anonim Centrifugo kanalı, web paneli ve NextAuth `qr-login` sağlayıcısı | Migration yok |
| Expo push (`fc6264e`, `e15a659`, `065375f`, `8e4e6c3`) | `push_devices`, `push_tickets`, `app:push:send`, `app:push:receipts`, `pushtext` kataloğu, `POST /v1/push-devices`, `DELETE /v1/push-devices/{token}`, kullanıcı başına 10 / dk kayıt sınırı | |

### 1.2 ComCode'da bugün ne var

| Konu | Durum | Kaynak |
|---|---|---|
| Mobil CI işleri | Port edildi (TEC-1093), `MOBILE_ENABLED` repo değişkenine kapılı; `mobile/package.json` yokken yeşil atlar | `.github/workflows/ci.yml`, [DEVELOPMENT.md](../DEVELOPMENT.md) |
| `mobile-ci` skill'i | **Yok** (yalnızca CI işleri port edildi) | — |
| Parola girişi, TOTP, refresh, logout, `/auth/me` | Var; token çifti JSON gövdesinde döner | [auth.md](../../backend/docs/auth.md) |
| E-posta kodu ile giriş / kayıt | Var (TEC-1121), `/v1/auth/email-code/*` | auth.md → Email code sign-in |
| Apple web girişi | Var (TEC-1123); `.p8` anahtarı panelden, şifreli (`oauth_provider_settings.apple_private_key_enc`) | auth.md → Sign in with Apple |
| Passkey girişi | Yalnız web: NextAuth WebAuthn + `/v1/internal/auth/authenticators*` adaptör uçları. Go'da giriş seremonisi yok; WebAuthn yalnız step-up için var (`internal/platform/stepup`) | |
| Native OAuth, id_token doğrulama | Yok | |
| Oturumlar | Refresh token **aileleri** (`family_uuid`, `sid`), `GET/DELETE /v1/auth/sessions`; satırda yalnız `user_agent`, `ip_address`. Döndürülmüş eski token'ın tekrar kullanımı aileyi iptal etmiyor (sadece `401`) | `000034_refresh_token_families` |
| Refresh süresi | `JWT_REFRESH_TTL` varsayılan 168 sa (7 gün), her refresh'te yeniden başlar | `internal/config` |
| Hesap modeli | Kişisel + şirket hesapları, aktif hesap `acc` claim'inde, `POST /v1/auth/accounts/switch` yeni token çifti döner. **Tenant core yok** | [TENANCY_AND_ACCESS.md](../TENANCY_AND_ACCESS.md) |
| Bildirimler | In-app, e-posta, realtime (Centrifugo `user:` kanalı), **web push (VAPID)** `push_subscriptions`; tercihler `notification_preferences` (`push_enabled` dahil) | `internal/modules/notifications` |
| Go API erişimi | Özel: `backend` servisine Dokploy domain'i bağlanmaz; BFF Go'ya `http://<prefix>-backend:8080/v1` ile gider | DEVELOPMENT.md → Dokploy domains |
| BFF | Token'lar HttpOnly çerezde; istemcinin gönderdiği `Authorization` yok sayılır, auth yanıtlarından token'lar silinir, `/v1/internal/*` reddedilir | `frontend/src/lib/server/bff-proxy.ts`, `bff-guard.ts` |
| CORS | Go'da CORS ara katmanı yok (gerek de yok: native istemci tarayıcı değil). `CORS_ALLOWED_ORIGINS` yalnız WebAuthn origin listesi olarak kullanılıyor | `httpserver/server.go` |
| GeoIP | Var (MaxMind, otomatik güncelleme) | [GEOIP.md](../GEOIP.md) |
| Hesap silme | Yok; tasarım bekliyor (TEC-1122) | [account-deletion.md](./account-deletion.md) |

### 1.3 Alınanlar / alınmayanlar

| Boilerplate parçası | Karar | Neden |
|---|---|---|
| Mobil CI sözleşmesi ve `mobile-ci` skill'i | **Al** (skill'i ComCode'a uyarlayarak ekle) | CI işleri zaten var; skill olmadan ajanlar sözleşmeyi bilmez |
| `oidc` doğrulayıcı + `appleauth` token istemcisi (`f8b28be`) | **Al** | Native Apple/Google için gerekli; `appleauth` paketi TEC-1123 ile kısmen var, `.p8` çözümleyicisi kullanılır |
| Native OAuth ucu (`e72f52a`) | **Al, uyarla** | Audience'lar env yerine admin panelinden (KARAR-4); başarılı girişte kişisel hesaba oturum açılır |
| Apple private relay "link ticket" akışı | **Şimdilik alma** (KARAR-5) | Web Apple akışında da yok; davranış iki istemcide aynı kalmalı |
| `AUTH_APPLE_*` env yedeği | **Alma** | ComCode Apple ayarını yalnız panelden okur (TEC-1123) |
| `AUTH_REVIEW_ACCOUNTS` (inceleme hesapları) | **Al** | E-posta kodu girişinde App Store / Play incelemesi koda erişemez |
| QR ile giriş (`15f5db1`, `a63dd0d`) | **Al, uyarla** | Konum için `AUTH_QR_GEOIP_DB` yerine mevcut GeoIP motoru; onaylayan oturumun aktif hesabı yerine kişisel hesap (§3.3) |
| `push_devices` / `push_tickets` + Expo gönderimi | **Al, genişlet** | Cihaz satırı oturum ailesine bağlanır, böylece oturum kapanınca push da durur (§4.1) |
| `pushtext` kataloğu | **Al, ComCode bildirim kodlarıyla yeniden yaz** | `lead.created`, `order.*`, `subscription.*` vb. |
| Tenant core (`X-Organization`, `/t/{slug}`) | **Alma** | ComCode hesap modeli kullanır |

### 1.4 Önerilen uygulama yığını

Kesin sürümler iskelet işinde (§7, A1) o günün en güncel kararlı Expo SDK'sına göre seçilir ve
`package.json`'da tam sürümle pin'lenir.

| Konu | Öneri |
|---|---|
| Çatı | Expo (managed workflow) + Expo Router, TypeScript strict |
| Paket yöneticisi | **npm** (`package-lock.json`): CI işi `npm ci` ile yazılı (KARAR-11) |
| API istemcisi | `openapi-typescript` (pin'li) → `src/core/api/schema.d.ts`; çağrılar için tipli ince bir `fetch` sarmalayıcı (ör. `openapi-fetch`), TanStack Query |
| Token saklama | `expo-secure-store` (iOS Keychain / Android Keystore). AsyncStorage'a token yazılmaz |
| Giriş SDK'ları | `expo-apple-authentication`; Google için native Google Sign-In kütüphanesi (Expo Go'da çalışmaz, **development build** gerekir) |
| QR okuma | `expo-camera` barkod tarayıcı |
| Push | `expo-notifications` (Expo Push Service) |
| i18n | tr / en; anahtarlar web'den bağımsız `mobile/src/locales` |
| Derleme / yayın | EAS Build + EAS Submit; OTA (`eas update`) v1'de kapalı (KARAR-12) |
| Hata takibi | Sentry React Native, DSN ayrı proje (web ile karışmasın) |

Klasör düzeni (öneri):

```
mobile/
├── app.json / app.config.ts     bundle id, EAS projectId, associated domains
├── eas.json                     development / preview / production profilleri
├── package.json                 lint, typecheck, test, api:generate (CI sözleşmesi)
├── src/app/                     Expo Router ekranları ((auth), (tabs), modals)
├── src/core/api/                schema.d.ts (üretilir, commit edilir), client.ts, auth refresh
├── src/core/auth/               secure-store, oturum durumu, tek-uçuşlu refresh
├── src/core/push/               izin, token kaydı, bildirime dokunma → deep link
├── src/features/                profiles, cards, leads, notifications, share, qr-login, analytics
└── src/locales/{tr,en}/
```

## 2. Mobil için kimlik doğrulama

### 2.1 API'ye erişim yolu (KARAR-1)

Mobil uygulama bir tarayıcı değildir; çerez kullanmaz, token'ı kendisi saklar ve her isteğe
`Authorization: Bearer <access>` ekler. Bugünkü BFF bunu desteklemiyor (istemci `Authorization`'ı
yok sayılıyor, giriş yanıtlarından token siliniyor). Seçenekler:

| | A. Next.js mobil geçidi (**önerilen**) | B. Ayrı API domain'i |
|---|---|---|
| Yol | `https://<site>/api/mobile/v1/*` → Go `/v1/*` | `https://api.<site>/v1/*` → Go (ayrı dinleyici, ör. `:8081`) |
| Go'nun durumu | Özel kalır (mevcut karar korunur) | `dokploy-network`'e geri katılır; ayrı dinleyici `/v1/internal/*` ve `/docs`'u kapatır |
| Çerez / CSRF | Geçit çerezleri hiç okumaz/yazmaz, yalnız gelen `Authorization`'ı iletir; CORS başlığı yok | Aynı |
| İstemci IP'si | BFF zaten `X-Forwarded-For` iletir, Go `TRUSTED_PROXIES` ile çözer | Traefik zinciri, aynı mantık |
| Maliyet | Bir ek atlama (Node), yükleme gövdeleri Node'dan akar (mevcut sınır geçerli) | Altyapı değişikliği, sahip Dokploy domain'i ekler, yeni saldırı yüzeyi |

Geçidin kuralları:

- `/v1/internal/*`, ödeme callback'leri ve yalnız web sunucusunun çağırdığı yollar (ör.
  `auth/qr/exchange`) reddedilir; izin listesi değil **ret listesi + `bff-guard` yol kontrolleri** (aynı kod).
- Gelen istekte çerez varsa yok sayılır; yanıtlara `Set-Cookie` eklenmez.
- Uygulama her istekte `X-Client: comcode-mobile`, `X-App-Version`, `X-Platform: ios|android`
  gönderir. Geçit bunları Go'ya iletir; Go bunları **yalnız gösterim ve sürüm kontrolü** için kullanır
  (güvenlik kararı vermez).
- Asgari sürüm: `GET /v1/app/config` yanıtına `mobile: {min_version, latest_version, store_urls}`
  eklenir (platform ayarlarından). Eski sürüme `426 APP_UPDATE_REQUIRED` döner ve uygulama
  mağazaya yönlendirir.
- Realtime gerekirse uygulama doğrudan `realtime.<site>` Centrifugo domain'ine bağlanır (token
  `/auth/me`'deki ipuçlarıyla). Native WebSocket'in `Origin` başlığı ve `CENTRIFUGO_ALLOWED_ORIGINS`
  etkileşimi uygulama iskeletinde doğrulanmalıdır. v1 için realtime şart değil.

### 2.2 Uç noktalar: yeniden kullanılan / yeni

| Akış | Uç | Durum |
|---|---|---|
| Parola girişi (+TOTP) | `POST /v1/auth/login` | **Aynen** (`MFA_REQUIRED` → `totp_code` ile tekrar) |
| E-posta kodu girişi / kaydı | `POST /v1/auth/email-code/request`, `/verify` | **Aynen**; v1'in ana giriş yolu |
| Parola ile kayıt | `POST /v1/auth/register` | Aynen (uygulamada gösterilmesi KARAR-3) |
| Parolamı unuttum | `/v1/auth/password/forgot`, `/reset` | Aynen |
| Refresh | `POST /v1/auth/refresh` | Aynen + mobil politika (§2.4) |
| Çıkış | `POST /v1/auth/logout` (refresh token gövdede) | Aynen + o cihazın push kaydını kapatır (§4.1) |
| Ben / hesaplar / hesap değiştir | `GET /v1/auth/me`, `GET /v1/auth/accounts`, `POST /v1/auth/accounts/switch` | Aynen; switch yeni çifti JSON'da döner, uygulama secure-store'a yazar |
| Oturumlar | `GET/DELETE /v1/auth/sessions`, `revoke-others` | Aynen + cihaz adı / platform alanları |
| Step-up | `/v1/auth/step-up/password`, `/totp` | Aynen. Passkey step-up native origin gerektirir (§2.5) |
| **Native Apple / Google** | `POST /v1/auth/oauth/{apple\|google}/native` | **Yeni** (§2.3) |
| **Native kimlik bağlama** | `POST /v1/auth/identities/{provider}/native` | **Yeni** (profilden Apple/Google bağlama) |
| **Passkey girişi** | `POST /v1/auth/passkey/options`, `/verify` | **Yeni**, v1.1 (KARAR-7) |
| **QR ile giriş** | `/v1/auth/qr/*` | **Yeni** (§3) |
| **Push cihazları** | `/v1/notifications/push-devices` | **Yeni** (§4) |
| **Hesap silme** | TEC-1122'de tanımlanacak | Bağımlılık: App Store uygulama içinden silmeyi başlatmayı şart koşar |

### 2.3 Native Apple / Google girişi

Akış (iki sağlayıcı aynı uçta):

1. Uygulama rastgele bir `nonce` üretir, SDK'ya SHA-256'sını verir, `id_token` (+ Apple'da
   `authorization_code`, ilk girişte `given_name` / `family_name`) alır.
2. `POST /v1/auth/oauth/{provider}/native {id_token, nonce, authorization_code?, given_name?,
   family_name?, totp_code?}`.
3. Go `id_token`'ı doğrular: sağlayıcı JWKS'i (önbellek `Cache-Control`'a göre, bilinmeyen `kid`'de
   en çok dakikada bir yeniden çekme), RS256 imzası, `iss`, `exp`, `aud`, `nonce`.
   - Apple `aud`: uygulamanın **bundle id**'si (web'deki Services ID değil).
   - Google `aud`: iOS ve Android OAuth client id'leri + web client id'si (Android Credential
     Manager web client id'sini `aud` olarak koyar).
4. Sonuç:
   - Bağlı kimlik → giriş (2FA'lı kullanıcıda `MFA_REQUIRED` turu, parola girişiyle aynı).
   - Bağlı değil + e-posta mevcut bir hesaba ait → `409 OAUTH_ACCOUNT_NOT_LINKED` (web NextAuth
     davranışıyla aynı: önce mevcut yöntemle giriş, sonra profilden bağla).
   - Bağlı değil + yeni e-posta → `registration_enabled` ve sağlayıcının `register` bayrağı açıksa
     kullanıcı oluşturulur (kişisel hesap trigger'la gelir), kimlik bağlanır.
   - Sağlayıcı kapalı ya da hiç audience yoksa `403 OAUTH_PROVIDER_DISABLED`.
   - E-posta doğrulanmamış Google token'ı (`email_verified=false`) mevcut hesaba asla eşlenmez.
5. Token çifti her zaman **kişisel hesap** aktifken döner (`active_account_id` boş); uygulama son
   kullanılan hesabı yerelde hatırlayıp `accounts/switch` çağırır.

Audience ayarları (KARAR-4): önerilen, `oauth_provider_settings`'e `native_client_ids TEXT[]`
sütunu eklemek ve Platform → Entegrasyonlar → Apple / Google sayfasında yönetmek (TEC-1123'te Apple
anahtarı da panelden yönetiliyor). Env (`AUTH_*_NATIVE_CLIENT_IDS`) yalnız boilerplate uyumu için
alternatif.

Apple token'ları:

- `authorization_code` gelirse ve panelde `.p8` anahtarı varsa Go kodu Apple'da takas eder
  (`client_id` = bundle id, `client_secret` = `.p8` ile imzalanmış ES256 JWT), dönen refresh token'ı
  `oauth_accounts.refresh_token_enc`'e şifreli yazar. Hangi `client_id` ile alındığı da saklanmalı
  (yeni sütun `oauth_accounts.client_id`), çünkü iptal aynı `client_id` ile yapılır.
- **İptal** (`https://appleid.apple.com/auth/revoke`): Apple kimliği kaldırılınca
  (`DELETE /v1/auth/identities/apple`) ve hesap silme temizliğinde (TEC-1122 §5.3). App Store,
  Sign in with Apple kullanan uygulamalarda hesap silmede bu iptali bekler.

Mağaza kuralı: uygulama Google girişi sunuyorsa Sign in with Apple (veya eşdeğeri) da sunulmalıdır.
Facebook native girişi v1'de yok (KARAR-3).

### 2.4 Refresh token, cihaz oturumları ve MFA

- **Cihaz bilgisi**: `refresh_tokens`'a `client_type` (`web` | `ios` | `android`), `device_name`,
  `app_version` sütunları. Değerler istemci başlıklarından gelir, yalnız oturum listesinde gösterilir.
  Oturumlar ekranı (web + uygulama) "iPhone 15 · ComCode 1.0.3 · İstanbul" gibi gösterir.
- **Mobil refresh süresi** (KARAR-8): web 7 gün kalır; mobil oturumlar için ayrı
  `JWT_REFRESH_TTL_MOBILE` (öneri 60 gün, her refresh'te kayar). Aksi halde bir hafta açılmayan
  uygulama kullanıcıyı çıkış yaptırır.
- **Yeniden kullanım tespiti** (öneri, KARAR-8 ile birlikte): döndürülmüş (revoked, `rotated`)
  bir refresh token yeniden sunulursa bütün aile iptal edilir ve güvenlik e-postası gider. Eşzamanlı
  refresh yarışı için kısa bir tolerans (ör. 30 sn içinde aynı eski token'ı sunan ikinci istek
  `409 REFRESH_RACE` alır, aile iptal edilmez). Uygulama tarafında refresh **tek uçuşlu** yapılır
  (web BFF'deki `refresh-single-flight` ile aynı fikir).
- **MFA**: parola, e-posta kodu ve native OAuth aynı `MFA_REQUIRED` / `totp_code` turunu kullanır.
  `password_login_totp_required` politikası e-posta kodu için de geçerli; native OAuth web OAuth ile
  aynı muameleyi görür (politika OAuth'a uygulanmıyor). QR ile giriş TOTP sormaz (onaylayan cihaz
  zaten oturum açık).
- **Uygulama kilidi** (opsiyonel, v1.1): Face ID / parmak izi ile uygulamayı açma
  (`expo-local-authentication`), yalnız istemci tarafı; sunucu tarafında güvenlik sayılmaz.

### 2.5 Passkey (KARAR-7)

Web'de oluşturulan passkey'ler aynı RP ID (`AUTH_WEBAUTHN_RP_ID`, ana domain) için uygulamada da
kullanılabilir, ama şunlar gerekir:

- Go'da giriş seremonisi: `POST /v1/auth/passkey/options` (challenge Redis'te, 5 dk) ve
  `POST /v1/auth/passkey/verify` → token çifti. Bugün yalnız NextAuth'ta var.
- RP origin listesine Android uygulama origin'i (`android:apk-key-hash:<imza özeti>`); iOS'ta
  Associated Domains `webcredentials:<domain>`.
- Next.js'in `/.well-known/apple-app-site-association` ve `/.well-known/assetlinks.json` sunması
  (QR evrensel bağlantısı için de gerekli, §3).

Öneri: v1'de passkey girişi yok, e-posta kodu + Apple + Google yeterli; v1.1'de eklenir.

### 2.6 Rate limit

Mevcut Redis sabit pencere sınırlayıcısı (`internal/platform/ratelimit`, 15 dk) kullanılır:

| Uç | Anahtar | Sınır (öneri) |
|---|---|---|
| Native OAuth | IP; sağlayıcı `sub` | 30; 10 |
| Passkey options / verify | IP | 60 / 30 |
| QR oturum oluşturma | IP | 60 |
| QR durum (`state`) / exchange | oturum; IP | 5 yanlış secret oturumu siler; 60 |
| QR görüntüle / onayla / reddet | kullanıcı | 30 |
| Push cihazı kaydı / silme | kullanıcı | 10 / dakika (boilerplate `8e4e6c3`) |
| Mevcutlar (login, e-posta kodu, …) | değişmez | [auth.md](../../backend/docs/auth.md#rate-limits) |

## 3. QR ile giriş

### 3.1 Yön (KARAR-6)

- **v1: web QR gösterir, telefon onaylar** (boilerplate modeli). Giriş sayfasında "Telefonla giriş"
  sekmesi. Avantajı: parola yazmadan, ortak bilgisayarda bile giriş; telefon güvenilen cihazdır.
- Ters yön (web oturumu, yeni telefonu onaylar) v1'de **yok**: QR'ı ele geçiren kişi doğrudan
  uzun ömürlü bir mobil oturum alır; e-posta kodu bu ihtiyacı zaten karşılıyor.

### 3.2 Akış

1. Web giriş sayfası → `POST /v1/auth/qr/sessions` (anonim, IP başına sınırlı). Yanıt:
   `session_id` (QR içeriği: `https://<site>/login/qr/{id}`), sekmede kalan `browser_secret`,
   tek bir `qrlogin:{rastgele}` kanalı için anonim Centrifugo bağlantı + abonelik token'ı.
   Durum yalnız Redis'te: `app:<env>:qrlogin:<sha256(id)>`, TTL 120 sn. Web sekmesi kanala abone olur,
   yoklama (polling) yapmaz; yeniden bağlanınca bir kez `POST …/{id}/state {browser_secret}` okur.
2. Telefon QR'ı uygulama içi tarayıcıyla okutur (ya da evrensel bağlantı uygulamayı açar) →
   `GET /v1/auth/qr/sessions/{id}` (Bearer). Yanıt: web isteğinin tarayıcı / işletim sistemi, IP,
   yaklaşık konum (mevcut **GeoIP motoru**), oluşturulma / bitiş zamanı. İlk görüntüleyen hesap
   oturumu sahiplenir (başka kullanıcıya `409 QR_SESSION_CLAIMED`); web'e `{type:"scanned"}` gider.
3. Kullanıcı onaylar ya da reddeder: `POST …/{id}/approve` / `…/reject`. Her geçiş tek Lua betiği
   (tam olarak bir onay/ret kazanır, `409 QR_SESSION_RESOLVED`). Onayda kanala tek kullanımlık
   `exchange_token` yayınlanır; sonrası 60 sn takas penceresi.
4. Web: NextAuth `qr-login` credentials sağlayıcısı sunucu tarafında
   `POST /v1/auth/qr/exchange {session_id, browser_secret, exchange_token}` çağırır → normal web
   oturumu. BFF bu yolu tarayıcıdan gelen isteklere kapatır (yalnız sunucu çağırır).

### 3.3 Güvenlik

- **Kısa ömür ve tek kullanım**: 120 sn QR, 60 sn takas; takasta anahtar silinir; 5 yanlış
  `browser_secret` oturumu siler.
- **Kim onaylayabilir**: aktif (pasif / askıda olmayan) kullanıcı; **impersonation oturumu
  onaylayamaz**; onay için `auth.session` izni.
- **Step-up**: onay ekranında cihaz yerel doğrulaması (Face ID / PIN) istenir (öneri). Sunucu
  tarafı step-up zorunlu değil, çünkü onay yeni bir web oturumu açar ama bu oturum telefonun
  oturumundan daha geniş yetki almaz. Platform yöneticileri (herhangi bir `platform.*` izni) için
  sunucu tarafı step-up zorunlu (öneri; admin oturumu daha hassas).
- **Hangi hesap**: web oturumu **kişisel hesapta** açılır (boilerplate onaylayanın aktif kurumunu
  taşıyordu; ComCode'da kullanıcı web'de hesap değiştirir). Daha basit ve sürpriz yok.
- **Kimlik avı**: saldırgan kurbana kendi QR'ını okutabilir. Onay ekranı tarayıcı, IP ve konumu
  büyük gösterir ve "Yalnızca önünüzdeki ekranda gördüğünüz kodu onaylayın" uyarısı verir. Konum
  için `CF-IPCountry` gibi edge başlıklarına yalnız Cloudflare arkasında güvenilir; ComCode'da bugün
  Cloudflare yok, bu yüzden GeoIP motoru kullanılır.
- **Bildirim**: onaydan sonra kullanıcıya "Yeni bir web oturumu açıldı" güvenlik e-postası ve in-app
  bildirim (yeni cihazdan giriş uyarısı ile aynı şablon).
- **Centrifugo**: `qrlogin` namespace'i, tüm istemci izinleri kapalı; anonim JWT bağlantısı
  `sub: ""` ile, `allow_anonymous_connect_without_token` kapalı kalır. `deploy/centrifugo/config*.json`
  değişir, deploy sonrası Centrifugo yeniden başlar.

### 3.4 Uç noktalar

| Yöntem | Yol | Kimlik |
|---|---|---|
| POST | `/v1/auth/qr/sessions` | Anonim (web sekmesi) |
| POST | `/v1/auth/qr/sessions/{id}/state` | `browser_secret` |
| POST | `/v1/auth/qr/exchange` | `browser_secret` + `exchange_token`, yalnız BFF sunucusu |
| GET | `/v1/auth/qr/sessions/{id}` | Bearer (telefon) |
| POST | `/v1/auth/qr/sessions/{id}/approve` | Bearer |
| POST | `/v1/auth/qr/sessions/{id}/reject` | Bearer |

Migration yok. Web tarafı (giriş sayfası paneli, `/login/qr/{id}` yedek sayfası) uygulamadan önce
yapılabilir; onay uçları curl / entegrasyon testiyle sınanır.

## 4. Expo push

### 4.1 Tablo ve uçlar

**Uygulandı (B9, TEC-1186):** migration `000080_push_devices`, `GET/POST /v1/notifications/push-devices`
ve `DELETE /v1/notifications/push-devices/{token}` (KARAR-10 (a)). Uygulamadaki farklar ve
ayrıntılar:

- `disabled_reason`: `signed_out` (çıkış), `session_revoked` (tek oturum kapatma, refresh yeniden
  kullanımı, impersonation sonu, "diğer oturumları kapat"), `all_sessions_revoked` (parola değişimi /
  sıfırlama), `device_not_registered` (B10 makbuzu). Devre dışı bırakma, refresh token iptaliyle aynı
  SQL deyiminde yapılır (`queries/auth_tokens.sql`, [auth.md](../../backend/docs/auth.md) → "Push
  devices follow the session").
- Kayıt yalnız canlı oturum ailesiyle olur; iptal edilmiş ya da süresi dolmuş oturumun access
  token'ı `409 PUSH_SESSION_INACTIVE` alır. Impersonation sırasında kayıt / silme `403
  PUSH_IMPERSONATION`.
- "İlk kayıt": kullanıcının daha önce hiç cihaz satırı yoksa (devre dışı olanlar dahil)
  `push_enabled` açılır; tercih satırı önceden varsa da açılır. Sonraki kayıtlar tercihe dokunmaz,
  böylece kullanıcının kapattığı push yeniden girişte açılmaz.
- Silme yalnız kendi token'ı içindir; başkasının ya da bilinmeyen token `404` alır (varlık sızmaz).
- Süresi kendiliğinden dolan aile işaretlenmez; B10 gönderirken canlı refresh token'ı olmayan
  ailenin cihazlarını atlamalıdır.
- POST ve DELETE kullanıcı başına dakikada 10 isteklik ortak bütçeyi paylaşır.

Önerilen şema (tasarım):

```
push_devices
  id, uuid, user_id → users ON DELETE CASCADE,
  token VARCHAR(255) UNIQUE,              -- ExponentPushToken[...]
  platform ('ios' | 'android'),
  locale, app_version, device_name,
  session_family_uuid UUID NULL,          -- ComCode eki: refresh ailesi
  last_seen_at, disabled_at, disabled_reason,
  created_at, updated_at
push_tickets
  id, ticket_id UNIQUE, push_device_id → push_devices ON DELETE CASCADE,
  notification_id → notifications ON DELETE SET NULL, created_at
```

- Kayıtlar **kullanıcı düzeyindedir**, hesap düzeyinde değil: bildirimler ve tercihler kullanıcıya
  gider. Bu yüzden tablolar `account_tables.txt`'e girmez.
- **Oturuma bağlama** (boilerplate'te yok): kayıt, isteğin `sid`'ini `session_family_uuid`'e yazar.
  Oturum kapatılınca / iptal edilince / hesap pasifleşince o ailenin cihazları `disabled_at`
  alır. Böylece çıkış yapılmış ya da uzaktan kapatılmış telefona bildirim gitmez. Aynı token başka
  kullanıcıyla tekrar kaydedilirse satır yeni kullanıcıya taşınır (cihaz el değiştirdi).

| Yöntem | Yol | Not |
|---|---|---|
| POST | `/v1/notifications/push-devices` | `{token, platform, locale?, app_version?, device_name?}` upsert; ilk kayıtta `push_enabled` açılır; kullanıcı başına 10 / dk |
| DELETE | `/v1/notifications/push-devices/{token}` | Yalnız kendi token'ı; 10 / dk |
| GET | `/v1/notifications/push-devices` | (opsiyonel) Bildirim ayarlarında cihaz listesi |

Yol adı (KARAR-10): ComCode'daki web push `/v1/notifications/push-subscriptions` altında olduğu için
`/v1/notifications/push-devices` önerilir. Boilerplate `/v1/push-devices` kullanıyor; port elle
yapıldığı için uyum zorunlu değil.

### 4.2 Gönderim yolu

```
notifications usecase (in-app satır teslim edildi)
  ├─ realtime (Centrifugo user: kanalı)          bugün var
  ├─ web push (VAPID, push_subscriptions)        bugün var, best effort
  └─ Asynq app:push:send (worker)                YENİ
        └─ Expo Push API, 100'lük paketler, opsiyonel EXPO_ACCESS_TOKEN
              └─ ticket'lar push_tickets'a
Asynq periyodik app:push:receipts (15 dk)         YENİ
  └─ makbuzlar; DeviceNotRegistered → cihaz disabled; diğer hatalar loglanır
```

- Gönderim API sürecinde değil **worker'da** ve kuyrukta (boilerplate `065375f`): Expo
  erişilemezse Asynq yeniden dener.
- **Kilit ekranı metni** (`pushtext` kataloğu): her bildirim kodu için tr/en başlık + gövde, yalnız
  izinli şablon değişkenleriyle, kullanıcının diliyle (yoksa cihaz dili). Kişisel veri yok:
  `lead.created` → "Yeni bir iletişim talebi aldınız", lead'in adı / telefonu **yazılmaz**.
  Güvenlik e-postaları (kod, sıfırlama) push'a hiç düşmez.
- `data`: `{url, notification_uuid, account_uuid?}`. Uygulama dokunulunca ilgili ekranı açar; bildirim
  başka hesaba aitse önce hesap değiştirir.
- iOS rozet sayısı: `unread-count` ile.

### 4.3 Tercihler

- `notification_preferences.push_enabled` hem web push'u hem mobil push'u yönetir (KARAR-9;
  alternatif: ayrı `mobile_push_enabled`). İşletim sistemi izni kapalıysa uygulama token
  kaydetmez / siler.
- Kategori bazlı tercih (yalnız lead'ler, yalnız siparişler) v1'de yok; ihtiyaç olursa
  `notification_preferences`'a JSON kategori haritası eklenir.

### 4.4 Ölü token temizliği

- Makbuzda `DeviceNotRegistered` → `disabled_at`, `disabled_reason = 'device_not_registered'`.
- Gönderim yanıtında `InvalidCredentials` → loglanır ve hata takibine gider (APNs / FCM
  kimlikleri EAS'ta eksik demektir); cihaz kapatılmaz.
- Bakım işi (mevcut günlük bakım kuyruğu): 30 günden eski `disabled` cihazlar ve 270 günden uzun
  süre `last_seen_at` güncellenmemiş cihazlar silinir; işlenmemiş `push_tickets` 24 saat sonra
  silinir.
- Uygulama her açılışta token'ı yeniden kaydeder (`last_seen_at` günceller); token değişirse eski
  token'ı siler.

### 4.5 KVKK

Hukuk danışmanı onayı gerekir (account-deletion KARAR-9 ile birlikte ele alınabilir):

- Push token, cihaz adı ve uygulama sürümü kişisel veridir (cihaz tanımlayıcısı). Aydınlatma
  metnine ve gizlilik politikasına mobil uygulama bölümü eklenir: amaç (bildirim gönderimi), saklama
  süresi (§4.4), alıcılar.
- **Yurt dışı aktarım**: Expo (ABD), Apple APNs ve Google FCM üzerinden. KVKK 9. madde (2024
  değişikliği) kapsamında standart sözleşme ve Kurul'a bildirim gerekebilir. Bildirim içeriği kişisel
  veri taşımadığı için (§4.2) aktarılan veri token + genel metinle sınırlı kalır.
- Hesap silmede `push_devices` kaskadla silinir; TEC-1122 temizlik listesine eklenir.
- Mağaza beyanları: App Store "App Privacy" etiketleri ve Google Play "Data safety" formu
  (cihaz kimliği, e-posta, ad, kullanım verisi; izleme yok).

## 5. v1 kapsamı

Öneri: küçük ve sık kullanılan; düzenlemenin ağırı webde kalır.

| Özellik | v1 | API |
|---|---|---|
| Giriş: e-posta kodu, parola + TOTP, Apple, Google; hesap değiştirici | ✓ | §2 |
| Profillerim: liste, görüntüle, temel düzenleme (ad, unvan, iletişim alanları, avatar) | ✓ | `/v1/profiles*` |
| Kartlarım: liste, durum, **kilitle / kilidi aç** (kayıp kart) | ✓ | `/v1/cards`, `PATCH …/toggle-lock` |
| Paylaş: profil QR'ı tam ekran, paylaşım sayfası, vCard | ✓ | `/v1/routes/{uuid}/qr/render`, `/v1/profiles/{uuid}/vcard` |
| Gelen kutusu (lead'ler): liste, detay, okundu, ara / e-posta / rehbere ekle | ✓ | `/v1/leads*` |
| Bildirimler: liste, okundu, push | ✓ | `/v1/notifications*`, §4 |
| QR ile web girişini onayla | ✓ | §3 |
| Analitik özeti: son 7 / 30 gün görüntülenme ve tarama | ✓ (tek ekran) | `/v1/analytics/summary`, `timeseries` |
| Oturumlarım, çıkış, hesabımı sil | ✓ | `/v1/auth/sessions*`, TEC-1122 |
| Kart aktivasyonu (NFC okuma) | v1.1 | `POST /v1/cards/activate` |
| Kartvizit takası (karşı tarafın QR'ını okut) | v1.1 | `POST /v1/exchanges` |
| Passkey girişi, uygulama kilidi | v1.1 | §2.5 |
| Plan satın alma, abonelik, sipariş | **Yok** | Dijital abonelik uygulama içinde satılırsa mağaza ödeme kuralları (IAP) devreye girer (KARAR-13) |
| Şirket yönetimi, ekip, ofisler, sayfa tasarımı, smart link, webhook | **Yok** | Webde kalır |

Yetki: uygulama aynı `app.*` izinlerini ve hesap rolünü kullanır; üye (member) rolündeki kullanıcı
düzenleme düğmelerini görmez (`/auth/me` `permissions` + aktif hesabın `role`'ü).

## 6. CI ve yayın

### 6.1 Depo tarafı (ajanlar yapar)

- `mobile/` iskeleti CI sözleşmesine uyar: `npm ci`, `lint`, `typecheck`, `test`, `api:generate`
  (`openapi-typescript` tam sürüm), `src/core/api/schema.d.ts` ve `package-lock.json` commit'li.
- `mobile-ci` skill'i ComCode'a uyarlanarak `.agents/skills/mobile-ci/` altına eklenir ve
  `.claude/skills/mobile-ci` sembolik bağlantısı oluşturulur; AGENTS.md skill tablosuna satır.
- Backend değişikliği `openapi.yaml`'ı değiştirirse ve mobilin kullandığı yüzey değiştiyse aynı
  PR'da `cd mobile && npm run api:generate` çalıştırılır (frontend'deki `pnpm api:generate` kuralının
  eşi). Aksi halde `mobile` işi "Stale mobile API types" ile kırılır. Bu kural AGENTS.md hard
  rules'a eklenir.
- `eas.json` profilleri: `development` (dev client, iç dağıtım), `preview` (TestFlight / Play
  internal), `production` (mağaza). `mobile-release` işi `production` profiliyle `--no-wait` derler;
  mağazaya gönderim (EAS Submit) ilk sürümlerde elle (KARAR-12).
- Yeni env değişkenleri `deploy/env/schema` ile ([env-core.md](./env-core.md)): `EXPO_ACCESS_TOKEN`
  (secret, opsiyonel), `JWT_REFRESH_TTL_MOBILE`, `AUTH_REVIEW_ACCOUNTS` (secret). `make env-check`
  geçmeli.

### 6.2 Sahibin yapacakları (ajanlar repo / mağaza ayarlarına dokunmaz)

1. **Apple Developer Program** (kurumsal hesap; D-U-N-S numarası gerekir): App ID (bundle id,
   KARAR-2), yetenekler: Sign in with Apple, Push Notifications, Associated Domains. APNs anahtarı
   (`.p8`) EAS'a yüklenir (Sign in with Apple `.p8`'inden ayrı tutulabilir).
2. **Google Play Console** (kurumsal hesap) ve **Firebase** projesi: FCM v1 servis hesabı JSON'u
   EAS'a yüklenir.
3. **Google Cloud OAuth**: iOS ve Android client id'leri (Android için Play App Signing SHA-1);
   client id'ler Platform → Entegrasyonlar → Google'a girilir (KARAR-4).
4. **Apple Services / bundle**: Platform → Entegrasyonlar → Apple'a bundle id native audience olarak
   girilir; mevcut `.p8` anahtarı native takas için de kullanılır.
5. **Expo hesabı / organizasyon**: `eas init` ile proje (projectId `mobile/app.json`'a commit'lenir),
   kişisel değil robot erişim token'ı → GitHub secret `EXPO_TOKEN`.
6. GitHub → Settings → Secrets and variables → Actions → Variables: `MOBILE_ENABLED=true`
   (iskelet merge edildikten sonra).
7. **Domain dosyaları**: Apple Team ID ve Android imza SHA-256'sı (AASA / assetlinks için) ajana
   iletilir.
8. **Mağaza kayıtları**: uygulama adı, açıklamalar, ekran görüntüleri, gizlilik politikası URL'si,
   App Privacy / Data safety formları, yaş derecelendirmesi, inceleme notu ve **inceleme hesabı**
   (`AUTH_REVIEW_ACCOUNTS` ile sabit kodlu e-posta).
9. Prod env: `EXPO_ACCESS_TOKEN` (enhanced push security açıksa), `AUTH_REVIEW_ACCOUNTS`,
   `JWT_REFRESH_TTL_MOBILE` → `.env.server` (`make prod-env-init` akışıyla).
10. KVKK: aydınlatma metni / gizlilik politikası güncellemesi ve yurt dışı aktarım işlemleri
    (hukuk danışmanı).

[OWNER_CHECKLIST.md](../OWNER_CHECKLIST.md) bu adımlarla, ilgili alt iş açıldığında güncellenir.

## 7. Alt işler

Sıra: önce uygulama olmadan test edilebilen backend işleri (B), sonra uygulama iskeleti (A1),
sonra özellikler. Boyut: S ≈ 1 gün, M ≈ 2-4 gün, L ≈ 1 hafta+.

| # | Başlık | Boyut | Bağımlılık |
|---|---|---|---|
| B1 | Mobil API geçidi ve istemci sürüm kontrolü | M | KARAR-1 |
| B2 | Oturum cihaz bilgisi, mobil refresh süresi, yeniden kullanım tespiti | M | KARAR-8 |
| B3 | id_token doğrulayıcı ve native Apple / Google girişi | L | B2, KARAR-4 |
| B4 | Apple token takası ve iptali | S | B3; TEC-1122 (silme kancası) |
| B5 | App Store / Play inceleme hesapları | S | — |
| B6 | Associated domains: AASA ve assetlinks | S | KARAR-2, sahip §6.2-7 |
| B7 | QR ile giriş: backend | L | — |
| B8 | QR ile giriş: web giriş paneli | M | B7 |
| B9 | Push cihaz kaydı (tablolar + uçlar) | M | B2 (oturum ailesi) |
| B10 | Expo push gönderimi, makbuzlar, temizlik | L | B9 |
| A1 | `mobile/` iskeleti, CI ve mobile-ci skill'i | L | B1 (adres), sahip §6.2-5/6 |
| A2 | Uygulama: giriş, oturum ve hesap değiştirme | L | A1, B1, B2, B3, B5 |
| A3 | Uygulama: bildirimler ve push | M | A2, B9, B10 |
| A4 | Uygulama: profiller ve kartlar | L | A2 |
| A5 | Uygulama: paylaş (QR) | S | A4 |
| A6 | Uygulama: gelen kutusu (lead'ler) | M | A2 |
| A7 | Uygulama: QR ile web girişi onayı | M | A2, B7, B6 |
| A8 | Uygulama: analitik özeti | S | A2 |
| A9 | Uygulama: oturumlar ve hesap silme | M | A2, TEC-1122 |
| A10 | Mağaza yayını (v1) | M | hepsi, sahip §6.2 |
| B11 | (v1.1) Native passkey girişi | M | B6, KARAR-7 |

Kabul ölçütleri her işin Linear açıklamasında (rapordaki yapılandırılmış liste). Ortak ölçütler:
yeni `/v1` uçları `openapi.yaml`'da, `openapi_routes.py` geçer, `pnpm api:generate` (ve `mobile/`
varsa `npm run api:generate`) çalıştırılır, migration'lar ComCode numaralandırmasıyla, Go entegrasyon
testleri yazılır, `docs/` ve `backend/docs/auth.md` güncellenir.

## 8. Kararlar (sahip)

Her maddede önerilen seçenek ilk sıradadır.

- **KARAR-1 — Mobilin API'ye erişimi**: (a) Next.js'te çerezsiz, yalnız Bearer ileten
  `/api/mobile/v1/*` geçidi; Go özel kalır. (b) `api.<domain>` ile Go'ya ayrı public dinleyici.
- **KARAR-2 — Bundle / paket kimliği ve uygulama adı**: (a) `software.technowide.comcode`
  (iOS ve Android aynı), görünen ad "ComCode". (b) Başka bir ters domain.
- **KARAR-3 — v1 giriş yöntemleri**: (a) E-posta kodu + parola (+TOTP) + Apple + Google; uygulamada
  kayıt yalnız e-posta kodu ve Apple/Google ile (parola ile kayıt formu yok). (b) Facebook native de
  eklensin. (c) Yalnız e-posta kodu + Apple.
- **KARAR-4 — Native client id'lerin yeri**: (a) Platform panelinde, `oauth_provider_settings`'de
  (Apple `.p8` gibi). (b) Env (`AUTH_APPLE_NATIVE_CLIENT_IDS`, `AUTH_GOOGLE_NATIVE_CLIENT_IDS`).
- **KARAR-5 — Apple private relay e-postası mevcut bir hesaba ait değilse**: (a) Web ile aynı: kayıt
  açıksa yeni hesap; kullanıcı sonra profilden bağlar. (b) Boilerplate'in link ticket akışı (iki
  istemcide birden).
- **KARAR-6 — QR ile giriş yönü**: (a) Yalnız web QR → telefon onaylar. (b) Ters yön de
  (web oturumu telefonu onaylar), step-up ile.
- **KARAR-7 — Passkey ile mobil giriş**: (a) v1.1. (b) v1'de.
- **KARAR-8 — Mobil oturum politikası**: (a) Mobil refresh 60 gün kayan + yeniden kullanım tespiti
  (aile iptali, 30 sn yarış toleransı). (b) Web ile aynı 7 gün, tespit yok. (c) 30 gün.
- **KARAR-9 — Push tercihi**: (a) Tek `push_enabled` (web + mobil). (b) Ayrı `mobile_push_enabled`.
- **KARAR-10 — Push cihaz uç yolu**: (a) `/v1/notifications/push-devices` (ComCode web push ile
  tutarlı). (b) `/v1/push-devices` (boilerplate ile aynı).
- **KARAR-11 — Mobil paket yöneticisi**: (a) npm (CI işi hazır). (b) pnpm (CI işi değişir).
- **KARAR-12 — Yayın modeli**: (a) Mağaza derlemesi CI'da, gönderim elle; OTA (`eas update`) kapalı.
  (b) EAS Submit otomatik. (c) OTA açık (JS düzeltmeleri mağazasız).
- **KARAR-13 — Uygulama içinde ödeme**: (a) v1'de satın alma yok; plan bilgisi salt okunur, satın
  alma bağlantısı da gösterilmez. (b) Webe yönlendirme bağlantısı (mağaza kurallarına göre bölgesel
  incelenmeli). (c) Uygulama içi satın alma (IAP) entegrasyonu.
- **KARAR-14 — v1 kapsamı**: (a) §5'teki liste. (b) Daha dar: giriş + gelen kutusu + bildirimler +
  paylaş. (c) Daha geniş: kart aktivasyonu (NFC) ve takas da v1'de.
- **KARAR-15 — QR onayında sunucu step-up'ı**: (a) Yalnız platform yetkili kullanıcılar için
  zorunlu, diğerlerinde cihaz yerel doğrulaması. (b) Herkes için zorunlu. (c) Hiç kimse için.
- **KARAR-16 — Mobil yayın hesap silmeye bağlı mı**: (a) Evet: mağaza sürümü TEC-1122'nin silme
  ucu hazır olmadan çıkmaz (App Store 5.1.1(v)). (b) İlk sürümde webdeki silme sayfasına bağlantı.
