import type { MetadataRoute } from "next"

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "runtz",
    short_name: "runtz",
    description:
      "Open source DevSecOps platform: SCA, SAST, host, container and Kubernetes scans.",
    start_url: "/",
    display: "standalone",
    background_color: "#050912",
    theme_color: "#050912",
    icons: [
      { src: "/brand/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/brand/icon-512.png", sizes: "512x512", type: "image/png" },
      { src: "/brand/icon-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  }
}
