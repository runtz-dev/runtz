"use client"

import * as React from "react"
import Link from "next/link"
import { ExternalLinkIcon, KeyRoundIcon, LoaderCircleIcon, UnplugIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { AzureCredentialFields, validAzureCredentials, type AzureCredentials } from "@/components/runtz/azure-credentials"
import { SettingsFeedback } from "@/components/runtz/settings-ui"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { apiRequest } from "@/lib/api"
import { cloudConnectionPath, cloudConsoleLink, cloudError, cloudScope, type CloudConnection } from "@/lib/cloud"
import { formatDate } from "@/lib/sca"

export function AzureConnectionControls({ connection, canManage }: { connection: CloudConnection; canManage: boolean }) {
  const { refresh } = useCloud()
  const [credentials, setCredentials] = React.useState<AzureCredentials>({ tenantId: connection.directoryTenantId ?? "", clientId: connection.clientId ?? "", clientSecret: "" })
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [notice, setNotice] = React.useState("")
  const [confirm, setConfirm] = React.useState("")
  async function replace() {
    setBusy(true); setError(""); setNotice("")
    try {
      await apiRequest(`/api/v1/cloud/connections/${connection.id}/credentials`, { method: "PATCH", body: { credentials } })
      setCredentials(value => ({ ...value, clientSecret: "" })); setNotice("Secret replaced and verified. Revoke the previous secret in Microsoft Entra ID, then run a scan.")
      await refresh()
    } catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  async function disconnect() {
    setBusy(true); setError(""); setNotice("")
    try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh() }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  return <div className="min-w-0 space-y-5">
    <dl className="grid gap-4 text-sm sm:grid-cols-2">{[["Directory tenant", connection.directoryTenantId], ["Application ID", connection.clientId], ["Last scan", connection.lastScanAt ? formatDate(connection.lastScanAt) : "Waiting for scan"], ["Scan schedule", "Every 12 hours"]].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl>
    <div className="flex flex-wrap gap-2"><Button variant="outline" size="sm" render={<Link href={cloudConnectionPath(connection)} />}>View findings</Button><Button variant="outline" size="sm" render={<a href={cloudConsoleLink(connection)} target="_blank" rel="noopener noreferrer" />}><ExternalLinkIcon />Open Azure subscription</Button></div>
    {canManage && <>
      <section className="space-y-4 rounded-lg border bg-card p-4"><h3 className="flex items-center gap-2 font-medium"><KeyRoundIcon className="size-4" />Application credentials</h3><p className="text-sm text-muted-foreground">Replace the client secret for this application. Runtz verifies subscription access before saving it. The directory and application identity stay the same.</p><fieldset disabled={busy}><AzureCredentialFields value={credentials} onChange={setCredentials} lockIdentity /></fieldset><Button variant="outline" disabled={busy || !validAzureCredentials(credentials)} onClick={() => void replace()}>{busy && <LoaderCircleIcon className="animate-spin" />}Replace client secret</Button></section>
      <section className="space-y-3 rounded-lg border border-destructive/25 p-4"><h3 className="flex items-center gap-2 font-medium"><UnplugIcon className="size-4" />Disconnect Azure</h3><p className="text-sm text-muted-foreground">Remove this connection, its saved credentials and scan results from Runtz. The Azure application, role assignments and subscription resources stay in Azure.</p><label htmlFor={`disconnect-${connection.id}`} className="block text-xs text-muted-foreground">Type {cloudScope(connection)} to confirm</label><Input id={`disconnect-${connection.id}`} autoComplete="off" value={confirm} onChange={event => setConfirm(event.target.value)} disabled={busy} /><Button variant="destructive" disabled={busy || confirm !== cloudScope(connection)} onClick={() => void disconnect()}>{busy && <LoaderCircleIcon className="animate-spin" />}Disconnect Azure</Button></section>
    </>}
    {notice && <SettingsFeedback>{notice}</SettingsFeedback>}{error && <SettingsFeedback error>{error}</SettingsFeedback>}
  </div>
}
