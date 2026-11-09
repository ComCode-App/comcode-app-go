# Sahip kontrol listesi: CI, GHCR ve Dokploy

Repoda hiçbir kodun yapamayacağı, yalnız repo sahibinin bir kez yapacağı ayarlar. Ajanlar ve CI bu ayarlara dokunmaz. Arka plan: [DEVELOPMENT.md](./DEVELOPMENT.md) → CI, Releases, Deploying a release.

## GitHub

- [ ] **Workflow izinleri:** Settings → Actions → General → Workflow permissions → **Read and write permissions**. Doğrulama: `main`'e ilk push'ta `publish-images` ve `release-tag` işleri yeşil biter.
- [ ] **Branch koruması:** `main` için tek zorunlu status check **`ci`** olsun (tek tek işler değil; onlar path filtresiyle bilerek atlanır). İstenirse aynısı `cc-collective-branch` için. Doğrulama: `ci` kırmızıyken PR'da merge düğmesi kapalı.
- [ ] **Self-hosted runner:** Artık kullanılmıyor (her iş `ubuntu-24.04`). Settings → Actions → Runners altındaki eski runner kaydı silinebilir. Doğrulama: liste boş, CI işleri yine de çalışıyor.
- [ ] **Mobil (yalnız istenirse):** Settings → Secrets and variables → Actions → Variables altında `MOBILE_ENABLED=true`; EAS sürümü için Secrets altında `EXPO_TOKEN`. Doğrulama: `mobile/package.json` olan bir push'ta `mobile` işi çalışır (atlanmaz).

## GHCR

- [ ] **Paket erişimi:** İlk publish'ten sonra `comcode-app-go-backend` ve `comcode-app-go-frontend` paketleri private oluşur. Ya ikisini public yap (organizasyon → Packages → paket → Package settings), ya da Dokploy'da `ghcr.io` için `read:packages` yetkili bir token ile registry ekle. Doğrulama: sunucuda `docker pull ghcr.io/comcode-app/comcode-app-go-backend:main` çalışır.

## Dokploy

- [ ] **Git kaynağı:** Compose uygulamasının kaynağı `ComCode-App/comcode-app-go`. Doğrulama: Dokploy'da uygulamanın repo alanı bu adı gösterir.
- [ ] **Compose dosyası:** Yalnız `compose.prod.yml` (`compose.build.yml` asla). Doğrulama: deploy logunda `build` adımı yok, image'lar `ghcr.io/...` adresinden çekiliyor.
- [ ] **Env yükseltme:** Master `.env.server` üzerinde `make prod-env-upgrade`, ardından `make prod-env-check`. Doğrulama: `check` 0 ile çıkar.
- [ ] **Image değişkenleri:** Dokploy environment'tan `BACKEND_IMAGE` ve `FRONTEND_IMAGE` satırlarını sil; `IMAGE_REPO=ghcr.io/comcode-app/comcode-app-go` ve `IMAGE_TAG=main` (ya da sabitlemek için `v0.N`) olsun. Doğrulama: Dokploy env'inde bu iki satır yok ve deploy logunda çekilen image `ghcr.io/comcode-app/comcode-app-go-backend:<IMAGE_TAG>`.
- [ ] **APP_ENCRYPTION_KEY:** Prod env'de dolu ve örnek değer değil (`make prod-env-init` üretir). Değer daha önce boş veya örnek idiyse önce [DEVELOPMENT.md](./DEVELOPMENT.md) → Required secrets uyarısını uygula. Doğrulama: `make prod-env-check` bu anahtar için hata vermez ve backend açılışta reddetmez.
- [ ] **Backend özel ağa (TEC-1115), deploy ÖNCESİ 1:** Dokploy environment'ta `API_URL` boş ya da tam olarak `http://comcode-backend:8080/v1`. Backend artık `dokploy-network`'te olmadığı için public bir backend adresine Traefik artık ulaşamaz ve BFF'nin tüm çağrıları düşer. Doğrulama: Dokploy env'inde `API_URL` satırı yok, boş ya da bu değer.
- [ ] **Backend özel ağa (TEC-1115), deploy ÖNCESİ 2:** PayTR mağaza paneli → bildirim (callback) URL'i `https://<frontend-domain>/api/v1/public/payment/paytr/callback` (örn. `https://comcode.technowide.software/api/v1/public/payment/paytr/callback`); backend domain'i olmamalı. Doğrulama: panelde bu URL görünür, deploy sonrası bir test ödemesinde sipariş "ödendi" olur.
- [ ] **Backend özel ağa (TEC-1115), deploy SONRASI:** Dokploy → Domains'ten backend servisinin domain'ini sil. Doğrulama: listede `backend` servisine bağlı domain yok; site ve giriş çalışır. Geri alma: `IMAGE_TAG`'i önceki `v0.N` yap ve önceki `v0.N` git tag'ini yeniden deploy et (eski `compose.prod.yml` backend'i tekrar `dokploy-network`'e alır).
- [ ] **Deploy tetikleyicisi:** `v0.*` ile eşleşen git tag'leri. Doğrulama: bir sonraki `main` release'inde (`v0.N` tag'i) Dokploy kendiliğinden deploy başlatır.

## İsteğe bağlı

- [ ] **`v0.0` başlangıç tag'i:** Release notlarının başlayacağı commit'e (komutu sahip çalıştırır, CI bu tag'i oluşturmaz):
  `git tag -a v0.0 -m baseline <sha> && git push origin v0.0`
  Doğrulama: `git ls-remote --tags origin v0.0` sonuç verir; ilk release `v0.1` olur ve notları bu commit'ten sonrasını listeler.
