import { registerOTel } from "@vercel/otel"

// The engine, same value the /api proxy route resolves. Kept in sync with
// src/app/api/[...path]/route.ts on purpose — see propagateContextUrls below.
const BACKEND_URL = process.env.RUNTZ_BACKEND_URL ?? "http://localhost:8080"

/**
 * Next.js calls this once per server process, before any request is handled.
 *
 * Everything else is configured through the standard OTEL_* environment
 * variables, which @vercel/otel reads on its own — OTEL_EXPORTER_OTLP_ENDPOINT
 * for the collector, OTEL_RESOURCE_ATTRIBUTES for the environment labels,
 * OTEL_SDK_DISABLED to turn it all off. With no endpoint set, @vercel/otel
 * registers no exporter at all, so `next dev` stays silent and a self-hosted
 * install sends nothing anywhere.
 *
 * propagateContextUrls is what makes /api/[...path] one trace instead of two.
 * @vercel/otel does not put `traceparent` on an outgoing fetch unless the
 * target is explicitly allowed: off Vercel, its default list is empty and only
 * http://localhost is trusted. Our proxy calls the engine over https on a real
 * hostname, so without this the engine would start a fresh trace on every
 * proxied request and the two services would never appear in one timeline.
 *
 * It lists the engine specifically rather than "*" so that trace headers only
 * ever reach our own backend, never a third party the app might call
 * server-side.
 */
export function register() {
  registerOTel({
    serviceName: "runtz-frontend",
    instrumentationConfig: {
      fetch: {
        propagateContextUrls: [BACKEND_URL],
      },
    },
  })
}
