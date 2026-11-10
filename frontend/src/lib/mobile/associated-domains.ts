/**
 * Domain association files for the mobile app (TEC-1183, docs/design/mobile.md B6):
 *
 * - `/.well-known/apple-app-site-association` (iOS universal links + shared web credentials)
 * - `/.well-known/assetlinks.json` (Android App Links + Credential Manager login sharing)
 *
 * Values come from the frontend container's runtime env (not NEXT_PUBLIC_*: the images are
 * prebuilt), see deploy/env/schema → "Mobile app". A file is only served when its required
 * values are set and valid; otherwise the route answers 404, because a malformed file is
 * cached by Apple's CDN / Google's verifier and is worse than none.
 *
 * This module has no imports on purpose: `associated-domains.test.mjs` transpiles it
 * standalone under `node --test`.
 */

/** KARAR-2: one id for both stores. */
export const DEFAULT_MOBILE_APP_ID = "software.technowide.comcode";

/** Universal / App Link paths the app handles: the QR web sign-in link (design §3). */
export const MOBILE_LINK_PATHS = ["/login/qr/*"] as const;

/** Both files are public and rarely change; Apple's CDN caches them on its own. */
export const ASSOCIATION_CACHE_CONTROL = "public, max-age=3600";

export type MobileAppConfig = {
  appleTeamId: string;
  iosBundleId: string;
  androidPackage: string;
  androidSha256Certs: string[];
};

type Env = Record<string, string | undefined>;

const TEAM_ID_RE = /^[A-Z0-9]{10}$/;
// Reverse-DNS id: at least two dot-separated labels (iOS also allows "-").
const BUNDLE_ID_RE = /^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$/;
// Android package: Java-identifier segments, at least two.
const ANDROID_PACKAGE_RE = /^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$/;
const SHA256_HEX_RE = /^[0-9A-F]{64}$/;

function trimmed(env: Env, key: string): string {
  return (env[key] ?? "").trim();
}

export function readMobileAppConfig(env: Env): MobileAppConfig {
  return {
    appleTeamId: trimmed(env, "MOBILE_APPLE_TEAM_ID"),
    iosBundleId: trimmed(env, "MOBILE_IOS_BUNDLE_ID") || DEFAULT_MOBILE_APP_ID,
    androidPackage:
      trimmed(env, "MOBILE_ANDROID_PACKAGE") || DEFAULT_MOBILE_APP_ID,
    androidSha256Certs: trimmed(env, "MOBILE_ANDROID_SHA256_CERTS")
      .split(",")
      .map((cert) => cert.trim())
      .filter(Boolean),
  };
}

/**
 * `AB:CD:...` (32 colon-separated upper-case bytes, the form Google requires). Accepts the
 * fingerprint with or without colons and in any case, as Play Console / keytool print it.
 * Returns null when it is not a SHA-256 fingerprint.
 */
export function normalizeSha256Fingerprint(raw: string): string | null {
  const hex = raw.trim().replace(/:/g, "").toUpperCase();
  if (!SHA256_HEX_RE.test(hex)) return null;
  return hex.match(/../g)!.join(":");
}

/** Apple: null (404) until a valid Team ID and bundle id are set. */
export function buildAppleAppSiteAssociation(config: MobileAppConfig) {
  if (!TEAM_ID_RE.test(config.appleTeamId)) return null;
  if (!BUNDLE_ID_RE.test(config.iosBundleId)) return null;
  const appID = `${config.appleTeamId}.${config.iosBundleId}`;
  return {
    applinks: {
      details: [
        {
          appIDs: [appID],
          components: MOBILE_LINK_PATHS.map((path) => ({ "/": path })),
        },
      ],
    },
    webcredentials: { apps: [appID] },
  };
}

/** Google: null (404) until at least one cert is set and every cert is a valid SHA-256. */
export function buildAssetLinks(config: MobileAppConfig) {
  if (!ANDROID_PACKAGE_RE.test(config.androidPackage)) return null;
  if (config.androidSha256Certs.length === 0) return null;
  const fingerprints: string[] = [];
  for (const cert of config.androidSha256Certs) {
    const normalized = normalizeSha256Fingerprint(cert);
    if (!normalized) return null;
    if (!fingerprints.includes(normalized)) fingerprints.push(normalized);
  }
  return [
    {
      relation: [
        "delegate_permission/common.handle_all_urls",
        "delegate_permission/common.get_login_creds",
      ],
      target: {
        namespace: "android_app",
        package_name: config.androidPackage,
        sha256_cert_fingerprints: fingerprints,
      },
    },
  ];
}

/**
 * The route response: JSON body + headers, or a 404 when unconfigured. Plain data so the
 * test can check it without Next; the route wraps it in a `Response`.
 */
export function associationResponse(body: unknown): {
  status: 200 | 404;
  body: string;
  headers: Record<string, string>;
} {
  if (body === null) {
    return {
      status: 404,
      body: JSON.stringify({ error: "not_configured" }),
      headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-store",
      },
    };
  }
  return {
    status: 200,
    body: JSON.stringify(body),
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": ASSOCIATION_CACHE_CONTROL,
    },
  };
}
