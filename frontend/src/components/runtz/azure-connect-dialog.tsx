"use client"

import * as React from "react"
import { ArrowLeftIcon, ArrowRightIcon, CheckCircle2Icon, ExternalLinkIcon, LoaderCircleIcon, LockKeyholeIcon } from "lucide-react"
import { AzureIcon } from "@/components/runtz/cloud-icons"
import { AzureCredentialFields, azureIDPattern, validAzureCredentials, type AzureCredentials } from "@/components/runtz/azure-credentials"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { apiRequest } from "@/lib/api"
import { azureSetupDocumentation, cloudError, type CloudConnection } from "@/lib/cloud"

const steps = ["Subscription", "Configure access", "Connect"]

export function AzureConnectDialog({ onClose, onBack, onCreated, onConnected }: { onClose: () => void; onBack: () => void; onCreated: () => Promise<void>; onConnected: (connection: CloudConnection) => Promise<void> }) {
  const { selectedWorkspaceId, workspaces, currentUser, deploymentMode } = useWorkspace()
  const manageable = workspaces.filter(w => deploymentMode === "self-hosted" ? currentUser.role === "admin" : w.createdBy === currentUser.id)
  const [workspaceId, setWorkspaceId] = React.useState(manageable.some(w => w.id === selectedWorkspaceId) ? selectedWorkspaceId : manageable[0]?.id ?? "")
  const [subscriptionId, setSubscriptionId] = React.useState("")
  const [credentials, setCredentials] = React.useState<AzureCredentials>({ tenantId: "", clientId: "", clientSecret: "" })
  const [name, setName] = React.useState("")
  const [step, setStep] = React.useState(1)
  const [validated, setValidated] = React.useState(false)
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [connection, setConnection] = React.useState<CloudConnection | null>(null)
  const headingRef = React.useRef<HTMLHeadingElement>(null)
  React.useEffect(() => { headingRef.current?.focus() }, [step])
  function go(next: number) { setStep(next); setError(""); if (next < 3) setValidated(false) }
  async function validate() {
    setBusy(true); setError("")
    try { await apiRequest("/api/v1/cloud/azure/validate", { method: "POST", body: { workspaceId, subscriptionId, credentials } }); setValidated(true); setStep(3) }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  async function connect() {
    setBusy(true); setError("")
    try {
      const response = await apiRequest<{ connection: CloudConnection }>("/api/v1/cloud/azure/connections", { method: "POST", body: { workspaceId, subscriptionId, credentials, name: name.trim() } })
      setCredentials({ tenantId: "", clientId: "", clientSecret: "" }); setConnection(response.connection); setStep(4)
      await onCreated()
    } catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}><DialogContent showCloseButton={!busy} className="max-h-[90dvh] min-w-0 gap-0 overflow-y-auto p-0 sm:max-w-3xl">
    <DialogHeader className="border-b px-6 py-5 pr-12 sm:px-8"><p className="mb-3 flex items-center gap-2 text-xs text-muted-foreground"><AzureIcon className="size-4" />CLOUD INTEGRATIONS / MICROSOFT AZURE</p><DialogTitle>{connection ? "Your cloud is connected" : "Connect Microsoft Azure"}</DialogTitle><DialogDescription>Read-only subscription configuration checks with findings grouped by severity.</DialogDescription></DialogHeader>
    {!connection && <ol aria-label="Connection progress" className="flex justify-between gap-3 border-b bg-muted/20 px-6 py-4 text-xs sm:px-8">{steps.map((label, index) => <li key={label} aria-current={step === index + 1 ? "step" : undefined} className={step === index + 1 ? "font-medium text-primary" : "text-muted-foreground"}>{index + 1}. {label}</li>)}</ol>}
    <div className="space-y-5 px-6 py-6 sm:px-8">
      {step === 1 && <><h3 ref={headingRef} tabIndex={-1} className="font-medium outline-none">Select your workspace and Azure subscription</h3><div className="space-y-2"><Label htmlFor="azure-workspace">Workspace</Label><select id="azure-workspace" value={workspaceId} onChange={event => setWorkspaceId(event.target.value)} className="h-10 w-full rounded-lg border bg-background px-3 text-sm">{manageable.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}</select></div><div className="space-y-2"><Label htmlFor="azure-subscription">Subscription ID</Label><Input id="azure-subscription" autoComplete="off" spellCheck={false} maxLength={36} placeholder="00000000-0000-0000-0000-000000000000" value={subscriptionId} onChange={event => setSubscriptionId(event.target.value.trim())} /><p className="text-xs text-muted-foreground">Find this ID in Azure Portal → Subscriptions. Connect each subscription separately.</p></div></>}
      {step === 2 && <><h3 ref={headingRef} tabIndex={-1} className="font-medium outline-none">Configure application access</h3><div className="space-y-3 rounded-xl border bg-muted/20 p-4 text-sm leading-relaxed"><ol className="list-decimal space-y-2 pl-5"><li>In Microsoft Entra ID → App registrations, select or register a dedicated application.</li><li>Have an authorized administrator grant its service principal the <strong>Reader</strong> role on the target subscription.</li><li>In Certificates &amp; secrets, create a client secret and copy its <strong>value</strong>.</li></ol><p className="text-xs text-muted-foreground">Runtz reads resource configuration. It does not assign roles, register resource providers or require Defender for Cloud. Missing read permissions appear as incomplete scan coverage.</p><a href={azureSetupDocumentation} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-primary hover:underline">Microsoft application setup guide<ExternalLinkIcon className="size-3.5" /></a></div><fieldset disabled={busy}><AzureCredentialFields value={credentials} onChange={value => { setCredentials(value); setValidated(false) }} /></fieldset><p className="flex items-start gap-2 text-xs text-muted-foreground"><LockKeyholeIcon className="size-4 shrink-0" />Your application secret is encrypted by the Security service. It is never included in scan results.</p></>}
      {step === 3 && <><h3 ref={headingRef} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-5 text-primary" />Subscription access verified</h3><p className="break-all font-mono text-xs text-muted-foreground">{subscriptionId}</p><p className="text-sm text-muted-foreground">Validation confirms subscription access. Review the first scan for missing permissions and incomplete coverage.</p><div className="space-y-2"><Label htmlFor="azure-name">Connection name</Label><Input id="azure-name" value={name} disabled={busy} maxLength={80} placeholder="Production Azure" onChange={event => setName(event.target.value)} /></div></>}
      {connection && <><h3 ref={headingRef} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-6 text-primary" />Your first scan is running</h3><p className="text-sm text-muted-foreground">Runtz runs 14 configuration checks and refreshes results automatically. Scheduled scans repeat every 12 hours.</p><p className="break-all text-sm">{connection.name} · {connection.subscriptionId}</p></>}
      {error && <p role="alert" className="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{error}</p>}
    </div>
    <div className="flex justify-between gap-3 border-t px-6 py-4 sm:px-8">{connection ? <><Button variant="ghost" onClick={onClose}>Close</Button><Button onClick={() => void onConnected(connection)}>View Azure findings<ArrowRightIcon /></Button></> : <><Button variant="ghost" disabled={busy} onClick={() => step === 1 ? onBack() : go(step - 1)}><ArrowLeftIcon />Back</Button>{step === 1 ? <Button disabled={!workspaceId || !azureIDPattern.test(subscriptionId)} onClick={() => go(2)}>Continue<ArrowRightIcon /></Button> : step === 2 ? <Button disabled={busy || !validAzureCredentials(credentials)} onClick={() => void validate()}>{busy && <LoaderCircleIcon className="animate-spin" />}Verify access</Button> : <Button disabled={busy || !validated || !name.trim()} onClick={() => void connect()}>{busy && <LoaderCircleIcon className="animate-spin" />}Connect and start scan</Button>}</>}</div>
  </DialogContent></Dialog>
}
