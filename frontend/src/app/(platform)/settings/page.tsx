"use client"

import * as React from "react"
import {
  ArrowUpRightIcon,
  CopyIcon,
  CreditCardIcon,
  CrownIcon,
  FolderKanbanIcon,
  KeyRoundIcon,
  PlusIcon,
  RefreshCcwIcon,
  SendIcon,
  Trash2Icon,
  TriangleAlertIcon,
  UsersIcon,
} from "lucide-react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { SettingsEmpty, SettingsFeedback, SettingsPending, SettingsSearch, SettingsSection } from "@/components/runtz/settings-ui"
import { WorkspaceDetailsPanel } from "@/components/runtz/workspace-details-panel"
import { WorkspaceSharingDialog } from "@/components/runtz/workspace-sharing-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardFooter,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  apiRequest,
  clearClientState,
  type Entitlement,
  type User,
  type Workspace,
} from "@/lib/api"

type SettingsTab =
  | "profile"
  | "workspaces"
  | "usage"
  | "billing"
  | "users"
  | "account"

const settingsTabs = [
  { value: "profile", label: "Profile" },
  { value: "workspaces", label: "Workspaces" },
  { value: "usage", label: "Usage" },
  { value: "billing", label: "Billing" },
  { value: "account", label: "Account" },
  { value: "users", label: "Users" },
] as const

type WorkspaceDeletionImpact = {
  workspaceId: string
  workspaceName: string
  scanCount: number
  apiKeyCount: number
  otherMemberCount: number
}

type AccountDeletionImpact = {
  confirmationValue: string
  ownedWorkspaceCount: number
  sharedWorkspaceCount: number
  scanCount: number
  apiKeyCount: number
  sharedOwnedWorkspaceCount: number
  subscriptionWillBeCanceled: boolean
  canDelete: boolean
}

export default function SettingsPage() {
  const { deploymentMode } = useWorkspace()
  const isCloud = deploymentMode === "cloud"
  const [activeTab, setActiveTab] = React.useState<SettingsTab>(
    defaultSettingsTab(isCloud)
  )

  React.useEffect(() => {
    function syncTabFromURL() {
      const tab = new URLSearchParams(window.location.search).get("tab")
      setActiveTab(isSettingsTab(tab, isCloud) ? tab : defaultSettingsTab(isCloud))
    }
    syncTabFromURL()

    function handleTabEvent(event: Event) {
      const nextTab = (event as CustomEvent<SettingsTab>).detail
      if (isSettingsTab(nextTab, isCloud)) {
        setActiveTab(nextTab)
        writeSettingsTabToURL(nextTab, isCloud)
      }
    }

    window.addEventListener("runtz-settings-tab", handleTabEvent)
    window.addEventListener("popstate", syncTabFromURL)
    return () => {
      window.removeEventListener("runtz-settings-tab", handleTabEvent)
      window.removeEventListener("popstate", syncTabFromURL)
    }
  }, [isCloud])

  function handleTabChange(tab: string) {
    if (!isSettingsTab(tab, isCloud)) {
      return
    }
    setActiveTab(tab)
    writeSettingsTabToURL(tab, isCloud)
  }

  return (
    <div className="flex min-w-0 flex-col gap-6 p-4 md:p-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Settings</h1>
        <p className="text-sm text-muted-foreground">
          {isCloud
            ? "Manage your profile, workspaces and subscription in one place."
            : "Manage your profile, workspaces and access for this installation."}
        </p>
      </div>
      <Tabs value={activeTab} onValueChange={handleTabChange} className="min-w-0 gap-6">
        <div className="min-w-0 border-b pb-px">
          <TabsList aria-label="Settings sections" variant="line" className="flex w-full flex-wrap justify-start gap-1 p-0 group-data-horizontal/tabs:h-auto sm:gap-4">
            {settingsTabs.filter(({ value }) => isSettingsTab(value, isCloud)).map(({ value, label }) => (
              <TabsTrigger key={value} value={value} className="h-9 flex-none rounded-none px-1 text-xs data-active:text-primary dark:data-active:text-primary after:bg-primary group-data-horizontal/tabs:after:bottom-0 sm:px-2 sm:text-sm">
                {label}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        <TabsContent value="profile" className="min-w-0">
          <ProfilePanel />
        </TabsContent>
        <TabsContent value="workspaces" className="min-w-0">
          <WorkspacesPanel />
        </TabsContent>
        <TabsContent value="usage" className="min-w-0">
          <UsagePanel />
        </TabsContent>
        <TabsContent value="billing" className="min-w-0">
          <BillingPanel />
        </TabsContent>
        {isCloud ? (
          <TabsContent value="account" className="min-w-0">
            <AccountPanel />
          </TabsContent>
        ) : null}
        {isCloud ? null : (
          <TabsContent value="users" className="min-w-0">
            <UsersPanel />
          </TabsContent>
        )}
      </Tabs>
    </div>
  )
}

function defaultSettingsTab(isCloud: boolean): SettingsTab {
  return isCloud ? "profile" : "workspaces"
}

function isSettingsTab(value: string | null | undefined, isCloud: boolean): value is SettingsTab {
  if (
    value === "profile" ||
    value === "workspaces" ||
    value === "usage" ||
    value === "billing"
  ) {
    return true
  }
  if (isCloud && value === "account") {
    return true
  }
  return !isCloud && value === "users"
}

function writeSettingsTabToURL(tab: SettingsTab, isCloud: boolean) {
  const url = new URL(window.location.href)
  if (tab === defaultSettingsTab(isCloud)) {
    url.searchParams.delete("tab")
  } else {
    url.searchParams.set("tab", tab)
  }
  window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`)
}

function openBillingTab() {
  const url = new URL(window.location.href)
  url.searchParams.set("tab", "billing")
  window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`)
  window.dispatchEvent(new CustomEvent<SettingsTab>("runtz-settings-tab", { detail: "billing" }))
}

function WorkspacesPanel() {
  const { deploymentMode, entitlement, workspaces, refreshWorkspaces } = useWorkspace()
  const [name, setName] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const [createOpen, setCreateOpen] = React.useState(false)
  const [search, setSearch] = React.useState("")
  const [message, setMessage] = React.useState("")
  const visibleWorkspaces = workspaces.filter((workspace) =>
    `${workspace.name} ${workspace.slug}`.toLowerCase().includes(search.toLowerCase())
  )

  async function createWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setError("")
    setMessage("")
    setPending(true)
    try {
      await apiRequest("/api/v1/workspaces", {
        method: "POST",
        body: { name },
      })
      setName("")
      await refreshWorkspaces()
      setCreateOpen(false)
      setMessage("Workspace created.")
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Failed to create workspace"
      )
    } finally {
      setPending(false)
    }
  }

  if (deploymentMode === "cloud") {
    return <CloudWorkspacesPanel />
  }
  if (!entitlement.features.includes("multiple_workspaces")) {
    return <SelfHostedWorkspacesLimitPanel />
  }

  return (
    <div className="space-y-4">
      {message ? <SettingsFeedback>{message}</SettingsFeedback> : null}
      <Dialog open={createOpen} onOpenChange={(open) => { if (!pending) { setCreateOpen(open); setError("") } }}>
        <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
          <DialogHeader>
            <DialogTitle>New workspace</DialogTitle>
            <DialogDescription>Create environments to separate results.</DialogDescription>
          </DialogHeader>
          <form onSubmit={createWorkspace} aria-busy={pending}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="workspace-name">Workspace Name</FieldLabel>
                <Input
                  id="workspace-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="e.g. Production"
                  required
                />
              </Field>
              {error ? (
                <Field>
                  <FieldError>{error}</FieldError>
                </Field>
              ) : null}
              <Button type="submit" disabled={pending}>
                {pending ? <SettingsPending>Creating workspace…</SettingsPending> : <><PlusIcon data-icon="inline-start" />Create workspace</>}
              </Button>
            </FieldGroup>
          </form>
        </DialogContent>
      </Dialog>
      <SettingsSection
        title="Workspaces"
        description={`${workspaces.length} registered`}
        action={<Button onClick={() => setCreateOpen(true)}><PlusIcon data-icon="inline-start" />New workspace</Button>}
      >
          <SettingsSearch value={search} onChange={setSearch} label="Search workspaces" />
          {visibleWorkspaces.length ? <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visibleWorkspaces.map((workspace) => (
                <TableRow key={workspace.id}>
                  <TableCell className="font-medium">{workspace.name}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{workspace.slug}</TableCell>
                  <TableCell>{formatDate(workspace.createdAt)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table> : <SettingsEmpty icon={FolderKanbanIcon} title={search ? "No matching workspaces" : "No workspaces yet"} description={search ? "Try a different name or slug." : "Create a workspace to organize your scan results."} />}
      </SettingsSection>
    </div>
  )
}

function SelfHostedWorkspacesLimitPanel() {
  const { workspaces } = useWorkspace()

  return (
    <SettingsSection
      title="Current workspace"
      description="This installation uses a shared workspace on the current plan."
      action={<Button variant="outline" onClick={openBillingTab}>
            <PlusIcon data-icon="inline-start" />
            Create another workspace
          </Button>}
    >
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Created</TableHead>
                <TableHead>Plan</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workspaces.map((workspace) => (
                <TableRow key={workspace.id}>
                  <TableCell className="font-medium">{workspace.name}</TableCell>
                  <TableCell>{workspace.slug}</TableCell>
                  <TableCell>{formatDate(workspace.createdAt)}</TableCell>
                  <TableCell>
                    <Badge variant="outline">initial workspace</Badge>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
    </SettingsSection>
  )
}

function CloudWorkspacesPanel() {
  const { currentUser, entitlement, workspaces, selectedWorkspaceId, refreshWorkspaces } = useWorkspace()
  const [activeWorkspaceId, setActiveWorkspaceId] = React.useState(selectedWorkspaceId)
  const [membersVersion, setMembersVersion] = React.useState(0)
  const canShareWorkspace = entitlement.plan === "pro" || entitlement.plan === "enterprise"
  const activeWorkspace = workspaces.find((workspace) => workspace.id === activeWorkspaceId) ?? workspaces[0]
  const [name, setName] = React.useState("personal")
  const [pending, setPending] = React.useState(false)
  const [error, setError] = React.useState("")
  const [createOpen, setCreateOpen] = React.useState(false)
  const ownedWorkspaceCount = workspaces.filter((workspace) => workspace.createdBy === currentUser.id).length
  const workspaceLimit = entitlement.plan === "free" ? 1 : entitlement.plan === "pro" ? 5 : Infinity
  const canCreateWorkspace = ownedWorkspaceCount < workspaceLimit

  async function createWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPending(true)
    setError("")
    try {
      const response = await apiRequest<{ workspace: Workspace }>("/api/v1/workspaces", {
        method: "POST",
        body: { name: name.trim() || "personal" },
      })
      await refreshWorkspaces()
      setActiveWorkspaceId(response.workspace.id)
      setName("personal")
      setCreateOpen(false)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create workspace")
    } finally {
      setPending(false)
    }
  }

  const [workspaceToDelete, setWorkspaceToDelete] = React.useState<Workspace | null>(null)

  return (
    <div className="flex flex-col gap-6">
      <Dialog open={createOpen} onOpenChange={(open) => {
        if (pending) return
        setCreateOpen(open)
        if (open) {
          setError("")
          setName("personal")
        }
      }}>
        <SettingsSection
          title={`Your workspaces (${workspaces.length})`}
          description="Select a workspace to view its members."
          action={<div className="flex flex-wrap items-center gap-3">
                {canCreateWorkspace ? (
                  <DialogTrigger render={<Button variant="outline" size="sm" className="h-10 sm:h-8" />}>
                    <PlusIcon data-icon="inline-start" />
                    New workspace
                  </DialogTrigger>
                ) : null}
              </div>}
        >
            {workspaces.length === 0 ? (
              <Empty>
                <EmptyHeader>
                  <EmptyTitle>No workspaces yet</EmptyTitle>
                  <EmptyDescription>Select New workspace to organize your scans and get started.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className="flex flex-col gap-2">
                <div aria-hidden="true" className="grid grid-cols-[minmax(0,1fr)_120px] gap-3 px-3 text-xs text-muted-foreground lg:grid-cols-[minmax(0,1fr)_80px_120px_120px]">
                  <span>Workspace</span>
                  <span className="hidden lg:block">Type</span>
                  <span className="hidden lg:block">Created</span>
                  <span className="text-right">Actions</span>
                </div>
                <ul aria-label="Workspaces" className="flex max-h-72 flex-col gap-1 overflow-y-auto p-1">
                  {workspaces.map((workspace) => {
                    const isActive = workspace.id === activeWorkspace?.id
                    const isOwner = workspace.createdBy === currentUser.id
                    return (
                      <li key={workspace.id} className="relative">
                        <Button
                          variant={isActive ? "secondary" : "ghost"}
                          className="absolute inset-0 size-full"
                          aria-label={`View ${workspace.name} workspace`}
                          aria-pressed={isActive}
                          aria-controls="workspace-details"
                          onClick={() => setActiveWorkspaceId(workspace.id)}
                        >
                          <span className="sr-only">{workspace.name}</span>
                        </Button>
                        <div className="pointer-events-none relative grid grid-cols-[minmax(0,1fr)_120px] items-center gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_80px_120px_120px]">
                          <div className="flex min-w-0 flex-col items-start gap-1.5">
                            <span className="w-full truncate text-sm font-medium" title={workspace.name}>{workspace.name}</span>
                            <Badge variant="outline" className="lg:hidden">{isOwner ? "Owner" : "Shared"}</Badge>
                          </div>
                          <Badge variant="outline" className="hidden justify-self-start lg:inline-flex">{isOwner ? "Owner" : "Shared"}</Badge>
                          <span className="hidden text-xs text-muted-foreground lg:block">{new Intl.DateTimeFormat("en-US", { dateStyle: "medium" }).format(new Date(workspace.createdAt))}</span>
                          <div className="pointer-events-auto flex justify-end gap-2">
                            {isOwner ? (
                              <>
                                {canShareWorkspace ? (
                                  <WorkspaceSharingDialog
                                    workspace={workspace}
                                    onOpen={() => setActiveWorkspaceId(workspace.id)}
                                    onAdded={() => {
                                      setActiveWorkspaceId(workspace.id)
                                      setMembersVersion((version) => version + 1)
                                    }}
                                  />
                                ) : (
                                  <Button variant="outline" size="sm" onClick={() => { setActiveWorkspaceId(workspace.id); openBillingTab() }}>Share</Button>
                                )}
                                <Button variant="destructive" size="sm" onClick={() => setWorkspaceToDelete(workspace)}>Delete</Button>
                              </>
                            ) : null}
                          </div>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              </div>
            )}
        </SettingsSection>
        <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
          <DialogHeader>
            <DialogTitle>New workspace</DialogTitle>
            <DialogDescription>
              {entitlement.plan === "free"
                ? "Your Free plan includes one workspace. Use the default name or choose your own."
                : "Give your workspace a name to keep your scans organized."}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={createWorkspace} className="grid gap-5">
            <Field data-invalid={Boolean(error)}>
              <FieldLabel htmlFor="cloud-workspace-name">Workspace name</FieldLabel>
              <Input
                id="cloud-workspace-name"
                value={name}
                placeholder="personal"
                onChange={(event) => setName(event.target.value)}
                disabled={pending}
                aria-invalid={Boolean(error)}
                aria-describedby={error ? "cloud-workspace-error" : undefined}
              />
              {error ? <FieldError id="cloud-workspace-error" role="alert">{error}</FieldError> : null}
            </Field>
            <DialogFooter>
              <Button type="button" variant="outline" disabled={pending} onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={pending || !canCreateWorkspace}>
                {pending ? "Creating workspace…" : "Create workspace"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      {activeWorkspace ? (
        <WorkspaceDetailsPanel
          key={activeWorkspace.id}
          workspace={activeWorkspace}
          refreshKey={membersVersion}
        />
      ) : null}
      <WorkspaceDeletionDialog
        workspace={workspaceToDelete}
        open={Boolean(workspaceToDelete)}
        onOpenChange={(open) => {
          if (!open) {
            setWorkspaceToDelete(null)
          }
        }}
        onDeleted={refreshWorkspaces}
      />
    </div>
  )
}

function WorkspaceDeletionDialog({
  workspace,
  open,
  onOpenChange,
  onDeleted,
}: {
  workspace: Workspace | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted: () => Promise<void>
}) {
  const [impact, setImpact] = React.useState<WorkspaceDeletionImpact | null>(null)
  const [confirmation, setConfirmation] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  React.useEffect(() => {
    if (!open || !workspace) {
      setImpact(null)
      setConfirmation("")
      setError("")
      return
    }

    let cancelled = false
    setImpact(null)
    setConfirmation("")
    setError("")
    apiRequest<WorkspaceDeletionImpact>(
      `/api/v1/workspaces/${workspace.id}/deletion-impact`
    )
      .then((response) => {
        if (!cancelled) {
          setImpact(response)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setError(
            error instanceof Error
              ? error.message
              : "Failed to calculate deletion impact"
          )
        }
      })

    return () => {
      cancelled = true
    }
  }, [open, workspace])

  async function deleteWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!workspace) {
      return
    }

    setError("")
    setPending(true)
    try {
      await apiRequest(`/api/v1/workspaces/${workspace.id}`, {
        method: "DELETE",
        body: { confirmation },
      })
      await onDeleted()
      onOpenChange(false)
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Failed to delete workspace"
      )
    } finally {
      setPending(false)
    }
  }

  const confirmed = Boolean(
    workspace && confirmation.trim() === workspace.name.trim()
  )

  return (
    <Dialog open={open} onOpenChange={pending ? undefined : onOpenChange}>
      <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
        <DialogHeader>
          <div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-destructive/10 text-destructive">
            <TriangleAlertIcon className="size-5" />
          </div>
          <DialogTitle>Delete workspace?</DialogTitle>
          <DialogDescription>
            This permanently deletes the workspace and all scan data stored in it.
            This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={deleteWorkspace}>
          <FieldGroup>
            {impact ? (
              <div className="flex flex-col gap-2 rounded-lg border bg-muted/35 p-3 text-sm">
                <p><strong>{impact.scanCount}</strong> scans will be deleted.</p>
                <p><strong>{impact.apiKeyCount}</strong> API keys will be revoked.</p>
                {impact.otherMemberCount > 0 ? (
                  <p>
                    <strong>{impact.otherMemberCount}</strong> other members will lose access.
                  </p>
                ) : null}
                <p>You can create a new workspace later from Settings → Workspaces.</p>
              </div>
            ) : error ? null : (
              <Skeleton className="h-24 w-full" />
            )}
            <Field data-invalid={Boolean(error)}>
              <FieldLabel htmlFor="delete-workspace-confirmation">
                Type <strong>{workspace?.name}</strong> to confirm
              </FieldLabel>
              <Input
                id="delete-workspace-confirmation"
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
                autoComplete="off"
                aria-invalid={Boolean(error)}
                disabled={pending}
              />
              {error ? <FieldError>{error}</FieldError> : null}
            </Field>
          </FieldGroup>
          <DialogFooter className="mt-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={pending || !impact || !confirmed}
            >
              <Trash2Icon data-icon="inline-start" />
              {pending ? "Deleting..." : "Delete workspace and data"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function UsersPanel() {
  const { workspaces } = useWorkspace()
  const [users, setUsers] = React.useState<User[]>([])
  const [username, setUsername] = React.useState("")
  const [email, setEmail] = React.useState("")
  const [role, setRole] = React.useState<"admin" | "viewer">("viewer")
  const [workspaceId, setWorkspaceId] = React.useState("")
  const [error, setError] = React.useState("")
  const [inviteLink, setInviteLink] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const [createOpen, setCreateOpen] = React.useState(false)
  const [search, setSearch] = React.useState("")
  const [loading, setLoading] = React.useState(true)
  const [loadError, setLoadError] = React.useState("")
  const [actionError, setActionError] = React.useState("")
  const [actionPending, setActionPending] = React.useState(false)
  const [copied, setCopied] = React.useState(false)
  const [copyError, setCopyError] = React.useState("")
  const visibleUsers = users.filter((user) =>
    `${user.username} ${user.role}`.toLowerCase().includes(search.toLowerCase())
  )

  const loadUsers = React.useCallback(async () => {

    setLoading(true)
    setLoadError("")
    try {
      const response = await apiRequest<{ users: User[] }>("/api/v1/users")
      setUsers(response.users ?? [])
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to load users")
    } finally {
      setLoading(false)
    }
  }, [])

  React.useEffect(() => {
    void loadUsers()
  }, [loadUsers])

  React.useEffect(() => {
    if (!workspaceId && workspaces[0]) {
      setWorkspaceId(workspaces[0].id)
    }
  }, [workspaceId, workspaces])

  async function createUser(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setError("")
    setPending(true)
    try {
      const response = await apiRequest<{ inviteLink: string }>("/api/v1/users", {
        method: "POST",
        body: {
          username,
          email: email || undefined,
          role,
          workspaceIds: workspaceId ? [workspaceId] : [],
        },
      })
      setUsername("")
      setEmail("")
      setCreateOpen(false)
      setCopied(false)
      setCopyError("")
      setInviteLink(response.inviteLink)
      await loadUsers()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create user")
    } finally {
      setPending(false)
    }
  }

  async function createInvite(user: User) {
    setActionPending(true)
    setActionError("")
    setCopied(false)
    setCopyError("")
    try {
      const response = await apiRequest<{ inviteLink: string }>(
        `/api/v1/users/${user.id}/invite`,
        { method: "POST" }
      )
      setInviteLink(response.inviteLink)
    } catch (error) {
      setActionError(error instanceof Error ? error.message : "Failed to generate invite")
    } finally {
      setActionPending(false)
    }
  }

  async function copyInvite() {
    setCopyError("")
    try {
      await navigator.clipboard.writeText(inviteLink)
      setCopied(true)
    } catch {
      setCopyError("Unable to copy. Select the link and copy it manually.")
    }
  }

  return (
    <div className="space-y-4">
      {actionError ? <SettingsFeedback error>{actionError}</SettingsFeedback> : null}
      <Dialog open={createOpen} onOpenChange={(open) => { if (!pending) { setCreateOpen(open); setError("") } }}>
        <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-md" showCloseButton={!pending}>
          <DialogHeader>
            <DialogTitle>New user</DialogTitle>
            <DialogDescription>
              Only a username is required — they set their own password from the
              invite link.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={createUser} aria-busy={pending}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="user-username">Username</FieldLabel>
                <Input
                  id="user-username"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="user-email">Email (optional)</FieldLabel>
                <Input
                  id="user-email"
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="user-role">Role</FieldLabel>
                <Select
                  value={role}
                  onValueChange={(value) => {
                    if (value === "admin" || value === "viewer") {
                      setRole(value)
                    }
                  }}
                >
                  <SelectTrigger id="user-role" className="w-full">
                    <SelectValue placeholder="Role" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="viewer">viewer</SelectItem>
                      <SelectItem value="admin">admin</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="user-workspace">Workspace</FieldLabel>
                <Select
                  value={workspaceId}
                  onValueChange={(value) => {
                    if (value) {
                      setWorkspaceId(value)
                    }
                  }}
                >
                  <SelectTrigger id="user-workspace" className="w-full">
                    <SelectValue placeholder="Workspace" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {workspaces.map((workspace) => (
                        <SelectItem key={workspace.id} value={workspace.id}>
                          {workspace.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              {error ? (
                <Field>
                  <FieldError>{error}</FieldError>
                </Field>
              ) : null}
              <Button type="submit" disabled={pending}>
                {pending ? <SettingsPending>Creating user…</SettingsPending> : <><PlusIcon data-icon="inline-start" />Create user</>}
              </Button>
            </FieldGroup>
          </form>
        </DialogContent>
      </Dialog>
      <SettingsSection
        title="Users & access"
        description={`${users.length} registered`}
        action={<div className="flex flex-wrap gap-2">
            <Button variant="outline" disabled={loading || actionPending} onClick={() => void loadUsers()}>
              <RefreshCcwIcon data-icon="inline-start" />
              Refresh
            </Button>
            <Button onClick={() => setCreateOpen(true)}><PlusIcon data-icon="inline-start" />New user</Button>
          </div>}
      >
          <SettingsSearch value={search} onChange={setSearch} label="Search users or roles" />
          {loadError ? <SettingsFeedback error>{loadError}</SettingsFeedback> : null}
          {loading ? <div role="status" className="flex items-center justify-center gap-2 py-8 text-muted-foreground"><SettingsPending>Loading users…</SettingsPending></div> : loadError ? null : visibleUsers.length ? <>
          <ul className="divide-y md:hidden">
            {visibleUsers.map((user) => (
              <li key={user.id} className="space-y-3 py-4 first:pt-0 last:pb-0">
                <div className="flex items-center justify-between gap-3">
                  <p className="min-w-0 truncate font-medium">{user.username}</p>
                  <Badge variant={user.role === "admin" ? "default" : "secondary"}>{user.role}</Badge>
                </div>
                <Badge variant={user.passwordSet ? "secondary" : "outline"}>{user.passwordSet ? "Active" : "Invited"}</Badge>
                <div><UserInviteButton user={user} pending={actionPending} onInvite={createInvite} /></div>
              </li>
            ))}
          </ul>
          <div className="hidden overflow-x-auto md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visibleUsers.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium">{user.username}</TableCell>
                    <TableCell>
                      <Badge
                        variant={user.role === "admin" ? "default" : "secondary"}
                      >
                        {user.role}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={user.passwordSet ? "secondary" : "outline"}>
                        {user.passwordSet ? "Active" : "Invited"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <UserInviteButton user={user} pending={actionPending} onInvite={createInvite} />
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          </> : <SettingsEmpty icon={UsersIcon} title={search ? "No matching users" : "No users yet"} description={search ? "Try a different username or role." : "Create a user to share access to this installation."} />}
      </SettingsSection>

      <Dialog open={Boolean(inviteLink)} onOpenChange={(open) => !open && setInviteLink("")}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Invitation link</DialogTitle>
            <DialogDescription>
              Link generated for the selected user.
            </DialogDescription>
          </DialogHeader>
          <div className="flex gap-2">
            <Input aria-label="Invitation link" value={inviteLink} readOnly onFocus={(event) => event.target.select()} />
            <Button
              size="icon"
              variant="outline"
              aria-label={copied ? "Invitation link copied" : "Copy invitation link"}
              onClick={copyInvite}
            >
              <CopyIcon />
            </Button>
          </div>
          {copied ? <SettingsFeedback>Invitation link copied.</SettingsFeedback> : null}
          {copyError ? <SettingsFeedback error>{copyError}</SettingsFeedback> : null}
        </DialogContent>
      </Dialog>
    </div>
  )
}

function UserInviteButton({ user, pending, onInvite }: {
  user: User
  pending: boolean
  onInvite: (user: User) => Promise<void>
}) {
  return (
    <Button variant="outline" size="sm" disabled={pending} aria-label={`${user.passwordSet ? "Reset" : "Resend"} invite for ${user.username}`} onClick={() => void onInvite(user)}>
      <SendIcon data-icon="inline-start" />{user.passwordSet ? "Reset" : "Resend"} invite
    </Button>
  )
}

type UsageWindow = {
  total: number
}

// A "total used / limit" pair. limit is -1 (unlimitedLimit on the engine
// side) for Enterprise, which is negotiated/custom rather than capped.
type AccountLimit = {
  total: number
  limit: number
}

type UsageResponse = {
  weekly: UsageWindow
  monthly: UsageWindow
  limits: {
    weekly: number
    monthly: number
  }
  plan: "free" | "pro" | "enterprise"
  workspaces: AccountLimit
  users: AccountLimit
  generatedAt: string
}

const EMPTY_USAGE_WINDOW: UsageWindow = { total: 0 }
const EMPTY_ACCOUNT_LIMIT: AccountLimit = { total: 0, limit: 0 }

function UsagePanel() {
  const { selectedWorkspaceId } = useWorkspace()
  const [usage, setUsage] = React.useState<UsageResponse | null>(null)
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(true)

  const loadUsage = React.useCallback(async () => {

    setPending(true)
    setError("")
    try {
      const query =
        selectedWorkspaceId && selectedWorkspaceId !== "all"
          ? `?workspaceId=${encodeURIComponent(selectedWorkspaceId)}`
          : ""
      setUsage(await apiRequest<UsageResponse>(`/api/v1/usage${query}`))
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to load usage")
    } finally {
      setPending(false)
    }
  }, [selectedWorkspaceId])

  React.useEffect(() => {
    loadUsage()
  }, [loadUsage])

  const weekly = usage?.weekly ?? EMPTY_USAGE_WINDOW
  const monthly = usage?.monthly ?? EMPTY_USAGE_WINDOW
  const weeklyLimit = usage?.limits.weekly ?? 0
  const monthlyLimit = usage?.limits.monthly ?? 0
  const workspaces = usage?.workspaces ?? EMPTY_ACCOUNT_LIMIT
  const users = usage?.users ?? EMPTY_ACCOUNT_LIMIT
  // Free allows up to one owned workspace and one user.
  const accountLimitCaption = usage?.plan === "free" ? "Personal Free Plan" : undefined

  return (
    <SettingsSection
      title="Scan usage"
      description="Scan usage and account limits."
      action={<div className="flex flex-wrap items-center gap-3">
          {usage ? <Badge variant="secondary">{planLabel(usage.plan)} plan</Badge> : null}
          <Button variant="outline" size="sm" onClick={loadUsage} disabled={pending}>
            <RefreshCcwIcon data-icon="inline-start" />
            Refresh
          </Button>
        </div>}
    >
      <div className="grid gap-5 md:grid-cols-2">
        <UsageWindowRow
          label="Weekly usage"
          total={weekly.total}
          limit={weeklyLimit}
          loading={pending && !usage}
        />
        <UsageWindowRow
          label="Monthly usage"
          total={monthly.total}
          limit={monthlyLimit}
          loading={pending && !usage}
        />
        <AccountLimitRow
          label="Users"
          total={users.total}
          limit={users.limit}
          loading={pending && !usage}
          caption={accountLimitCaption}
        />
        <AccountLimitRow
          label="Workspaces"
          total={workspaces.total}
          limit={workspaces.limit}
          loading={pending && !usage}
          caption={accountLimitCaption}
        />
        {error ? (
          <p role="alert" className="rounded-lg border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive md:col-span-2">
            {error}
          </p>
        ) : null}
      </div>
    </SettingsSection>
  )
}

function UsageWindowRow({
  label,
  total,
  limit,
  loading,
}: {
  label: string
  total: number
  limit: number
  loading: boolean
}) {
  // Self-hosted never caps scan volume (any plan), so the engine reports
  // limit as -1 there the same way cloud Enterprise does — render both as
  // "Unlimited" rather than a nonsensical "42/-1".
  const unlimited = limit < 0
  const percentage = !unlimited && limit > 0 ? Math.min(100, (total / limit) * 100) : 0
  const progressColor =
    percentage >= 100
      ? "bg-destructive"
      : percentage >= 80
        ? "bg-amber-500"
        : "bg-primary"

  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-medium">{label}</p>
        {loading ? (
          <Skeleton className="h-4 w-16" />
        ) : (
          <span className="text-xs font-medium tabular-nums text-muted-foreground">
            {unlimited ? `${total} · Unlimited` : `${total}/${limit}`}
          </span>
        )}
      </div>
      {unlimited ? null : loading ? (
        <Skeleton className="h-2 w-full rounded-full" />
      ) : (
        <div
          className="h-2 overflow-hidden rounded-full bg-muted"
          role="progressbar"
          aria-label={`${label}: ${total} of ${limit} scans used`}
          aria-valuemin={0}
          aria-valuemax={limit}
          aria-valuenow={Math.min(total, limit)}
        >
          <div
            className={`h-full rounded-full transition-[width] duration-300 motion-reduce:transition-none ${progressColor}`}
            style={{ width: `${percentage}%` }}
          />
        </div>
      )}
    </div>
  )
}

// Users/Workspaces row: same shape as UsageWindowRow, plus an "unlimited"
// state (limit < 0, Enterprise) that skips the total/limit fraction and the
// progress bar entirely, and an optional caption (used on Free to label the
// fixed 1/1 as "Personal Free Plan" rather than a bug-looking low cap).
function AccountLimitRow({
  label,
  total,
  limit,
  loading,
  caption,
}: {
  label: string
  total: number
  limit: number
  loading: boolean
  caption?: string
}) {
  const unlimited = limit < 0
  const percentage = !unlimited && limit > 0 ? Math.min(100, (total / limit) * 100) : 0
  // Free's fixed 1/1 (caption set) is a deliberate ceiling, not a warning —
  // the "Personal Free Plan" caption already explains it, so keep the bar
  // calm there instead of alarming red/amber at a "limit" nobody is at risk
  // of blowing through.
  const progressColor = caption
    ? "bg-primary"
    : percentage >= 100
      ? "bg-destructive"
      : percentage >= 80
        ? "bg-amber-500"
        : "bg-primary"

  return (
    <div className="grid gap-2">
      <div className="flex items-center justify-between gap-3">
        <p className="text-sm font-medium">{label}</p>
        {loading ? (
          <Skeleton className="h-4 w-16" />
        ) : (
          <span className="text-xs font-medium tabular-nums text-muted-foreground">
            {unlimited ? `${total} · Unlimited` : `${total}/${limit}`}
          </span>
        )}
      </div>
      {!loading && caption ? (
        <p className="text-xs text-muted-foreground">{caption}</p>
      ) : null}
      {unlimited ? null : loading ? (
        <Skeleton className="h-2 w-full rounded-full" />
      ) : (
        <div
          className="h-2 overflow-hidden rounded-full bg-muted"
          role="progressbar"
          aria-label={`${label}: ${total} of ${limit} used`}
          aria-valuemin={0}
          aria-valuemax={limit}
          aria-valuenow={Math.min(total, limit)}
        >
          <div
            className={`h-full rounded-full transition-[width] duration-300 motion-reduce:transition-none ${progressColor}`}
            style={{ width: `${percentage}%` }}
          />
        </div>
      )}
    </div>
  )
}

type BillingStatusResponse = {
  entitlement: Entitlement
  subscription?: {
    plan: string
    status: string
    currentPeriodEnd?: string
    cancelAtPeriodEnd?: boolean
  }
  instance?: {
    installationId?: string
    licenseKeyPrefix?: string
    lastValidatedAt?: string
    lastValidationError?: string
  }
}

function BillingPanel() {
  const { deploymentMode, entitlement, refreshEntitlement } = useWorkspace()
  const [status, setStatus] = React.useState<BillingStatusResponse | null>(null)
  const [message, setMessage] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const activatedSessionRef = React.useRef("")

  const loadStatus = React.useCallback(async () => {
    const response = await apiRequest<BillingStatusResponse>("/api/v1/billing/status")
    setStatus(response)
    return response
  }, [])

  React.useEffect(() => {
    loadStatus().catch(() => setStatus({ entitlement }))
  }, [entitlement, loadStatus])

  React.useEffect(() => {
    const sessionId = new URLSearchParams(window.location.search).get(
      "license_checkout_session"
    )
    if (!sessionId || activatedSessionRef.current === sessionId) {
      return
    }
    activatedSessionRef.current = sessionId


    setPending(true)
    setError("")
    setMessage("Activating license after payment confirmation...")
    apiRequest("/api/v1/license/checkout/activate", {
      method: "POST",
      body: { sessionId },
    })
      .then(async () => {
        await refreshEntitlement()
        setMessage("License activated automatically.")
        window.history.replaceState(null, "", "/settings?tab=billing")
        return loadStatus()
      })
      .catch((error) => {
        setError(
          error instanceof Error ? error.message : "Failed to activate license"
        )
      })
      .finally(() => setPending(false))
  }, [loadStatus, refreshEntitlement])

  React.useEffect(() => {
    const sessionId = new URLSearchParams(window.location.search).get(
      "billing_checkout_session"
    )
    if (!sessionId) {
      return
    }

    let cancelled = false
    setPending(true)
    setError("")
    setMessage("Confirming subscription...")
    async function confirmSubscription() {
      try {
        for (let attempt = 0; attempt < 10 && !cancelled; attempt++) {
          const checkout = await apiRequest<{
            status: string
            plan: Entitlement["plan"]
            accountMatches: boolean
          }>(
            `/api/v1/billing/checkout-session/${encodeURIComponent(sessionId!)}`
          )
          if (cancelled) return
          if (checkout.status === "active" || checkout.status === "trialing") {
            if (checkout.accountMatches !== true) {
              throw new Error("This checkout belongs to a different account. Sign in with the account used for the purchase, then reopen this return URL.")
            }
            const billing = await loadStatus()
            if (cancelled) return
            if (
              billing.entitlement.plan !== checkout.plan ||
              !["active", "trialing"].includes(billing.entitlement.status)
            ) {
              throw new Error(`Your account has not activated the ${planLabel(checkout.plan)} plan yet. Refresh this page to check again, or contact support if it persists.`)
            }
            await refreshEntitlement()
            if (cancelled) return
            setMessage("Subscription activated.")
            window.history.replaceState(null, "", "/settings?tab=billing")
            return
          }
          if (checkout.status === "expired" || checkout.status === "canceled") {
            throw new Error("This checkout did not activate a subscription.")
          }
          if (attempt < 9) await new Promise((resolve) => setTimeout(resolve, 2000))
        }
        if (!cancelled) {
          setMessage("Payment confirmation is still pending. Refresh this page in a moment to check again.")
        }
      } catch (error) {
        if (!cancelled) {
          setMessage("")
          setError(error instanceof Error ? error.message : "Failed to confirm subscription")
        }
      } finally {
        if (!cancelled) setPending(false)
      }
    }
    void confirmSubscription()
    return () => { cancelled = true }
  }, [loadStatus, refreshEntitlement])

  async function startCheckout(plan: "pro" | "enterprise") {
    setError("")
    setMessage("")
    setPending(true)
    try {
      const response = await apiRequest<{ url: string }>("/api/v1/billing/checkout", {
        method: "POST",
        body: {
          plan,
          deploymentMode,
          successUrl:
            deploymentMode === "self-hosted"
              ? window.location.origin +
                "/settings?tab=billing&license_checkout_session={CHECKOUT_SESSION_ID}"
              : window.location.origin +
                "/settings?tab=billing&billing_checkout_session={CHECKOUT_SESSION_ID}",
          cancelUrl: window.location.origin + "/settings?tab=billing",
        },
      })
      window.location.assign(response.url)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to start checkout")
    } finally {
      setPending(false)
    }
  }

  async function openPortal() {
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<{ url: string }>("/api/v1/billing/portal", {
        method: "POST",
        body: { returnUrl: window.location.origin + "/settings?tab=billing" },
      })
      window.location.assign(response.url)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to open billing")
    } finally {
      setPending(false)
    }
  }

  async function refreshLicense() {
    setMessage("")
    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/license/refresh", {
        method: "POST",
      })
      setMessage("Heartbeat validated with the central engine.")
      await loadStatus()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to validate license")
    } finally {
      setPending(false)
    }
  }

  const activeEntitlement = status?.entitlement ?? entitlement
  const isPaidPlan = activeEntitlement.plan === "pro" || activeEntitlement.plan === "enterprise"
  // One upgrade path: free buys Pro, Pro buys Enterprise, Enterprise is the top.
  const upgradePlan: "pro" | "enterprise" | null =
    activeEntitlement.plan === "free"
      ? "pro"
      : activeEntitlement.plan === "pro"
        ? "enterprise"
        : null
  const renewalText = activeEntitlement.currentPeriodEnd
    ? `${activeEntitlement.cancelAtPeriodEnd ? "Ends" : "Renews"} on ${formatDate(activeEntitlement.currentPeriodEnd)}`
    : activeEntitlement.plan === "free"
      ? "No active renewal"
      : "Renewal awaiting confirmation"
  // Nothing to refresh on a Free install that has never activated a license
  // or started a Stripe checkout — showing the button there just leads to a
  // guaranteed "no self-hosted license has been activated" error.
  const canRefreshLicense = Boolean(status?.instance?.licenseKeyPrefix) || isPaidPlan

  return (
    <div className="space-y-4">
      <SettingsSection
        title="Plan & subscription"
        description="Review your current plan, billing cycle and license details."
        action={<Badge variant="outline">{deploymentLabel(deploymentMode)}</Badge>}
        footer={<div className="flex w-full flex-wrap justify-end gap-2">
          {upgradePlan ? (
            <Button onClick={() => startCheckout(upgradePlan)} disabled={pending}>
              <ArrowUpRightIcon data-icon="inline-start" />
              {pending ? "Opening checkout..." : `Upgrade to ${planLabel(upgradePlan)}`}
            </Button>
          ) : null}
          {deploymentMode === "cloud" && isPaidPlan ? (
            <Button variant="outline" onClick={openPortal} disabled={pending}>
              <CreditCardIcon data-icon="inline-start" />
              Manage subscription
            </Button>
          ) : null}
          {deploymentMode === "self-hosted" && canRefreshLicense ? (
            <Button type="button" variant="outline" onClick={refreshLicense} disabled={pending}>
              <RefreshCcwIcon data-icon="inline-start" />
              Validate license
            </Button>
          ) : null}
        </div>}
      >
        <div className="mb-5 space-y-2">
          <div className="flex flex-wrap items-center gap-3">
            <CrownIcon aria-hidden="true" className="size-5 text-primary" />
            <h3 className="text-2xl font-semibold">{planLabel(activeEntitlement.plan)}</h3>
            <Badge variant="secondary">Current plan</Badge>
          </div>
          <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{planDescription(activeEntitlement.plan, deploymentMode)}</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <BillingMetric label="Status" value={statusLabel(activeEntitlement.status)} />
          <BillingMetric label="Billing cycle" value={renewalText} />
        </div>
          {deploymentMode === "self-hosted" ? (
            <div className="mt-5 grid gap-3 border-t pt-5 text-sm lg:grid-cols-3">
              <BillingRow
                label="Installation"
                value={
                  status?.instance?.installationId ??
                  activeEntitlement.installationId ??
                  "pending"
                }
                mono
              />
              <BillingRow
                label="License"
                value={
                  status?.instance?.licenseKeyPrefix ??
                  activeEntitlement.licenseKeyPrefix ??
                  (activeEntitlement.plan === "free" ? "not activated" : "Stripe Checkout")
                }
                mono
              />
              <BillingRow
                label="Heartbeat"
                value={
                  status?.instance?.lastValidatedAt
                    ? formatDate(status.instance.lastValidatedAt)
                    : "not validated"
                }
              />
            </div>
          ) : null}
      </SettingsSection>

      {error || message ? (
        <div className="space-y-2">
          {error ? <SettingsFeedback error>{error}</SettingsFeedback> : null}
          {message ? <SettingsFeedback>{message}</SettingsFeedback> : null}
        </div>
      ) : null}
    </div>
  )
}

function BillingMetric({
  label,
  value,
  mono = false,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="rounded-lg border bg-background/55 p-3">
      <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      <p
        className={`mt-1.5 break-words text-sm font-semibold ${
          mono ? "font-mono" : ""
        }`}
      >
        {value}
      </p>
    </div>
  )
}

function BillingRow({
  label,
  value,
  mono = false,
}: {
  label: string
  value: string
  mono?: boolean
}) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-lg border bg-muted/20 px-3 py-2.5">
      <span className="text-muted-foreground">{label}</span>
      <span className={`min-w-0 text-right ${mono ? "break-all font-mono" : "font-medium"}`}>
        {value}
      </span>
    </div>
  )
}

function AccountPanel() {
  const [impact, setImpact] = React.useState<AccountDeletionImpact | null>(null)
  const [dialogOpen, setDialogOpen] = React.useState(false)
  const [confirmation, setConfirmation] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  const loadImpact = React.useCallback(async () => {
    setError("")
    try {
      setImpact(
        await apiRequest<AccountDeletionImpact>("/api/v1/me/deletion-impact")
      )
    } catch (error) {
      setError(
        error instanceof Error
          ? error.message
          : "Failed to calculate account deletion impact"
      )
    }
  }, [])

  React.useEffect(() => {
    loadImpact()
  }, [loadImpact])

  function handleDialogChange(open: boolean) {
    if (pending) {
      return
    }
    setDialogOpen(open)
    if (!open) {
      setConfirmation("")
      setError("")
    }
  }

  async function deleteAccount(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/me", {
        method: "DELETE",
        body: { confirmation },
      })
      clearClientState()
      window.location.replace("/login?account_deleted=1")
    } catch (error) {
      setError(
        error instanceof Error ? error.message : "Failed to delete account"
      )
      setPending(false)
    }
  }

  const confirmed = Boolean(
    impact &&
      confirmation.trim().toLowerCase() ===
        impact.confirmationValue.trim().toLowerCase()
  )

  return (
    <>
      <SettingsSection
        title="Delete account"
        description="Permanently delete your Runtz account and all data that belongs to it."
        footer={<div className="flex w-full justify-end">
          <Button
            variant="destructive"
            onClick={() => setDialogOpen(true)}
            disabled={!impact?.canDelete}
          >
            <Trash2Icon data-icon="inline-start" />
            Delete account
          </Button>
        </div>}
      >
        <div className="flex flex-col gap-4">
          {impact ? (
            <div className="flex flex-col gap-2 text-sm text-muted-foreground">
              <p>
                This will delete {impact.ownedWorkspaceCount} owned workspaces, {" "}
                {impact.scanCount} scans and {impact.apiKeyCount} API keys.
              </p>
              {impact.sharedWorkspaceCount > 0 ? (
                <p>
                  You will lose access to {impact.sharedWorkspaceCount} workspaces shared
                  with you. Their scans will not be deleted.
                </p>
              ) : null}
              {impact.subscriptionWillBeCanceled ? (
                <p>Your active subscription will be canceled immediately.</p>
              ) : null}
            </div>
          ) : error ? null : (
            <Skeleton className="h-16 w-full" />
          )}
          {impact && !impact.canDelete ? (
            <FieldError>
              You own {impact.sharedOwnedWorkspaceCount} workspaces used by other
              members. Delete those workspaces before deleting your account.
            </FieldError>
          ) : null}
          {error && !dialogOpen ? (
            <div className="flex flex-col items-start gap-2">
              <FieldError>{error}</FieldError>
              <Button variant="outline" size="sm" onClick={loadImpact}>
                <RefreshCcwIcon data-icon="inline-start" />
                Retry
              </Button>
            </div>
          ) : null}
        </div>
      </SettingsSection>

      <Dialog open={dialogOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
          <DialogHeader>
            <div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-destructive/10 text-destructive">
              <TriangleAlertIcon className="size-5" />
            </div>
            <DialogTitle>Permanently delete your account?</DialogTitle>
            <DialogDescription>
              Your profile, owned workspaces, scans and API keys will be permanently
              deleted. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={deleteAccount}>
            <FieldGroup>
              {impact ? (
                <div className="flex flex-col gap-2 rounded-lg border bg-muted/35 p-3 text-sm">
                  <p><strong>{impact.ownedWorkspaceCount}</strong> workspaces will be deleted.</p>
                  <p><strong>{impact.scanCount}</strong> scans will be deleted.</p>
                  <p><strong>{impact.apiKeyCount}</strong> API keys will be revoked.</p>
                  {impact.subscriptionWillBeCanceled ? (
                    <p>Your subscription will be canceled.</p>
                  ) : null}
                </div>
              ) : null}
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="delete-account-confirmation">
                  Type <strong>{impact?.confirmationValue}</strong> to confirm
                </FieldLabel>
                <Input
                  id="delete-account-confirmation"
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  autoComplete="off"
                  aria-invalid={Boolean(error)}
                  disabled={pending}
                />
                {error ? <FieldError>{error}</FieldError> : null}
              </Field>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => handleDialogChange(false)}
                disabled={pending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="destructive"
                disabled={pending || !confirmed}
              >
                <Trash2Icon data-icon="inline-start" />
                {pending ? "Deleting..." : "Delete account and data"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}

function ProfilePanel() {
  const { deploymentMode } = useWorkspace()

  return deploymentMode === "cloud" ? (
    <CloudProfilePanel />
  ) : (
    <SelfHostedProfilePanel />
  )
}

function CloudProfilePanel() {
  const { currentUser } = useWorkspace()
  const name = currentUser.email?.split("@")[0] || currentUser.username

  return (
    <SettingsSection title="Personal information" description="Your account details and sign-in information.">
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="profile-name">Name</FieldLabel>
            <Input id="profile-name" value={name} readOnly className="bg-muted/40" />
          </Field>
          <Field>
            <FieldLabel htmlFor="profile-email">Email</FieldLabel>
            <Input id="profile-email" type="email" value={currentUser.email ?? ""} readOnly className="bg-muted/40" />
          </Field>
          <div className="flex items-start gap-3 rounded-lg border bg-muted/35 p-3 text-sm text-muted-foreground">
            <UsersIcon className="mt-0.5 size-4 shrink-0 text-primary" />
            Team features are unlocked by the Pro and Enterprise plans.
          </div>
        </FieldGroup>
    </SettingsSection>
  )
}

function SelfHostedProfilePanel() {
  const { currentUser } = useWorkspace()
  const [email, setEmail] = React.useState(currentUser.email ?? "")
  const [emailMessage, setEmailMessage] = React.useState("")
  const [emailError, setEmailError] = React.useState("")
  const [emailPending, setEmailPending] = React.useState(false)
  const [currentPassword, setCurrentPassword] = React.useState("")
  const [newPassword, setNewPassword] = React.useState("")
  const [passwordMessage, setPasswordMessage] = React.useState("")
  const [passwordError, setPasswordError] = React.useState("")
  const [passwordPending, setPasswordPending] = React.useState(false)

  async function saveEmail(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setEmailMessage("")
    setEmailError("")
    setEmailPending(true)
    try {
      await apiRequest("/api/v1/me/profile", {
        method: "PATCH",
        body: { email },
      })
      setEmailMessage("Email updated.")
    } catch (error) {
      setEmailError(error instanceof Error ? error.message : "Failed to update email")
    } finally {
      setEmailPending(false)
    }
  }

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setPasswordMessage("")
    setPasswordError("")
    setPasswordPending(true)
    try {
      await apiRequest("/api/v1/me/password", {
        method: "PATCH",
        body: { currentPassword, newPassword },
      })
      setCurrentPassword("")
      setNewPassword("")
      setPasswordMessage("Password updated.")
    } catch (error) {
      setPasswordError(error instanceof Error ? error.message : "Failed to change password")
    } finally {
      setPasswordPending(false)
    }
  }

  return (
    <Card className="w-full gap-0 py-0">
      <CardContent className="grid gap-6 p-4 md:grid-cols-2 md:gap-8">
        <section aria-labelledby="profile-account-heading" className="min-w-0 space-y-4">
          <div className="min-w-0 space-y-1">
            <h2 id="profile-account-heading" className="text-base font-medium leading-snug">Your account</h2>
            <p className="text-sm text-muted-foreground">Your account on this installation.</p>
          </div>
          <form id="profile-email-form" onSubmit={saveEmail} aria-busy={emailPending}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="profile-username">Username</FieldLabel>
                <Input id="profile-username" value={currentUser.username} readOnly className="bg-muted/40" />
              </Field>
              <Field>
                <FieldLabel htmlFor="profile-email">Email</FieldLabel>
                <Input id="profile-email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@example.com" />
              </Field>
              {emailMessage ? <SettingsFeedback>{emailMessage}</SettingsFeedback> : null}
              {emailError ? <SettingsFeedback error>{emailError}</SettingsFeedback> : null}
            </FieldGroup>
          </form>
        </section>
        <section aria-labelledby="profile-security-heading" className="min-w-0 space-y-4">
          <div className="min-w-0 space-y-1">
            <h2 id="profile-security-heading" className="text-base font-medium leading-snug">Password &amp; security</h2>
            <p className="text-sm text-muted-foreground">Requires your current password.</p>
          </div>
          <form id="profile-password-form" onSubmit={changePassword} aria-busy={passwordPending}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="current-password">Current password</FieldLabel>
                <Input id="current-password" type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required />
              </Field>
              <Field>
                <FieldLabel htmlFor="new-password">New password</FieldLabel>
                <Input id="new-password" type="password" autoComplete="new-password" aria-describedby="new-password-help" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} minLength={8} required />
                <FieldDescription id="new-password-help">Use at least 8 characters.</FieldDescription>
              </Field>
              {passwordMessage ? <SettingsFeedback>{passwordMessage}</SettingsFeedback> : null}
              {passwordError ? <SettingsFeedback error>{passwordError}</SettingsFeedback> : null}
            </FieldGroup>
          </form>
        </section>
      </CardContent>
      <CardFooter className="flex-wrap justify-end gap-2">
        <Button type="submit" form="profile-email-form" variant="outline" disabled={emailPending}>
          {emailPending ? <SettingsPending>Saving email…</SettingsPending> : "Save email"}
        </Button>
        <Button type="submit" form="profile-password-form" disabled={passwordPending}>
          {passwordPending ? <SettingsPending>Updating password…</SettingsPending> : <><KeyRoundIcon data-icon="inline-start" />Update password</>}
        </Button>
      </CardFooter>
    </Card>
  )
}

function planLabel(plan: string) {
  if (plan === "enterprise") {
    return "Enterprise"
  }
  if (plan === "pro") {
    return "Pro"
  }
  return "Free"
}

function deploymentLabel(deploymentMode: string) {
  return deploymentMode === "cloud" ? "Cloud" : "Self-hosted"
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    active: "Active",
    trialing: "Trial active",
    free: "Free",
    expired: "Expired",
    validation_failed: "Validation failed",
    checkout_pending: "Checkout pending",
    canceled: "Canceled",
    incomplete: "Payment pending",
    past_due: "Payment past due",
  }
  return labels[status] ?? status
}

function planDescription(plan: string, deploymentMode: string) {
  if (plan === "enterprise") {
    return deploymentMode === "cloud"
      ? "Multiple cloud workspaces for teams, products, customers and environments."
      : "Multiple workspaces in your installation, with central activation and data in your environment."
  }
  if (plan === "pro") {
    return deploymentMode === "cloud"
      ? "Shared workspace with smart reports, smart alerts and the AI Alert Agent."
      : "Team features for self-hosted, including Google/GitHub auth and smart alerts."
  }
  return deploymentMode === "cloud"
    ? "Free personal workspace to start running scans with no infrastructure."
    : "Free self-hosted installation with an initial workspace and manually managed users."
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value))
}
