import type { NextConfig } from "next";

import { securityHeaderRules } from "./src/config/security-headers";

/** Company management moved from /account/* to /company/* (TEC-949); old links keep working. */
const companyRedirects = [
  ["offices", "offices"],
  ["groups", "groups"],
  ["structure", "members"],
  ["bulk", "bulk"],
  ["analytics", "analytics"],
  ["branding", "branding"],
] as const;

/**
 * Pages whose URL can carry an invite token (the invite link itself, and login/sign-up with the
 * token in `invite`/`next`) never send their URL as a Referer (TEC-1104).
 */
const noReferrerPaths = [
  "/platform/invite/:path*",
  "/platform/login",
  "/platform/register",
];

const nextConfig: NextConfig = {
  output: "standalone",
  // Security headers (TEC-1102); public profile framing is still open, see the module.
  // Invite-token pages come last so their no-referrer wins over the base policy (TEC-1104).
  async headers() {
    return [
      ...securityHeaderRules(process.env.NODE_ENV === "production"),
      ...noReferrerPaths.map((source) => ({
        source,
        headers: [{ key: "Referrer-Policy", value: "no-referrer" }],
      })),
    ];
  },
  async redirects() {
    return [
      ...companyRedirects.map(([from, to]) => ({
        source: `/account/${from}`,
        destination: `/company/${to}`,
        permanent: true,
      })),
      // Cmd+K role hits indexed before TEC-1112 point at /platform/roles/{uuid};
      // roles live under System Settings. Drop after the next `make search-reindex`.
      {
        source: "/platform/roles/:path*",
        destination: "/platform/settings/roles/:path*",
        permanent: false,
      },
    ];
  },
};

export default nextConfig;
