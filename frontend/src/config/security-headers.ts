/**
 * HTTP security headers for every Next route (TEC-1102), applied from
 * `next.config.ts` `headers()`. Plain module (no `@/` imports): next.config
 * loads it before the app's path aliases exist.
 *
 * No script-src CSP: Next's inline bootstrap scripts would need nonces, which
 * is a separate change.
 */

type Header = { key: string; value: string };
type HeaderRule = { source: string; headers: Header[] };

/** Headers sent on every route. */
export function baseSecurityHeaders(production: boolean): Header[] {
  return [
    { key: "X-Content-Type-Options", value: "nosniff" },
    { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
    // ComCode uses no camera / microphone / geolocation in the browser (no QR
    // scanning; GeoIP is server-side). Turn them back on here if a feature needs them.
    {
      key: "Permissions-Policy",
      value: "camera=(), microphone=(), geolocation=()",
    },
    ...(production
      ? [
          {
            key: "Strict-Transport-Security",
            value: "max-age=31536000; includeSubDomains",
          },
        ]
      : []),
  ];
}

/**
 * Clickjacking protection (TEC-1119): no page may be framed, by another site or
 * by ComCode itself. This covers the panel and the public visitor pages
 * (`/{username}`, `/c/*`, `/s/*`, `/share/*`, `/kvkk`): they are not
 * embeddable on other sites, see docs/PUBLIC_PROFILES.md.
 */
export const DENY_FRAMING_HEADERS: Header[] = [
  { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
  { key: "X-Frame-Options", value: "DENY" },
];

/** Same-origin framing for the few responses the app shows in its own iframe. */
export const SAME_ORIGIN_FRAMING_HEADERS: Header[] = [
  { key: "Content-Security-Policy", value: "frame-ancestors 'self'" },
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
];

/**
 * BFF responses the app frames itself: PDF / text previews in the platform
 * storage explorer (`storage-preview.tsx`) and the `/share/*` viewer
 * (`storage-public-viewer.tsx`), both via `storage-media-preview.tsx`. Keep in
 * sync when a new same-origin `<iframe src>` is added.
 */
export const SAME_ORIGIN_FRAMEABLE_SOURCES = [
  "/api/v1/platform/storage/objects/preview",
  "/api/v1/public/storage/:path*",
] as const;

export function securityHeaderRules(production: boolean): HeaderRule[] {
  return [
    {
      source: "/:path*",
      headers: [...baseSecurityHeaders(production), ...DENY_FRAMING_HEADERS],
    },
    // Later rules override the same header keys (Next applies them in order).
    ...SAME_ORIGIN_FRAMEABLE_SOURCES.map((source) => ({
      source,
      headers: SAME_ORIGIN_FRAMING_HEADERS,
    })),
  ];
}
