"use client"

import * as React from "react"
import {
  ArrowRightIcon,
  ArrowUpRightIcon,
  BookOpenIcon,
  CheckIcon,
  EyeIcon,
  EyeOffIcon,
  GlobeIcon,
  LoaderCircleIcon,
  LockKeyholeIcon,
  ServerIcon,
} from "lucide-react"

import { RuntzWordmark } from "@/components/runtz/logo"
import { ThemeToggle } from "@/components/runtz/theme-provider"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { apiRequest } from "@/lib/api"
import { cn } from "@/lib/utils"

export function SelfHostedAccess({
  configured,
  onAuthenticated,
}: {
  configured: boolean
  onAuthenticated: () => void
}) {
  return (
    <div className="runtz-welcome flex min-h-svh flex-col bg-background text-foreground">
      <header className="mx-auto flex w-full max-w-7xl items-center justify-between px-6 py-6 sm:px-10 sm:py-8">
        <div className="flex items-center gap-4">
          <RuntzWordmark className="text-[25px]" />
          <Separator orientation="vertical" className="my-1" />
          <span className="font-mono text-[11px] tracking-widest text-muted-foreground uppercase">
            Self-hosted
          </span>
        </div>
        <ThemeToggle />
      </header>

      <main className="mx-auto grid w-full max-w-xl flex-1 items-center gap-10 px-6 py-8 sm:px-10 sm:py-12 lg:max-w-6xl lg:grid-cols-[1fr_0.95fr] lg:gap-16 lg:py-16 xl:gap-24">
        <section aria-labelledby="welcome-title" className="relative min-w-0">
          <div aria-hidden="true" className="runtz-welcome-grid pointer-events-none absolute -inset-x-6 -top-16 h-80" />
          <div className="relative flex flex-col items-start">
            <h1 id="welcome-title" className="text-[40px] leading-[1.12] font-semibold sm:text-5xl lg:text-[56px]">
              Your instance.
              <br />
              <span className="text-primary">Your control.</span>
            </h1>
            <p className="mt-5 max-w-sm text-base leading-7 text-muted-foreground">
              {configured
                ? "Your security workspace, on your infrastructure. Sign in to pick up where you left off."
                : "Runtz is up and running on your infrastructure. Just one more step to make it yours."}
            </p>

            {!configured && <SetupProgress />}

            <nav aria-label="Runtz resources" className="mt-8 w-full max-w-sm lg:mt-10">
              <Separator className="mb-5" />
              <div className="flex flex-wrap gap-x-7 gap-y-2">
                <a className="runtz-welcome-link" href="https://runtz.dev/docs" target="_blank" rel="noopener noreferrer">
                  <BookOpenIcon aria-hidden="true" className="size-4" />
                  Documentation
                  <ArrowUpRightIcon aria-hidden="true" className="size-3.5" />
                  <span className="sr-only"> (opens in a new tab)</span>
                </a>
                <a className="runtz-welcome-link" href="https://runtz.dev" target="_blank" rel="noopener noreferrer">
                  <GlobeIcon aria-hidden="true" className="size-4" />
                  runtz.dev
                  <ArrowUpRightIcon aria-hidden="true" className="size-3.5" />
                  <span className="sr-only"> (opens in a new tab)</span>
                </a>
              </div>
            </nav>
          </div>
        </section>

        <section aria-labelledby="access-title" className="runtz-welcome-form relative min-w-0 rounded-2xl border border-border bg-card p-6 sm:p-9 lg:p-10">
          <div className="mb-7 flex flex-col gap-3">
            <div className="mb-2 flex items-center gap-2 font-mono text-[11px] tracking-widest text-muted-foreground uppercase">
              <LockKeyholeIcon aria-hidden="true" className="size-3.5" />
              {configured ? "Your private workspace" : "One-time setup"}
            </div>
            <h2 id="access-title" className="text-[28px] leading-tight font-semibold">
              {configured ? "Welcome back." : "Initial setup"}
            </h2>
            <p className="text-sm leading-6 text-muted-foreground">
              {configured
                ? "Sign in with your account for this installation."
                : "Create your admin account and the first workspace for this installation."}
            </p>
          </div>
          <AccessForm configured={configured} onAuthenticated={onAuthenticated} />
        </section>
      </main>

      <footer className="mx-auto flex w-full max-w-7xl flex-wrap items-center justify-between gap-3 px-6 py-6 text-xs text-muted-foreground sm:px-10">
        <span>Security starts here.</span>
        <span className="flex items-center gap-2">
          <ServerIcon aria-hidden="true" className="size-3.5" />
          Hosted by you. Powered by runtz.
        </span>
      </footer>
    </div>
  )
}

function SetupProgress() {
  const steps = [
    { title: "Deploy runtz", description: "Your instance is up and running.", state: "complete" },
    { title: "Make it yours", description: "Create an admin and a workspace.", state: "current" },
    { title: "Run your first scan", description: "Connect the CLI and start finding risks.", state: "upcoming" },
  ] as const

  return (
    <ol aria-label="Getting started" className="mt-10 hidden w-full flex-col lg:flex">
      {steps.map((step, index) => (
        <li key={step.title} aria-current={step.state === "current" ? "step" : undefined} className="relative flex gap-4 pb-6 last:pb-0">
          {index < steps.length - 1 && <span aria-hidden="true" className="absolute top-8 bottom-0 left-3.5 w-px bg-border" />}
          <span aria-hidden="true" className={cn(
            "relative flex size-7 shrink-0 items-center justify-center rounded-full border font-mono text-xs",
            step.state === "complete" && "border-chart-2/25 bg-chart-2/10 text-chart-2",
            step.state === "current" && "border-primary/40 bg-primary/10 text-primary",
            step.state === "upcoming" && "border-border text-muted-foreground"
          )}>
            {step.state === "complete" ? <CheckIcon className="size-3.5" /> : `0${index + 1}`}
          </span>
          <div className="pt-0.5">
            <p className={cn("text-sm font-medium", step.state === "upcoming" && "text-muted-foreground")}>
              {step.title}
              {step.state === "complete" && <span className="sr-only"> — complete</span>}
            </p>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">{step.description}</p>
          </div>
        </li>
      ))}
    </ol>
  )
}

function AccessForm({ configured, onAuthenticated }: { configured: boolean; onAuthenticated: () => void }) {
  const [username, setUsername] = React.useState(configured ? "" : "admin")
  const [password, setPassword] = React.useState("")
  const [workspaceName, setWorkspaceName] = React.useState("default")
  const [showPassword, setShowPassword] = React.useState(false)
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending) return
    setError("")

    if (!username.trim() || (!configured && !workspaceName.trim())) {
      setError(configured ? "Enter a username, not just spaces." : "Enter a username and workspace name, not just spaces.")
      return
    }

    setPending(true)
    try {
      await apiRequest(configured ? "/api/v1/auth/login" : "/api/v1/setup", {
        method: "POST",
        body: configured
          ? { username: username.trim(), password }
          : { username: username.trim(), password, workspaceName: workspaceName.trim() },
      })
      onAuthenticated()
    } catch (error) {
      setError(error instanceof Error ? error.message : configured ? "Sign-in failed" : "Setup failed")
      setPending(false)
    }
  }

  return (
    <form onSubmit={submit} aria-labelledby="access-title" aria-busy={pending}>
      <FieldGroup className="gap-6">
        <Field data-disabled={pending}>
          <FieldLabel htmlFor="access-username">{configured ? "Username" : "Admin username"}</FieldLabel>
          <Input
            id="access-username"
            name="username"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            className="h-12 px-3.5"
            placeholder="Your username"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            disabled={pending}
            required
          />
        </Field>
        <Field data-disabled={pending}>
          <div className="flex min-h-8 items-center justify-between gap-2">
            <FieldLabel htmlFor="access-password">Password</FieldLabel>
            <Button type="button" variant="ghost" size="sm" className="h-8" aria-label={showPassword ? "Hide password" : "Show password"} aria-controls="access-password" aria-pressed={showPassword} onClick={() => setShowPassword(!showPassword)} disabled={pending}>
              {showPassword ? <EyeOffIcon aria-hidden="true" data-icon="inline-start" /> : <EyeIcon aria-hidden="true" data-icon="inline-start" />}
              {showPassword ? "Hide" : "Show"}
            </Button>
          </div>
          <Input
            id="access-password"
            name="password"
            type={showPassword ? "text" : "password"}
            autoComplete={configured ? "current-password" : "new-password"}
            aria-describedby={configured ? undefined : "password-hint"}
            className="h-12 px-3.5"
            placeholder={configured ? "Enter your password" : "Create a strong password"}
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            minLength={configured ? undefined : 8}
            disabled={pending}
            required
          />
          {!configured && <FieldDescription id="password-hint">Use at least 8 characters.</FieldDescription>}
        </Field>
        {!configured && (
          <Field data-disabled={pending}>
            <FieldLabel htmlFor="access-workspace">Workspace name</FieldLabel>
            <Input
              id="access-workspace"
              name="workspaceName"
              autoComplete="off"
              className="h-12 px-3.5"
              placeholder="Your team or project"
              value={workspaceName}
              onChange={(event) => setWorkspaceName(event.target.value)}
              aria-describedby="workspace-hint"
              disabled={pending}
              required
            />
            <FieldDescription id="workspace-hint">A home for your projects and scan results.</FieldDescription>
          </Field>
        )}
        {error && <FieldError>{error}</FieldError>}
        <Button type="submit" className="h-12 w-full gap-3" disabled={pending}>
          {pending ? (
            <>
              <LoaderCircleIcon aria-hidden="true" data-icon="inline-start" className="motion-safe:animate-spin" />
              {configured ? "Signing in…" : "Creating your workspace…"}
            </>
          ) : (
            <>
              {configured ? "Sign in" : "Create workspace"}
              <ArrowRightIcon aria-hidden="true" data-icon="inline-end" />
            </>
          )}
        </Button>
      </FieldGroup>
      <p className="mt-5 flex items-start justify-center gap-2 text-center text-xs leading-5 text-muted-foreground">
        <LockKeyholeIcon aria-hidden="true" className="mt-1 size-3 shrink-0" />
        {configured ? "Access is managed by your instance administrator." : "Your admin account is local to this installation."}
      </p>
    </form>
  )
}
