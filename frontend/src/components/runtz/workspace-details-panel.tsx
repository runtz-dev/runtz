"use client"

import * as React from "react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { FieldError } from "@/components/ui/field"
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
  refreshKey,
}: {
  workspace: Workspace
  refreshKey: number
}) {
  const { currentUser } = useWorkspace()
  const isOwner = workspace.createdBy === currentUser.id
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
  }, [loadMembers, refreshKey])

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
        <CardHeader>
          <CardTitle>
            <h2 id={`workspace-title-${workspace.id}`} className="break-words">
              Members <span className="font-normal text-muted-foreground">· {workspace.name}</span>
            </h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="flex min-w-0 flex-col gap-4">
            {message ? <p role="status" className="break-words text-sm text-muted-foreground">{message}</p> : null}
            {error ? <FieldError role="alert">{error}</FieldError> : null}
            {loading ? (
              <div aria-label="Loading members" className="flex flex-col gap-4">
                {[0, 1].map((item) => <div key={item} className="flex items-center gap-3"><Skeleton className="h-9 w-48 max-w-full" /></div>)}
              </div>
            ) : loadError ? (
              <div className="flex flex-col items-start gap-3">
                <FieldError role="alert">{loadError}</FieldError>
                <Button variant="outline" size="sm" onClick={() => void loadMembers()}>Retry</Button>
              </div>
            ) : sortedMembers.length === 0 ? (
              <Empty><EmptyHeader><EmptyTitle>No members to display</EmptyTitle><EmptyDescription>Workspace access will appear here.</EmptyDescription></EmptyHeader></Empty>
            ) : (
              <ul className="flex flex-col divide-y divide-border">
                {sortedMembers.map((member) => (
                  <li key={member.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
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
        </CardContent>
      </Card>
    </section>
  )
}
