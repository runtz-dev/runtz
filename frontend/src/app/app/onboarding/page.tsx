"use client"

import { useRouter } from "next/navigation"
import * as React from "react"
import {
  ArrowRightIcon,
  CheckIcon,
  CircleIcon,
  CopyIcon,
  KeyRoundIcon,
  ServerIcon,
  TerminalIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest, getStoredToken, type ApiKey } from "@/lib/api"
import { cn } from "@/lib/utils"

const installCommand = "curl -fsSL https://runtz.dev/install.sh | bash"

export default function OnboardingPage() {
  const router = useRouter()
  const { deploymentMode, workspaces } = useWorkspace()
  const [apiKey, setAPIKey] = React.useState("")
  const [copied, setCopied] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const workspace = workspaces[0]
  const endpoint =
    deploymentMode === "cloud"
      ? "https://engine.runtz.dev"
      : "http://localhost:8080"
  const tokenValue = apiKey || "rtz_live_..."
  const scanCommand =
    deploymentMode === "cloud"
      ? `runtz host --token ${tokenValue}`
      : `runtz host --endpoint ${endpoint} --token ${tokenValue}`

  async function createAPIKey() {
    const token = getStoredToken()
    if (!token || !workspace) {
      setError("Initial workspace not found.")
      return
    }

    setPending(true)
    setError("")
    try {
      const response = await apiRequest<{ apiKey: ApiKey; key: string }>(
        "/api/v1/api-keys",
        {
          method: "POST",
          token,
          body: {
            workspaceId: workspace.id,
            name: "Onboarding CLI key",
            scopes: ["ingest:write"],
            expiresInDays: 90,
          },
        }
      )
      setAPIKey(response.key)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create API key")
    } finally {
      setPending(false)
    }
  }

  async function copy(value: string, id: string) {
    await navigator.clipboard.writeText(value)
    setCopied(id)
    window.setTimeout(() => setCopied(""), 1800)
  }

  async function completeOnboarding() {
    const token = getStoredToken()
    if (!token || !apiKey) {
      return
    }

    setPending(true)
    setError("")
    try {
      await apiRequest("/api/v1/me/onboarding", {
        method: "PATCH",
        token,
      })
      router.replace("/app/overview")
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to complete onboarding")
      setPending(false)
    }
  }

  async function skipOnboarding() {
    const token = getStoredToken()
    if (!token) {
      router.replace("/app/overview")
      return
    }
    setPending(true)
    try {
      await apiRequest("/api/v1/me/onboarding", { method: "PATCH", token })
    } catch {
      // best-effort: navigate regardless
    } finally {
      router.replace("/app/overview")
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 md:p-8">
      <div className="max-w-2xl">
        <p className="font-mono text-xs font-semibold uppercase tracking-wide text-primary">
          first steps
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-normal">
          Scan this machine
        </h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Generate a workspace key, install the CLI and let it scan this
          host&apos;s own OS packages with Runtz.
        </p>
      </div>

      <Card>
        <CardContent className="p-5 md:p-8">
          <div className="relative flex flex-col gap-0">
            <div className="absolute top-4 bottom-4 left-[15px] w-px bg-border" />
            <OnboardingStep
              active
              complete={Boolean(apiKey)}
              icon={KeyRoundIcon}
              title="Add an API key"
              description="The key automatically identifies your workspace when the CLI submits a scan. It expires in 90 days."
            >
              {apiKey ? (
                <InlineSecret
                  value={apiKey}
                  copied={copied === "key"}
                  onCopy={() => copy(apiKey, "key")}
                />
              ) : (
                <Button onClick={createAPIKey} disabled={pending || !workspace}>
                  <KeyRoundIcon data-icon="inline-start" />
                  Gerar API key
                </Button>
              )}
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={Boolean(apiKey)}
              icon={TerminalIcon}
              title="Install the CLI"
              description="One command installs the runtz binary and puts it on your PATH."
            >
              <div className="flex flex-col gap-2">
                <CommandLine
                  value={installCommand}
                  copied={copied === "install"}
                  onCopy={() => copy(installCommand, "install")}
                />
                <OSBadge />
              </div>
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={false}
              icon={ServerIcon}
              title="Scan this host"
              description="Run it from anywhere on this machine. The CLI fingerprints the installed OS packages, matches them against known CVEs and sends the report straight to your workspace."
            >
              <CommandLine
                value={scanCommand}
                copied={copied === "scan"}
                onCopy={() => copy(scanCommand, "scan")}
              />
            </OnboardingStep>
          </div>

          {error ? (
            <p className="mt-5 rounded-lg border border-destructive/35 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </p>
          ) : null}

          <div className="mt-8 flex items-center justify-between border-t pt-5">
            <Button variant="ghost" onClick={skipOnboarding} disabled={pending} className="text-muted-foreground">
              Pular por agora
            </Button>
            <Button onClick={completeOnboarding} disabled={pending || !apiKey}>
              Ir para o Overview
              <ArrowRightIcon data-icon="inline-end" />
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function OnboardingStep({
  active,
  complete,
  icon: Icon,
  title,
  description,
  children,
}: {
  active: boolean
  complete: boolean
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  title: string
  description: string
  children: React.ReactNode
}) {
  return (
    <section className={cn("relative grid gap-4 pb-10 pl-14", !active && "opacity-55")}>
      <div
        className={cn(
          "absolute top-0 left-0 z-10 flex size-8 items-center justify-center rounded-full border bg-background",
          active && "border-primary text-primary"
        )}
      >
        {complete ? <CheckIcon className="size-4" /> : <Icon className="size-4" />}
      </div>
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
      {children}
    </section>
  )
}

function OSBadge() {
  return (
    <div className="inline-flex w-fit items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
      <TerminalIcon className="size-3" />
      <AppleIcon className="size-3" />
      <span>Linux · macOS</span>
    </div>
  )
}

function AppleIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 384 512"
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      <path d="M318.7 268.7c-.2-36.7 16.4-64.4 50-84.8-18.8-26.9-47.2-41.7-84.7-44.6-35.5-2.8-74.3 20.7-88.5 20.7-15 0-49.4-19.7-76.4-19.7C63.3 141.2 4 184.8 4 273.5q0 39.3 14.4 81.2c12.8 36.7 59 126.7 107.2 125.2 25.2-.6 43-17.9 75.8-17.9 31.8 0 48.3 17.9 76.4 17.9 48.6-.7 90.4-82.5 102.6-119.3-65.2-30.7-61.7-90-61.7-91.9zm-56.6-164.2c27.3-32.4 24.8-61.9 24-72.5-24.1 1.4-52 16.4-67.9 34.9-17.5 19.8-27.8 44.3-25.6 71.9 26.1 2 49.9-11.4 69.5-34.3z" />
    </svg>
  )
}

function InlineSecret({
  value,
  copied,
  onCopy,
}: {
  value: string
  copied: boolean
  onCopy: () => void
}) {
  return (
    <div className="flex max-w-3xl gap-2">
      <Input className="font-mono text-xs" value={value} readOnly />
      <Button variant="outline" size="icon" onClick={onCopy} aria-label="Copy API key">
        {copied ? <CheckIcon /> : <CopyIcon />}
      </Button>
    </div>
  )
}

function CommandLine({
  value,
  copied,
  onCopy,
}: {
  value: string
  copied: boolean
  onCopy: () => void
}) {
  return (
    <div className="flex max-w-3xl items-start gap-2 rounded-lg border bg-[#050912] p-3 text-[#eaf4ff]">
      <CircleIcon className="mt-1 size-2 shrink-0 fill-[#80d673] text-[#80d673]" />
      <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs leading-5">
        {value}
      </code>
      <Button
        variant="ghost"
        size="icon-sm"
        className="shrink-0 text-[#b8cbe4] hover:bg-[#172844] hover:text-[#eaf4ff]"
        onClick={onCopy}
        aria-label="Copy command"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </Button>
    </div>
  )
}
