/**
 * In-process cache for the NextAuth provider list (TEC-1113).
 *
 * NextAuth's config callback runs on every `/api/auth/*` hit; building providers costs one
 * app-config call plus one OAuth-config call per provider. The cache keeps the last
 * complete build for `PROVIDERS_CACHE_TTL_MS` and shares one in-flight build between
 * concurrent requests. A build where a backend call failed is used for that request but not
 * cached, so a short API outage does not hide sign-in methods for the whole TTL.
 */

/**
 * How long an auth-settings change (enabling / disabling a provider) may take to reach the
 * sign-in page. Single knob; owner decision pending (5 min proposed, 60 s for now).
 */
export const PROVIDERS_CACHE_TTL_MS = 60_000;

export type ProviderBuild<T> = {
  value: T;
  /** False when a backend call failed and defaults were used. */
  complete: boolean;
  /**
   * Epoch ms after which this build must not be served, e.g. a generated Apple
   * client secret's expiry minus a margin (TEC-1123). The cache keeps the build
   * until the earlier of this and the TTL.
   */
  expiresAt?: number | null;
};

export function createProviderCache<T>(
  build: () => Promise<ProviderBuild<T>>,
  ttlMs: number = PROVIDERS_CACHE_TTL_MS,
  now: () => number = Date.now,
) {
  let cached: { value: T; expiresAt: number } | null = null;
  let inflight: Promise<T> | null = null;

  async function get(): Promise<T> {
    if (cached && now() < cached.expiresAt) return cached.value;
    if (!inflight) {
      inflight = build()
        .then((result) => {
          if (result.complete) {
            let expiresAt = now() + ttlMs;
            if (typeof result.expiresAt === "number") {
              expiresAt = Math.min(expiresAt, result.expiresAt);
            }
            cached = { value: result.value, expiresAt };
          }
          return result.value;
        })
        .finally(() => {
          inflight = null;
        });
    }
    return inflight;
  }

  function invalidate() {
    cached = null;
  }

  return { get, invalidate };
}
