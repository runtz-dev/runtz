"use client"

import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import * as React from "react"
import { ArrowLeftIcon, ArrowRightIcon, LoaderCircleIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ApiError, apiRequest, clearToken, getStoredToken } from "@/lib/api"
import type { Entitlement, User, Workspace } from "@/lib/api"
import type { DeploymentMode } from "@/components/runtz/workspace-context"

type CheckoutPlan = "free" | "pro" | "enterprise"

type MeResponse = {
  user: User
  workspaces: Workspace[]
  deploymentMode: DeploymentMode
  entitlement: Entitlement
}

const planLabels: Record<CheckoutPlan, string> = {
  free: "Free",
  pro: "Pro",
  enterprise: "Enterprise",
}

function normalizePlan(value: string | null): CheckoutPlan {
  if (value === "enterprise" || value === "pro") {
    return value
  }

  return "free"
}

function normalizeDeploymentMode(value: string | null): DeploymentMode {
  return value === "self-hosted" ? "self-hosted" : "cloud"
}

function planRank(plan: CheckoutPlan) {
  if (plan === "enterprise") {
    return 3
  }
  if (plan === "pro") {
    return 2
  }
  return 1
}

function currentPathWithSearch() {
  return `${window.location.pathname}${window.location.search}`
}

export default function CheckoutPage() {
  return (
    <React.Suspense fallback={<CheckoutShell title="Preparing checkout" />}>
      <CheckoutFlow />
    </React.Suspense>
  )
}

function CheckoutFlow() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const queryString = searchParams.toString()
  const startedQueryRef = React.useRef<string | null>(null)
  const [state, setState] = React.useState<{
    title: string
    description?: string
    currentPlan?: CheckoutPlan
    error?: string
  }>({
    title: "Preparing checkout",
    description: "Checking your Runtz session before opening Stripe.",
  })

  React.useEffect(() => {
    if (startedQueryRef.current === queryString) {
      return
    }
    startedQueryRef.current = queryString

    let cancelled = false
    const params = new URLSearchParams(queryString)

    async function start() {
      const plan = normalizePlan(params.get("plan"))
      const deploymentMode = normalizeDeploymentMode(params.get("deploymentMode"))
      const token = getStoredToken()

      if (!token) {
        router.replace(`/login?next=${encodeURIComponent(currentPathWithSearch())}`)
        return
      }

      try {
        const response = await apiRequest<MeResponse>("/api/v1/me", { token })
        if (cancelled) {
          return
        }

        const currentPlan = response.entitlement.plan
        const currentRank = planRank(currentPlan)
        const requestedRank = planRank(plan)

        if (currentPlan === plan) {
          setState({
            title: "Your current plan",
            description: `You are already on Runtz ${planLabels[plan]}. No checkout is needed.`,
            currentPlan: plan,
          })
          return
        }

        if (currentRank > requestedRank) {
          setState({
            title: "Included in your current plan",
            description: `Your Runtz ${planLabels[currentPlan]} plan already includes ${planLabels[plan]}.`,
            currentPlan,
          })
          return
        }

        if (plan === "free") {
          setState({
            title: "Free plan ready",
            description: "Your free Runtz workspace is available in the platform.",
            currentPlan: "free",
          })
          return
        }

        setState({
          title: "Opening Stripe Checkout",
          description: response.user.email
            ? `Stripe will use ${response.user.email} for this purchase.`
            : "Stripe will use the email from your Runtz account.",
        })

        const successUrl =
          params.get("successUrl") ||
          `${window.location.origin}/app/settings?tab=billing&billing_checkout_session={CHECKOUT_SESSION_ID}`
        const cancelUrl =
          params.get("cancelUrl") ||
          `${window.location.origin}/app/settings?tab=billing`

        const checkout = await apiRequest<{ url: string }>("/api/v1/billing/checkout", {
          method: "POST",
          token,
          body: {
            plan,
            deploymentMode,
            successUrl,
            cancelUrl,
          },
        })

        window.location.assign(checkout.url)
      } catch (error) {
        if (cancelled) {
          return
        }

        if (error instanceof ApiError && error.status === 401) {
          clearToken()
          router.replace(`/login?next=${encodeURIComponent(currentPathWithSearch())}`)
          return
        }

        setState({
          title: "Checkout unavailable",
          description: "We could not open Stripe Checkout.",
          error: error instanceof Error ? error.message : "Checkout failed",
        })
      }
    }

    start()

    return () => {
      cancelled = true
    }
  }, [queryString, router])

  return (
    <CheckoutShell
      title={state.title}
      description={state.description}
      currentPlan={state.currentPlan}
      error={state.error}
    />
  )
}

function CheckoutShell({
  title,
  description,
  currentPlan,
  error,
}: {
  title: string
  description?: string
  currentPlan?: CheckoutPlan
  error?: string
}) {
  return (
    <main className="relative flex min-h-svh items-center justify-center overflow-hidden bg-[#050912] px-6 py-10 text-[#eaf4ff]">
      <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 opacity-[0.10]" />
      <Card className="relative z-10 w-full max-w-md border-[#6db5ff]/20 bg-[#0d1420]/92 py-6 text-[#eaf4ff] shadow-[0_28px_90px_rgb(0_0_0/0.36)]">
        <CardHeader>
          <CardTitle className="text-xl font-bold">{title}</CardTitle>
          <CardDescription className="text-[#b8cbe4]">
            {error || description || "This should only take a moment."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {error ? (
            <>
              <Link
                href="/app/settings?tab=billing"
                className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[#6db5ff] px-5 text-sm font-bold text-[#071222] transition hover:bg-[#9fd6ff]"
              >
                Billing settings
                <ArrowRightIcon className="size-4" />
              </Link>
              <Link
                href="/app/overview"
                className="inline-flex h-11 items-center justify-center gap-2 rounded-full border border-[#6db5ff]/24 bg-transparent px-5 text-sm font-medium text-[#eaf4ff] transition hover:bg-[#101827] hover:text-[#eaf4ff]"
              >
                <ArrowLeftIcon className="size-4" />
                Back to app
              </Link>
            </>
          ) : currentPlan ? (
            <>
              <Button
                disabled
                className="h-11 rounded-full bg-[#2a3446] px-5 font-bold text-[#8da4c0] opacity-100"
              >
                Your current plan
              </Button>
              <Link
                href="/app/overview"
                className="inline-flex h-11 items-center justify-center gap-2 rounded-full bg-[#6db5ff] px-5 text-sm font-bold text-[#071222] transition hover:bg-[#9fd6ff]"
              >
                Open Runtz
                <ArrowRightIcon className="size-4" />
              </Link>
            </>
          ) : (
            <div className="flex items-center gap-3 text-sm text-[#b8cbe4]">
              <LoaderCircleIcon className="size-5 animate-spin text-[#6db5ff]" />
              Redirecting to Stripe...
            </div>
          )}
        </CardContent>
      </Card>
    </main>
  )
}
