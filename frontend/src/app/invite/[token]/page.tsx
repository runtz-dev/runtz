"use client"

import { useParams, useRouter } from "next/navigation"
import * as React from "react"
import { ArrowRightIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { RuntzWordmark } from "@/components/runtz/logo"
import { ThemeToggle } from "@/components/runtz/theme-provider"
import { apiRequest } from "@/lib/api"

// Same dark auth shell as /login (components/runtz/setup-login.tsx) — kept
// as literal classes here rather than importing from that file, since its
// helpers are private to that module.
const authCardClassName =
  "relative w-full border-[#6db5ff]/20 bg-[#0d1420]/92 py-6 text-[#eaf4ff] shadow-[0_28px_90px_rgb(0_0_0/0.36)] backdrop-blur"
const authInputClassName =
  "h-11 rounded-xl border-[#6db5ff]/22 bg-[#050912]/65 px-3 text-[#eaf4ff] placeholder:text-[#7f93ad] focus-visible:border-[#6db5ff] focus-visible:ring-[#6db5ff]/30"
const authButtonClassName =
  "h-11 rounded-full bg-[#6db5ff] px-5 font-bold text-[#071222] shadow-lg shadow-black/20 hover:bg-[#9fd6ff] focus-visible:ring-[#6db5ff]/40"

type InviteResponse = {
  username: string
}

export default function InvitePage() {
  const params = useParams<{ token: string }>()
  const router = useRouter()
  const token = params.token
  const [invite, setInvite] = React.useState<InviteResponse | null>(null)
  const [loadError, setLoadError] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [confirmPassword, setConfirmPassword] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  React.useEffect(() => {
    apiRequest<InviteResponse>(`/api/v1/invites/${token}`)
      .then(setInvite)
      .catch((error) =>
        setLoadError(
          error instanceof Error ? error.message : "Invite link is invalid or has expired"
        )
      )
  }, [token])

  async function acceptInvite(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    if (password !== confirmPassword) {
      setError("Passwords do not match")
      return
    }

    setPending(true)
    try {
      await apiRequest(`/api/v1/invites/${token}/accept`, {
        method: "POST",
        body: { password },
      })
      router.replace("/overview")
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to accept invite")
    } finally {
      setPending(false)
    }
  }

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
          <Card className={authCardClassName}>
            <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 z-0 opacity-[0.18]" />
            {!invite && !loadError ? (
              <>
                <CardHeader className="relative z-10">
                  <Skeleton className="h-7 w-28 bg-[#172844]" />
                  <Skeleton className="h-4 w-64 bg-[#172844]" />
                </CardHeader>
                <CardContent className="relative z-10">
                  <Skeleton className="h-32 w-full bg-[#172844]" />
                </CardContent>
              </>
            ) : loadError ? (
              <>
                <CardHeader className="relative z-10">
                  <CardTitle className="text-xl font-bold">Invite link invalid</CardTitle>
                  <CardDescription className="text-[#b8cbe4]">{loadError}</CardDescription>
                </CardHeader>
                <CardContent className="relative z-10">
                  <p className="text-sm text-[#b8cbe4]">
                    Ask an admin on this installation for a new invite link.
                  </p>
                </CardContent>
              </>
            ) : (
              <>
                <CardHeader className="relative z-10">
                  <CardTitle className="text-xl font-bold">
                    Set a password for {invite?.username}
                  </CardTitle>
                  <CardDescription className="text-[#b8cbe4]">
                    Choose a password to finish setting up your account.
                  </CardDescription>
                </CardHeader>
                <CardContent className="relative z-10">
                  <form onSubmit={acceptInvite}>
                    <FieldGroup>
                      <Field>
                        <FieldLabel htmlFor="invite-password">Password</FieldLabel>
                        <Input
                          id="invite-password"
                          type="password"
                          autoComplete="new-password"
                          className={authInputClassName}
                          value={password}
                          onChange={(event) => setPassword(event.target.value)}
                          minLength={8}
                          required
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor="invite-confirm-password">Confirm password</FieldLabel>
                        <Input
                          id="invite-confirm-password"
                          type="password"
                          autoComplete="new-password"
                          className={authInputClassName}
                          value={confirmPassword}
                          onChange={(event) => setConfirmPassword(event.target.value)}
                          minLength={8}
                          required
                        />
                      </Field>
                      {error ? (
                        <Field>
                          <FieldError>{error}</FieldError>
                        </Field>
                      ) : null}
                      <Button type="submit" className={authButtonClassName} disabled={pending}>
                        Set password and sign in
                        <ArrowRightIcon data-icon="inline-end" />
                      </Button>
                    </FieldGroup>
                  </form>
                </CardContent>
              </>
            )}
          </Card>
        </div>
      </main>
    </div>
  )
}
