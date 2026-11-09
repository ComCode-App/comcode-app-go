# Eski kart verilerini dışa aktarma (COM-560 / eski TEC-560)

Tek dosyalık geçici araç; PHP 7.4+, PDO MySQL ve zlib gerekir, Composer/SSH gerekmez.
Normalize eşleme Go `migrate-legacy sync` komutundadır.

1. `export.php` içindeki `EXPORT_TOKEN` değerini parola yöneticinizin ürettiği en az
   32 karakterli uzun rastgele değerle değiştirin (örn. `openssl rand -hex 32`).
   Varsayılan/kısa token ile araç hiç çalışmaz; eksik/yanlış token sade 404 döner.
2. cPanel → MySQL Databases içinde mümkünse yalnızca eski veritabanında **SELECT**
   yetkili geçici kullanıcı oluşturun. Üstteki `EXPORT_DB_*` sabitlerini doldurun.
   Host genelde `localhost`, port `3306` olur. Araç yalnızca SELECT/SHOW kullanır.
3. Alternatif: `LEGACY_CONFIG_PATH` içine sunucudaki PHP config veya Laravel `.env`
   dosyasının **mutlak yolunu** yazın (örn. `/home/kullanici/proje/.env`).
   `.env`: `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD`.
   PHP: `define('DB_NAME', '...');`, `const DB_HOST = '...';`, `$DB_USER = '...';`
   gibi düz metin sabit/değişkenleri (`DB_PASS` dahil) desteklenir. Değişken adları
   büyük/küçük harfe duyarsızdır. Dosya çalıştırılmaz; ifadeler, fonksiyonlar, diziler,
   ortam değişkenleri ve başka dosyalardaki değerler çözümlenmez. Bu durumda sabitleri
   elle doldurun. Boş olmayan açık sabitler keşfedilen değerden önceliklidir.
   Boş parola isteniyorsa config parolası da boş olmalı veya config kullanılmamalı.
4. FTP ile HTTPS kullanan eski sitenin web kökünde rastgele uzun adlı yeni klasör
   açın (örn. `aktarim-<rastgele-deger>`), yalnızca `export.php` yükleyin.
   Config dosyasını web köküne taşımayın. Rastgele klasör token yerine geçmez.
5. `https://comcode.co/aktarim-.../export.php?token=TOKEN&mode=schema` açın.
   Token özel karakter içeriyorsa URL kodlayın. JSON'u yerel güvenli dosyaya kaydedin:
   tablo/kolon bilgileri, tam satır sayıları ve tablo başına en çok 3 örnek satır
   (hücre başına 80 karakter) gelir. COUNT büyük tabloda uzun sürebilir.
   Örnekler kişisel veri içerebilir; herkese açık yerlere yüklemeyin.
6. Gerekli her tabloyu `...?token=TOKEN&mode=dump&table=TABLO_ADI` ile indirin.
   Tablo adını schema listesinden aynen alın, URL kodlayın. `TABLO_ADI.csv.gz`
   UTF-8 CSV başlığı boş tabloda da bulunur. NULL ve boş metin CSV'de boş hücre olur;
   eski binary/NULL dönüşümlerini normalize eşleme aşamasında değerlendirin.
   İndirme sırasında eski sisteme yazmayı durdurun: ayrı dosyalar arası snapshot yoktur.
7. `gzip -t TABLO_ADI.csv.gz` ile indirmeyi kontrol edin. Kesilmiş gzip'i tekrar indirin.
   CSV sunucuda dosya yazmaz: unbuffered PDO, tek CSV satırı için bellek ve 64 KiB gzip
   parçaları kullanılır. Bellek tablo boyutuna değil tek satıra bağlıdır. Hosting zaman
   sınırı yine geçerlidir. Schema yanıtı meta verileri bellekte toplar.
8. **CUTOVER TAMAMLANINCA FTP'DEN DOSYAYI VE GEÇİCİ KLASÖRÜ SİLİN!** URL'nin 404 verdiğini kontrol
   edin. Geçici MySQL kullanıcısını kaldırın, token'ı iptal edin. Token içeren URL,
   tarayıcı geçmişi ve erişim logları hassas bilgidir.

Token/DB bilgileri doldurulmuş dosyayı Git'e eklemeyin. Bağlantıyı paylaşmayın.
DB bağlantı bilgileri çıktıya/loga yazılmaz; hatalar bilerek genel mesaj döner.

Yerel kontrol: `php -l tools/legacy-export/export.php` ve
`python3 tools/legacy-export/test_export.py`. Davranış testleri gerçek eski MySQL'e
bağlanmaz; token/config parser, PDO fixture, CSV/hits ve medya akışlarını kontrol eder. PHP 7.4 söz dizimi korunur; yerel
PHP sürümünüzü `php -v` ile ayrıca kontrol edin.

## Cutover gününe kadar tekrarlanabilir sync

Araç artık şeması doğrulanmış v1 kaynağı için de kullanılır. FTP ile
`/home/comcode/public_html/aktarim-<uzun-rastgele-deger>/export.php` konumuna
koyun. `EXPORT_PUBLIC_ROOT` boşsa bu klasörün üst dizini public_html kabul edilir;
farklı yerleşimde mutlak yolu açıkça yazın. Token en az 32 karakter olmalı.
Araç **cutover gününe kadar kalır**, her sync aynı URL/token ile çalışır.
Cutover doğrulaması tamamlanınca dosyayı, rastgele klasörü, dışarıdaki hash cache'ini
ve geçici SELECT kullanıcısını silin; URL'nin 404 döndüğünü doğrulayın.

Mümkünse aynı klasörde `.htaccess` ile yalnızca sync bilgisayarının IP'sine izin verin:

```apache
<Files "export.php">
  Require ip YOUR_PUBLIC_IP_ADDRESS
</Files>
```

Apache 2.4 gerektirir; `YOUR_PUBLIC_IP_ADDRESS` yerine kendi sabit genel IP'nizi yazın.

- `mode=dump&table=users|hits|main_link_types|pages`: gzip CSV. Diğer tablolar,
  özellikle `settings`, `payments`, `activation` reddedilir. Schema örnekleri de
  aynı tablo/kolon sınırlarına uyar. `password`, `lost_password_code`,
  `email_activation_code`, `facebook_id`, `instagram_id` kullanıcı verisinde yoktur.
- `mode=hits_since&since=2026-10-19%2000%3A00%3A00`: bu zamandan **sonraki** hits,
  tarih/id sıralı gzip CSV. Tarihler eski sunucunun yerel saatidir; CLI aynı değerleri
  watermark olarak tutar, bir saniye örtüşme ile tekrar okur ve hit ID ile tekilleştirir.
- `mode=media_manifest`: JSON `[ {path,size,mtime,sha1}, ... ]`. Yalnızca resim/PDF,
  izinli kökler: `uploads/avatars`, `uploads/comps`, `uploads/kataloglar`, `usercard`,
  `template/images`. Symlink takip edilmez. SHA1 değişmeyen path/size/mtime için
  web kökü dışındaki `../.comcode-export-cache.json` dosyasında önbelleklenir.
  `EXPORT_CACHE_PATH` ile dışarıda başka mutlak yol verilebilir; yazılamazsa cache atlanır.
- `mode=media_file&path=uploads/avatars/example.png`: tek dosya. `..`, mutlak yol,
  symlink ve izin verilmeyen uzantı reddedilir.
- `mode=media_bundle&since=0&roots=uploads/avatars,usercard`: mtime > since olan
  dosyaların tar.gz akışı. `since` Unix saniyesidir; roots opsiyoneldir.
  CSV ve bundle tek satır/64 KiB parçalarla akar; manifest yalnızca küçük dosya
  metaverisi/hash cache'ini bellekte tutar. Sunucuda arşiv dosyası oluşturulmaz.

`migrate-legacy sync` users/main_link_types, hits watermark ve manifest'i okur;
değişen referanslı dosyaları `media_file` üzerinden indirir. `media_bundle` FTP ile
alınan zip'e alternatif ilk toplu indirmedir. Zip/tar'ı güvenli bir yerel klasöre açıp
`--media-dir` verin; `public_html/uploads/...` ve `uploads/...` ikisi de kabul edilir.
Canlı sync tutarlı bir DB snapshot'ı değildir; sonraki tur içerik değişikliklerini alır.
Son cutover turunda eski yazmaları durdurun. Token içeren erişim loglarını özel tutun.
