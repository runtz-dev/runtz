"use client"

import * as React from "react"
import {
  CheckIcon,
  CopyIcon,
  KeyRoundIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  RefreshCcwIcon,
  SearchIcon,
  Trash2Icon,
  TriangleAlertIcon,
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
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
import { apiRequest, type ApiKey } from "@/lib/api"

const ALL_WORKSPACES = "all"

export default function APIKeysPage() {
  const { workspaces, selectedWorkspaceId } = useWorkspace()
  const [apiKeys, setAPIKeys] = React.useState<ApiKey[]>([])
  const [workspaceId, setWorkspaceId] = React.useState("")
  const [name, setName] = React.useState("CLI key")
  const [newKey, setNewKey] = React.useState("")
  const [copied, setCopied] = React.useState(false)
  const [query, setQuery] = React.useState("")
  const [error, setError] = React.useState("")
  const [loading, setLoading] = React.useState(true)
  const [pending, setPending] = React.useState(false)
  const [createOpen, setCreateOpen] = React.useState(false)
  const [editingKey, setEditingKey] = React.useState<ApiKey | null>(null)
  const [editingName, setEditingName] = React.useState("")
  const [deletingKey, setDeletingKey] = React.useState<ApiKey | null>(null)

  const effectiveWorkspaceId =
    selectedWorkspaceId !== ALL_WORKSPACES ? selectedWorkspaceId : ""
  const availableWorkspaces = React.useMemo(
    () => workspaces.filter((workspace) => workspace.kind !== "playground"),
    [workspaces]
  )

  const loadAPIKeys = React.useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const workspaceQuery = effectiveWorkspaceId
        ? `?workspaceId=${encodeURIComponent(effectiveWorkspaceId)}`
        : ""
      const response = await apiRequest<{ apiKeys: ApiKey[] }>(
        `/api/v1/api-keys${workspaceQuery}`
      )
      setAPIKeys(response.apiKeys ?? [])
    } catch (loadError) {
      setError(
        loadError instanceof Error ? loadError.message : "Failed to load API keys"
      )
    } finally {
      setLoading(false)
    }
  }, [effectiveWorkspaceId])

  React.useEffect(() => {
    loadAPIKeys()
  }, [loadAPIKeys])

  React.useEffect(() => {
    if (
      workspaceId &&
      availableWorkspaces.some((workspace) => workspace.id === workspaceId)
    ) {
      return
    }
    if (
      effectiveWorkspaceId &&
      availableWorkspaces.some((workspace) => workspace.id === effectiveWorkspaceId)
    ) {
      setWorkspaceId(effectiveWorkspaceId)
      return
    }
    setWorkspaceId(availableWorkspaces[0]?.id ?? "")
  }, [availableWorkspaces, effectiveWorkspaceId, workspaceId])

  const workspaceNameById = React.useMemo(
    () => new Map(workspaces.map((workspace) => [workspace.id, workspace.name])),
    [workspaces]
  )

  const filteredAPIKeys = React.useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) {
      return apiKeys
    }

    return apiKeys.filter((apiKey) => {
      const workspaceName = workspaceNameById.get(apiKey.workspaceId) ?? ""
      return [apiKey.name, apiKey.prefix, workspaceName].some((value) =>
        value.toLowerCase().includes(normalizedQuery)
      )
    })
  }, [apiKeys, query, workspaceNameById])

  async function createAPIKey(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!workspaceId) {
      return
    }

    setError("")
    setPending(true)
    try {
      const response = await apiRequest<{ apiKey: ApiKey; key: string }>(
        "/api/v1/api-keys",
        {
          method: "POST",
          body: { workspaceId, name, scopes: ["ingest:write"] },
        }
      )
      setAPIKeys((current) => [response.apiKey, ...current])
      setCreateOpen(false)
      setNewKey(response.key)
      setName("CLI key")
      setCopied(false)
    } catch (createError) {
      setError(
        createError instanceof Error
          ? createError.message
          : "Failed to create API key"
      )
    } finally {
      setPending(false)
    }
  }

  function openEditDialog(apiKey: ApiKey) {
    setError("")
    setEditingName(apiKey.name)
    setEditingKey(apiKey)
  }

  async function updateAPIKey(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!editingKey || !editingName.trim()) {
      return
    }

    setError("")
    setPending(true)
    try {
      const response = await apiRequest<{ apiKey: ApiKey }>(
        `/api/v1/api-keys/${editingKey.id}`,
        { method: "PATCH", body: { name: editingName.trim() } }
      )
      setAPIKeys((current) =>
        current.map((apiKey) =>
          apiKey.id === response.apiKey.id ? response.apiKey : apiKey
        )
      )
      setEditingKey(null)
    } catch (updateError) {
      setError(
        updateError instanceof Error ? updateError.message : "Failed to update API key"
      )
    } finally {
      setPending(false)
    }
  }

  async function deleteAPIKey() {
    if (!deletingKey) {
      return
    }

    setError("")
    setPending(true)
    try {
      await apiRequest<void>(`/api/v1/api-keys/${deletingKey.id}`, {
        method: "DELETE",
      })
      setAPIKeys((current) =>
        current.filter((apiKey) => apiKey.id !== deletingKey.id)
      )
      setDeletingKey(null)
    } catch (deleteError) {
      setError(
        deleteError instanceof Error ? deleteError.message : "Failed to delete API key"
      )
    } finally {
      setPending(false)
    }
  }

  async function copyNewKey() {
    await navigator.clipboard.writeText(newKey)
    setCopied(true)
  }

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">API keys</h1>
          <p className="text-sm text-muted-foreground">
            Create and manage keys used to send scans from the CLI.
          </p>
        </div>
        <Button
          size="lg"
          className="w-full shadow-sm sm:w-auto"
          onClick={() => {
            setError("")
            setCreateOpen(true)
          }}
        >
          <PlusIcon data-icon="inline-start" />
          Create API key
        </Button>
      </div>

      {error && !createOpen && !editingKey && !deletingKey ? (
        <p className="rounded-lg border border-destructive/25 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      ) : null}

      <Card className="overflow-hidden">
        <CardHeader className="border-b bg-background/35">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <CardTitle>Active keys</CardTitle>
                <Badge variant="secondary">{apiKeys.length}</Badge>
              </div>
              <CardDescription className="mt-1">
                Keys are scoped to one workspace.
              </CardDescription>
            </div>
            <div className="flex w-full gap-2 lg:max-w-md">
              <div className="relative min-w-0 flex-1">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  className="pl-8"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Search keys..."
                  aria-label="Search API keys"
                />
              </div>
              <Button
                variant="outline"
                size="icon"
                onClick={loadAPIKeys}
                disabled={loading || pending}
                aria-label="Refresh API keys"
              >
                <RefreshCcwIcon className={loading ? "animate-spin" : ""} />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <div className="md:hidden">
            {loading ? (
              <p className="px-4 py-12 text-center text-sm text-muted-foreground">
                Loading API keys...
              </p>
            ) : null}
            {!loading && filteredAPIKeys.length === 0 ? (
              <APIKeysEmptyState hasQuery={Boolean(query)} />
            ) : null}
            {!loading
              ? filteredAPIKeys.map((apiKey) => (
                  <div key={apiKey.id} className="grid gap-3 border-b p-4 last:border-b-0">
                    <div className="flex min-w-0 items-start gap-3">
                      <div className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-primary/10 text-primary">
                        <KeyRoundIcon className="size-4" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium">{apiKey.name}</p>
                        <p className="truncate text-xs text-muted-foreground">
                          {workspaceNameById.get(apiKey.workspaceId) ?? apiKey.workspaceId}
                        </p>
                      </div>
                      <APIKeyActions
                        apiKey={apiKey}
                        onEdit={openEditDialog}
                        onDelete={(key) => {
                          setError("")
                          setDeletingKey(key)
                        }}
                      />
                    </div>
                    <div className="grid grid-cols-2 gap-3 text-xs">
                      <div className="min-w-0">
                        <p className="mb-1 text-muted-foreground">Token</p>
                        <code className="block truncate rounded-md bg-muted px-2 py-1 font-mono">
                          {apiKey.prefix}...
                        </code>
                      </div>
                      <div>
                        <p className="mb-1 text-muted-foreground">Last used</p>
                        <p className="py-1">
                          {apiKey.lastUsedAt
                            ? formatRelativeDate(apiKey.lastUsedAt)
                            : "No activity"}
                        </p>
                      </div>
                    </div>
                  </div>
                ))
              : null}
          </div>

          <div className="hidden md:block">
            <Table>
              <TableHeader className="bg-muted/35">
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Token</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead className="w-14 pr-4 text-right">
                    <span className="sr-only">Actions</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
              {loading ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={5} className="h-36 text-center text-muted-foreground">
                    Loading API keys...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && filteredAPIKeys.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={5} className="p-0 whitespace-normal">
                    <APIKeysEmptyState hasQuery={Boolean(query)} />
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading
                ? filteredAPIKeys.map((apiKey) => (
                    <TableRow key={apiKey.id}>
                      <TableCell className="pl-4 font-medium">
                        <div className="flex items-center gap-3">
                          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border bg-primary/10 text-primary">
                            <KeyRoundIcon className="size-4" />
                          </div>
                          <div className="min-w-0">
                            <p className="max-w-52 truncate">{apiKey.name}</p>
                            <p className="max-w-52 truncate text-xs font-normal text-muted-foreground">
                              {workspaceNameById.get(apiKey.workspaceId) ?? apiKey.workspaceId}
                            </p>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <code className="rounded-md bg-muted px-2 py-1 font-mono text-xs text-muted-foreground">
                          {apiKey.prefix}...
                        </code>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {apiKey.lastUsedAt
                          ? formatRelativeDate(apiKey.lastUsedAt)
                          : "No activity"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatRelativeDate(apiKey.createdAt)}
                      </TableCell>
                      <TableCell className="pr-4 text-right">
                        <APIKeyActions
                          apiKey={apiKey}
                          onEdit={openEditDialog}
                          onDelete={(key) => {
                            setError("")
                            setDeletingKey(key)
                          }}
                        />
                      </TableCell>
                    </TableRow>
                  ))
                : null}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={createAPIKey}>
            <DialogHeader>
              <DialogTitle>Create API key</DialogTitle>
              <DialogDescription>
                The full key is shown once after creation. Store it somewhere secure.
              </DialogDescription>
            </DialogHeader>
            <FieldGroup className="mt-5">
              <Field>
                <FieldLabel htmlFor="api-key-name">Name</FieldLabel>
                <Input
                  id="api-key-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  maxLength={80}
                  autoFocus
                  required
                />
              </Field>
              <Field>
                <FieldLabel>Workspace</FieldLabel>
                <Select
                  value={workspaceId}
                  onValueChange={(value) => value && setWorkspaceId(value)}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Select workspace">
                      {(value) =>
                        availableWorkspaces.find(
                          (workspace) => workspace.id === String(value)
                        )?.name ?? "Select workspace"
                      }
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {availableWorkspaces.map((workspace) => (
                        <SelectItem key={workspace.id} value={workspace.id}>
                          {workspace.name}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              {error ? <FieldError>{error}</FieldError> : null}
            </FieldGroup>
            <DialogFooter className="mt-5">
              <Button
                type="button"
                variant="outline"
                onClick={() => setCreateOpen(false)}
                disabled={pending}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={pending || !workspaceId || !name.trim()}>
                <PlusIcon data-icon="inline-start" />
                {pending ? "Creating..." : "Create API key"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(editingKey)} onOpenChange={(open) => !open && setEditingKey(null)}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={updateAPIKey}>
            <DialogHeader>
              <DialogTitle>Edit API key</DialogTitle>
              <DialogDescription>
                Rename this key so its purpose stays easy to identify.
              </DialogDescription>
            </DialogHeader>
            <FieldGroup className="mt-5">
              <Field>
                <FieldLabel htmlFor="edit-api-key-name">Name</FieldLabel>
                <Input
                  id="edit-api-key-name"
                  value={editingName}
                  onChange={(event) => setEditingName(event.target.value)}
                  maxLength={80}
                  autoFocus
                  required
                />
              </Field>
              {error ? <FieldError>{error}</FieldError> : null}
            </FieldGroup>
            <DialogFooter className="mt-5">
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditingKey(null)}
                disabled={pending}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={pending || !editingName.trim()}>
                {pending ? "Saving..." : "Save changes"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog
        open={Boolean(deletingKey)}
        onOpenChange={(open) => !open && !pending && setDeletingKey(null)}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <div className="mb-1 flex size-9 items-center justify-center rounded-lg bg-destructive/10 text-destructive">
              <TriangleAlertIcon className="size-4" />
            </div>
            <DialogTitle>Delete API key</DialogTitle>
            <DialogDescription>
              <strong className="font-medium text-foreground">{deletingKey?.name}</strong>
              {" will stop working immediately. This action cannot be undone."}
            </DialogDescription>
          </DialogHeader>
          {error ? <FieldError>{error}</FieldError> : null}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeletingKey(null)}
              disabled={pending}
            >
              Cancel
            </Button>
            <Button variant="destructive" onClick={deleteAPIKey} disabled={pending}>
              <Trash2Icon data-icon="inline-start" />
              {pending ? "Deleting..." : "Delete API key"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(newKey)} onOpenChange={(open) => !open && setNewKey("")}>
        <DialogContent className="w-[calc(100vw-2rem)] sm:max-w-2xl">
          <DialogHeader>
            <div className="mb-1 flex size-9 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-500">
              <CheckIcon className="size-4" />
            </div>
            <DialogTitle>API key created</DialogTitle>
            <DialogDescription>
              Copy this key now. For security, it will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="flex min-w-0 gap-2">
            <Input className="min-w-0 flex-1 font-mono text-xs" value={newKey} readOnly />
            <Button
              className="shrink-0"
              variant="outline"
              onClick={copyNewKey}
              aria-label="Copy API key"
            >
              {copied ? <CheckIcon data-icon="inline-start" /> : <CopyIcon data-icon="inline-start" />}
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
          <div className="min-w-0 overflow-x-auto rounded-lg border bg-muted/60 px-3 py-2.5">
            <code className="block w-max whitespace-nowrap font-mono text-xs leading-5">
              RUNTZ_API_KEY={newKey}
            </code>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewKey("")}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function APIKeyActions({
  apiKey,
  onEdit,
  onDelete,
}: {
  apiKey: ApiKey
  onEdit: (apiKey: ApiKey) => void
  onDelete: (apiKey: ApiKey) => void
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Actions for ${apiKey.name}`}
          />
        }
      >
        <MoreHorizontalIcon />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuItem onClick={() => onEdit(apiKey)}>
          <PencilIcon />
          Edit API key
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={() => onDelete(apiKey)}>
          <Trash2Icon />
          Delete API key
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function APIKeysEmptyState({ hasQuery }: { hasQuery: boolean }) {
  return (
    <Empty className="min-h-52 border-0">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <KeyRoundIcon />
        </EmptyMedia>
        <EmptyTitle>{hasQuery ? "No matching API keys" : "No API keys yet"}</EmptyTitle>
        <EmptyDescription>
          {hasQuery
            ? "Try another name, token prefix or workspace."
            : "Create a key to connect the Runtz CLI to this workspace."}
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

function formatRelativeDate(value: string) {
  const date = new Date(value)
  const elapsed = Date.now() - date.getTime()
  const days = Math.floor(elapsed / 86_400_000)

  if (days <= 0) {
    return "Today"
  }
  if (days === 1) {
    return "Yesterday"
  }
  if (days < 30) {
    return `${days} days ago`
  }

  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "medium",
  }).format(date)
}
