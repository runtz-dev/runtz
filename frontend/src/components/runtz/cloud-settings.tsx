"use client"

import * as React from "react"
import Link from "next/link"
import { ChevronDownIcon, CloudIcon, ExternalLinkIcon, KeyRoundIcon, LoaderCircleIcon, PlusIcon, Trash2Icon, UnplugIcon, UploadIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { CloudCommand } from "@/components/runtz/cloud-command"
import { GCPIcon } from "@/components/runtz/cloud-icons"
import { SettingsEmpty, SettingsFeedback, SettingsSearch, SettingsSection } from "@/components/runtz/settings-ui"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { apiRequest } from "@/lib/api"
import { cloudError, gcpSetupDocumentation, type CloudConnection } from "@/lib/cloud"
import { formatDate } from "@/lib/sca"

export function CloudSettingsPanel() {
  const { connections, canManage, loading, error, refresh, openConnect } = useCloud()
  const { workspaces } = useWorkspace()
  const [search, setSearch] = React.useState("")
  const [selected, setSelected] = React.useState<string | null>(null)
  React.useEffect(() => { setSelected(new URLSearchParams(window.location.search).get("connection")) }, [])
  const visible = connections.filter(c => `${c.name} ${c.projectId} ${c.serviceAccount}`.toLowerCase().includes(search.toLowerCase()))
  return <SettingsSection title="Cloud connections" description="Manage your connected projects, service accounts and scan access." action={workspaces.some(w => canManage(w.id)) && <Button onClick={openConnect}><PlusIcon />Add cloud</Button>}>
    <div className="space-y-4">
      {error && <SettingsFeedback error>{error}<Button variant="link" size="sm" onClick={() => void refresh()}>Retry</Button></SettingsFeedback>}
      <SettingsSearch value={search} onChange={setSearch} label="Search cloud connections" />
      {loading && !connections.length ? <Skeleton className="h-36" /> : visible.length ? visible.map(connection => <div key={connection.id} className="min-w-0 overflow-hidden rounded-xl border">
        <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center">
          <div className="flex min-w-0 flex-1 items-center gap-3"><GCPIcon className="size-5 text-muted-foreground" />
          <div className="min-w-0 flex-1"><p className="break-words font-medium">{connection.name}</p><p className="mt-1 break-all text-xs text-muted-foreground">{connection.projectId} · {workspaces.find(w => w.id === connection.workspaceId)?.name ?? "Workspace"}</p></div></div>
          <div className="flex items-center justify-between gap-3"><Badge variant="outline" className="text-muted-foreground">{connection.status.replaceAll("_", " ")}</Badge>
          <Button variant="ghost" size="sm" aria-expanded={selected === connection.id} aria-controls={`cloud-settings-${connection.id}`} onClick={() => setSelected(selected === connection.id ? null : connection.id)}>{canManage(connection.workspaceId) ? "Manage" : "Details"}<ChevronDownIcon className={selected === connection.id ? "rotate-180" : ""} /></Button></div>
        </div>
        {selected === connection.id && <div id={`cloud-settings-${connection.id}`} className="min-w-0 border-t bg-muted/10 p-4"><CloudConnectionControls connection={connection} canManage={canManage(connection.workspaceId)} /></div>}
      </div>) : <SettingsEmpty icon={CloudIcon} title={search ? "No matching connections" : "No cloud connections"} description={search ? "Try a project name or service account." : "Connect a cloud project to start scanning your resources."} />}
      <Button variant="link" className="h-auto px-0 text-xs" render={<a href={gcpSetupDocumentation} target="_blank" rel="noopener noreferrer" />}>Google Cloud setup and required permissions<ExternalLinkIcon /></Button>
    </div>
  </SettingsSection>
}

function CloudConnectionControls({ connection, canManage }: { connection: CloudConnection; canManage: boolean }) {
  const { refresh, connections } = useCloud()
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [notice, setNotice] = React.useState("")
  const [confirm, setConfirm] = React.useState("")
  const accountProject = connection.serviceAccount.split("@")[1]?.replace(/\.iam\.gserviceaccount\.com$/, "")
  const sameAccount = connections.filter(c => c.id !== connection.id && c.serviceAccount === connection.serviceAccount)
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
  return <div className="min-w-0 space-y-5">
    <dl className="grid gap-4 text-sm sm:grid-cols-2">{[["Service account", connection.serviceAccount], ["Last scan", connection.lastScanAt ? formatDate(connection.lastScanAt) : "Waiting for scan"], ["Scan schedule", "Every 12 hours"], ["Access", "Read-only"]].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl>
    <Button variant="outline" size="sm" render={<Link href={`/cloud/gcp/${connection.id}`} />}>View findings</Button>
    {canManage && <>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-4"><h3 className="flex items-center gap-2 font-medium"><KeyRoundIcon className="size-4" />Service account credentials</h3><p className="text-sm text-muted-foreground">Upload a new JSON key to replace the integration credentials. Runtz verifies project access before saving the new key.</p><Button variant="outline" disabled={busy} onClick={() => document.getElementById(`replace-key-${connection.id}`)?.click()}>{busy ? <LoaderCircleIcon className="animate-spin" /> : <UploadIcon />}Replace JSON key</Button><input id={`replace-key-${connection.id}`} aria-label="Replace service account JSON key" className="sr-only" type="file" accept=".json,application/json" disabled={busy} onChange={event => { void replace(event.target.files?.[0]); event.target.value = "" }} /></section>
      <section className="min-w-0 space-y-3 rounded-lg border bg-card p-4"><h3 className="flex items-center gap-2 font-medium"><Trash2Icon className="size-4" />Delete service account in Google Cloud</h3><p className="text-sm leading-relaxed text-muted-foreground">Open the Console, select this service account and choose Delete, or use the command below with your administrator account. Deleting it stops scans for every integration using it. Disconnect the connection in Runtz separately.</p>
        {sameAccount.length > 0 && <p className="text-xs text-amber-500">Also used by {sameAccount.map(c => c.name).join(", ")}.</p>}
        <Button variant="outline" size="sm" render={<a href={`https://console.cloud.google.com/iam-admin/serviceaccounts?project=${encodeURIComponent(accountProject || connection.projectId)}`} target="_blank" rel="noopener noreferrer" />}><ExternalLinkIcon />Manage service account</Button>
        {connection.serviceAccount && <CloudCommand label="Show deletion command" value={`gcloud iam service-accounts delete '${connection.serviceAccount.replaceAll("'", "'\\''")}'`} />}
      </section>
      <section className="space-y-3 rounded-lg border border-destructive/25 p-4"><h3 className="flex items-center gap-2 font-medium"><UnplugIcon className="size-4" />Disconnect cloud</h3><p className="text-sm leading-relaxed text-muted-foreground">Remove this connection, its saved credentials and scan results from Runtz. This does not delete the service account in Google Cloud.</p><label htmlFor={`disconnect-${connection.id}`} className="block text-xs text-muted-foreground">Type {connection.projectId} to confirm</label><Input id={`disconnect-${connection.id}`} autoComplete="off" value={confirm} onChange={event => setConfirm(event.target.value)} disabled={busy} /><Button variant="destructive" disabled={busy || confirm !== connection.projectId} onClick={async () => {
        setBusy(true); setError(""); setNotice("")
        try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh() }
        catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
      }}>{busy && <LoaderCircleIcon className="animate-spin" />}Disconnect GCP</Button></section>
    </>}
    {notice && <SettingsFeedback>{notice}</SettingsFeedback>}{error && <SettingsFeedback error>{error}</SettingsFeedback>}
  </div>
}
