"use client"

import * as React from "react"
import {
  ActivityIcon,
  ArrowUpRightIcon,
  CopyIcon,
  CreditCardIcon,
  CrownIcon,
  KeyRoundIcon,
  PlusIcon,
  RefreshCcwIcon,
  SendIcon,
  Share2Icon,
  Trash2Icon,
  TriangleAlertIcon,
  UsersIcon,
} from "lucide-react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
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
  FieldSet,
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
    const queryTab = new URLSearchParams(window.location.search).get("tab")
    if (isSettingsTab(queryTab, isCloud)) {
      setActiveTab(queryTab)
    }

    function handleTabEvent(event: Event) {
      const nextTab = (event as CustomEvent<SettingsTab>).detail
      if (isSettingsTab(nextTab, isCloud)) {
        setActiveTab(nextTab)
      }
    }

    window.addEventListener("runtz-settings-tab", handleTabEvent)
    return () => window.removeEventListener("runtz-settings-tab", handleTabEvent)
  }, [isCloud])

  function handleTabChange(tab: string) {
    if (!isSettingsTab(tab, isCloud)) {
      return
    }
    setActiveTab(tab)
    writeSettingsTabToURL(tab, isCloud)
  }

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">Settings</h1>
        <p className="text-sm text-muted-foreground">
          {isCloud
            ? "Account profile and workspace features."
            : "Workspaces, users and profile for the current installation."}
        </p>
      </div>
      <Tabs value={activeTab} onValueChange={handleTabChange}>
        <TabsList>
          <TabsTrigger value="profile">Profile</TabsTrigger>
          <TabsTrigger value="workspaces">Workspaces</TabsTrigger>
          <TabsTrigger value="usage">Usage</TabsTrigger>
          <TabsTrigger value="billing">Billing</TabsTrigger>
          {isCloud ? <TabsTrigger value="account">Account</TabsTrigger> : null}
          {isCloud ? null : <TabsTrigger value="users">Users</TabsTrigger>}
        </TabsList>
        <TabsContent value="profile">
          <ProfilePanel />
        </TabsContent>
        <TabsContent value="workspaces">
          <WorkspacesPanel />
        </TabsContent>
        <TabsContent value="usage">
          <UsagePanel />
        </TabsContent>
        <TabsContent value="billing">
          <BillingPanel />
        </TabsContent>
        {isCloud ? (
          <TabsContent value="account">
            <AccountPanel />
          </TabsContent>
        ) : null}
        {isCloud ? null : (
          <TabsContent value="users">
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

  async function createWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/workspaces", {
        method: "POST",
        body: { name },
      })
      setName("")
      await refreshWorkspaces()
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
    <div className="grid gap-6 lg:grid-cols-[360px_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>New workspace</CardTitle>
          <CardDescription>Create environments to separate results.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={createWorkspace}>
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="workspace-name">Workspace Name</FieldLabel>
                <Input
                  id="workspace-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  required
                />
              </Field>
              {error ? (
                <Field>
                  <FieldError>{error}</FieldError>
                </Field>
              ) : null}
              <Button type="submit" disabled={pending}>
                <PlusIcon data-icon="inline-start" />
                Create workspace
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Workspaces</CardTitle>
          <CardDescription>{workspaces.length} registered</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workspaces.map((workspace) => (
                <TableRow key={workspace.id}>
                  <TableCell className="font-medium">{workspace.name}</TableCell>
                  <TableCell>{workspace.slug}</TableCell>
                  <TableCell>{formatDate(workspace.createdAt)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}

function SelfHostedWorkspacesLimitPanel() {
  const { workspaces } = useWorkspace()

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <CardTitle>Current workspace</CardTitle>
            <CardDescription>
              This installation uses a shared workspace on the current plan.
            </CardDescription>
          </div>
          <Button variant="outline" onClick={openBillingTab}>
            <PlusIcon data-icon="inline-start" />
            Create another workspace
          </Button>
        </div>
      </CardHeader>
      <CardContent>
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
      </CardContent>
    </Card>
  )
}

function CloudWorkspacesPanel() {
  const { currentUser, entitlement, workspaces, refreshWorkspaces } = useWorkspace()
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
      await apiRequest("/api/v1/workspaces", {
        method: "POST",
        body: { name: name.trim() || "personal" },
      })
      await refreshWorkspaces()
      setName("personal")
      setCreateOpen(false)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create workspace")
    } finally {
      setPending(false)
    }
  }

  const [shareUpgradeOpen, setShareUpgradeOpen] = React.useState(false)
  const [workspaceToDelete, setWorkspaceToDelete] = React.useState<Workspace | null>(null)
  const canShareWorkspace = entitlement.plan === "pro" || entitlement.plan === "enterprise"

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
        <Card>
          <CardHeader>
            <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <CardTitle>Your workspaces</CardTitle>
                <CardDescription>
                  Manage the workspaces you own or have access to.
                </CardDescription>
              </div>
              <div className="flex flex-wrap items-center gap-3 sm:shrink-0">
                <Badge variant={canShareWorkspace ? "secondary" : "outline"}>
                  Current plan: {planLabel(entitlement.plan)}
                </Badge>
                {canCreateWorkspace ? (
                  <DialogTrigger render={<Button variant="outline" size="sm" className="h-10 sm:h-8" />}>
                    <PlusIcon data-icon="inline-start" />
                    New workspace
                  </DialogTrigger>
                ) : null}
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {workspaces.length === 0 ? (
              <Empty>
                <EmptyHeader>
                  <EmptyTitle>No workspaces yet</EmptyTitle>
                  <EmptyDescription>Select New workspace to organize your scans and get started.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Slug</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {workspaces.map((workspace) => {
                    const isOwner = workspace.createdBy === currentUser.id

                    return (
                      <TableRow key={workspace.id}>
                        <TableCell className="font-medium">{workspace.name}</TableCell>
                        <TableCell>{workspace.slug}</TableCell>
                        <TableCell>
                          <Badge variant="outline">{isOwner ? "Owner" : "Shared"}</Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex justify-end gap-2">
                            {isOwner ? (
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setShareUpgradeOpen(true)}
                              >
                                <Share2Icon data-icon="inline-start" />
                                Share
                              </Button>
                            ) : null}
                            {isOwner ? (
                              <Button
                                variant="destructive"
                                size="sm"
                                onClick={() => setWorkspaceToDelete(workspace)}
                              >
                                <Trash2Icon data-icon="inline-start" />
                                Delete
                              </Button>
                            ) : null}
                          </div>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>}
          </CardContent>
        </Card>
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
      <WorkspaceShareUpgradeDialog
        canShareWorkspace={canShareWorkspace}
        open={shareUpgradeOpen}
        onOpenChange={setShareUpgradeOpen}
      />
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

function WorkspaceShareUpgradeDialog({
  canShareWorkspace,
  open,
  onOpenChange,
}: {
  canShareWorkspace: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  function goToBilling() {
    onOpenChange(false)
    openBillingTab()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <div className="mb-2 flex size-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Share2Icon className="size-5" />
          </div>
          <DialogTitle>
            {canShareWorkspace
              ? "Sharing unlocked"
              : "Workspace sharing is Pro"}
          </DialogTitle>
          <DialogDescription>
            {canShareWorkspace
              ? "Your plan already unlocks shared workspaces for teams."
              : "Activate Pro to invite people and unlock advanced authentication, smart alerts and the AI Alert Agent in Slack threads."}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-2 pt-2 sm:grid-cols-2">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Not now
          </Button>
          <Button onClick={canShareWorkspace ? () => onOpenChange(false) : goToBilling}>
            {canShareWorkspace ? "Got it" : "Upgrade to Pro"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function UsersPanel() {
  const { workspaces } = useWorkspace()
  const [users, setUsers] = React.useState<User[]>([])
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [role, setRole] = React.useState<"admin" | "member">("member")
  const [workspaceId, setWorkspaceId] = React.useState("")
  const [requirePasswordChange, setRequirePasswordChange] = React.useState(true)
  const [error, setError] = React.useState("")
  const [inviteLink, setInviteLink] = React.useState("")
  const [pending, setPending] = React.useState(false)

  const loadUsers = React.useCallback(async () => {

    const response = await apiRequest<{ users: User[] }>("/api/v1/users")
    setUsers(response.users ?? [])
  }, [])

  React.useEffect(() => {
    loadUsers().catch(() => setUsers([]))
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
      await apiRequest("/api/v1/users", {
        method: "POST",
        body: {
          username,
          password,
          role,
          workspaceIds: workspaceId ? [workspaceId] : [],
          requirePasswordChange,
        },
      })
      setUsername("")
      setPassword("")
      setRequirePasswordChange(true)
      await loadUsers()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create user")
    } finally {
      setPending(false)
    }
  }

  async function togglePasswordChange(user: User) {

    await apiRequest(`/api/v1/users/${user.id}`, {
      method: "PATCH",
      body: { requirePasswordChange: !user.requirePasswordChange },
    })
    await loadUsers()
  }

  async function createInvite(user: User) {

    const response = await apiRequest<{ inviteLink: string }>(
      `/api/v1/users/${user.id}/invite`,
      { method: "POST" }
    )
    setInviteLink(response.inviteLink)
  }

  return (
    <div className="grid gap-6 xl:grid-cols-[380px_1fr]">
      <Card>
        <CardHeader>
          <CardTitle>New user</CardTitle>
          <CardDescription>Create access with an initial password.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={createUser}>
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
                <FieldLabel htmlFor="user-password">Password</FieldLabel>
                <Input
                  id="user-password"
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  minLength={8}
                  required
                />
              </Field>
              <Field>
                <FieldLabel>Role</FieldLabel>
                <Select
                  value={role}
                  onValueChange={(value) => {
                    if (value === "admin" || value === "member") {
                      setRole(value)
                    }
                  }}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Role" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="member">member</SelectItem>
                      <SelectItem value="admin">admin</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel>Workspace</FieldLabel>
                <Select
                  value={workspaceId}
                  onValueChange={(value) => {
                    if (value) {
                      setWorkspaceId(value)
                    }
                  }}
                >
                  <SelectTrigger className="w-full">
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
              <FieldSet>
                <Field orientation="horizontal">
                  <Checkbox
                    checked={requirePasswordChange}
                    onCheckedChange={(checked) =>
                      setRequirePasswordChange(Boolean(checked))
                    }
                  />
                  <FieldDescription>
                    Require a password change after first sign-in
                  </FieldDescription>
                </Field>
              </FieldSet>
              {error ? (
                <Field>
                  <FieldError>{error}</FieldError>
                </Field>
              ) : null}
              <Button type="submit" disabled={pending}>
                <PlusIcon data-icon="inline-start" />
                Create user
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle>Users</CardTitle>
              <CardDescription>{users.length} registered</CardDescription>
            </div>
            <Button variant="outline" size="sm" onClick={() => loadUsers()}>
              <RefreshCcwIcon data-icon="inline-start" />
              Refresh
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Username</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Password change</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
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
                      <Badge
                        variant={
                          user.requirePasswordChange ? "outline" : "secondary"
                        }
                      >
                        {user.requirePasswordChange ? "yes" : "no"}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => togglePasswordChange(user)}
                        >
                          <KeyRoundIcon data-icon="inline-start" />
                          Toggle
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => createInvite(user)}
                        >
                          <SendIcon data-icon="inline-start" />
                          Invite
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Dialog open={Boolean(inviteLink)} onOpenChange={(open) => !open && setInviteLink("")}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Invitation link</DialogTitle>
            <DialogDescription>
              Link generated for the selected user.
            </DialogDescription>
          </DialogHeader>
          <div className="flex gap-2">
            <Input value={inviteLink} readOnly />
            <Button
              size="icon"
              variant="outline"
              onClick={() => navigator.clipboard.writeText(inviteLink)}
            >
              <CopyIcon />
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
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
    <Card className="max-w-xl">
      <CardHeader className="border-b">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <ActivityIcon className="size-4 text-primary" />
            <CardTitle>Scan usage</CardTitle>
            {usage ? (
              <Badge variant="secondary">{planLabel(usage.plan)} plan</Badge>
            ) : null}
          </div>
          <Button variant="outline" size="sm" onClick={loadUsage} disabled={pending}>
            <RefreshCcwIcon data-icon="inline-start" />
            Refresh
          </Button>
        </div>
      </CardHeader>
      <CardContent className="grid gap-5 p-4 md:p-5">
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
          <p className="rounded-lg border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        ) : null}
      </CardContent>
    </Card>
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
  const percentage = limit > 0 ? Math.min(100, (total / limit) * 100) : 0
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
            {total}/{limit}
          </span>
        )}
      </div>
      {loading ? (
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
        window.history.replaceState(null, "", "/app/settings?tab=billing")
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
            window.history.replaceState(null, "", "/app/settings?tab=billing")
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
                "/app/settings?tab=billing&license_checkout_session={CHECKOUT_SESSION_ID}"
              : window.location.origin +
                "/app/settings?tab=billing&billing_checkout_session={CHECKOUT_SESSION_ID}",
          cancelUrl: window.location.origin + "/app/settings?tab=billing",
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
        body: { returnUrl: window.location.origin + "/app/settings?tab=billing" },
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

  return (
    <div className="grid max-w-xl content-start gap-4">
      <Card className="relative overflow-hidden border-primary/25">
        <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 opacity-[0.08]" />
        <CardHeader className="relative p-4">
          <div className="flex min-w-0 items-start gap-3">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm shadow-primary/20">
              <CrownIcon className="size-5" />
            </div>
            <div className="min-w-0">
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <Badge variant="secondary">Current plan</Badge>
                <Badge variant="outline">{deploymentLabel(deploymentMode)}</Badge>
              </div>
              <CardTitle className="text-2xl leading-tight">
                {planLabel(activeEntitlement.plan)}
              </CardTitle>
              <CardDescription className="mt-1.5 text-sm leading-6">
                {planDescription(activeEntitlement.plan, deploymentMode)}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="relative grid gap-2 px-4 pb-4 pt-0">
          <BillingMetric label="Status" value={statusLabel(activeEntitlement.status)} />
          <BillingMetric label="Billing cycle" value={renewalText} />
          {deploymentMode === "self-hosted" ? (
            <div className="grid gap-2 text-sm">
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
          {upgradePlan ? (
            <Button className="mt-1 w-full" onClick={() => startCheckout(upgradePlan)} disabled={pending}>
              <ArrowUpRightIcon data-icon="inline-start" />
              {pending ? "Opening checkout..." : `Upgrade to ${planLabel(upgradePlan)}`}
            </Button>
          ) : null}
          {deploymentMode === "cloud" && isPaidPlan ? (
            <Button className="w-full" variant="outline" onClick={openPortal} disabled={pending}>
              <CreditCardIcon data-icon="inline-start" />
              Manage subscription
            </Button>
          ) : null}
          {deploymentMode === "self-hosted" ? (
            <Button type="button" variant="outline" onClick={refreshLicense} disabled={pending}>
              <RefreshCcwIcon data-icon="inline-start" />
              Validate license
            </Button>
          ) : null}
        </CardContent>
      </Card>

      {error || message ? (
        <div className="grid gap-2 rounded-xl border bg-background/55 p-3">
          {error ? <p className="text-sm text-destructive">{error}</p> : null}
          {message ? <p className="text-sm text-muted-foreground">{message}</p> : null}
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
        className={`mt-1.5 truncate text-sm font-semibold ${
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
      <Card className="max-w-xl">
        <CardHeader>
          <CardTitle>Delete account</CardTitle>
          <CardDescription>
            Permanently delete your Runtz account and all data that belongs to it.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
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
        </CardContent>
        <CardFooter className="justify-end">
          <Button
            variant="destructive"
            onClick={() => setDialogOpen(true)}
            disabled={!impact?.canDelete}
          >
            <Trash2Icon data-icon="inline-start" />
            Delete account
          </Button>
        </CardFooter>
      </Card>

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
    <Card className="max-w-xl">
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>
          Your cloud account uses passwordless sign-in. The initial name comes from your email.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="profile-name">Name</FieldLabel>
            <Input id="profile-name" value={name} readOnly />
          </Field>
          <Field>
            <FieldLabel htmlFor="profile-email">Email</FieldLabel>
            <Input id="profile-email" type="email" value={currentUser.email ?? ""} readOnly />
          </Field>
          <div className="flex items-start gap-3 rounded-lg border bg-muted/35 p-3 text-sm text-muted-foreground">
            <UsersIcon className="mt-0.5 size-4 shrink-0 text-primary" />
            Team features are unlocked by the Pro and Enterprise plans.
          </div>
        </FieldGroup>
      </CardContent>
    </Card>
  )
}

function SelfHostedProfilePanel() {
  const [currentPassword, setCurrentPassword] = React.useState("")
  const [newPassword, setNewPassword] = React.useState("")
  const [message, setMessage] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()

    setMessage("")
    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/me/password", {
        method: "PATCH",
        body: { currentPassword, newPassword },
      })
      setCurrentPassword("")
      setNewPassword("")
      setMessage("Password updated.")
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to change password")
    } finally {
      setPending(false)
    }
  }

  return (
    <Card className="max-w-md">
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>Change the current user&apos;s password.</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={changePassword}>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="current-password">Current password</FieldLabel>
              <Input
                id="current-password"
                type="password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-password">New password</FieldLabel>
              <Input
                id="new-password"
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                minLength={8}
                required
              />
            </Field>
            {message ? (
              <Field>
                <FieldDescription>{message}</FieldDescription>
              </Field>
            ) : null}
            {error ? (
              <Field>
                <FieldError>{error}</FieldError>
              </Field>
            ) : null}
            <Button type="submit" disabled={pending}>
              <KeyRoundIcon data-icon="inline-start" />
              Update password
            </Button>
          </FieldGroup>
        </form>
      </CardContent>
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
