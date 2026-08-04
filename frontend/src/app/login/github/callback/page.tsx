"use client"

import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import * as React from "react"
import { LoaderCircleIcon } from "lucide-react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { apiRequest } from "@/lib/api"

function safeNextPath(value: string | null) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/app/overview"
  }

  return value
}

export default function GitHubCallbackPage() {
  return (
    <React.Suspense fallback={<CallbackCard />}>
      <GitHubCallback />
    </React.Suspense>
  )
}

function GitHubCallback() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    const code = searchParams.get("code")
    const state = searchParams.get("state")
    const oauthError = searchParams.get("error_description") ?? searchParams.get("error")
    const expectedState = sessionStorage.getItem("runtz_github_oauth_state")
    const nextPath = safeNextPath(sessionStorage.getItem("runtz_login_next"))
    sessionStorage.removeItem("runtz_github_oauth_state")
    sessionStorage.removeItem("runtz_login_next")

    if (oauthError) {
      setError(oauthError)
      return
    }
    if (!code || !state || !expectedState || state !== expectedState) {
      setError("Could not validate the GitHub callback. Try again.")
      return
    }

    apiRequest("/api/v1/auth/github", {
      method: "POST",
      body: {
        code,
        redirectUri: `${window.location.origin}/login/github/callback`,
      },
    })
      .then(() => {
        router.replace(nextPath)
      })
      .catch((requestError) => {
        setError(
          requestError instanceof Error
            ? requestError.message
            : "GitHub sign-in failed"
        )
      })
  }, [router, searchParams])

  return <CallbackCard error={error} />
}

function CallbackCard({ error = "" }: { error?: string }) {
  return (
    <main className="flex min-h-svh items-center justify-center bg-[#050912] px-6 text-[#eaf4ff]">
      <Card className="w-full max-w-md border-[#6db5ff]/20 bg-[#0d1420]/92 py-6 text-[#eaf4ff] shadow-[0_28px_90px_rgb(0_0_0/0.36)]">
        <CardHeader>
          <CardTitle className="text-xl font-bold">
            {error ? "Could not sign in" : "Connecting to GitHub"}
          </CardTitle>
          <CardDescription className="text-[#b8cbe4]">
            {error || "Validating your account and preparing the workspace."}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error ? (
            <Link
              href="/login"
              className="flex h-9 w-full items-center justify-center rounded-full bg-[#6db5ff] px-3 text-sm font-bold text-[#071222] transition hover:bg-[#9fd6ff]"
            >
              Voltar para o login
            </Link>
          ) : (
            <LoaderCircleIcon className="size-6 animate-spin text-[#6db5ff]" aria-label="Loading" />
          )}
        </CardContent>
      </Card>
    </main>
  )
}
