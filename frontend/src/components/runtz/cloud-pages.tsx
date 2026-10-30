"use client"

import * as React from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { ArrowLeftIcon, ArrowRightIcon, ChevronLeftIcon, ChevronRightIcon, ExternalLinkIcon, KeyRoundIcon, LoaderCircleIcon, PlusIcon, RefreshCwIcon, SearchIcon, SettingsIcon, ShieldCheckIcon, UnplugIcon, UploadIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { GCPIcon } from "@/components/runtz/cloud-icons"
import { SeverityBadge } from "@/components/runtz/sca-components"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { Table, TableHeader, TableHead, TableBody, TableRow, TableCell } from "@/components/ui/table"
import { apiRequest } from "@/lib/api"
import { cloudError, cloudSeverities, cloudSeverityColors, emptyCloudCounts, safeCloudLink, type CloudConnection, type CloudCounts, type CloudFinding, type CloudFindingResponse } from "@/lib/cloud"
import { formatDate } from "@/lib/sca"

function CloudSummary({ summary, selected, onSelect }: { summary: CloudCounts; selected?: string; onSelect?: (severity: string) => void }) {
  return <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
    {(["total", ...cloudSeverities] as const).map(severity => {
      const value = severity === "total" ? "all" : severity
      const content = <><span className="text-xs text-muted-foreground">{severity === "total" ? "Active findings" : severity === "unknown" ? "Unspecified" : severity[0].toUpperCase() + severity.slice(1)}</span><span className={`mt-2 block text-2xl font-semibold tabular-nums ${cloudSeverityColors[severity] ?? ""}`}>{summary[severity]}</span></>
      const className = `rounded-xl border bg-card p-4 text-left ${selected === value ? "border-primary/70 ring-1 ring-primary/20" : ""}`
      return onSelect ? <button key={severity} aria-pressed={selected === value} className={`${className} transition-colors hover:border-primary/50 focus-visible:outline-2 focus-visible:outline-primary`} onClick={() => onSelect(value)}>{content}</button> : <div key={severity} className={className}>{content}</div>
    })}
  </div>
}

function ConnectionStatus({ connection }: { connection: CloudConnection }) {
  return <Badge variant="outline" className={connection.status === "error" ? "text-destructive" : "text-muted-foreground"}><span className={`size-1.5 rounded-full ${connection.status === "error" ? "bg-destructive" : connection.status === "syncing" ? "animate-pulse bg-primary" : "bg-emerald-500"}`} />{connection.status === "error" ? "Import failed" : connection.status === "syncing" ? "Importing" : "Connected"}</Badge>
}

export function GCPConnectionsPage() {
  const { connections, loading, error, configured, openConnect, refresh, canManage } = useCloud()
  const { workspaces } = useWorkspace()
  const router = useRouter()
  const noConnections = !loading && !error && connections.length === 0
  React.useEffect(() => {
    if (noConnections) router.replace("/overview")
  }, [noConnections, router])
  if (noConnections) return null
  const canAdd = workspaces.some(w => canManage(w.id))
  const totals = connections.reduce((sum, c) => {
    for (const key of Object.keys(sum) as (keyof CloudCounts)[]) sum[key] += c.summary[key]
    return sum
  }, { ...emptyCloudCounts })
  return <div className="flex flex-col gap-6 p-4 md:p-6">
    <div className="flex flex-wrap items-start justify-between gap-4"><div className="space-y-2"><div className="flex items-center gap-2"><Badge variant="outline">CLOUD</Badge><Badge variant="secondary">Native findings</Badge></div><h1 className="flex items-center gap-2 text-2xl font-semibold"><GCPIcon className="size-6" />Google Cloud</h1><p className="text-sm text-muted-foreground">Your GCP projects and their security findings, together in Runtz.</p></div>{canAdd && <Button onClick={openConnect}><PlusIcon />Add GCP connection</Button>}</div>
    {error && <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 p-4 text-sm"><span className="text-destructive">{error}</span><Button variant="outline" size="sm" onClick={() => void refresh()}>Retry</Button></div>}
    {!configured && !loading && !error && canAdd && <p className="rounded-xl border bg-muted/30 p-4 text-sm text-muted-foreground">Cloud integrations need an encryption key configured by your Runtz administrator. Set <code className="break-all text-xs">RUNTZ_CLOUD_ENCRYPTION_KEY</code> on the engine before uploading credentials.</p>}
    {loading && connections.length === 0 ? <><div className="grid grid-cols-2 gap-3 md:grid-cols-6">{cloudSeverities.map(s => <Skeleton key={s} className="h-24" />)}</div><Skeleton className="h-52" /></> : connections.length > 0 && <>
      <CloudSummary summary={totals} />
      <Card><CardHeader><CardTitle>GCP connections <span className="ml-1 text-muted-foreground">{connections.length}</span></CardTitle><CardDescription>Open a project to review findings or import its latest results.</CardDescription></CardHeader><CardContent>
        <Table><TableHeader><TableRow><TableHead>Connection / project</TableHead><TableHead>Workspace</TableHead><TableHead>Active findings</TableHead><TableHead>Status</TableHead><TableHead>Last import</TableHead><TableHead><span className="sr-only">Open</span></TableHead></TableRow></TableHeader><TableBody>{connections.map(c => <TableRow key={c.id}>
          <TableCell><Link className="flex items-center gap-3 font-medium hover:text-primary" href={`/cloud/gcp/${c.id}`}><GCPIcon className="size-5 shrink-0" /><span>{c.name}<span className="mt-1 block font-mono text-xs font-normal text-muted-foreground">{c.projectId} · {c.location}</span></span></Link></TableCell>
          <TableCell className="text-muted-foreground">{workspaces.find(w => w.id === c.workspaceId)?.name ?? "Workspace"}</TableCell><TableCell><div className="flex items-center gap-2"><span className="tabular-nums">{c.summary.total}</span>{c.summary.critical > 0 && <span className="text-xs text-severity-critical">{c.summary.critical} critical</span>}{c.summary.high > 0 && <span className="text-xs text-severity-high">{c.summary.high} high</span>}</div></TableCell><TableCell><ConnectionStatus connection={c} /></TableCell><TableCell className="text-xs text-muted-foreground">{c.lastScanAt ? formatDate(c.lastScanAt) : "Not imported"}</TableCell><TableCell><Button variant="ghost" size="icon-sm" aria-label={`Open ${c.name}`} render={<Link href={`/cloud/gcp/${c.id}`} />}><ArrowRightIcon /></Button></TableCell>
        </TableRow>)}</TableBody></Table>
      </CardContent></Card>
      <p className="text-xs text-muted-foreground">Counts include active, unmuted native findings from the latest successful import of each connection.</p>
    </>}
  </div>
}

export function GCPProjectPage({ connectionId }: { connectionId: string }) {
  const { connections, loading, error: connectionError, refresh, canManage } = useCloud()
  const { selectedWorkspaceId } = useWorkspace()
  const connection = connections.find(c => c.id === connectionId)
  const [severity, setSeverity] = React.useState("all")
  const [state, setState] = React.useState("active")
  const [search, setSearch] = React.useState("")
  const [query, setQuery] = React.useState("")
  const [page, setPage] = React.useState(1)
  const [data, setData] = React.useState<CloudFindingResponse | null>(null)
  const [fetching, setFetching] = React.useState(true)
  const [error, setError] = React.useState("")
  const [syncing, setSyncing] = React.useState(false)
  const [version, setVersion] = React.useState(0)
  const [finding, setFinding] = React.useState<CloudFinding | null>(null)
  const [settings, setSettings] = React.useState(false)
  const inScope = !!connection
  React.useEffect(() => { const timer = setTimeout(() => { setQuery(search); setPage(1) }, 300); return () => clearTimeout(timer) }, [search])
  React.useEffect(() => {
    setData(null); setFinding(null); setSettings(false); setPage(1)
  }, [connectionId, selectedWorkspaceId])
  React.useEffect(() => {
    if (!inScope) return
    const abort = new AbortController()
    setFetching(true); setError("")
    const params = new URLSearchParams({ severity, state, search: query, page: String(page) })
    apiRequest<CloudFindingResponse>(`/api/v1/cloud/connections/${connectionId}/findings?${params}`, { signal: abort.signal })
      .then(result => { if (!abort.signal.aborted) setData(result) })
      .catch(err => { if (!abort.signal.aborted) { setError(cloudError(err)); setData(null) } })
      .finally(() => { if (!abort.signal.aborted) setFetching(false) })
    return () => abort.abort()
  }, [inScope, connectionId, connection?.updatedAt, selectedWorkspaceId, severity, state, query, page, version])

  async function scan() {
    setSyncing(true); setError("")
    try { await apiRequest(`/api/v1/cloud/connections/${connectionId}/scan`, { method: "POST" }); await refresh(); setPage(1); setVersion(v => v + 1) }
    catch (err) { setError(cloudError(err)); await refresh() }
    finally { setSyncing(false) }
  }
  if (!connection) return <div className="space-y-4 p-6"><Link href="/cloud/gcp" className="inline-flex items-center gap-2 text-sm text-muted-foreground"><ArrowLeftIcon className="size-4" />Google Cloud</Link>{loading ? <Skeleton className="h-60" /> : <p role="alert" className="rounded-xl border p-6 text-sm text-muted-foreground">{connectionError || "This connection is unavailable in the selected workspace. Choose its workspace or return to Google Cloud."}</p>}</div>
  return <div className="flex flex-col gap-6 p-4 md:p-6">
    <Link href="/cloud/gcp" className="inline-flex w-fit items-center gap-2 text-xs text-muted-foreground hover:text-primary"><ArrowLeftIcon className="size-3.5" />Google Cloud</Link>
    <div className="flex flex-wrap items-start justify-between gap-4"><div className="space-y-2"><div className="flex flex-wrap items-center gap-2"><Badge variant="outline">CLOUD</Badge><ConnectionStatus connection={connection} /></div><h1 className="flex items-center gap-2 text-2xl font-semibold"><GCPIcon className="size-6" />{connection.name}</h1><p className="font-mono text-xs text-muted-foreground">{connection.projectId} · {connection.location}</p></div>
      {canManage(connection.workspaceId) && <div className="flex gap-2"><Button variant="outline" onClick={() => setSettings(true)} disabled={syncing}><SettingsIcon />Configure</Button><Button onClick={() => void scan()} disabled={syncing || connection.status === "syncing"}>{syncing ? <LoaderCircleIcon className="animate-spin" /> : <RefreshCwIcon />}{syncing ? "Importing…" : "Import findings"}</Button></div>}
    </div>
    {(error || connection.lastError) && <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"><p className="text-destructive">{error || connection.lastError}</p>{connection.lastScanAt && <p className="mt-1 text-xs text-muted-foreground">Showing the last successful import from {formatDate(connection.lastScanAt)}.</p>}<Button variant="ghost" size="sm" className="mt-2" onClick={() => { setError(""); setVersion(v => v + 1); void refresh() }}>Retry loading</Button></div>}
    <CloudSummary summary={connection.summary} selected={severity} onSelect={value => { setSeverity(value); setPage(1) }} />
    <Card><CardHeader><div className="flex flex-wrap items-start justify-between gap-2"><div className="space-y-1"><CardTitle>Security findings</CardTitle><CardDescription>Native findings from Security Command Center.</CardDescription></div><p className="text-xs text-muted-foreground">{syncing ? "Import in progress…" : connection.lastScanAt ? `Last import ${formatDate(connection.lastScanAt)}` : "No successful import"}</p></div></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-3"><div className="relative min-w-48 flex-1"><SearchIcon className="absolute top-2.5 left-3 size-4 text-muted-foreground" /><Input aria-label="Search findings" maxLength={200} placeholder="Search findings, resources or sources" className="pl-9" value={search} onChange={event => setSearch(event.target.value)} /></div>
          <select aria-label="Filter severity" value={severity} onChange={event => { setSeverity(event.target.value); setPage(1) }} className="h-9 rounded-lg border bg-background px-3 text-sm"><option value="all">All severities</option>{cloudSeverities.map(s => <option key={s} value={s}>{s === "unknown" ? "Unspecified" : s[0].toUpperCase() + s.slice(1)}</option>)}</select>
          <select aria-label="Filter finding state" value={state} onChange={event => { setState(event.target.value); setPage(1) }} className="h-9 rounded-lg border bg-background px-3 text-sm"><option value="active">Active, unmuted</option><option value="all">All states</option></select>
        </div>
        {fetching ? <div role="status" className="space-y-3"><span className="sr-only">Loading findings</span>{[1, 2, 3].map(i => <Skeleton key={i} className="h-12" />)}</div> : data && data.findings.length > 0 ? <>
          <Table><TableHeader><TableRow><TableHead>Finding</TableHead><TableHead>Severity</TableHead><TableHead>Resource</TableHead><TableHead>Source</TableHead><TableHead>State</TableHead></TableRow></TableHeader><TableBody>{data.findings.map(f => <TableRow key={f.id}><TableCell className="max-w-sm whitespace-normal"><button onClick={() => setFinding(f)} className="text-left font-medium underline-offset-4 hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-primary">{f.title}</button><p className="mt-1 text-xs text-muted-foreground">{f.category}</p></TableCell><TableCell><SeverityBadge severity={f.severity} /></TableCell><TableCell className="max-w-72"><p className="truncate font-mono text-xs" title={f.resourceName}>{f.resourceName || "Project resource"}</p><p className="mt-1 text-xs text-muted-foreground">{f.location || connection.location}</p></TableCell><TableCell className="max-w-48 whitespace-normal text-xs text-muted-foreground">{f.source || "Security Command Center"}</TableCell><TableCell><Badge variant="outline">{f.state === "INACTIVE" ? "Inactive" : "Active"}{f.mute === "MUTED" ? " · Muted" : ""}</Badge></TableCell></TableRow>)}</TableBody></Table>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4 text-xs text-muted-foreground"><span>{(data.page - 1) * data.pageSize + 1}–{Math.min(data.page * data.pageSize, data.total)} of {data.total} findings</span><div className="flex items-center gap-2"><Button variant="outline" size="icon-sm" aria-label="Previous page" disabled={page <= 1} onClick={() => setPage(p => p - 1)}><ChevronLeftIcon /></Button><span>Page {page}</span><Button variant="outline" size="icon-sm" aria-label="Next page" disabled={page * data.pageSize >= data.total} onClick={() => setPage(p => p + 1)}><ChevronRightIcon /></Button></div></div>
        </> : data && <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center"><ShieldCheckIcon className="size-8 text-primary" /><h3 className="font-medium">{query || severity !== "all" || state === "all" ? "No findings match these filters" : "No active findings reported"}</h3><p className="max-w-md text-sm text-muted-foreground">{query || severity !== "all" || state === "all" ? "Adjust the search or filters to see more findings." : "SCC returned no active, unmuted findings in this data location. Check SCC activation and detector coverage in GCP before assessing your project's security."}</p></div>}
      </CardContent>
    </Card>
    <p className="text-xs text-muted-foreground">Importing reads existing GCP findings. To run or configure cloud detectors, use Security Command Center.</p>
    {finding && <Dialog open onOpenChange={open => { if (!open) setFinding(null) }}><DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl"><DialogHeader><DialogTitle className="pr-6 leading-snug">{finding.title}</DialogTitle><DialogDescription>{finding.category}</DialogDescription></DialogHeader><div className="flex flex-wrap gap-2"><SeverityBadge severity={finding.severity} /><Badge variant="outline">{finding.state}</Badge>{finding.mute === "MUTED" && <Badge variant="secondary">Muted</Badge>}</div><p className="whitespace-pre-wrap text-sm leading-relaxed">{finding.description || "This provider finding has no additional description."}</p><dl className="space-y-3 rounded-lg border bg-muted/20 p-4 text-sm">{[["Resource", finding.resourceName], ["Resource type", finding.resourceType], ["Project", finding.projectName || connection.projectId], ["Source", finding.source || "Security Command Center"], ["Location", finding.location || connection.location], ["Event time", finding.eventTime ? formatDate(finding.eventTime) : "Not provided"]].map(([label, value]) => <div key={label}><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl>{finding.remediation && <div><h4 className="mb-2 font-medium">Provider remediation</h4><p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{finding.remediation}</p></div>}<Button variant="outline" render={<a href={safeCloudLink(finding.externalUri) ?? `https://console.cloud.google.com/security/command-center/findings?project=${encodeURIComponent(connection.projectId)}`} target="_blank" rel="noopener noreferrer" />}><ExternalLinkIcon />Open in Google Cloud</Button></DialogContent></Dialog>}
    {settings && canManage(connection.workspaceId) && <CloudConnectionSettings connection={connection} onClose={() => setSettings(false)} />}
  </div>
}

function CloudConnectionSettings({ connection, onClose }: { connection: CloudConnection; onClose: () => void }) {
  const { refresh, connections } = useCloud()
  const router = useRouter()
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [notice, setNotice] = React.useState("")
  const [confirm, setConfirm] = React.useState("")
  async function replace(file?: File) {
    if (!file) return
    setError(""); setNotice("")
    if (file.size > 32768) { setError("Choose a JSON key smaller than 32 KB."); return }
    setBusy(true)
    try {
      let credentials: unknown
      try { credentials = JSON.parse(await file.text()) } catch { throw new Error("Choose a valid JSON service account key.") }
      await apiRequest(`/api/v1/cloud/connections/${connection.id}/credentials`, { method: "PATCH", body: { credentials } })
      setNotice("Credentials replaced and verified. Revoke the previous key in GCP, then import findings to refresh your results.")
      await refresh()
    } catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}><DialogContent showCloseButton={!busy} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg"><DialogHeader><DialogTitle>Configure {connection.name}</DialogTitle><DialogDescription>Manage access to {connection.projectId}.</DialogDescription></DialogHeader>
    <div className="space-y-3 rounded-lg border p-4"><h3 className="flex items-center gap-2 font-medium"><KeyRoundIcon className="size-4" />Service account credentials</h3><p className="break-all text-xs text-muted-foreground">{connection.serviceAccount}</p><p className="text-sm text-muted-foreground">Upload a new JSON key to rotate credentials. Runtz verifies access before replacing the encrypted key.</p><label className={`inline-flex h-9 cursor-pointer items-center gap-2 rounded-lg border px-3 text-sm ${busy ? "pointer-events-none opacity-50" : "hover:bg-accent"}`}>{busy ? <LoaderCircleIcon className="size-4 animate-spin" /> : <UploadIcon className="size-4" />}Replace JSON key<input aria-label="Replace service account JSON key" className="sr-only" type="file" accept=".json,application/json" disabled={busy} onChange={event => { void replace(event.target.files?.[0]); event.target.value = "" }} /></label></div>
    <div className="space-y-3 rounded-lg border border-destructive/25 p-4"><h3 className="flex items-center gap-2 font-medium"><UnplugIcon className="size-4" />Disconnect cloud</h3><p className="text-sm text-muted-foreground">Removes this connection, its saved credentials and imported findings from Runtz. Revoke the service account key in GCP separately.</p><Input aria-label="Type project ID to confirm disconnection" placeholder={`Type ${connection.projectId} to confirm`} value={confirm} onChange={event => setConfirm(event.target.value)} disabled={busy} /><Button variant="destructive" disabled={busy || confirm !== connection.projectId} onClick={async () => {
      setBusy(true); setError("")
      try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh(); onClose(); router.push(connections.length > 1 ? "/cloud/gcp" : "/overview") }
      catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
    }}>Disconnect GCP</Button></div>
    {notice && <p role="status" className="text-sm text-primary">{notice}</p>}{error && <p role="alert" className="text-sm text-destructive">{error}</p>}
  </DialogContent></Dialog>
}
