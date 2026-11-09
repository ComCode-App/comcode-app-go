// Slug preview helper; mirrors backend/internal/platform/slug.FromName (TEC-1111).
// Turkish letters fold to ASCII ("Çağrı Şirketi" -> "cagri-sirketi") and other
// Latin diacritics keep their base letter, instead of being stripped.

const TURKISH_FOLD: Record<string, string> = {
  İ: "i",
  I: "i",
  ı: "i",
  Ş: "s",
  ş: "s",
  Ğ: "g",
  ğ: "g",
  Ç: "c",
  ç: "c",
  Ö: "o",
  ö: "o",
  Ü: "u",
  ü: "u",
};

const TURKISH_RE = /[İIıŞşĞğÇçÖöÜü]/g;

export function slugify(value: string, separator = "-"): string {
  const sep = separator.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return value
    .trim()
    .replace(TURKISH_RE, (ch) => TURKISH_FOLD[ch] ?? ch)
    .normalize("NFD")
    .replace(/\p{Mn}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, separator)
    .replace(new RegExp(`^(?:${sep})+|(?:${sep})+$`, "g"), "");
}
