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

const nextConfig: NextConfig = {
  output: "standalone",
  // Security headers (TEC-1102); public profile framing is still open, see the module.
  async headers() {
    return securityHeaderRules(process.env.NODE_ENV === "production");
  },
  async redirects() {
    return companyRedirects.map(([from, to]) => ({
      source: `/account/${from}`,
      destination: `/company/${to}`,
      permanent: true,
    }));
  },
};

export default nextConfig;
