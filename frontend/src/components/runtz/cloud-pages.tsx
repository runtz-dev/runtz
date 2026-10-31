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
import { cloudError, cloudSeverities, cloudSeverityColors, emptyCloudCounts, safeCloudLink, type CloudConnection, type CloudCounts, type CloudFinding, type CloudFindingResponse, type CloudCheckResponse } from "@/lib/cloud"
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
  const labels: Record<string, string> = { connecting: "Connecting", syncing: "Scanning", partial: "Partial coverage", error: "Scan failed", unavailable: "Unavailable", reconnect_required: "Reconnect", connected: "Connected" }
  const failed = ["error", "unavailable", "reconnect_required"].includes(connection.status)
  const scanning = ["connecting", "syncing"].includes(connection.status)
  return <Badge variant="outline" className={failed ? "text-destructive" : connection.status === "partial" ? "text-amber-500" : "text-muted-foreground"}><span className={`size-1.5 rounded-full ${failed ? "bg-destructive" : scanning ? "animate-pulse bg-primary" : connection.status === "partial" ? "bg-amber-500" : "bg-emerald-500"}`} />{labels[connection.status] ?? connection.status}</Badge>
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
    <div className="flex flex-wrap items-start justify-between gap-4"><div className="space-y-2"><div className="flex items-center gap-2"><Badge variant="outline">CLOUD</Badge><Badge variant="secondary">Configuration checks</Badge></div><h1 className="flex items-center gap-2 text-2xl font-semibold"><GCPIcon className="size-6" />Google Cloud</h1><p className="text-sm text-muted-foreground">Your GCP projects and their security findings, together in Runtz.</p></div>{canAdd && <Button onClick={openConnect}><PlusIcon />Add GCP connection</Button>}</div>
    {error && <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 p-4 text-sm"><span className="text-destructive">{error}</span><Button variant="outline" size="sm" onClick={() => void refresh()}>Retry</Button></div>}
    {!configured && !loading && !error && canAdd && <p className="rounded-xl border bg-muted/30 p-4 text-sm text-muted-foreground">Cloud checks are unavailable. Ask your Runtz administrator to connect the Security service.</p>}
    {loading && connections.length === 0 ? <><div className="grid grid-cols-2 gap-3 md:grid-cols-6">{cloudSeverities.map(s => <Skeleton key={s} className="h-24" />)}</div><Skeleton className="h-52" /></> : connections.length > 0 && <>
      <CloudSummary summary={totals} />
      <Card><CardHeader><CardTitle>GCP connections <span className="ml-1 text-muted-foreground">{connections.length}</span></CardTitle><CardDescription>Open a project to review findings and scan coverage.</CardDescription></CardHeader><CardContent>
        <Table><TableHeader><TableRow><TableHead>Connection / project</TableHead><TableHead>Workspace</TableHead><TableHead>Active findings</TableHead><TableHead>Status</TableHead><TableHead>Last scan</TableHead><TableHead><span className="sr-only">Open</span></TableHead></TableRow></TableHeader><TableBody>{connections.map(c => <TableRow key={c.id}>
          <TableCell><Link className="flex items-center gap-3 font-medium hover:text-primary" href={`/cloud/gcp/${c.id}`}><GCPIcon className="size-5 shrink-0" /><span>{c.name}<span className="mt-1 block font-mono text-xs font-normal text-muted-foreground">{c.projectId}</span></span></Link></TableCell>
          <TableCell className="text-muted-foreground">{workspaces.find(w => w.id === c.workspaceId)?.name ?? "Workspace"}</TableCell><TableCell><div className="flex items-center gap-2"><span className="tabular-nums">{c.summary.total}</span>{c.summary.critical > 0 && <span className="text-xs text-severity-critical">{c.summary.critical} critical</span>}{c.summary.high > 0 && <span className="text-xs text-severity-high">{c.summary.high} high</span>}</div></TableCell><TableCell><ConnectionStatus connection={c} /></TableCell><TableCell className="text-xs text-muted-foreground">{c.lastScanAt ? formatDate(c.lastScanAt) : "Waiting for scan"}</TableCell><TableCell><Button variant="ghost" size="icon-sm" aria-label={`Open ${c.name}`} render={<Link href={`/cloud/gcp/${c.id}`} />}><ArrowRightIcon /></Button></TableCell>
        </TableRow>)}</TableBody></Table>
      </CardContent></Card>
      <p className="text-xs text-muted-foreground">Counts show findings from the latest published scan. Review partial coverage before assessing a project.</p>
    </>}
  </div>
}

export function GCPProjectPage({ connectionId }: { connectionId: string }) {
  const { connections, loading, error: connectionError, refresh, canManage } = useCloud()
  const { selectedWorkspaceId } = useWorkspace()
  const connection = connections.find(c => c.id === connectionId)
  const [tab, setTab] = React.useState<"findings" | "checks">("findings")
  const [severity, setSeverity] = React.useState("all")
  const [search, setSearch] = React.useState("")
  const [query, setQuery] = React.useState("")
  const [page, setPage] = React.useState(1)
  const [data, setData] = React.useState<CloudFindingResponse | null>(null)
  const [fetching, setFetching] = React.useState(true)
  const [error, setError] = React.useState("")
  const [queuing, setQueuing] = React.useState(false)
  const [version, setVersion] = React.useState(0)
  const [finding, setFinding] = React.useState<CloudFinding | null>(null)
  const [settings, setSettings] = React.useState(false)
  const inScope = !!connection
  React.useEffect(() => { const timer = setTimeout(() => { setQuery(search); setPage(1) }, 300); return () => clearTimeout(timer) }, [search])
  React.useEffect(() => { setData(null); setFinding(null); setSettings(false); setPage(1) }, [connectionId, selectedWorkspaceId])
  React.useEffect(() => {
    if (!inScope || tab !== "findings") return
    const abort = new AbortController()
    setFetching(true); setError("")
    const params = new URLSearchParams({ severity, search: query, page: String(page) })
    apiRequest<CloudFindingResponse>(`/api/v1/cloud/connections/${connectionId}/findings?${params}`, { signal: abort.signal })
      .then(result => { if (!abort.signal.aborted) setData(result) })
      .catch(err => { if (!abort.signal.aborted) { setError(cloudError(err)); setData(null) } })
      .finally(() => { if (!abort.signal.aborted) setFetching(false) })
    return () => abort.abort()
  }, [inScope, tab, connectionId, connection?.updatedAt, connection?.lastScanAt, connection?.status, selectedWorkspaceId, severity, query, page, version])
  async function scan() {
    setQueuing(true); setError("")
    try { await apiRequest(`/api/v1/cloud/connections/${connectionId}/scan`, { method: "POST" }); await refresh(); setPage(1); setVersion(v => v + 1) }
    catch (err) { setError(cloudError(err)); await refresh() }
    finally { setQueuing(false) }
  }
  if (!connection) return <div className="space-y-4 p-6"><Link href="/cloud/gcp" className="inline-flex items-center gap-2 text-sm text-muted-foreground"><ArrowLeftIcon className="size-4" />Google Cloud</Link>{loading ? <Skeleton className="h-60" /> : <p role="alert" className="rounded-xl border p-6 text-sm text-muted-foreground">{connectionError || "This connection is unavailable in the selected workspace. Choose its workspace or return to Google Cloud."}</p>}</div>
  const scanning = connection.status === "syncing" || connection.status === "connecting"
  const coverage = connection.coverage ?? data?.coverage
  const incomplete = coverage && coverage.skipped + coverage.errors > 0
  return <div className="flex flex-col gap-6 p-4 md:p-6">
    <Link href="/cloud/gcp" className="inline-flex w-fit items-center gap-2 text-xs text-muted-foreground hover:text-primary"><ArrowLeftIcon className="size-3.5" />Google Cloud</Link>
    <div className="flex flex-wrap items-start justify-between gap-4"><div className="space-y-2"><div className="flex flex-wrap items-center gap-2"><Badge variant="outline">CLOUD</Badge><ConnectionStatus connection={connection} /></div><h1 className="flex items-center gap-2 text-2xl font-semibold"><GCPIcon className="size-6" />{connection.name}</h1><p className="font-mono text-xs text-muted-foreground">{connection.projectId}</p></div>
      {canManage(connection.workspaceId) && <div className="flex gap-2"><Button variant="outline" onClick={() => setSettings(true)} disabled={queuing}><SettingsIcon />Configure</Button><Button onClick={() => void scan()} disabled={queuing || scanning}>{queuing || scanning ? <LoaderCircleIcon className="animate-spin" /> : <RefreshCwIcon />}{queuing ? "Queuing…" : scanning ? "Scan in progress" : "Run scan"}</Button></div>}
    </div>
    {(error || connection.lastError) && <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"><p className="text-destructive">{error || connection.lastError}</p>{connection.lastScanAt && <p className="mt-1 text-xs text-muted-foreground">Previous results from {formatDate(connection.lastScanAt)} are preserved.</p>}<Button variant="ghost" size="sm" className="mt-2" onClick={() => { setError(""); setVersion(v => v + 1); void refresh() }}>Retry loading</Button></div>}
    {scanning && <p role="status" className="flex items-center gap-2 rounded-xl border bg-primary/5 p-4 text-sm"><LoaderCircleIcon className="size-4 animate-spin text-primary" />The scan runs in the background. Results update automatically.</p>}
    {coverage && <div className={`flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4 ${incomplete ? "border-amber-500/30 bg-amber-500/5" : "bg-muted/20"}`}><div><p className="text-sm font-medium">{incomplete ? "Partial scan coverage" : "Scan coverage"}</p><p className="mt-1 text-xs text-muted-foreground">{coverage.passed + coverage.failed} evaluated · {coverage.skipped} skipped · {coverage.errors} errors · {coverage.notApplicable} not applicable</p>{incomplete && <p className="mt-2 text-xs text-muted-foreground">Missing permissions or unavailable APIs require review. An empty findings list does not establish that the project is secure.</p>}</div><Button variant="outline" size="sm" onClick={() => setTab("checks")}>Review checks<ArrowRightIcon /></Button></div>}
    <CloudSummary summary={connection.summary} selected={severity} onSelect={value => { setSeverity(value); setPage(1); setTab("findings") }} />
    <div role="tablist" aria-label="Cloud results" className="flex gap-1 border-b">{(["findings", "checks"] as const).map(value => <button key={value} role="tab" aria-selected={tab === value} aria-controls={`cloud-${value}-panel`} id={`cloud-${value}-tab`} className={`border-b-2 px-4 py-3 text-sm font-medium ${tab === value ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"}`} onClick={() => setTab(value)}>{value === "findings" ? "Findings" : "Checks"}</button>)}</div>
    {tab === "checks" ? <div role="tabpanel" id="cloud-checks-panel" aria-labelledby="cloud-checks-tab"><CloudChecksPanel connection={connection} version={version} /></div> : <Card role="tabpanel" id="cloud-findings-panel" aria-labelledby="cloud-findings-tab"><CardHeader><div className="flex flex-wrap items-start justify-between gap-2"><div className="space-y-1"><CardTitle>Security findings</CardTitle><CardDescription>Evidence and remediation from Runtz configuration checks.</CardDescription></div><p className="text-xs text-muted-foreground">{connection.lastScanAt ? `Last scan ${formatDate(connection.lastScanAt)}` : "Waiting for the first scan"}</p></div></CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-3"><div className="relative min-w-48 flex-1"><SearchIcon className="absolute top-2.5 left-3 size-4 text-muted-foreground" /><Input aria-label="Search findings" maxLength={200} placeholder="Search findings, rules or resources" className="pl-9" value={search} onChange={event => setSearch(event.target.value)} /></div><select aria-label="Filter severity" value={severity} onChange={event => { setSeverity(event.target.value); setPage(1) }} className="h-9 rounded-lg border bg-background px-3 text-sm"><option value="all">All severities</option>{cloudSeverities.map(s => <option key={s} value={s}>{s === "unknown" ? "Unspecified" : s[0].toUpperCase() + s.slice(1)}</option>)}</select></div>
        {fetching ? <div role="status" className="space-y-3"><span className="sr-only">Loading findings</span>{[1, 2, 3].map(i => <Skeleton key={i} className="h-12" />)}</div> : data && data.findings.length > 0 ? <>
          <Table><TableHeader><TableRow><TableHead>Finding</TableHead><TableHead>Severity</TableHead><TableHead>Resource</TableHead><TableHead>Rule</TableHead></TableRow></TableHeader><TableBody>{data.findings.map(f => <TableRow key={f.id}><TableCell className="max-w-sm whitespace-normal"><button onClick={() => setFinding(f)} className="text-left font-medium underline-offset-4 hover:text-primary hover:underline focus-visible:outline-2 focus-visible:outline-primary">{f.title}</button><p className="mt-1 text-xs text-muted-foreground">{f.category}</p></TableCell><TableCell><SeverityBadge severity={f.severity} /></TableCell><TableCell className="max-w-80"><p className="truncate font-mono text-xs" title={f.resourceName}>{f.resourceName || "Project resource"}</p></TableCell><TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">{f.ruleId || "Imported finding"}</TableCell></TableRow>)}</TableBody></Table>
          <CloudPagination page={page} total={data.total} pageSize={data.pageSize} onPage={setPage} />
        </> : data && <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center"><ShieldCheckIcon className="size-8 text-primary" /><h3 className="font-medium">{data.resultAvailable === false ? "Waiting for scan results" : query || severity !== "all" ? "No findings match these filters" : "No findings from evaluated checks"}</h3><p className="max-w-md text-sm text-muted-foreground">{data.resultAvailable === false ? "The first scan has not published a result yet. Follow its status above." : query || severity !== "all" ? "Adjust the search or filters to see more findings." : "Review the Checks tab to confirm which resources were evaluated and whether any checks were skipped."}</p></div>}
      </CardContent></Card>}
    <p className="text-xs text-muted-foreground">Read-only scans run automatically every 12 hours. Findings reflect configuration checks, not a guarantee of network reachability or complete security coverage.</p>
    {finding && <Dialog open onOpenChange={open => { if (!open) setFinding(null) }}><DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl"><DialogHeader><DialogTitle className="pr-6 leading-snug">{finding.title}</DialogTitle><DialogDescription>{finding.ruleId || finding.category}</DialogDescription></DialogHeader><SeverityBadge severity={finding.severity} /><p className="whitespace-pre-wrap text-sm leading-relaxed">{finding.description || "Review the observed project configuration and remediation below."}</p>{finding.evidence && <div className="rounded-lg border bg-muted/30 p-4"><h4 className="mb-2 text-sm font-medium">Evidence</h4><p className="whitespace-pre-wrap break-words text-sm text-muted-foreground">{finding.evidence}</p></div>}<dl className="space-y-3 rounded-lg border bg-muted/20 p-4 text-sm">{[["Resource", finding.resourceName], ["Project", finding.projectName || connection.projectId], ["Source", finding.source || "Runtz configuration checks"], ["Observed", finding.eventTime ? formatDate(finding.eventTime) : "Not provided"]].map(([label, value]) => <div key={label}><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl>{finding.remediation && <div><h4 className="mb-2 font-medium">Remediation</h4><p className="whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">{finding.remediation}</p></div>}<Button variant="outline" render={<a href={safeCloudLink(finding.externalUri) ?? `https://console.cloud.google.com/home/dashboard?project=${encodeURIComponent(connection.projectId)}`} target="_blank" rel="noopener noreferrer" />}><ExternalLinkIcon />View guidance</Button></DialogContent></Dialog>}
    {settings && canManage(connection.workspaceId) && <CloudConnectionSettings connection={connection} onClose={() => setSettings(false)} />}
  </div>
}

function CloudPagination({ page, total, pageSize, onPage }: { page: number; total: number; pageSize: number; onPage: (page: number) => void }) {
  return <div className="flex flex-wrap items-center justify-between gap-3 border-t pt-4 text-xs text-muted-foreground"><span>{total === 0 ? "0 results" : `${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, total)} of ${total}`}</span><div className="flex items-center gap-2"><Button variant="outline" size="icon-sm" aria-label="Previous page" disabled={page <= 1} onClick={() => onPage(page - 1)}><ChevronLeftIcon /></Button><span>Page {page}</span><Button variant="outline" size="icon-sm" aria-label="Next page" disabled={page * pageSize >= total} onClick={() => onPage(page + 1)}><ChevronRightIcon /></Button></div></div>
}

function CloudChecksPanel({ connection, version }: { connection: CloudConnection; version: number }) {
  const [status, setStatus] = React.useState("all")
  const [page, setPage] = React.useState(1)
  const [data, setData] = React.useState<CloudCheckResponse | null>(null)
  const [error, setError] = React.useState("")
  const [loading, setLoading] = React.useState(true)
  const labels: Record<string, string> = { pass: "Passed", fail: "Finding", skipped: "Skipped", error: "Error", not_applicable: "Not applicable" }
  React.useEffect(() => {
    const abort = new AbortController()
    setLoading(true); setError("")
    apiRequest<CloudCheckResponse>(`/api/v1/cloud/connections/${connection.id}/checks?${new URLSearchParams({ status, page: String(page) })}`, { signal: abort.signal })
      .then(result => { if (!abort.signal.aborted) setData(result) })
      .catch(err => { if (!abort.signal.aborted) setError(cloudError(err)) })
      .finally(() => { if (!abort.signal.aborted) setLoading(false) })
    return () => abort.abort()
  }, [connection.id, connection.updatedAt, connection.lastScanAt, connection.status, status, page, version])
  return <Card><CardHeader><CardTitle>Check coverage</CardTitle><CardDescription>See evaluated resources, skipped permissions and the evidence behind each result.</CardDescription></CardHeader><CardContent className="space-y-4"><select aria-label="Filter check status" className="h-9 rounded-lg border bg-background px-3 text-sm" value={status} onChange={event => { setStatus(event.target.value); setPage(1) }}><option value="all">All statuses</option>{Object.entries(labels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select>{error && <p role="alert" className="text-sm text-destructive">{error}</p>}{loading ? <Skeleton className="h-40" /> : data && data.checks.length > 0 ? <><Table><TableHeader><TableRow><TableHead>Check</TableHead><TableHead>Status</TableHead><TableHead>Resource / evidence</TableHead></TableRow></TableHeader><TableBody>{data.checks.map((check, index) => <TableRow key={`${check.ruleId}:${check.resource}:${index}`}><TableCell className="max-w-72 whitespace-normal"><p className="font-medium">{check.title}</p><p className="mt-1 font-mono text-xs text-muted-foreground">{check.ruleId}</p></TableCell><TableCell><Badge variant="outline" className={check.status === "skipped" || check.status === "error" ? "text-amber-500" : check.status === "fail" ? "text-destructive" : "text-muted-foreground"}>{labels[check.status] ?? check.status}</Badge></TableCell><TableCell className="max-w-xl whitespace-normal"><p className="break-all font-mono text-xs">{check.resource}</p><p className="mt-2 text-xs leading-relaxed text-muted-foreground">{check.evidence}</p></TableCell></TableRow>)}</TableBody></Table><CloudPagination page={page} total={data.total} pageSize={data.pageSize} onPage={setPage} /></> : data && <p className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">{data.resultAvailable ? "No checks match this status." : "The first scan has not published check results yet."}</p>}</CardContent></Card>
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
      setNotice("Credentials replaced and verified. Revoke the previous key in GCP, then run a scan to refresh your results.")
      await refresh()
    } catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}><DialogContent showCloseButton={!busy} className="max-h-[85dvh] overflow-y-auto sm:max-w-lg"><DialogHeader><DialogTitle>Configure {connection.name}</DialogTitle><DialogDescription>Manage access to {connection.projectId}.</DialogDescription></DialogHeader>
    <div className="space-y-3 rounded-lg border p-4"><h3 className="flex items-center gap-2 font-medium"><KeyRoundIcon className="size-4" />Service account credentials</h3><p className="break-all text-xs text-muted-foreground">{connection.serviceAccount}</p><p className="text-sm text-muted-foreground">Upload a new JSON key to rotate credentials. Runtz verifies access before replacing the encrypted key.</p><label className={`inline-flex h-9 cursor-pointer items-center gap-2 rounded-lg border px-3 text-sm ${busy ? "pointer-events-none opacity-50" : "hover:bg-accent"}`}>{busy ? <LoaderCircleIcon className="size-4 animate-spin" /> : <UploadIcon className="size-4" />}Replace JSON key<input aria-label="Replace service account JSON key" className="sr-only" type="file" accept=".json,application/json" disabled={busy} onChange={event => { void replace(event.target.files?.[0]); event.target.value = "" }} /></label></div>
    <div className="space-y-3 rounded-lg border border-destructive/25 p-4"><h3 className="flex items-center gap-2 font-medium"><UnplugIcon className="size-4" />Disconnect cloud</h3><p className="text-sm text-muted-foreground">Removes this connection, its saved credentials and scan results from Runtz. Revoke the service account key in GCP separately.</p><Input aria-label="Type project ID to confirm disconnection" placeholder={`Type ${connection.projectId} to confirm`} value={confirm} onChange={event => setConfirm(event.target.value)} disabled={busy} /><Button variant="destructive" disabled={busy || confirm !== connection.projectId} onClick={async () => {
      setBusy(true); setError("")
      try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh(); onClose(); router.push(connections.length > 1 ? "/cloud/gcp" : "/overview") }
      catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
    }}>Disconnect GCP</Button></div>
    {notice && <p role="status" className="text-sm text-primary">{notice}</p>}{error && <p role="alert" className="text-sm text-destructive">{error}</p>}
  </DialogContent></Dialog>
}
