"use client"

import * as React from "react"
import {
  CopyIcon,
  CreditCardIcon,
  CrownIcon,
  FolderKanbanIcon,
  KeyRoundIcon,
  PlusIcon,
  RefreshCcwIcon,
  SendIcon,
  Share2Icon,
  ShieldCheckIcon,
  UsersIcon,
} from "lucide-react"

import { useWorkspace } from "@/components/runtz/workspace-context"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
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
import { apiRequest, getStoredToken, type Entitlement, type User } from "@/lib/api"

type SettingsTab = "profile" | "workspaces" | "billing" | "users"

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
          <TabsTrigger value="billing">Billing</TabsTrigger>
          {isCloud ? null : <TabsTrigger value="users">Users</TabsTrigger>}
        </TabsList>
        <TabsContent value="profile">
          <ProfilePanel />
        </TabsContent>
        <TabsContent value="workspaces">
          <WorkspacesPanel />
        </TabsContent>
        <TabsContent value="billing">
          <BillingPanel />
        </TabsContent>
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
  if (value === "profile" || value === "workspaces" || value === "billing") {
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
    const token = getStoredToken()
    if (!token) {
      return
    }

    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/workspaces", {
        method: "POST",
        token,
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
          <CardTitle>Novo workspace</CardTitle>
          <CardDescription>Crie ambientes para separar resultados.</CardDescription>
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
                Criar workspace
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Workspaces</CardTitle>
          <CardDescription>{workspaces.length} cadastrados</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Nome</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Criado em</TableHead>
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
            Criar outro workspace
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Nome</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Criado em</TableHead>
                <TableHead>Plano</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workspaces.map((workspace) => (
                <TableRow key={workspace.id}>
                  <TableCell className="font-medium">{workspace.name}</TableCell>
                  <TableCell>{workspace.slug}</TableCell>
                  <TableCell>{formatDate(workspace.createdAt)}</TableCell>
                  <TableCell>
                    <Badge variant="outline">workspace inicial</Badge>
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
  const { entitlement, workspaces } = useWorkspace()
  const [shareUpgradeOpen, setShareUpgradeOpen] = React.useState(false)
  const canShareWorkspace = entitlement.plan === "pro" || entitlement.plan === "enterprise"

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <CardTitle>Your workspace</CardTitle>
              <CardDescription>
                The personal workspace remains available on the free plan.
              </CardDescription>
            </div>
            <Badge variant={canShareWorkspace ? "secondary" : "outline"}>
              Current plan: {planLabel(entitlement.plan)}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nome</TableHead>
                  <TableHead>Slug</TableHead>
                  <TableHead>Tipo</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {workspaces.map((workspace) => (
                  <TableRow key={workspace.id}>
                    <TableCell className="font-medium">{workspace.name}</TableCell>
                    <TableCell>{workspace.slug}</TableCell>
                    <TableCell>
                      <Badge variant="outline">workspace inicial</Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShareUpgradeOpen(true)}
                        >
                          <Share2Icon data-icon="inline-start" />
                          Compartilhar workspace
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
      <WorkspaceShareUpgradeDialog
        canShareWorkspace={canShareWorkspace}
        open={shareUpgradeOpen}
        onOpenChange={setShareUpgradeOpen}
      />
    </>
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
    const token = getStoredToken()
    if (!token) {
      return
    }

    const response = await apiRequest<{ users: User[] }>("/api/v1/users", {
      token,
    })
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
    const token = getStoredToken()
    if (!token) {
      return
    }

    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/users", {
        method: "POST",
        token,
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
    const token = getStoredToken()
    if (!token) {
      return
    }

    await apiRequest(`/api/v1/users/${user.id}`, {
      method: "PATCH",
      token,
      body: { requirePasswordChange: !user.requirePasswordChange },
    })
    await loadUsers()
  }

  async function createInvite(user: User) {
    const token = getStoredToken()
    if (!token) {
      return
    }

    const response = await apiRequest<{ inviteLink: string }>(
      `/api/v1/users/${user.id}/invite`,
      { method: "POST", token }
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
              <CardDescription>{users.length} cadastrados</CardDescription>
            </div>
            <Button variant="outline" size="sm" onClick={() => loadUsers()}>
              <RefreshCcwIcon data-icon="inline-start" />
              Atualizar
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
            <DialogTitle>Link de convite</DialogTitle>
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
  const { deploymentMode, entitlement } = useWorkspace()
  const [status, setStatus] = React.useState<BillingStatusResponse | null>(null)
  const [message, setMessage] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)
  const activatedSessionRef = React.useRef("")

  const loadStatus = React.useCallback(async () => {
    const token = getStoredToken()
    if (!token) {
      return
    }
    const response = await apiRequest<BillingStatusResponse>("/api/v1/billing/status", {
      token,
    })
    setStatus(response)
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

    const token = getStoredToken()
    if (!token) {
      return
    }

    setPending(true)
    setError("")
    setMessage("Activating license after payment confirmation...")
    apiRequest("/api/v1/license/checkout/activate", {
      method: "POST",
      token,
      body: { sessionId },
    })
      .then(() => {
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
  }, [loadStatus])

  React.useEffect(() => {
    const sessionId = new URLSearchParams(window.location.search).get(
      "billing_checkout_session"
    )
    if (!sessionId || activatedSessionRef.current === sessionId) {
      return
    }
    activatedSessionRef.current = sessionId

    setPending(true)
    setError("")
    setMessage("Confirming subscription...")
    apiRequest(`/api/v1/billing/checkout-session/${encodeURIComponent(sessionId)}`)
      .then(() => {
        setMessage("Subscription activated.")
        window.history.replaceState(null, "", "/app/settings?tab=billing")
        return loadStatus()
      })
      .catch((error) => {
        setError(
          error instanceof Error ? error.message : "Failed to confirm subscription"
        )
      })
      .finally(() => setPending(false))
  }, [loadStatus])

  async function startCheckout(plan: "pro" | "enterprise") {
    const token = getStoredToken()
    if (!token) {
      return
    }
    setError("")
    setMessage("")
    setPending(true)
    try {
      const response = await apiRequest<{ url: string }>("/api/v1/billing/checkout", {
        method: "POST",
        token,
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
    const token = getStoredToken()
    if (!token) {
      return
    }
    setError("")
    setPending(true)
    try {
      const response = await apiRequest<{ url: string }>("/api/v1/billing/portal", {
        method: "POST",
        token,
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
    const token = getStoredToken()
    if (!token) {
      return
    }
    setMessage("")
    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/license/refresh", {
        method: "POST",
        token,
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
  const renewalText = activeEntitlement.currentPeriodEnd
    ? `${activeEntitlement.cancelAtPeriodEnd ? "Encerra" : "Renova"} em ${formatDate(activeEntitlement.currentPeriodEnd)}`
    : activeEntitlement.plan === "free"
      ? "No active renewal"
      : "Renewal awaiting confirmation"

  return (
    <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
      <div className="grid content-start gap-4">
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
            <BillingMetric label="Ciclo" value={renewalText} />
            <BillingMetric
              label={deploymentMode === "self-hosted" ? "Installation" : "Subscription"}
              value={
                deploymentMode === "self-hosted"
                  ? status?.instance?.installationId ??
                    activeEntitlement.installationId ??
                    "pending"
                  : status?.subscription?.status
                    ? statusLabel(status.subscription.status)
                    : activeEntitlement.plan === "free"
                      ? "no subscription"
                      : "ativa"
              }
              mono={deploymentMode === "self-hosted"}
            />
            {deploymentMode === "cloud" && isPaidPlan ? (
              <Button
                className="mt-1 w-full"
                variant="outline"
                onClick={openPortal}
                disabled={pending}
              >
                <CreditCardIcon data-icon="inline-start" />
                Gerenciar assinatura
              </Button>
            ) : null}
            {deploymentMode === "self-hosted" ? (
              <div className="mt-1 grid gap-2 text-sm">
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
                <Button
                  type="button"
                  variant="outline"
                  onClick={refreshLicense}
                  disabled={pending}
                >
                  <RefreshCcwIcon data-icon="inline-start" />
                  Validate license
                </Button>
              </div>
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

      <Card className="relative min-w-0 overflow-hidden border-primary/20">
        <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 opacity-[0.08]" />
        <CardHeader className="relative gap-3 p-4 md:p-5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Badge className="w-fit">
              <CrownIcon data-icon="inline-start" />
              Runtz Pro / Enterprise
            </Badge>
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <ShieldCheckIcon className="size-4 text-primary" />
              Free remains available in the initial workspace.
            </div>
          </div>
          <div className="max-w-3xl">
            <CardTitle className="text-2xl leading-tight md:text-3xl">
              Create shared workspaces to work as a team.
            </CardTitle>
            <CardDescription className="mt-2 max-w-2xl text-sm leading-6">
              Activate Pro or Enterprise to unlock advanced authentication,
              smart alerts and team-ready workspaces.
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent className="relative grid gap-3 p-4 pt-0 md:grid-cols-2 md:p-5 md:pt-0">
          {billingUpgradePlans(deploymentMode).map((plan) => (
            <BillingPlanCard
              key={plan.plan}
              activePlan={activeEntitlement.plan}
              pending={pending}
              plan={plan}
              onCheckout={startCheckout}
            />
          ))}
        </CardContent>
      </Card>
    </div>
  )
}

type BillingUpgradePlan = {
  plan: "pro" | "enterprise"
  title: string
  price: string
  description: string
  features: string[]
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
}

function billingUpgradePlans(deploymentMode: string): BillingUpgradePlan[] {
  return [
    {
      plan: "pro",
      title: "Pro",
      price: "$20/month",
      description:
        "Shared workspace with advanced auth, reports and smart alerts.",
      icon: UsersIcon,
      features:
        deploymentMode === "self-hosted"
          ? [
              "Google e GitHub auth no self-hosted",
              "Smart email reports",
              "Smart alerts",
              "AI Alert Agent in Slack threads",
            ]
          : [
              "Workspace compartilhado",
              "Smart email reports",
              "Smart alerts",
              "AI Alert Agent in Slack threads",
            ],
    },
    {
      plan: "enterprise",
      title: "Enterprise",
      price: "$99/month",
      description:
        "Multiple workspaces for teams, customers, environments and products.",
      icon: FolderKanbanIcon,
      features:
        deploymentMode === "self-hosted"
          ? [
              "Everything in Pro",
              "Multiple workspaces",
              "1 self-hosted installation per license",
              "Suporte dedicado no Slack",
            ]
          : [
              "Everything in Pro",
              "Multiple cloud workspaces",
              "Organization by teams and products",
              "Suporte dedicado no Slack",
            ],
    },
  ]
}

function BillingPlanCard({
  activePlan,
  pending,
  plan,
  onCheckout,
}: {
  activePlan: Entitlement["plan"]
  pending: boolean
  plan: BillingUpgradePlan
  onCheckout: (plan: "pro" | "enterprise") => void
}) {
  const Icon = plan.icon
  const isCurrent = activePlan === plan.plan
  const isIncluded = activePlan === "enterprise" && plan.plan === "pro"
  const disabled = pending || isCurrent || isIncluded
  const label = isCurrent
    ? "Current plan"
    : isIncluded
      ? "Included in Enterprise"
      : pending
        ? "Abrindo checkout..."
        : activePlan === "free"
          ? `Ativar ${plan.title}`
          : `Upgrade para ${plan.title}`

  return (
    <div
      className={`relative flex min-h-[260px] flex-col overflow-hidden rounded-xl border bg-background/55 p-4 transition-colors ${
        plan.plan === "enterprise"
          ? "border-primary/35 shadow-sm shadow-primary/10"
          : "border-border"
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background/70">
            <Icon className="size-4 text-primary" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="font-semibold">{plan.title}</h3>
              {isCurrent || isIncluded ? (
                <Badge variant={isCurrent ? "default" : "secondary"}>
                  {isCurrent ? "Current" : "Included"}
                </Badge>
              ) : null}
            </div>
            <p className="mt-1 text-sm leading-5 text-muted-foreground">
              {plan.description}
            </p>
          </div>
        </div>
        <p className="shrink-0 font-mono text-sm text-muted-foreground">
          {plan.price}
        </p>
      </div>
      <ul className="mt-4 grid gap-2 text-sm">
        {plan.features.map((feature) => (
          <li key={feature} className="flex gap-2">
            <ShieldCheckIcon className="mt-0.5 size-4 shrink-0 text-primary" />
            <span>{feature}</span>
          </li>
        ))}
      </ul>
      <Button
        className="mt-auto w-full"
        disabled={disabled}
        onClick={() => onCheckout(plan.plan)}
      >
        <ShieldCheckIcon data-icon="inline-start" />
        {label}
      </Button>
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
            <FieldLabel htmlFor="profile-name">Nome</FieldLabel>
            <Input id="profile-name" value={name} readOnly />
          </Field>
          <Field>
            <FieldLabel htmlFor="profile-email">E-mail</FieldLabel>
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
    const token = getStoredToken()
    if (!token) {
      return
    }

    setMessage("")
    setError("")
    setPending(true)
    try {
      await apiRequest("/api/v1/me/password", {
        method: "PATCH",
        token,
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
  return new Intl.DateTimeFormat("pt-BR", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value))
}
