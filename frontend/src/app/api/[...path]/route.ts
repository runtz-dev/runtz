import { NextRequest } from "next/server"

const BACKEND_URL = process.env.RUNTZ_BACKEND_URL ?? "http://localhost:8080"

type RouteContext = {
  params: Promise<{
    path: string[]
  }>
}

async function proxy(request: NextRequest, context: RouteContext) {
  const { path } = await context.params
  const incomingURL = new URL(request.url)
  const targetURL = new URL(`/api/${path.join("/")}`, BACKEND_URL)
  targetURL.search = incomingURL.search

  const headers = new Headers(request.headers)
  headers.delete("host")
  headers.delete("content-length")

  const hasBody = request.method !== "GET" && request.method !== "HEAD"
  const response = await fetch(targetURL, {
    method: request.method,
    headers,
    body: hasBody ? await request.arrayBuffer() : undefined,
    redirect: "manual",
  })

  const responseHeaders = new Headers(response.headers)
  responseHeaders.delete("content-encoding")
  responseHeaders.delete("content-length")

  // Set-Cookie has to survive this hop intact — it is what carries the session.
  // The Headers constructor folds repeated headers into a single comma-joined
  // value, which corrupts cookies whose attributes contain commas (Expires) and
  // merges separate cookies into one, so re-emit each one on its own line.
  const setCookies = response.headers.getSetCookie?.() ?? []
  if (setCookies.length > 0) {
    responseHeaders.delete("set-cookie")
    for (const cookie of setCookies) {
      responseHeaders.append("set-cookie", cookie)
    }
  }

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: responseHeaders,
  })
}

export {
  proxy as DELETE,
  proxy as GET,
  proxy as OPTIONS,
  proxy as PATCH,
  proxy as POST,
  proxy as PUT,
}
