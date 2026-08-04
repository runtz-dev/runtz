"use client"

import * as React from "react"
import { CheckIcon, CopyIcon, PlusIcon, RefreshCcwIcon, XIcon } from "lucide-react"

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
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
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
import { apiRequest, type ApiKey } from "@/lib/api"

const ALL_WORKSPACES = "all"

export default function APIKeysPage() {
  const { workspaces, selectedWorkspaceId } = useWorkspace()
  const [apiKeys, setAPIKeys] = React.useState<ApiKey[]>([])
  const [workspaceId, setWorkspaceId] = React.useState("")
  const [name, setName] = React.useState("CLI key")
  const [newKey, setNewKey] = React.useState("")
  const [copied, setCopied] = React.useState(false)
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  const effectiveWorkspaceId =
    selectedWorkspaceId !== ALL_WORKSPACES ? selectedWorkspaceId : ""

  const loadAPIKeys = React.useCallback(async () => {

    const query = effectiveWorkspaceId
      ? `?workspaceId=${encodeURIComponent(effectiveWorkspaceId)}`
      : ""
    const response = await apiRequest<{ apiKeys: ApiKey[] }>(
      `/api/v1/api-keys${query}`
    )
    setAPIKeys(response.apiKeys ?? [])
  }, [effectiveWorkspaceId])

  React.useEffect(() => {
    loadAPIKeys().catch(() => setAPIKeys([]))
  }, [loadAPIKeys])

  React.useEffect(() => {
    if (workspaceId && workspaces.some((workspace) => workspace.id === workspaceId)) {
      return
    }
    if (effectiveWorkspaceId) {
      setWorkspaceId(effectiveWorkspaceId)
      return
    }
    setWorkspaceId(workspaces[0]?.id ?? "")
  }, [effectiveWorkspaceId, workspaceId, workspaces])

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
      setNewKey(response.key)
      setName("CLI key")
      setCopied(false)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to create API key")
    } finally {
      setPending(false)
    }
  }

  async function revokeAPIKey(apiKey: ApiKey) {

    setError("")
    setPending(true)
    try {
      await apiRequest(`/api/v1/api-keys/${apiKey.id}/revoke`, {
        method: "PATCH",
      })
      await loadAPIKeys()
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to revoke API key")
    } finally {
      setPending(false)
    }
  }

  async function copyNewKey() {
    await navigator.clipboard.writeText(newKey)
    setCopied(true)
  }

  const workspaceNameById = new Map(
    workspaces.map((workspace) => [workspace.id, workspace.name])
  )

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-normal">API Keys</h1>
        <p className="text-sm text-muted-foreground">
          Chaves para conectar o CLI ao workspace selecionado.
        </p>
      </div>

      <div className="grid gap-6 xl:grid-cols-[380px_1fr]">
        <Card>
          <CardHeader>
            <CardTitle>Nova API key</CardTitle>
            <CardDescription>A chave completa aparece apenas uma vez.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={createAPIKey}>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="api-key-name">Nome</FieldLabel>
                  <Input
                    id="api-key-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    required
                  />
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
                  <FieldDescription>
                    A API key sempre resolve o workspace por baixo dos panos.
                  </FieldDescription>
                </Field>
                {error ? (
                  <Field>
                    <FieldError>{error}</FieldError>
                  </Field>
                ) : null}
                <Button type="submit" disabled={pending || !workspaceId}>
                  <PlusIcon data-icon="inline-start" />
                  Gerar API key
                </Button>
              </FieldGroup>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-3">
              <div>
                <CardTitle>API keys ativas</CardTitle>
                <CardDescription>{apiKeys.length} cadastradas</CardDescription>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => loadAPIKeys()}
                disabled={pending}
              >
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
                    <TableHead>Nome</TableHead>
                    <TableHead>Workspace</TableHead>
                    <TableHead>Prefixo</TableHead>
                    <TableHead>Último uso</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {apiKeys.map((apiKey) => (
                    <TableRow key={apiKey.id}>
                      <TableCell className="font-medium">{apiKey.name}</TableCell>
                      <TableCell>
                        {workspaceNameById.get(apiKey.workspaceId) ??
                          apiKey.workspaceId}
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline">{apiKey.prefix}</Badge>
                      </TableCell>
                      <TableCell>
                        {apiKey.lastUsedAt ? formatDate(apiKey.lastUsedAt) : "nunca"}
                      </TableCell>
                      <TableCell>
                        <div className="flex justify-end">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => revokeAPIKey(apiKey)}
                            disabled={pending}
                          >
                            <XIcon data-icon="inline-start" />
                            Revogar
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
      </div>

      <Dialog open={Boolean(newKey)} onOpenChange={(open) => !open && setNewKey("")}>
        <DialogContent className="w-[calc(100vw-2rem)] sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>API key gerada</DialogTitle>
            <DialogDescription>
              Use this key in the CLI. It will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="flex min-w-0 gap-2">
            <Input className="min-w-0 flex-1 font-mono text-xs" value={newKey} readOnly />
            <Button
              className="shrink-0"
              size="icon"
              variant="outline"
              onClick={copyNewKey}
              aria-label="Copy API key"
            >
              {copied ? <CheckIcon /> : <CopyIcon />}
            </Button>
          </div>
          <div className="min-w-0 overflow-x-auto rounded-md bg-muted px-3 py-2">
            <code className="block w-max whitespace-nowrap font-mono text-xs leading-5">
              RUNTZ_API_KEY={newKey}
            </code>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("pt-BR", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value))
}
