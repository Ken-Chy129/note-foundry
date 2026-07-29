import type { NextConfig } from "next";

const apiOrigin = process.env.API_ORIGIN ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${apiOrigin}/api/:path*` },
      { source: "/auth/:path*", destination: `${apiOrigin}/auth/:path*` },
      { source: "/healthz", destination: `${apiOrigin}/healthz` },
      { source: "/readyz", destination: `${apiOrigin}/readyz` }
    ];
  }
};

export default nextConfig;
