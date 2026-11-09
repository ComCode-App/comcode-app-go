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
const DENY_FRAME_ANCESTORS: Header = {
  key: "Content-Security-Policy",
  value: "frame-ancestors 'none'",
};
const DENY_X_FRAME_OPTIONS: Header = { key: "X-Frame-Options", value: "DENY" };

/** PayTR iFrame origin (`https://www.paytr.com/odeme/guvenli/{token}`). */
export const PAYTR_ORIGIN = "https://www.paytr.com";

/**
 * Payment result pages (TEC-1125). PayTR sends the browser to
 * `merchant_ok_url` / `merchant_fail_url` INSIDE its own iFrame, so the page
 * loads as app > www.paytr.com > app. `frame-ancestors` checks every
 * ancestor, so these pages must allow PayTR or the browser blocks them before
 * the frame-exit script (`PaymentFrameExit`) can move the result to the top
 * window. Exactly the paths the backend builds (`{FRONTEND_URL}/<prefix>/{session uuid}?result=success|fail`):
 * `billing/usecase/service.go` (plan checkout), `billing/usecase/cards.go`
 * (card add), `billing/usecase/orders.go` (card order). Keep in sync.
 */
export const PAYMENT_RESULT_PREFIXES = [
  "/billing/checkout",
  "/billing/cards/add",
  "/orders/checkout",
] as const;

/** Exactly one session segment after a prefix: `/billing/checkout/{uuid}`. */
export function isPaymentResultPath(pathname: string): boolean {
  return PAYMENT_RESULT_PREFIXES.some((prefix) => {
    if (!pathname.startsWith(`${prefix}/`)) return false;
    const rest = pathname.slice(prefix.length + 1);
    return rest.length > 0 && !rest.includes("/");
  });
}

/**
 * Framable by the app itself and PayTR only. No X-Frame-Options: it cannot
 * name another origin (ALLOW-FROM is dead), and SAMEORIGIN / DENY would block
 * the PayTR frame in browsers that check XFO against the whole frame chain.
 */
export const PAYMENT_RESULT_FRAMING_HEADERS: Header[] = [
  {
    key: "Content-Security-Policy",
    value: `frame-ancestors 'self' ${PAYTR_ORIGIN}`,
  },
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

/** `(?:billing/checkout|...)/[^/]+/?$` for the negative lookahead below. */
const paymentResultPattern = `(?:${PAYMENT_RESULT_PREFIXES.map((p) =>
  p.slice(1),
).join("|")})/[^/]+/?$`;

export function securityHeaderRules(production: boolean): HeaderRule[] {
  return [
    {
      source: "/:path*",
      headers: [...baseSecurityHeaders(production), DENY_FRAME_ANCESTORS],
    },
    // Next merges rules by header key (later wins) but cannot drop a header, so
    // X-Frame-Options: DENY is sent on every path EXCEPT the payment result
    // pages (TEC-1125).
    {
      source: `/:path((?!${paymentResultPattern}).*)`,
      headers: [DENY_X_FRAME_OPTIONS],
    },
    // Later rules override the same header keys (Next applies them in order).
    ...SAME_ORIGIN_FRAMEABLE_SOURCES.map((source) => ({
      source,
      headers: SAME_ORIGIN_FRAMING_HEADERS,
    })),
    ...PAYMENT_RESULT_PREFIXES.map((prefix) => ({
      source: `${prefix}/:sessionUuid`,
      headers: PAYMENT_RESULT_FRAMING_HEADERS,
    })),
  ];
}
