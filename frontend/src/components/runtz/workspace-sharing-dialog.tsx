"use client"

import * as React from "react"
import { Share2Icon } from "lucide-react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest, type Workspace } from "@/lib/api"

type WorkspaceMember = {
  id: string
  email: string
  name: string
  role: "owner" | "member"
}

export function WorkspaceSharingDialog({ workspace, onUpgrade }: { workspace: Workspace; onUpgrade: () => void }) {
  const { entitlement } = useWorkspace()
  const canShare = entitlement.plan === "pro" || entitlement.plan === "enterprise"
  const [open, setOpen] = React.useState(false)
  const [members, setMembers] = React.useState<WorkspaceMember[]>([])
  const [email, setEmail] = React.useState("")
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState("")
  const [error, setError] = React.useState("")
  const [message, setMessage] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const endpoint = `/api/v1/workspaces/${workspace.id}/members`

  const loadMembers = React.useCallback(async (signal?: AbortSignal) => {
    setLoading(true)
    setLoadError("")
    try {
      const response = await apiRequest<{ members: WorkspaceMember[] }>(endpoint, { signal })
      if (!signal?.aborted) setMembers(response.members)
    } catch (error) {
      if (!signal?.aborted) setLoadError(error instanceof Error ? error.message : "Failed to load members")
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [endpoint])

  React.useEffect(() => {
    if (!open) return
    const controller = new AbortController()
    void loadMembers(controller.signal)
    return () => controller.abort()
  }, [open, loadMembers])

  function handleOpenChange(nextOpen: boolean) {
    if (pending) return
    setOpen(nextOpen)
    setEmail("")
    setError("")
    setMessage("")
  }

  async function shareWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending || !canShare) return
    setPending(true)
    setError("")
    setMessage("")
    try {
      const address = email.trim().toLowerCase()
      await apiRequest(endpoint, { method: "POST", body: { email: address } })
      setEmail("")
      setMessage(`Access granted to ${address}.`)
      await loadMembers()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to share workspace")
    } finally {
      setPending(false)
    }
  }

  async function removeMember(member: WorkspaceMember) {
    setPending(true)
    setError("")
    setMessage("")
    try {
      await apiRequest(`${endpoint}/${member.id}`, { method: "DELETE" })
      setMembers((current) => current.filter((item) => item.id !== member.id))
      setMessage(`Access removed for ${member.email || member.name}.`)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to remove member")
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <Share2Icon data-icon="inline-start" />
        Share
      </DialogTrigger>
      <DialogContent className="max-h-[85svh] overflow-y-auto sm:max-w-lg" showCloseButton={!pending}>
        <DialogHeader>
          <DialogTitle>Share workspace</DialogTitle>
          <DialogDescription className="break-words">
            Manage access to <strong>{workspace.name}</strong>. Members can view scans and manage workspace API keys.
          </DialogDescription>
        </DialogHeader>
        {canShare ? (
          <form onSubmit={shareWorkspace}>
            <FieldGroup>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor={`share-email-${workspace.id}`}>Email address</FieldLabel>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <Input
                    id={`share-email-${workspace.id}`}
                    type="email"
                    autoComplete="email"
                    placeholder="colleague@company.com"
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    required
                    disabled={pending}
                    aria-invalid={Boolean(error)}
                    aria-describedby={`share-help-${workspace.id}`}
                  />
                  <Button type="submit" disabled={pending || !email.trim()}>
                    {pending ? "Saving…" : "Add member"}
                  </Button>
                </div>
                <FieldDescription id={`share-help-${workspace.id}`}>
                  Use the email of an existing Runtz account. Access is granted immediately.
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        ) : (
          <div className="flex flex-col items-start gap-3">
            <p className="text-sm text-muted-foreground">Upgrade to Pro to add members. You can still remove existing access below.</p>
            <Button onClick={() => { handleOpenChange(false); onUpgrade() }}>Upgrade to Pro</Button>
          </div>
        )}
        {error ? <FieldError role="alert">{error}</FieldError> : null}
        {message ? <p role="status" className="break-words text-sm text-muted-foreground">{message}</p> : null}
        <section aria-label="People with access" className="flex flex-col gap-3">
          <h3 className="text-sm font-medium">People with access</h3>
          {loading ? <Skeleton className="h-16 w-full" /> : loadError ? (
            <div className="flex flex-col items-start gap-2">
              <FieldError role="alert">{loadError}</FieldError>
              <Button variant="outline" size="sm" onClick={() => void loadMembers()}>Retry</Button>
            </div>
          ) : (
            <ul className="flex max-h-64 flex-col gap-3 overflow-y-auto">
              {members.map((member) => (
                <li key={member.id} className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{member.name || member.email}</p>
                    <p className="truncate text-xs text-muted-foreground" title={member.email}>{member.email}</p>
                  </div>
                  {member.role === "owner" ? <Badge variant="secondary">Owner</Badge> : (
                    <Button variant="outline" size="sm" disabled={pending} onClick={() => void removeMember(member)} aria-label={`Remove access for ${member.email || member.name}`}>
                      Remove
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
        <DialogFooter>
          <Button variant="outline" disabled={pending} onClick={() => handleOpenChange(false)}>Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
