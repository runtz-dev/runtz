"use client"

import { useRouter } from "next/navigation"
import * as React from "react"
import {
  ArrowRightIcon,
  CheckIcon,
  CircleIcon,
  CopyIcon,
  KeyRoundIcon,
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

const installCommand = "curl -fsSL https://get.runtz.dev | bash"

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
  const scanCommand = `runtz sca ./ --endpoint ${endpoint} --token ${
    apiKey || "rtz_live_..."
  }`

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
          Run your first scan
        </h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Generate a workspace key, install the CLI and scan your
          repository&apos;s dependencies with Runtz.
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
              description="The key automatically identifies your workspace when the CLI submits a scan."
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
              description="The script installs the runtz binary to /usr/local/bin. Confirm with runtz version."
            >
              <CommandLine
                value={installCommand}
                copied={copied === "install"}
                onCopy={() => copy(installCommand, "install")}
              />
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={false}
              icon={TerminalIcon}
              title="Run an SCA scan"
              description="From the root of your repository, run the command below. The CLI discovers the supported manifests (package.json, requirements.txt, go.mod...) and the token already points to the right workspace."
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
