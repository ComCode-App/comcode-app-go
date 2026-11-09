# Hesap silme (kullanıcının kendi hesabını silmesi)

Durum: **tasarım önerisi, sahibin onayını bekliyor** (TEC-1122). Bu belgede kod yok. Kararlar
onaylanınca §12'deki takip işleri açılır. `KARAR-n` ile işaretli maddeler sahibin vermesi gereken
kararlardır; her birinde önerilen seçenek ilk sıradadır.

Bu belge hukuki görüş değildir. §7'deki saklama süreleri mevzuattan derlenmiş başlangıç
değerleridir; yayına çıkmadan önce hukuk danışmanı onaylamalıdır (KARAR-9).

## 1. Özet: önerilen akış

1. Kullanıcı `/profile` sayfasındaki "Hesabımı sil" bölümünü açar. Ekran önce **ön kontrolü**
   gösterir: silmeyi engelleyen durumlar (§3) ve silinecek / saklanacak verilerin dökümü (§4, §7).
2. Kullanıcıya **verilerini indirmesi** önerilir (§9). Bu adım zorunlu değildir.
3. Kullanıcı kimliğini doğrular: step-up (parola, passkey veya TOTP). Parolası olmayan
   kullanıcı (OAuth ile kayıt olmuş, `users.password_set = false`) e-postasına gelen kodu girer
   (§8). Ayrıca onay kutusuna kullanıcı adını yazar.
4. Talep kaydedilir. Hesap **30 günlük bekleme süresine** girer (KARAR-5):
   - bütün oturumlar kapatılır;
   - herkese açık profil sayfaları, smart linkler ve kart yönlendirmeleri hemen yayından kalkar;
   - abonelik dönem sonunda bitecek şekilde iptal edilir, yenileme tahsilatı yapılmaz;
   - kullanıcıya iptal bağlantısı içeren bir e-posta gider.
5. Bekleme süresinde kullanıcı tekrar giriş yapabilir. Giriş yapınca yalnızca "Silmeyi iptal et"
   ve "Verilerimi indir" seçeneklerini görür. E-postadaki bağlantıyla da iptal edebilir.
6. Süre dolunca bakım kuyruğundaki günlük iş **temizlemeyi** yapar (§5.3): kişisel veriler silinir
   ya da anonimleştirilir, kayıtlı kartlar ödeme sağlayıcısından silinir, herkese açık yollar
   emekliye ayrılır. Yasal saklama yükümlülüğü olan finansal kayıtlar silinmez (§7).
7. `users` satırı fiziksel olarak **silinmez**. Anonim bir "mezar taşı" satırına dönüşür (§5.1).

## 2. Mevcut durum (kodda ne var, ne yok)

| Konu | Bugün | Dosya / kaynak |
|---|---|---|
| Kullanıcının kendi hesabını silmesi | Yok. `users.deleted_at` sütunu var ama hiçbir sorgu bu sütuna değer yazmıyor; bütün okumalar `deleted_at IS NULL` filtresi kullanıyor | `backend/migrations/000003_create_users.up.sql`, `backend/internal/database/queries/users.sql` |
| Kullanıcı pasifleştirme | Yalnızca platform tarafında var: `PATCH /v1/platform/users/{uuid}` ile `status = disabled` ya da bulk `disable`. Son süper admin pasifleştirilemez | `internal/platform/bulkengine/adapters/adapters.go` |
| Şirket hesabını müşterinin silmesi | Yok ("Not implemented"). Platform hesabı `suspended` yapabiliyor | [COMPANIES.md](../COMPANIES.md#platform-management) |
| Son sahip kuralı | Ertelenmiş (deferred) constraint trigger: hesap kalmaya devam ediyorsa en az bir `owner` olmak zorunda | `backend/migrations/000036_account_owner_guard.up.sql` |
| Profil / kart / smart link silme | Soft delete (`deleted_at`). Profil silinince yalnızca vCard nesneleri silinir; **avatar ve kapak S3'te kalır**, bucket anonim `GET`'e açık | `profiles/usecase/service.go` `Delete`, [DEVELOPMENT.md](../DEVELOPMENT.md) |
| Silinen smart link yolu | 30 gün karantinada kalır, sonra boşa çıkar. Kart yolu hiç boşa çıkmaz. Profil silinince kullanıcı adı rotası (`canonical_profile`) aktif kalır | `queries/routes.sql` `CheckPathExists`, [URL_ROUTING…](../URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md) |
| Kartı emekliye ayırma | Sipariş iptal / iade olunca ve platform destekte (`POST /v1/platform/cards/{uuid}/retire`) var. Tek yönlü bir işlem. `/c/{code}` "Kart devre dışı" gösterir | [COMMERCE_AND_ORDERS.md](../COMMERCE_AND_ORDERS.md#cancel-or-refund-tec-585) |
| Kayıtlı kart silme | `DELETE /v1/billing/cards/{uuid}`: önce sağlayıcıdan siler (iyzico / PayTR `DeleteSavedCard`), ardından satırı | `billing/usecase/cards.go` |
| Abonelik iptali | Dönem sonunda iptal (`DELETE /v1/billing/subscription`). Abonelikte iade otomasyonu yok | [BILLING.md](../BILLING.md#cancellation-and-changes) |
| Oturumlar | `RevokeAllRefresh`, `RevokeOtherSessions` var. `LoadPrincipal` her istekte kullanıcıyı ve oturumu yeniden okur | `auth/repository/postgres.go` |
| Step-up | Parola, passkey ve TOTP var. Grant süresi admin politikasıyla saat cinsinden belirlenir. Uygulama tarafında bugün yalnızca lead export bu korumayı kullanıyor | `internal/platform/stepup` |
| E-posta OTP | `otp_codes` tablosunda `password_reset` ve `email_verification` tipleri var. Hesap silme tipi yok | `auth/usecase/account.go` |
| Veri dışa aktarma | io-engine kaynak bazlı çalışıyor. Uygulama tarafında yalnızca `POST /v1/leads/export` var. "Bütün verilerim" dışa aktarması yok | `internal/modules/exports` |
| Lead KVKK saklama | Dokümanda "KVKK retention" geçiyor ama lead'ler için bir saklama/temizleme işi yok. Yalnızca log ve analitik temizleme işleri var | [PUBLIC_PROFILES.md](../PUBLIC_PROFILES.md#locked-leads-tec-941), `internal/queue` |

### 2.1 `DELETE FROM users` neden kullanılamaz

Yabancı anahtar kuralları bugün bir kullanıcıyı fiziksel olarak silmeyi hem imkânsız hem de
tehlikeli hâle getiriyor:

| FK | Kural | Sonuç |
|---|---|---|
| `orders.user_id` | `RESTRICT` | Sipariş vermiş bir kullanıcı (kişisel hesapta ya da şirket adına) silinemez |
| `nfc_cards.user_id` (NOT NULL, kartı aktive eden kişi) | `CASCADE` | Silme işlemi, kullanıcının **şirket adına** aktive ettiği kartları da siler |
| `profiles.user_id` (profili oluşturan kişi) | `CASCADE` | Silme işlemi, kullanıcının şirkette oluşturduğu profilleri de siler |
| `order_payment_sessions.user_id`, `payment_checkout_sessions.user_id`, `payment_card_sessions.user_id` | `CASCADE` | Saklanması zorunlu ödeme kayıtları kaybolur |
| `accounts.personal_user_id` | `CASCADE` | Kişisel hesap silinir, ama `orders.account_id` ve `nfc_cards.account_id` `RESTRICT` olduğu için işlem yine durur |
| `account_members.user_id` | `CASCADE` | Kullanıcı bir şirketin son sahibiyse 000036 trigger'ı işlemi reddeder. Bu kural doğru çalışıyor |

**Karar önerisi:** Kullanıcı satırı silinmez, anonimleştirilir (§5.1). Ek güvenlik olarak
yukarıdaki `CASCADE` kuralları `SET NULL` yapılır (§12, iş 1). Böylece birisi ileride elle
`DELETE` çalıştırsa bile şirket verisi ve finansal kayıtlar kaybolmaz.

## 3. Engelleyici durumlar ve son sahip kuralı

Silme talebi aşağıdaki durumlarda reddedilir. Ön kontrol ekranı (`GET …/deletion`) her durumu
nasıl çözüleceğiyle birlikte listeler:

| Durum | Hata kodu (öneri) | Çözüm |
|---|---|---|
| Kullanıcı, başka üyeleri olan bir şirket hesabının **tek sahibi** | `409 LAST_OWNER` (`details` içinde şirketler) | Başka bir üyeyi sahip yapmak (mevcut `PATCH /v1/account/members/{user_uuid}`) |
| Kullanıcı tek üyeli bir şirket hesabının sahibi | `409 SOLE_COMPANY` | KARAR-1'e göre |
| Kullanıcı son süper admin | `409 LAST_SUPER_ADMIN` | Başka bir süper admin atamak (boilerplate'teki kural) |
| Kişisel hesapta ödemesi alınmış ama henüz teslim edilmemiş bir sipariş var (`paid` … `shipped`) | `409 OPEN_ORDERS` | Siparişin teslim edilmesini beklemek ya da destekten iptal / iade istemek |
| Abonelikte sonuçlanmamış bir tahsilat var (`processing` / `unknown` deneme) | `409 PAYMENT_UNRESOLVED` | Operatörün mutabakat yapması ([BILLING.md](../BILLING.md#charge-safety-and-reconciliation)) |
| Kullanıcı başka biri adına giriş yapmış (impersonation) | `403` | — |
| Hesapta yasal saklama kaydı var (§10) | `409 LEGAL_HOLD` | Destek |

Ödenmemiş (`pending` / `unpaid`) siparişler engel oluşturmaz. Talep anında otomatik olarak iptal
edilir.

### KARAR-1: Şirket hesabının son sahibi

| Seçenek | Açıklama | Artı | Eksi |
|---|---|---|---|
| **A. Engelle ve devri göster (önerilen)** | Başka üyesi olan bir şirketin tek sahibi silme yapamaz. Ekran aynı yerde "sahip yap" işlemini sunar (mevcut rol değiştirme uç noktası). Tek üyeli şirket için de engellenir ve destek talebi önerilir | Yeni bir veri modeli gerekmez. Çalışanların kartları ve profilleri yerinde kalır. Kimse istemeden sahip olmaz | Tek üyeli şirketi olan kullanıcı destek beklemek zorunda kalır |
| B. Zorunlu devir | Silme sihirbazında bir halef seçilir (yalnızca mevcut `admin`'ler arasından) ve devir talep işlemiyle birlikte yapılır | Tek adımda biter | Halefin onayı alınmaz, ödeme sorumluluğu ona geçer. Bekleme süresinde iptal edilirse devri geri almak gerekir |
| C. Şirketi de sil | Kullanıcının sahip olduğu şirket hesapları aynı akışla kapatılır | Hiç engel kalmaz | Çalışanların kartları ve profilleri, şirket aboneliği ve lead'ler birlikte gider. Bu iş TEC-1122'nin kapsamını aşar. Şirket kapatma ayrı bir tasarım gerektirir (§12, iş 10) |

Öneri A. Tek üyeli şirketler için ikinci aşamada "şirketi kapat" akışı (iş 10) açılır. O akış
hazır olunca tek üyeli şirket bu sihirbazdan kapatılabilir.

### Şirket üyeliği (kullanıcı sahip değilse)

- Bekleme süresinde üyelikler yerinde kalır, ama kullanıcı oturum açamadığı için hiçbir işlem
  yapamaz. Üye listesinde "silinme bekliyor" etiketi görünür.
- Temizlik anında üyelikler silinir (`account_members`). `company_member_assignments`,
  `account_member_scopes` ve `company_member_profiles` bağları cascade ile kalkar.
- **Çalışan profili** (şirket hesabına ait, kullanıcının adını ve fotoğrafını taşıyan profil):
  KARAR-2.

### KARAR-2: Şirketteki çalışan profili

| Seçenek | Açıklama |
|---|---|
| **A. Şirkette kalır, yayından kalkar (önerilen)** | Profil şirketin verisidir; veri sorumlusu şirkettir. Temizlik anında profil `enabled = false` (rota `locked`) yapılır ve sahiplere / adminlere "çalışan profili yayından kaldırıldı" bildirimi gider. Profili silmek ya da başka birine atamak şirketin kararıdır |
| B. Hiç dokunulmaz | Şirket kartları çalışmaya devam eder, ama ayrılan kişinin bilgileri yayında kalır |
| C. Silinir | Şirketin kartları boş profile düşer. Şirket verisine müdahale edilmiş olur |

Şirket kartlarında `nfc_cards.user_id` (kartı aktive eden kişi) mezar taşı satırını göstermeye
devam eder. Bu sütun sahipliği değil, işlemi yapan kişiyi gösterir.

## 4. Kişisel hesap verisi: profiller, kartlar, rotalar, analitik, lead'ler

Kişisel hesabın kaynakları bekleme süresinde **yayından kaldırılır**, temizlik anında
**silinir veya anonimleştirilir**.

| Veri | Talep anında (bekleme başlar) | Temizlik anında |
|---|---|---|
| Profiller (`profiles` ve çevirileri, adresleri, iletişim bilgileri, sosyal linkleri, `profile_bank_accounts`) | Herkese açık sayfa "Bu profil artık kullanılmıyor" sayfası döner (KARAR-3) | Alt tablolar hard delete edilir. Profil satırında kişisel alanlar boşaltılır, `deleted_at` yazılır. Satırın iskeleti (id, uuid, account_id) FK bütünlüğü için kalır. `profiles/{uuid}/*` nesneleri (avatar, kapak, vCard, firma medyası) S3'ten silinir |
| Kullanıcı adı rotası (`canonical_profile`) | Çözümlenmez | `retired` ve `deleted_at` yazılır. Karantina süresi KARAR-3'e göre |
| Smart linkler (`custom_smart_link`) | Çözümlenmez | `retired` ve `deleted_at` yazılır, `presentation_config` temizlenir. Karantina süresi KARAR-3'e göre |
| Fiziksel NFC kartları | `/c/{code}` "Kart geçici olarak kullanılamıyor" gösterir | KARAR-4 |
| Kart siparişleri, ödemeler | Dokunulmaz | Saklanır (§7) |
| Lead ve kartvizit takası (`leads`) | Form kapanır (rota çözülmediği için yeni lead gelmez) | Hard delete. `leads.exchange_copy` bildirim satırları da silinir |
| Banka hesapları (`user_bank_accounts`) | — | Hard delete (IBAN kişisel veridir) |
| Arka plan tasarımları (`backgrounds`) | — | Satırlar ve `backgrounds/{uuid}/*` nesneleri silinir |
| Webhook uç noktaları ve teslimatları | Uç noktalar duraklatılır (`is_active = false`) | Hard delete. Teslimat payload'larında lead bilgisi olabilir |
| Profil doğrulama talepleri | — | Hard delete |
| Analitik | Yeni olay oluşmaz (rotalar kapalı) | KARAR-6 |
| Cmd+K (Meilisearch) | Profil, kart ve rota kayıtları `EnqueueDelete` ile kaldırılır | Kullanıcı kaydı da kaldırılır |
| Rota önbelleği (Redis) | Hesabın bütün rotaları için `Invalidate*` çağrılır | Aynı işlem tekrarlanır |

### KARAR-3: Herkese açık URL'ler (kullanıcı adı ve smart link)

Kullanıcı adları ve smart linkler kartvizitlere, QR kodlarına ve e-posta imzalarına basılı olabilir.
Silinen birinin adresini başka biri alırsa o kişi gibi davranabilir (kimliğe bürünme riski).

| Seçenek | Kullanıcı adı | Smart link |
|---|---|---|
| **A. 12 ay karantina, sonra boşa çıkar (önerilen)** | 12 ay | 12 ay (normal silmede bu süre 30 gün) |
| B. Kalıcı rezervasyon | Hiç boşa çıkmaz | Hiç boşa çıkmaz |
| C. Mevcut kural | Bugün süresiz tutuluyor (rota aktif kalıyor) | 30 gün |

Öneri A. Bu süre normal silmedeki 30 günden uzun, çünkü hesap silmede adresin sahibi geri
dönmeyecek ve yönlendirme yapılacak bir hedef de yok. Platform ekibi süreyi mevcut "karantinayı
kaldır" işlemiyle erken bitirebilir (TEC-950). Bunun için `CheckPathExists` sorgusuna
`canonical_profile` için de karantina kuralı eklenir (iş 4).

Ziyaretçiye gösterilen yanıt: Bekleme süresinde ve sonrasında `404` yerine **`410 Gone`** ve
nötr bir "Bu profil artık kullanılmıyor" sayfası döner. Sayfada ad, fotoğraf ya da başka bir
kişisel veri bulunmaz.

### KARAR-4: Satılmış fiziksel NFC kartları

Kart üzerindeki `/c/{code}` yolu basılıdır ve değiştirilemez. Kural hâlihazırda belli: kart yolu
hiçbir zaman başkasına verilmez.

| Seçenek | Açıklama | Artı | Eksi |
|---|---|---|---|
| **A. Emekliye ayır (önerilen)** | Sipariş iptalindeki işlem uygulanır: rota `retired` olur, kullanılmamış aktivasyon kodu temizlenir, kart soft delete edilir, `custom_data`, `passcode` ve `name` alanları boşaltılır. `/c/{code}` "Kart devre dışı" gösterir | Mevcut kodla yapılır (`RetireOrderCardRoutes` / platform retire). Geri dönüşü olmadığı için güvenlidir | Kartı başkasına devretmek isteyen kullanıcı devredemez |
| B. Sahipsiz hâle getir, destekten yeniden aktive edilebilsin | Rota `reserved` durumuna döner, destek yeni bir aktivasyon kodu üretir | Kart ikinci el kullanılabilir | Yeni bir durum ve yeni bir destek işlemi gerekir. Kodun ele geçirilme riski oluşur |
| C. Silmeden önce devret | Kart başka bir hesaba taşınır | En iyi kullanıcı deneyimi | Hesaplar arası devir bugün yok ("Out of scope", TENANCY_AND_ACCESS) |

Öneri A. Silme ekranı "Kartlarınız kalıcı olarak devre dışı kalacak" uyarısını kart sayısıyla
birlikte gösterir. C seçeneği hesaplar arası devir işi gelince (iş 11) silme sihirbazına bir adım
olarak eklenebilir.

### KARAR-6: Analitik

Ham olaylarda (`events`) kullanıcının değil, **ziyaretçilerin** verisi bulunur: tuzlanmış IP özeti,
konum ve cihaz. Olaylar `subject_id` üzerinden profile, karta ve rotaya bağlanır.

| Seçenek | Açıklama |
|---|---|
| **A. Hesabın konularına ait ham olayları ve özet tablolarını sil (önerilen)** | Temizlik işi `subject_type`/`subject_id` ile `events` bölümlerinden (partition), `event_aggregates_hourly/daily` ve `analytics_visitors_hourly` tablolarından ilgili satırları siler. KVKK açısından en temiz seçenektir. Platform özetlerindeki geçmiş toplamlar azalır |
| B. Saklama süresine bırak | Ham olaylar 730 gün, özetler 1095 gün sonra kendiliğinden temizlenir. Konu satırları anonimleştirildiği için olaylar kimseyle eşleşmez | Kod gerekmez, platform toplamları korunur |

## 5. Durum makinesi ve veri modeli

```
active ──talep──▶ pending_deletion ──(30 gün)──▶ purging ──▶ deleted (mezar taşı)
   ▲                  │
   └──────iptal───────┘        (legal hold: pending_deletion'da bekletir)
```

### 5.1 Şema önerisi (migration numaraları iş açılırken verilir)

- `users`: `deletion_requested_at`, `deletion_scheduled_for`, `anonymized_at`. `deleted_at`
  zaten var. `status` alanı `active | disabled | pending` olarak kalır; silme durumu ayrı
  sütunlarda tutulur, böylece iptal edilince eski durum korunur.
- `accounts.status`: `active | suspended` değerlerine `closed` eklenir (temizlenen kişisel hesap).
- Yeni tablo `account_deletion_requests`: `uuid`, `user_id` (mezar taşını gösterir),
  `email_hash` (SHA-256; yedekten geri yüklemede tekrar silme için), `requested_by` (`self` |
  `platform`), `actor_user_id`, `reason`, `requested_at`, `scheduled_for`, `canceled_at`,
  `completed_at`, `legal_hold` ve `hold_reason`, `steps` (JSONB: hangi adım ne zaman bitti,
  sağlayıcı sonuçları). Denetim izi bu tablodur.
- `otp_codes.type` için yeni değer: `account_deletion`.
- FK düzeltmeleri (§2.1): `nfc_cards.user_id`, `profiles.user_id`,
  `order_payment_sessions.user_id`, `payment_checkout_sessions.user_id` ve
  `payment_card_sessions.user_id` `ON DELETE SET NULL` olur (gerekenlerde `DROP NOT NULL`).
  `legacy_sync_users.profile_id` için açık bir kural tanımlanır.

**Mezar taşı satırı:** `email = deleted+{uuid}@deleted.invalid`, `name = "Silinmiş"`,
`surname = "Kullanıcı"`, `password_hash` rastgele değer, `status = disabled`, `deleted_at` ve
`anonymized_at` dolu. `uq_users_email_active` indeksi `deleted_at IS NULL` koşuluna sahip
olduğu için aynı e-posta adresi temizlikten sonra yeniden kayıt olabilir. Bekleme süresinde adres
dolu kalır, bu sayede kullanıcı iptal edebilir.

### 5.2 Bekleme süresi (`pending_deletion`)

- `LoadPrincipal` silme bekleyen kullanıcıyı tanır ve **kısıtlı mod** uygular. Yalnızca
  `GET /v1/auth/me`, silme durumu ve iptal uç noktaları, kendi export'unu indirme ve logout
  çalışır. Diğer bütün uç noktalar `403 ACCOUNT_PENDING_DELETION` döner. Frontend bu hatayı
  aldığında "Hesabınız {tarih} tarihinde silinecek" ekranını gösterir.
- Giriş yollarının hepsi (parola, OAuth, passkey, refresh) açık kalır, ama oturum yine kısıtlı
  modda açılır. Alternatif (boilerplate'teki gibi girişi tamamen kapatıp yalnızca e-postayla iptal
  etmek) KARAR-5'te.
- Kişisel hesabın herkese açık yüzeyleri kapalıdır (§4). Kullanıcının üye olduğu şirketler
  etkilenmez.
- Abonelik `cancel_at_period_end = true` yapılır, böylece yenileme sweep'i tahsilat yapmaz. İptal
  edilirse ve dönem henüz bitmemişse abonelik `resume` ile geri gelir.

### KARAR-5: Bekleme süresi ve geri alma

| Seçenek | Açıklama |
|---|---|
| **A. 30 gün, giriş yapılınca kısıtlı mod ve e-posta bağlantısı (önerilen)** | Yukarıda anlatılan akış |
| B. 14 gün | KVKK'nın 30 günlük yanıt süresine (md. 13) daha rahat sığar. Yanlışlıkla silmeyi düzeltmek için daha az zaman kalır |
| C. Bekleme yok, hemen temizlik | Geri alınamaz. Kart sahipleri için riskli |
| D. Giriş kapalı, yalnızca e-postayla iptal | Boilerplate'teki `ACCOUNT_DEACTIVATED` davranışı. Erişimini kaybetmiş kullanıcı iptal edemez |

Not: Kullanıcının kendi başlattığı silme işlemi KVKK md. 11/13 kapsamında resmî bir başvuru
sayılmayabilir. Resmî başvurularda destek, bekleme süresini atlayıp temizliği hemen başlatabilir
(§10).

### 5.3 Temizlik işi

- Asynq zamanlayıcıya `app:accounts:deletion_sweep` görevi eklenir: maintenance kuyruğu, günde
  bir kez. `scheduled_for <= now`, `legal_hold = false` ve `completed_at IS NULL` koşullarını
  sağlayan talepleri seçer.
- Her adım **idempotent** olmalı ve `steps` alanına kaydedilmelidir. Yarıda kalan iş bir sonraki
  çalışmada kaldığı yerden devam eder. Önerilen sıra:
  1. Ödeme sağlayıcısı: kayıtlı kartları `DeleteSavedCard` ile sil, `payment_provider_customers`
     satırlarını sil. Kart sağlayıcıda zaten yoksa işlem başarılı sayılır. Diğer hatalar tekrar
     denenir; 3 başarısız denemeden sonra adım atlanır, loglanır ve platforma bildirilir.
  2. Abonelik: `canceled`, `ends_at = now` (satır kayıt olarak kalır).
  3. Herkese açık yüzeyler: rotalar ve kartlar KARAR-3 ve KARAR-4'e göre işlenir, önbellek
     temizlenir.
  4. Hesap içeriği: §4'teki tabloya göre profiller, lead'ler, banka hesapları, arka planlar ve
     webhook'lar.
  5. S3 nesneleri: `profiles/{uuid}/`, `backgrounds/{uuid}/`, kullanıcının `exports/{job}/` ve
     `imports/{job}/` dosyaları (`export_jobs` / `import_jobs` satırları da silinir).
  6. Bildirimler: kullanıcının `notifications`, `notification_preferences` ve
     `push_subscriptions` satırları silinir. Başka satırlarda kalan `recipient` e-postaları
     maskelenir.
  7. Kimlik: `refresh_tokens`, `webauthn_credentials`, `oauth_accounts` (saklanan Apple refresh
     token'ı varsa önce iptal edilir, boilerplate `revokeAppleTokens`), `user_totp`, `otp_codes`,
     `user_roles` silinir, step-up grant iptal edilir.
  8. Şirket üyelikleri silinir, KARAR-2 uygulanır.
  9. Kişisel hesap: `status = closed`, ad ve slug anonimleştirilir (`deleted-{uuid8}`).
  10. Kullanıcı satırı mezar taşına dönüştürülür. `activity_events` içinde bu kullanıcının
      olaylarının `ip_address` ve `user_agent` alanları boşaltılır.
  11. Meilisearch'ten kullanıcı kaydı silinir, "silme tamamlandı" e-postası **anonimleştirmeden
      önce** alınan adrese gönderilir.
- Transaction sınırları: Ağ çağrıları (sağlayıcı, S3) transaction dışında yapılır. Veritabanı
  adımları kısa transaction'larla çalışır. Zamanlayıcıya bir de kilit (advisory lock ya da
  Asynq unique) konur, böylece iki worker aynı talebi aynı anda işlemez.

## 6. Abonelik, iade, kayıtlı kartlar

- **Abonelik:** Talep anında dönem sonunda bitecek şekilde iptal edilir, temizlik anında hemen
  biter. Manuel (platform) abonelikler de temizlikte `canceled` yapılır.
- **Kayıtlı kartlar:** Sağlayıcıdan silme temizlik anında yapılır (§5.3, adım 1). Bekleme
  süresinde kartlar kullanılamaz (kısıtlı mod). Alternatif olarak talep anında silinebilir. Bu
  KVKK açısından daha iyidir, ama kullanıcı iptal ederse kartı yeniden eklemesi gerekir (KARAR-8).
- **Şirket aboneliği:** Kişisel hesabın silinmesinden etkilenmez. Ödeme ve kartlar hesaba aittir.

### KARAR-7: İade politikası

| Seçenek | Açıklama |
|---|---|
| **A. Kalan süre için iade yok (önerilen)** | Mevcut "plan düşürmede iade yok" kuralıyla tutarlıdır. Erişim dönem sonuna kadar sürer, ancak hesap kısıtlı modda olduğu için fiilen kullanılamaz. İstisnalar destek aracılığıyla sağlayıcı panelinden elle yapılır (abonelik iadesi için bugün otomasyon yok) |
| B. Kıst (prorata) iade | Kalan gün oranında iade edilir. Abonelik iadesi otomasyonu (yeni bir iş) gerekir |
| C. İlk satın alımdan sonraki 14 gün içinde tam iade | Mesafeli satış cayma hakkına benzer. Dijital hizmette anında ifa istisnası hukukçuyla netleştirilmeli |

Silme ekranı seçilen politikayı açıkça yazar ("Kalan {n} gün için iade yapılmaz").

## 7. Saklama, anonimleştirme, denetim izi (KVKK)

### 7.1 Ne saklanır, ne silinir

| Veri | İşlem | Süre (başlangıç değeri, hukuk onayı gerekli) | Dayanak |
|---|---|---|---|
| Sipariş, sipariş kalemleri, fatura adresi, ödeme oturumları, iadeler, `subscription_payment_attempts`, makbuzlar | **Saklanır**. Hesap kapandığı için yalnızca platform görebilir | 10 yıl | TTK md. 82, VUK md. 253 (5 yıl), Mesafeli Sözleşmeler Yönetmeliği |
| Sipariş teslimat adresi ve telefon | Saklanır (teslimatın kanıtı) | 10 yıl ya da teslimattan 2 yıl sonra maskelenir (KARAR-9) | — |
| Sipariş tasarım dosyaları (`orders/{n}/assets/`) | Teslim edilmiş siparişlerde silinir | Teslimattan 90 gün sonra ya da temizlik anında (KARAR-9) | Amaç sona erdi |
| Silme talebi kaydı (`account_deletion_requests`) | Saklanır, minimum veriyle (e-posta yalnızca özet olarak) | Finansal kayıtlarla aynı süre | Hesap verebilirlik |
| Güvenlik ve işlem günlüğü (`activity_events`, `app_logs`) | Kullanıcı mezar taşına bağlı kalır, IP ve UA alanları temizlenir | 2 yıl, sonra `log_purge_rules` | 5651 sayılı Kanun (trafik bilgisi 1–2 yıl) |
| Ticari ileti onayı (varsa) | Saklanır | Onay geçersiz olduktan sonra 3 yıl | Ticari İletişim ve Ticari Elektronik İletiler Hakkında Yönetmelik |
| Profil, lead, banka hesabı, medya, kimlik bilgileri, bildirimler | **Silinir** | Temizlik anında | KVKK md. 7 |
| Kullanıcı satırı | **Anonimleştirilir** | Temizlik anında | FK bütünlüğü |
| Analitik | KARAR-6 | — | — |
| Veritabanı ve S3 yedekleri | Rotasyonla kendiliğinden düşer | Yedek saklama süresi (KARAR-9) | Yedekten geri yükleme yapılırsa `account_deletion_requests` tablosuna göre silmeler yeniden uygulanır (runbook) |

Not: KVKK Silme, Yok Etme veya Anonim Hale Getirme Yönetmeliği periyodik imhanın en fazla 6 ayda
bir yapılmasını öngörür. Günlük temizlik işi bu şartı karşılar. Süresi dolan finansal kayıtlar
için ayrı bir yıllık iş gerekir (iş 9).

### 7.2 Denetim izi

- `activity_events` kayıtları: `account.deletion_requested`, `account.deletion_canceled`,
  `account.deletion_completed`, `account.deletion_hold_set`, `account.deletion_executed_now`
  (aktör: kullanıcı ya da platform yöneticisi).
- `account_deletion_requests.steps` alanı hangi adımın ne zaman yapıldığını ve sağlayıcıdan gelen
  sonucu tutar. Kişisel veri içermez.

## 8. Güvenlik

| Kontrol | Öneri |
|---|---|
| Yeniden kimlik doğrulama | **Taze** bir step-up gerekir (parola, passkey ya da TOTP; admin politikasındaki yöntemler). Grant süresi saatlerle ölçüldüğü için silmede ayrıca `verified_at` değerinin en fazla 5 dakika önce olması aranır. Step-up yöntemi olmayan kullanıcı (`password_set = false`, passkey ve TOTP yok) e-posta kodu kullanır: 6 haneli kod, 10 dakika geçerli, 5 deneme hakkı, tek kullanımlık (boilerplate `email_code`) |
| Niyet onayı | Kullanıcı adını (ya da `SİL` kelimesini) yazması istenir |
| E-posta | Talep e-postası (iptal bağlantısı ve tarih), T-7 gün hatırlatması, iptal edildi ve tamamlandı e-postaları. Hepsi güvenlik e-postasıdır, bildirim tercihleri yok sayılır |
| İptal bağlantısı | `{request uuid}.{exp}.{hmac}` biçiminde bir token. Exchange silme token'ındaki yöntem kullanılır. Bağlantı açılınca yalnızca onay sayfası gösterilir, iptal POST ile yapılır (posta tarayıcıları bağlantıyı tüketmesin) |
| Oturumlar | Talep anında `RevokeAllRefresh` çağrılır ve step-up grant iptal edilir. Centrifugo bağlantıları token süresi bitince düşer. BFF `isSessionInvalidatePath` listesine silme uç noktasını ekler (boilerplate 60ae434) |
| Hız sınırı | Kullanıcı başına: kod isteği 5 / 15 dk, talep 10 / 15 dk (boilerplate değerleri) |
| Impersonation | Silme talebi ve iptal reddedilir |
| Son süper admin | Silemez (boilerplate kuralı, `CountUsersWithRole`) |
| Ele geçirilmiş hesap | Saldırgan hesabı silmeye kalkarsa 30 günlük bekleme ve e-posta bağlantısı sahibin iptal etmesini sağlar. Saldırgan e-posta adresini de değiştiremez (ComCode'da e-posta değiştirme uç noktası yok) |

## 9. Silmeden önce veri dışa aktarma

io-engine yalnızca kaynak listeleri için tasarlanmış (CSV/XLSX/PDF/JSON, async iş,
`/v1/exports/{uuid}/download`). Uygulama tarafında yalnızca lead export var.

| Seçenek | Açıklama |
|---|---|
| **A. Yeni io-engine kaynağı `app.account_data`, JSON (önerilen)** | Tek bir JSON dosyasında: kullanıcı, profiller (çevirileriyle), kartlar, rotalar, lead'ler, banka hesapları, siparişler, ödeme geçmişi, abonelik ve analitik özeti. Mevcut export işi, bildirimi ve indirme altyapısı kullanılır. Step-up gerekir. Kısıtlı modda da çalışır |
| B. Mevcut export'ları göstermek | Lead export'u ve makbuz PDF'leri yeterli sayılır. Profil ve kart verisi eksik kalır |
| C. Medya dahil ZIP | Avatar ve kapak görselleri de pakete girer. io-engine'in tek dosya modelini aşar, sonraya bırakılabilir |

Lead export aylık kotası (`leads.export.monthly_max`) bu dışa aktarmaya uygulanmaz. KVKK md. 11
kapsamında kişi kendi verisini alma hakkına sahiptir. Lead'ler kullanıcının topladığı üçüncü
kişi verisidir; bu nedenle pakete hangi alanların girdiği KARAR-10'da sorulur.

## 10. Platform (admin) tarafı ve destek akışları

| İşlem | Uç nokta (öneri) | Yetki |
|---|---|---|
| Kullanıcı detayında silme durumu | `GET /v1/platform/users/{uuid}` yanıtına `deletion` alanı | `platform.users.read` |
| Kullanıcı adına talep başlatma (KVKK başvurusu, e-posta ya da dilekçe) | `POST /v1/platform/users/{uuid}/deletion` `{reason, execute_now?}` | Yeni `platform.users.delete` yetkisi + step-up |
| Talebi iptal etme | `DELETE /v1/platform/users/{uuid}/deletion` | Aynı |
| Yasal saklama (uyuşmazlık, chargeback, resmî talep) | `POST …/deletion/hold` `{reason}` / `DELETE …/deletion/hold` | Aynı, süper admin |
| Bekleme süresini atlama | `execute_now: true` | Süper admin + step-up |

- Platformun açtığı talepte ön kontroller (§3) aynen çalışır. Son sahip olunan şirketler için
  destek önce mevcut platform üye uç noktalarıyla devri yapar.
- Yeni yetki slug'ı kurallara uygun eklenir: migration, `internal/platform/rbac` ve
  `frontend/src/config/permissions.ts`.
- Destek ekranları (`/platform/support/cards`, `/routes`) silinmiş hesapların kartlarını ve
  rotalarını "hesap silindi" etiketiyle göstermeye devam eder.
- Kimlik doğrulama: E-postayla gelen KVKK başvurusunda destek, başvuranın hesabın e-posta
  adresine sahip olduğunu doğrulamalıdır (hesabın e-postasından yazması ya da e-posta kodu).
  Bu süreç `/kvkk` sayfasında ve aydınlatma metninde anlatılır (iş 8).
- Bugünkü platform "disable" işlemi (pasifleştirme) değişmez. Silmeden ayrı bir işlemdir.

## 11. Boilerplate'ten ne alınır

Boilerplate'te tam silme yok. Yalnızca **pasifleştirme** var: veri silinmez, `deactivated_at`
yazılır, admin hesabı tekrar açabilir. ComCode'un KVKK yükümlülüğü gerçek silmeyi gerektirir.
Bu nedenle mantık elle port edilir, birebir kopyalanmaz ([BOILERPLATE_SYNC.md](../BOILERPLATE_SYNC.md), TEC-1114).

| Commit | Parça | Karar |
|---|---|---|
| `e72f52a` | `deactivation.go`: onay için "step-up **veya** e-posta kodu" mantığı, son süper admin kontrolü, `RevokeAllRefresh` ve grant iptali, güvenlik e-postası, impersonation reddi, hız sınırları | **Al**, silme talebi uç noktasına uyarla |
| `e72f52a` | `revokeAppleTokens` | **Al**. ComCode'da Apple, Google ve Facebook OAuth var ve `oauth_accounts.refresh_token_enc` sütunu mevcut |
| `e72f52a` | Bütün giriş yollarının `ACCOUNT_DEACTIVATED` dönmesi | **Uyarla**. ComCode'da giriş kapanmıyor, onun yerine kısıtlı mod ve `ACCOUNT_PENDING_DELETION` var (KARAR-5) |
| `e72f52a` / `1eac3a9` | `otp_codes` tipi `account_deactivation` | **Uyarla** → `account_deletion` |
| `e72f52a` | E-posta koduyla giriş, native Apple/Google, OAuth link ticket | **Alma** (bu işin kapsamında değil, TEC-1114) |
| `60ae434` | `delete-account-dialog.tsx` (kod gönder → kodu gir → logout akışı), BFF `isSessionInvalidatePath`, tr/en metinler | **Al**. Dialogun önüne ön kontrol ve export adımı eklenir, step-up seçeneği eklenir |
| `60ae434` | Giriş sayfasındaki e-posta kodu sekmesi | **Alma** |

## 12. Uygulama planı (takip işleri)

Boyutlar: S (≤1 gün), M (2–4 gün), L (1–2 hafta).

| # | İş | Boyut | Bağımlılık |
|---|---|---|---|
| 1 | **Şema ve FK güvenliği:** §5.1'deki sütunlar, `accounts.status = closed`, `account_deletion_requests`, OTP tipi, `CASCADE` → `SET NULL` düzeltmeleri. `DELETE FROM users` kullanılmasını yasaklayan bir test | M | — |
| 2 | **Talep API'si ve kısıtlı mod:** `GET/POST/DELETE /v1/auth/account/deletion`, `POST …/deletion/code`, `POST /v1/public/account-deletion/cancel`, ön kontroller (§3), taze step-up / e-posta kodu, oturum iptali, `LoadPrincipal` kısıtlı modu, hız sınırları, bildirim şablonları (tr/en), OpenAPI, `pnpm api:generate` | M | 1 |
| 3 | **Talep anında yayından kaldırma:** rota çözümleyici (410 sayfası), kart "geçici olarak kullanılamıyor" ekranı, lead formunun kapanması, webhook'ların duraklatılması, aboneliğin dönem sonunda iptali, arama indeksinden çıkarma. İptal edilince bunların hepsi geri alınır | M | 2 |
| 4 | **Temizlik işi:** zamanlayıcı, adım kaydı, sağlayıcıdan kart silme, S3 temizliği, anonimleştirme, rota ve kart emekliliği (KARAR-3/4), `CheckPathExists` karantinası, analitik (KARAR-6). Postgres entegrasyon testleri ve sahte sağlayıcı | L | 1, 3 |
| 5 | **Frontend:** `/profile` sayfasında "Hesabımı sil" bölümü, ön kontrol ve engeller, export çağrısı, onay dialogu (step-up veya kod), kısıtlı mod ekranı ve iptal, herkese açık "profil kullanılmıyor" sayfası, i18n | M | 2 |
| 6 | **Veri dışa aktarma:** io-engine `app.account_data` (JSON) | M | — |
| 7 | **Platform:** kullanıcı detayında silme bölümü, başlatma / iptal / hold / hemen çalıştırma uç noktaları, `platform.users.delete` yetkisi, aktivite kayıtları | M | 2, 4 |
| 8 | **KVKK metinleri ve süreç:** aydınlatma metni, `/kvkk` başvuru açıklaması, saklama tablosu (§7.1), yedekten geri yükleme runbook'u | S (+ hukuk) | KARAR-9 |
| 9 | **Saklama süresi dolan kayıtlar:** finansal kayıtlar ve günlükler için yıllık temizlik işi, sipariş tasarım dosyalarının temizliği | S | 8 |
| 10 | **Şirket hesabını kapatma** (ayrı tasarım): çalışanların kartları, şirket aboneliği, üyelerin bilgilendirilmesi | L | 4 |
| 11 | **Hesaplar arası kart ve profil devri** (KARAR-4 C seçeneği için) | M | — |
| 12 | **Mevcut boşluk:** normal profil silmede avatar ve kapak nesneleri S3'te kalıyor; lead'ler için KVKK saklama işi yok (bu tasarımdan bağımsız) | S | — |

Önerilen sıra: 1 → 2 → 3 ve 6 paralel → 5 → 4 → 7 → 8 ve 9. Silme özelliği kullanıcıya iş 4
bitmeden açılmamalı. İş 4 tamamlanmadan açılırsa talepler bekleme süresinin sonunda takılı
kalır. İş 2'ye bir özellik bayrağı eklenebilir.

## 13. Sahibin vermesi gereken kararlar

- **KARAR-1** Son sahip: engelle ve devri göster (önerilen) / zorunlu devir / şirketi de sil.
- **KARAR-2** Şirketteki çalışan profili: şirkette kalır ve yayından kalkar (önerilen) / hiç dokunulmaz / silinir.
- **KARAR-3** Kullanıcı adı ve smart link karantinası: 12 ay (önerilen) / kalıcı rezervasyon / mevcut kural.
- **KARAR-4** Satılmış NFC kartları: emekliye ayır (önerilen) / sahipsiz bırakıp yeniden aktivasyona aç / önce devret.
- **KARAR-5** Bekleme süresi: 30 gün, kısıtlı giriş ve e-posta bağlantısı (önerilen) / 14 gün / bekleme yok / yalnızca e-postayla iptal.
- **KARAR-6** Analitik: hesabın olaylarını ve özetlerini sil (önerilen) / saklama süresine bırak.
- **KARAR-7** İade: iade yok, istisnalar destekten (önerilen) / kıst iade / 14 gün içinde tam iade.
- **KARAR-8** Kayıtlı kartlar sağlayıcıdan ne zaman silinsin: temizlik anında (önerilen) / talep anında.
- **KARAR-9** Saklama süreleri ve yedekler (hukuk onayıyla): finansal kayıtlar 10 yıl, günlükler 2 yıl, teslimat adresinin maskelenmesi, tasarım dosyaları için 90 gün, yedeklerin saklama süresi.
- **KARAR-10** Veri paketine lead'ler girsin mi: evet, tam alanlarıyla (önerilen, kullanıcı zaten görüyor) / maskeli / hayır.
- **KARAR-11** Tek üyeli şirket hesabı olan kullanıcı: iş 10 gelene kadar destek talebi (önerilen) / silme akışına dahil edilsin.
