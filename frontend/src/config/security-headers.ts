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

/**
 * Top-level app/panel paths (route folders under `app/(shell)/*` plus `/kvkk`
 * and the BFF). Everything else at the top level is a public profile
 * (`/{username}`) or a card / smart link (`/c/*`, `/s/*`), see
 * docs/PUBLIC_PROFILES.md. `security-headers.test.mjs` fails when a new
 * `(shell)` route folder is missing here.
 */
export const APP_ROUTE_PREFIXES = [
  // (app)
  "account",
  "analytics",
  "background-studio",
  "bank-accounts",
  "billing",
  "cards",
  "company",
  "leads",
  "orders",
  "profile",
  "profiles",
  "smart-links",
  "webhooks",
  // (platform) and (guest)
  "platform",
  "login",
  "forgot-password",
  "reset-password",
  "verify-email",
  // (public)
  "unauthorized",
  "forbidden",
  "health",
  // share links, legal page, BFF and Auth.js
  "share",
  "kvkk",
  "api",
] as const;

/** Headers safe on every route, public profiles included. */
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

/** Clickjacking protection: only our own pages may frame the app/panel. */
export const FRAMING_HEADERS: Header[] = [
  { key: "Content-Security-Policy", value: "frame-ancestors 'self'" },
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
];

export function securityHeaderRules(production: boolean): HeaderRule[] {
  const rules: HeaderRule[] = [
    { source: "/:path*", headers: baseSecurityHeaders(production) },
    // Panel home ("/" is the signed-in dashboard).
    { source: "/", headers: FRAMING_HEADERS },
  ];
  for (const prefix of APP_ROUTE_PREFIXES) {
    rules.push({ source: `/${prefix}`, headers: FRAMING_HEADERS });
    rules.push({ source: `/${prefix}/:path*`, headers: FRAMING_HEADERS });
  }
  // TODO(TEC-1102): public profile pages (/{username}, /c/*, /s/*) are left
  // frameable until the owner decides whether they are embedded on other
  // sites. Once decided, either add FRAMING_HEADERS for them too or set an
  // explicit `frame-ancestors` allowlist.
  return rules;
}
