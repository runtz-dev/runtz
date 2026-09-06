"use client"

import * as React from "react"
import { LockKeyholeIcon, RefreshCcwIcon, Trash2Icon, UsersIcon } from "lucide-react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { WorkspaceSharingDialog } from "@/components/runtz/workspace-sharing-dialog"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { FieldError } from "@/components/ui/field"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest, type Workspace } from "@/lib/api"

type WorkspaceMember = {
  id: string
  email: string
  name: string
  role: "owner" | "member"
}

export function WorkspaceDetailsPanel({
  workspace,
  onDelete,
  onUpgrade,
}: {
  workspace: Workspace
  onDelete: () => void
  onUpgrade: () => void
}) {
  const { currentUser, entitlement } = useWorkspace()
  const isOwner = workspace.createdBy === currentUser.id
  const canShare = entitlement.plan === "pro" || entitlement.plan === "enterprise"
  const [members, setMembers] = React.useState<WorkspaceMember[]>([])
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState("")
  const [error, setError] = React.useState("")
  const [message, setMessage] = React.useState("")
  const [removingId, setRemovingId] = React.useState<string | null>(null)
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
    const controller = new AbortController()
    void loadMembers(controller.signal)
    return () => controller.abort()
  }, [loadMembers])

  async function removeMember(member: WorkspaceMember) {
    if (removingId) return
    setRemovingId(member.id)
    setError("")
    setMessage("")
    try {
      await apiRequest(`${endpoint}/${member.id}`, { method: "DELETE" })
      setMembers((current) => current.filter((item) => item.id !== member.id))
      setMessage(`Access removed for ${member.email || member.name}.`)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to remove member")
    } finally {
      setRemovingId(null)
    }
  }

  const sortedMembers = [...members].sort((a, b) =>
    Number(b.role === "owner") - Number(a.role === "owner") || (a.name || a.email).localeCompare(b.name || b.email)
  )

  return (
    <section id="workspace-details" aria-labelledby={`workspace-title-${workspace.id}`}>
      <Card>
        <CardHeader className="gap-4 sm:flex sm:items-center sm:justify-between">
          <div className="flex min-w-0 flex-col gap-1.5">
            <p className="text-xs font-medium text-muted-foreground">Workspace details</p>
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <CardTitle><h2 id={`workspace-title-${workspace.id}`} className="break-all">{workspace.name}</h2></CardTitle>
              <Badge variant="outline">{isOwner ? "Owner" : "Shared"}</Badge>
            </div>
            <CardDescription>{isOwner ? "Manage your workspace and the people with access." : "A shared workspace you belong to."}</CardDescription>
          </div>
          {isOwner ? (
            <div className="shrink-0 self-start sm:self-center">
              {canShare ? (
                <WorkspaceSharingDialog workspace={workspace} disabled={Boolean(removingId)} onAdded={(email) => {
                  setError("")
                  setMessage(`Access granted to ${email}.`)
                  void loadMembers()
                }} />
              ) : (
                <Button variant="outline" onClick={onUpgrade}>
                  <LockKeyholeIcon data-icon="inline-start" />
                  Upgrade to add members
                </Button>
              )}
            </div>
          ) : null}
        </CardHeader>
        <Separator />
        <CardContent className="grid gap-6 lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-8">
          <section aria-label="Workspace information" className="flex min-w-0 flex-col gap-4 lg:border-r lg:pr-8">
            <h3 className="text-sm font-medium">Overview</h3>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-5 lg:grid-cols-1">
              <div className="min-w-0">
                <dt className="text-xs text-muted-foreground">Slug</dt>
                <dd className="mt-1.5 break-all text-sm">{workspace.slug}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">Created</dt>
                <dd className="mt-1.5 text-sm">{new Intl.DateTimeFormat("en-US", { dateStyle: "medium" }).format(new Date(workspace.createdAt))}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">Your access</dt>
                <dd className="mt-1.5 text-sm">{isOwner ? "Workspace owner" : "Workspace member"}</dd>
              </div>
              <div>
                <dt className="text-xs text-muted-foreground">Members</dt>
                <dd className="mt-1.5 text-sm">{loading ? <Skeleton className="h-5 w-8" /> : loadError ? "Unavailable" : members.length}</dd>
              </div>
            </dl>
          </section>
          <section aria-label="Workspace members" className="flex min-w-0 flex-col gap-4">
            <div>
              <h3 className="flex items-center gap-2 text-sm font-medium"><UsersIcon className="size-4 text-muted-foreground" />Members</h3>
              <p className="mt-1 text-sm text-muted-foreground">Members can view scans and manage workspace API keys.</p>
            </div>
            {message ? <p role="status" className="break-words text-sm text-muted-foreground">{message}</p> : null}
            {error ? <FieldError role="alert">{error}</FieldError> : null}
            {loading ? (
              <div aria-label="Loading members" className="flex flex-col gap-4">
                {[0, 1].map((item) => <div key={item} className="flex items-center gap-3"><Skeleton className="size-8 rounded-full" /><Skeleton className="h-9 w-48 max-w-full" /></div>)}
              </div>
            ) : loadError ? (
              <div className="flex flex-col items-start gap-3">
                <FieldError role="alert">{loadError}</FieldError>
                <Button variant="outline" size="sm" onClick={() => void loadMembers()}><RefreshCcwIcon data-icon="inline-start" />Retry</Button>
              </div>
            ) : sortedMembers.length === 0 ? (
              <Empty><EmptyHeader><EmptyTitle>No members to display</EmptyTitle><EmptyDescription>Workspace access will appear here.</EmptyDescription></EmptyHeader></Empty>
            ) : (
              <ul className="flex flex-col divide-y divide-border">
                {sortedMembers.map((member) => (
                  <li key={member.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
                    <Avatar><AvatarFallback>{(member.name || member.email).slice(0, 2).toUpperCase()}</AvatarFallback></Avatar>
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium" title={member.name || member.email}>{member.name || member.email}{member.id === currentUser.id ? <span className="ml-1 font-normal text-muted-foreground">(you)</span> : null}</p>
                      <p className="truncate text-xs text-muted-foreground" title={member.email}>{member.email}</p>
                    </div>
                    <Badge variant={member.role === "owner" ? "secondary" : "outline"}>{member.role === "owner" ? "Owner" : "Member"}</Badge>
                    {isOwner && member.role !== "owner" ? (
                      <Button variant="ghost" size="sm" disabled={Boolean(removingId)} onClick={() => void removeMember(member)} aria-label={`Remove access for ${member.email || member.name}`}>
                        {removingId === member.id ? "Removing…" : "Remove"}
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            )}
            {!isOwner ? <p className="text-xs text-muted-foreground">Only the workspace owner can add or remove members.</p> : null}
          </section>
        </CardContent>
        {isOwner ? (
          <CardFooter className="flex-col items-start justify-between gap-3 sm:flex-row sm:items-center">
            <p className="text-xs text-muted-foreground">Deleting this workspace permanently removes its scans and API keys.</p>
            <Button variant="destructive" size="sm" disabled={Boolean(removingId)} onClick={onDelete}><Trash2Icon data-icon="inline-start" />Delete workspace</Button>
          </CardFooter>
        ) : null}
      </Card>
    </section>
  )
}
