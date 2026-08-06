import { registerOTel } from "@vercel/otel"

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
 * The payoff is /api/[...path]: the fetch instrumentation stamps `traceparent`
 * on the proxied request, and the engine reads it back, so one trace spans the
 * browser hit, this process, the engine and the MongoDB query behind it.
 */
export function register() {
  registerOTel({ serviceName: "runtz-frontend" })
}
