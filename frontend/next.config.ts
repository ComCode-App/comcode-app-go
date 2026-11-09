import type { NextConfig } from "next";

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
  async redirects() {
    return companyRedirects.map(([from, to]) => ({
      source: `/account/${from}`,
      destination: `/company/${to}`,
      permanent: true,
    }));
  },
};

export default nextConfig;
