/**
 * Adds `?via={route path}` to a card / smart link target so the profile page attributes leads,
 * contact exchanges and visitor events to that card or link (TEC-552). Every card and smart link
 * appends it, also when its own lead form is off (TEC-940); the profile page then shows the
 * card's form if it has one, else the profile's own. Only profile pages read `via`; external
 * targets (direct URLs, vCard downloads) are returned unchanged.
 */
export function withVia(targetUrl: string, routePath: string): string {
  try {
    const base =
      typeof window === "undefined"
        ? "http://localhost"
        : window.location.origin;
    const url = new URL(targetUrl, base);
    const isProfilePage =
      /^\/[^/]+\/?$/.test(url.pathname) && !url.pathname.startsWith("/api/");
    if (!isProfilePage) return targetUrl;
    url.searchParams.set("via", routePath);
    return /^https?:\/\//i.test(targetUrl)
      ? url.toString()
      : `${url.pathname}${url.search}`;
  } catch {
    return targetUrl;
  }
}

/** `c/{code}` or `s/{slug}`: what cards and smart links append as `?via=`. */
export const VIA_RE = /^[cs]\/[A-Za-z0-9._-]{1,120}$/;
