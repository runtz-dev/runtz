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
