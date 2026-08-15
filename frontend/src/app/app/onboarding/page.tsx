"use client"

import { useRouter } from "next/navigation"
import * as React from "react"
import {
  ArrowRightIcon,
  BlocksIcon,
  CheckIcon,
  CircleIcon,
  CopyIcon,
  ExternalLinkIcon,
  KeyRoundIcon,
  LogInIcon,
  RocketIcon,
  TerminalIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Card,
  CardContent,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest, type ApiKey } from "@/lib/api"
import { cn } from "@/lib/utils"

const installCommands = {
  unix: "curl -fsSL https://runtz.dev/install.sh | bash",
  windows: "irm https://runtz.dev/install.ps1 | iex",
} as const

const vscodeExtensionUrl = "vscode:extension/runtz.runtz-security"
const vscodeMarketplaceUrl =
  "https://marketplace.visualstudio.com/items?itemName=Runtz.runtz-security"

export default function OnboardingPage() {
  const router = useRouter()
  const { deploymentMode, workspaces } = useWorkspace()
  const [apiKey, setAPIKey] = React.useState("")
  const [copied, setCopied] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const [hideOnboarding, setHideOnboarding] = React.useState(false)
  const [installOS, setInstallOS] = React.useState<keyof typeof installCommands>("unix")
  const workspace = workspaces[0]
  const endpoint =
    deploymentMode === "cloud"
      ? "https://engine.runtz.dev"
      : "http://localhost:8080"
  const tokenValue = apiKey || "rtz_live_..."
  const loginCommand =
    deploymentMode === "cloud"
      ? `runtz login --token ${tokenValue}`
      : `runtz login --endpoint ${endpoint} --token ${tokenValue}`
  const installCommand = installCommands[installOS]

  async function createAPIKey() {
    if (!workspace) {
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

  async function leaveOnboarding(requireAPIKey: boolean) {
    if (requireAPIKey && !apiKey) {
      return
    }

    setPending(true)
    setError("")
    try {
      if (hideOnboarding) {
        await apiRequest("/api/v1/me/onboarding", {
          method: "PATCH",
        })
      }
      router.replace("/app/overview")
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to complete onboarding")
      setPending(false)
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 md:p-8">
      <div className="max-w-2xl">
        <p className="font-mono text-xs font-semibold uppercase tracking-wide text-primary">
          first steps
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-normal">
          Start scanning with Runtz
        </h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          Connect your workspace, install the tools that fit your workflow,
          and start scanning from the terminal or VS Code.
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
                  Generate API key
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
                <div className="inline-flex w-fit gap-1 rounded-full border p-1">
                  <Button
                    type="button"
                    variant={installOS === "unix" ? "default" : "ghost"}
                    size="sm"
                    className="rounded-full"
                    onClick={() => setInstallOS("unix")}
                  >
                    Linux & macOS
                  </Button>
                  <Button
                    type="button"
                    variant={installOS === "windows" ? "default" : "ghost"}
                    size="sm"
                    className="rounded-full"
                    onClick={() => setInstallOS("windows")}
                  >
                    Windows (PowerShell)
                  </Button>
                </div>
                <CommandLine
                  value={installCommand}
                  copied={copied === "install"}
                  onCopy={() => copy(installCommand, "install")}
                />
                <OSBadge os={installOS} />
              </div>
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={Boolean(apiKey)}
              icon={LogInIcon}
              title="Log in once"
              description="runtz login verifies the key and stores it locally (~/.config/runtz), so scan commands no longer need --token. In CI/CD, keep passing the key from a secret instead."
            >
              <CommandLine
                value={loginCommand}
                copied={copied === "login"}
                onCopy={() => copy(loginCommand, "login")}
              />
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={false}
              icon={BlocksIcon}
              title="Install the VS Code extension"
              description="Run SAST and dependency scans without leaving your editor."
              optional
            >
              <div className="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
                <Button
                  className="w-fit"
                  render={<a href={vscodeExtensionUrl} />}
                >
                  <BlocksIcon data-icon="inline-start" />
                  Install in VS Code
                </Button>
                <Button
                  variant="ghost"
                  className="w-fit text-muted-foreground"
                  render={
                    <a
                      href={vscodeMarketplaceUrl}
                      target="_blank"
                      rel="noreferrer"
                    />
                  }
                >
                  View in Marketplace
                  <ExternalLinkIcon data-icon="inline-end" />
                </Button>
              </div>
            </OnboardingStep>

            <OnboardingStep
              active={Boolean(apiKey)}
              complete={Boolean(apiKey)}
              icon={RocketIcon}
              title="You’re ready to start scanning"
              description="Run your first scan from the terminal or open Runtz in VS Code whenever you’re ready."
            />
          </div>

          {error ? (
            <p className="mt-5 rounded-lg border border-destructive/35 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </p>
          ) : null}

          <div className="mt-8 flex flex-col gap-4 border-t pt-5 sm:flex-row sm:items-center sm:justify-between">
            <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
              <Checkbox
                checked={hideOnboarding}
                onCheckedChange={(checked) => setHideOnboarding(Boolean(checked))}
                disabled={pending}
              />
              Don&apos;t show onboarding again
            </label>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              <Button
                variant="ghost"
                onClick={() => leaveOnboarding(false)}
                disabled={pending}
                className="text-muted-foreground"
              >
                Skip for now
              </Button>
              <Button onClick={() => leaveOnboarding(true)} disabled={pending || !apiKey}>
                Go to Overview
                <ArrowRightIcon data-icon="inline-end" />
              </Button>
            </div>
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
  optional = false,
}: {
  active: boolean
  complete: boolean
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  title: string
  description?: string
  children?: React.ReactNode
  optional?: boolean
}) {
  return (
    <section
      className={cn(
        "relative grid gap-4 pb-10 pl-14 last:pb-0",
        !active && "opacity-55"
      )}
    >
      <div
        className={cn(
          "absolute top-0 left-0 z-10 flex size-8 items-center justify-center rounded-full border bg-background",
          active && "border-primary text-primary"
        )}
      >
        {complete ? <CheckIcon className="size-4" /> : <Icon className="size-4" />}
      </div>
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold">{title}</h2>
          {optional ? (
            <span className="rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
              Optional
            </span>
          ) : null}
        </div>
        {description ? (
          <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">
            {description}
          </p>
        ) : null}
      </div>
      {children ?? null}
    </section>
  )
}

function OSBadge({ os }: { os: "unix" | "windows" }) {
  return (
    <div className="inline-flex w-fit items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
      <TerminalIcon className="size-3" />
      {os === "unix" ? (
        <>
          <AppleIcon className="size-3" />
          <span>Linux · macOS</span>
        </>
      ) : (
        <>
          <WindowsIcon className="size-3" />
          <span>Windows (PowerShell)</span>
        </>
      )}
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

function WindowsIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      <path d="M3 5.5 10.5 4.4V11H3V5.5Z" />
      <path d="M11.5 4.26 21 3v7.9h-9.5V4.26Z" />
      <path d="M3 12h7.5v6.6L3 17.5V12Z" />
      <path d="M11.5 12H21v9l-9.5-1.32V12Z" />
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
