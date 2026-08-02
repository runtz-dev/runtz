"use client"

import { useRouter, useSearchParams } from "next/navigation"
import * as React from "react"
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  CheckCircle2Icon,
  MailIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { RuntzWordmark } from "@/components/runtz/logo"
import { ThemeToggle } from "@/components/runtz/theme-provider"
import { apiRequest, clearToken, getStoredToken, storeToken } from "@/lib/api"
import { DEFAULT_GOOGLE_CLIENT_ID } from "@/lib/google"
import { cn } from "@/lib/utils"

type DeploymentMode = "cloud" | "self-hosted"

type SetupStatusResponse = {
  configured: boolean
  deploymentMode: DeploymentMode
  auth?: {
    email: boolean
    google: boolean
    github: boolean
    githubClientId: string
    googleClientId?: string
  }
}

type AuthResponse = {
  token: string
}

const BUILD_TIME_GOOGLE_CLIENT_ID =
  process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID?.trim() || DEFAULT_GOOGLE_CLIENT_ID

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (options: {
            client_id: string
            callback: (response: { credential?: string }) => void
          }) => void
          renderButton: (
            element: HTMLElement,
            options: {
              type?: "standard" | "icon"
              theme?: "outline" | "filled_blue" | "filled_black"
              size?: "large" | "medium" | "small"
              shape?: "rectangular" | "pill" | "circle" | "square"
              text?: "signin_with" | "signup_with" | "continue_with" | "signin"
              logo_alignment?: "left" | "center"
              locale?: string
              width?: number
            }
          ) => void
        }
      }
    }
  }
}

const authCardClassName =
  "relative w-full border-[#6db5ff]/20 bg-[#0d1420]/92 py-6 text-[#eaf4ff] shadow-[0_28px_90px_rgb(0_0_0/0.36)] backdrop-blur"
const authInputClassName =
  "h-11 rounded-xl border-[#6db5ff]/22 bg-[#050912]/65 px-3 text-[#eaf4ff] placeholder:text-[#7f93ad] focus-visible:border-[#6db5ff] focus-visible:ring-[#6db5ff]/30"
const authButtonClassName =
  "h-11 rounded-full bg-[#6db5ff] px-5 font-bold text-[#071222] shadow-lg shadow-black/20 hover:bg-[#9fd6ff] focus-visible:ring-[#6db5ff]/40"

function safeNextPath(value: string | null) {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "/app/overview"
  }

  return value
}

export function SetupLogin() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const nextPath = safeNextPath(searchParams.get("next"))
  const [status, setStatus] = React.useState<SetupStatusResponse | null>(null)
  const [restoringSession, setRestoringSession] = React.useState(true)

  React.useEffect(() => {
    apiRequest<SetupStatusResponse>("/api/v1/setup/status")
      .then(setStatus)
      .catch(() =>
        setStatus({ configured: true, deploymentMode: "self-hosted" })
      )
  }, [])

  // The session token lives in localStorage, so a second tab (or coming back
  // from the landing page) is still signed in — send it straight to the app
  // instead of asking for the credentials again. A token the engine rejects is
  // dropped here so the form is shown.
  React.useEffect(() => {
    const token = getStoredToken()
    if (!token) {
      setRestoringSession(false)
      return
    }

    let cancelled = false
    apiRequest("/api/v1/me", { token })
      .then(() => {
        if (!cancelled) {
          router.replace(nextPath)
        }
      })
      .catch(() => {
        clearToken()
        if (!cancelled) {
          setRestoringSession(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [nextPath, router])

  return (
    <div className="relative min-h-svh overflow-hidden bg-[#050912] text-[#eaf4ff]">
      <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 opacity-[0.10]" />
      <div className="absolute top-4 right-4 z-20">
        <ThemeToggle />
      </div>
      <main className="relative z-10 flex min-h-svh items-center justify-center px-6 py-8">
        <div className="w-full max-w-md">
          <div className="mb-8 flex justify-center">
            <RuntzWordmark
              className="text-[26px] text-[#eaf4ff] dark:text-[#eaf4ff]"
              cursorClassName="bg-[#6db5ff]"
            />
          </div>
          {!status || restoringSession ? (
            <LoginSkeleton />
          ) : status.deploymentMode === "cloud" ? (
            <CloudLoginForm
              githubClientId={status.auth?.githubClientId ?? ""}
              googleClientId={status.auth?.googleClientId || BUILD_TIME_GOOGLE_CLIENT_ID}
              nextPath={nextPath}
              onAuthenticated={() => router.replace(nextPath)}
            />
          ) : status.configured ? (
            <SelfHostedLoginForm
              onAuthenticated={() => router.replace(nextPath)}
            />
          ) : (
            <SetupForm onConfigured={() => router.replace(nextPath)} />
          )}
        </div>
      </main>
    </div>
  )
}

function LoginSkeleton() {
  return (
    <Card className={authCardClassName}>
      <CardHeader>
        <Skeleton className="h-7 w-28 bg-[#172844]" />
        <Skeleton className="h-4 w-64 bg-[#172844]" />
      </CardHeader>
      <CardContent>
        <Skeleton className="h-48 w-full bg-[#172844]" />
      </CardContent>
    </Card>
  )
}

function CloudLoginForm({
  githubClientId,
  googleClientId,
  nextPath,
  onAuthenticated,
}: {
  githubClientId: string
  googleClientId: string
  nextPath: string
  onAuthenticated: () => void
}) {
  const [email, setEmail] = React.useState("")
  const [code, setCode] = React.useState("")
  const [codeSent, setCodeSent] = React.useState(false)
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  async function signInWithGoogle(credential: string) {
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<AuthResponse>("/api/v1/auth/google", {
        method: "POST",
        body: { credential },
      })
      storeToken(response.token)
      onAuthenticated()
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Google sign-in failed"
      )
    } finally {
      setPending(false)
    }
  }

  async function requestCode(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/auth/email/request", {
        method: "POST",
        body: { email },
      })
      setCodeSent(true)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to send code")
    } finally {
      setPending(false)
    }
  }

  async function verifyCode(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<AuthResponse>("/api/v1/auth/email/verify", {
        method: "POST",
        body: { email, code },
      })
      storeToken(response.token)
      onAuthenticated()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Invalid code")
    } finally {
      setPending(false)
    }
  }

  return (
    <Card className={authCardClassName}>
      <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 z-0 opacity-[0.18]" />
      <CardHeader className="relative z-10">
        <CardTitle className="text-xl font-bold">Acesse a Runtz</CardTitle>
        <CardDescription className="text-[#b8cbe4]">
          Sign in without a password. We will email you a code.
        </CardDescription>
      </CardHeader>
      <CardContent className="relative z-10">
        {!codeSent ? (
          <div className="flex flex-col gap-4">
            <div className="grid gap-3">
              <GoogleSignInButton
                clientId={googleClientId}
                disabled={pending}
                onCredential={signInWithGoogle}
                onError={setError}
              />
              <GitHubLoginButton
                clientId={githubClientId}
                disabled={pending}
                nextPath={nextPath}
              />
            </div>
            <div className="my-1 flex items-center gap-3 text-xs text-[#9fb4cf]">
              <div className="h-px flex-1 bg-[#6db5ff]/18" />
              <span>ou continue com e-mail</span>
              <div className="h-px flex-1 bg-[#6db5ff]/18" />
            </div>
            <form onSubmit={requestCode}>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="login-email">E-mail</FieldLabel>
                  <Input
                    id="login-email"
                    type="email"
                    autoComplete="email"
                    placeholder="voce@empresa.com"
                    className={authInputClassName}
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    required
                  />
                </Field>
                <LoginError message={error} />
                <Button type="submit" className={authButtonClassName} disabled={pending}>
                  <MailIcon data-icon="inline-start" />
                  Send code
                </Button>
              </FieldGroup>
            </form>
          </div>
        ) : (
          <form onSubmit={verifyCode}>
            <FieldGroup>
              <div className="rounded-xl border border-[#6db5ff]/18 bg-[#050912]/55 p-4">
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <CheckCircle2Icon className="size-4 text-[#80d673]" />
                  Code sent
                </div>
                <p className="mt-2 text-sm text-[#b8cbe4]">
                  Confira a caixa de entrada de <strong>{email}</strong>.
                </p>
              </div>
              <Field>
                <FieldLabel htmlFor="login-code">Access code</FieldLabel>
                <Input
                  id="login-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  placeholder="000000"
                  className={cn(authInputClassName, "font-mono text-lg tracking-[0.35em]")}
                  value={code}
                  onChange={(event) =>
                    setCode(event.target.value.replace(/\D/g, "").slice(0, 6))
                  }
                  required
                />
                <FieldDescription className="text-[#9fb4cf]">
                  The code expires in 10 minutes.
                </FieldDescription>
              </Field>
              <LoginError message={error} />
              <Button
                type="submit"
                className={authButtonClassName}
                disabled={pending || code.length !== 6}
              >
                Entrar
                <ArrowRightIcon data-icon="inline-end" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="text-[#b8cbe4] hover:text-[#eaf4ff]"
                onClick={() => {
                  setCode("")
                  setCodeSent(false)
                  setError("")
                }}
              >
                <ArrowLeftIcon data-icon="inline-start" />
                Usar outro e-mail
              </Button>
            </FieldGroup>
          </form>
        )}
      </CardContent>
    </Card>
  )
}

function GitHubLoginButton({
  clientId,
  disabled,
  nextPath,
}: {
  clientId: string
  disabled?: boolean
  nextPath: string
}) {
  function signIn() {
    const state = crypto.randomUUID()
    const redirectURI = `${window.location.origin}/login/github/callback`
    sessionStorage.setItem("runtz_github_oauth_state", state)
    sessionStorage.setItem("runtz_login_next", nextPath)
    const authorizationURL = new URL("https://github.com/login/oauth/authorize")
    authorizationURL.searchParams.set("client_id", clientId)
    authorizationURL.searchParams.set("redirect_uri", redirectURI)
    authorizationURL.searchParams.set("scope", "user:email")
    authorizationURL.searchParams.set("state", state)
    window.location.assign(authorizationURL.toString())
  }

  const unavailable = disabled || !clientId
  return (
    <button
      type="button"
      disabled={unavailable}
      onClick={signIn}
      className={cn(
        "relative flex h-12 w-full items-center justify-center rounded-full border border-[#6db5ff]/22 bg-[#050912]/65 px-5 text-sm font-medium text-[#eaf4ff] shadow-sm transition hover:-translate-y-0.5 hover:border-[#6db5ff]/45 hover:bg-[#101827] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#6db5ff]/30",
        unavailable && "cursor-not-allowed opacity-60"
      )}
    >
      <span className="absolute left-4 flex size-7 items-center justify-center rounded-full border border-[#6db5ff]/24 bg-[#1b2333]">
        <svg viewBox="0 0 24 24" className="size-4 fill-[#eaf4ff]" aria-hidden="true">
          <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
        </svg>
      </span>
      <span>{clientId ? "Continue with GitHub" : "GitHub unavailable"}</span>
    </button>
  )
}

function GoogleSignInButton({
  clientId,
  disabled,
  onCredential,
  onError,
}: {
  clientId: string
  disabled: boolean
  onCredential: (credential: string) => void
  onError: (message: string) => void
}) {
  const buttonRef = React.useRef<HTMLDivElement | null>(null)
  const disabledRef = React.useRef(disabled)
  const onCredentialRef = React.useRef(onCredential)
  const onErrorRef = React.useRef(onError)
  const [ready, setReady] = React.useState(false)

  React.useEffect(() => {
    disabledRef.current = disabled
    onCredentialRef.current = onCredential
    onErrorRef.current = onError
  }, [disabled, onCredential, onError])

  React.useEffect(() => {
    if (!clientId || !buttonRef.current) {
      return
    }

    let cancelled = false
    let started = false
    let googleScript: HTMLScriptElement | null = null
    let resizeObserver: ResizeObserver | null = null
    let resizeFrame = 0

    function renderGoogleButton() {
      if (cancelled || !buttonRef.current || !window.google) {
        return
      }
      const width = Math.min(
        Math.floor(buttonRef.current.getBoundingClientRect().width) || 360,
        400
      )
      buttonRef.current.innerHTML = ""
      window.google.accounts.id.initialize({
        client_id: clientId,
        callback: (response) => {
          if (disabledRef.current) {
            return
          }
          if (!response.credential) {
            onErrorRef.current("Google credential was not returned")
            return
          }
          onCredentialRef.current(response.credential)
        },
      })
      window.google.accounts.id.renderButton(buttonRef.current, {
        type: "standard",
        theme: "outline",
        size: "large",
        shape: "pill",
        text: "continue_with",
        logo_alignment: "left",
        locale: "pt_BR",
        width,
      })
      setReady(true)
    }

    function startGoogleButton() {
      if (started) {
        return
      }
      started = true
      renderGoogleButton()
      if (buttonRef.current && "ResizeObserver" in window) {
        resizeObserver = new ResizeObserver(() => {
          window.cancelAnimationFrame(resizeFrame)
          resizeFrame = window.requestAnimationFrame(renderGoogleButton)
        })
        resizeObserver.observe(buttonRef.current)
      }
    }

    function handleGoogleScriptError() {
      onErrorRef.current("Failed to load Google sign-in")
    }

    if (window.google) {
      startGoogleButton()
    } else {
      const existingScript = document.querySelector<HTMLScriptElement>(
        'script[src="https://accounts.google.com/gsi/client"]'
      )
      const script = existingScript ?? document.createElement("script")
      googleScript = script
      script.src = "https://accounts.google.com/gsi/client"
      script.async = true
      script.defer = true
      script.addEventListener("load", startGoogleButton)
      script.addEventListener("error", handleGoogleScriptError)
      if (!existingScript) {
        document.head.appendChild(script)
      }
    }

    return () => {
      cancelled = true
      window.cancelAnimationFrame(resizeFrame)
      resizeObserver?.disconnect()
      googleScript?.removeEventListener("load", startGoogleButton)
      googleScript?.removeEventListener("error", handleGoogleScriptError)
    }
  }, [clientId])

  return (
    <div
      aria-disabled={disabled || !ready}
      className={cn(
        "group/google relative flex h-12 w-full items-center justify-center overflow-hidden rounded-full border border-[#6db5ff]/22 bg-[#050912]/65 px-5 text-sm font-medium text-[#eaf4ff] shadow-sm transition hover:-translate-y-0.5 hover:border-[#6db5ff]/45 hover:bg-[#101827] focus-within:ring-2 focus-within:ring-[#6db5ff]/30",
        (disabled || !ready) && "pointer-events-none opacity-60"
      )}
    >
      <span className="absolute left-4 flex size-7 items-center justify-center rounded-full border border-[#6db5ff]/24 bg-[#eaf4ff] text-[18px] font-semibold leading-none text-[#071222]">
        G
      </span>
      <span>{ready ? "Continue with Google" : "Loading Google..."}</span>
      <div
        className="absolute inset-0 z-10 opacity-[0.01] [&>div]:!h-full [&>div]:!w-full [&_iframe]:!h-full [&_iframe]:!w-full"
        ref={buttonRef}
      />
    </div>
  )
}

function SelfHostedLoginForm({
  onAuthenticated,
}: {
  onAuthenticated: () => void
}) {
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<AuthResponse>("/api/v1/auth/login", {
        method: "POST",
        body: { username, password },
      })
      storeToken(response.token)
      onAuthenticated()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Sign-in failed")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthFormCard
      title="Login self-hosted"
      description="Use the admin user configured for this installation."
    >
      <form onSubmit={submit}>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="login-username">Username</FieldLabel>
            <Input
              id="login-username"
              autoComplete="username"
              className={authInputClassName}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="login-password">Password</FieldLabel>
            <Input
              id="login-password"
              type="password"
              autoComplete="current-password"
              className={authInputClassName}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </Field>
          <LoginError message={error} />
          <Button type="submit" className={authButtonClassName} disabled={pending}>
            Entrar
            <ArrowRightIcon data-icon="inline-end" />
          </Button>
        </FieldGroup>
      </form>
    </AuthFormCard>
  )
}

function SetupForm({ onConfigured }: { onConfigured: () => void }) {
  const [username, setUsername] = React.useState("admin")
  const [password, setPassword] = React.useState("")
  const [workspaceName, setWorkspaceName] = React.useState("default")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<AuthResponse>("/api/v1/setup", {
        method: "POST",
        body: { username, password, workspaceName },
      })
      storeToken(response.token)
      onConfigured()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Setup failed")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthFormCard
      title="Initial setup"
      description="Create the admin user and the first workspace for this installation."
    >
      <form onSubmit={submit}>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="setup-username">Admin username</FieldLabel>
            <Input
              id="setup-username"
              className={authInputClassName}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="setup-password">Password</FieldLabel>
            <Input
              id="setup-password"
              type="password"
              className={authInputClassName}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              minLength={8}
              required
            />
            <FieldDescription className="text-[#9fb4cf]">
              Minimum of 8 characters.
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="setup-workspace">Workspace</FieldLabel>
            <Input
              id="setup-workspace"
              className={authInputClassName}
              value={workspaceName}
              onChange={(event) => setWorkspaceName(event.target.value)}
              required
            />
          </Field>
          <LoginError message={error} />
          <Button type="submit" className={authButtonClassName} disabled={pending}>
            Criar admin
            <ArrowRightIcon data-icon="inline-end" />
          </Button>
        </FieldGroup>
      </form>
    </AuthFormCard>
  )
}

function AuthFormCard({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children: React.ReactNode
}) {
  return (
    <Card className={authCardClassName}>
      <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 z-0 opacity-[0.18]" />
      <CardHeader className="relative z-10">
        <CardTitle className="text-xl font-bold">{title}</CardTitle>
        <CardDescription className="text-[#b8cbe4]">{description}</CardDescription>
      </CardHeader>
      <CardContent className="relative z-10">{children}</CardContent>
    </Card>
  )
}

function LoginError({ message }: { message: string }) {
  return message ? (
    <Field>
      <FieldError>{message}</FieldError>
    </Field>
  ) : null
}
