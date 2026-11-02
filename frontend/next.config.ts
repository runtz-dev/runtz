import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  // Compatibility for every /app/... link already published (docs, emails,
  // bookmarks) from before the platform moved to the domain root. Kept
  // indefinitely — cheap, and the ingress still has to route /app* here for
  // these to fire. Mirrors the redirect /app itself used to do on its own.
  async redirects() {
    return [
      // The legacy /docs/cloud path can collide with the hosted cloud route.
      { source: "/docs/cloud/google-cloud", destination: "/docs/scans/google-cloud", permanent: true },
      { source: "/:lang(pt-br|es)/docs/cloud/google-cloud", destination: "/:lang/docs/scans/google-cloud", permanent: true },
      { source: "/en/docs/cloud/google-cloud", destination: "/docs/scans/google-cloud", permanent: true },
      { source: "/app", destination: "/overview", permanent: true },
      { source: "/app/:path*", destination: "/:path*", permanent: true },
    ];
  },
};

export default nextConfig;
