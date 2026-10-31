"use client"

import * as React from "react"
import { ArrowLeftIcon, ArrowRightIcon, CheckIcon, CheckCircle2Icon, CloudIcon, CopyIcon, ExternalLinkIcon, FileJsonIcon, LoaderCircleIcon, LockKeyholeIcon, UploadIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { GCPIcon } from "@/components/runtz/cloud-icons"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest } from "@/lib/api"
import { cloudError, type CloudConnection } from "@/lib/cloud"

const steps = ["Project", "Enable services", "Configure access", "Upload key", "Connect"]
const projectPattern = /^[a-z][a-z0-9-]{4,61}[a-z0-9]$/

function ConsoleLink({ href, children }: { href: string; children: React.ReactNode }) {
  return <a href={href} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 font-medium text-primary underline-offset-4 hover:underline">{children}<ExternalLinkIcon className="size-3.5" /></a>
}
function Command({ value }: { value: string }) {
  const [copied, setCopied] = React.useState(false)
  const [failed, setFailed] = React.useState(false)
  return <div className="relative rounded-lg border bg-background p-3 pr-12">
    <pre className="overflow-x-auto text-xs leading-relaxed"><code>{value}</code></pre>
    <Button size="icon-sm" variant="ghost" className="absolute top-2 right-2" aria-label={copied ? "Copied" : "Copy command"} onClick={async () => {
      try { await navigator.clipboard.writeText(value); setCopied(true); setFailed(false) } catch { setFailed(true) }
    }}>{copied ? <CheckIcon /> : <CopyIcon />}</Button>
    {failed && <span className="mt-2 block text-xs text-muted-foreground">Select and copy the command manually.</span>}
  </div>
}

export function CloudConnectDialog({ onClose, onCreated, onConnected }: { onClose: () => void; onCreated: () => Promise<void>; onConnected: (connection: CloudConnection) => Promise<void> }) {
  const { selectedWorkspaceId, workspaces, currentUser, deploymentMode } = useWorkspace()
  const manageable = workspaces.filter(w => deploymentMode === "self-hosted" ? currentUser.role === "admin" : w.createdBy === currentUser.id)
  const [step, setStep] = React.useState(0)
  const [workspaceId, setWorkspaceId] = React.useState(manageable.some(w => w.id === selectedWorkspaceId) ? selectedWorkspaceId : manageable[0]?.id ?? "")
  const [projectId, setProjectId] = React.useState("")
  const [name, setName] = React.useState("")
  const [fileName, setFileName] = React.useState("")
  const [credentials, setCredentials] = React.useState<Record<string, unknown> | null>(null)
  const [validated, setValidated] = React.useState(false)
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [connection, setConnection] = React.useState<CloudConnection | null>(null)
  const uploadRef = React.useRef<HTMLInputElement>(null)
  const headingRef = React.useRef<HTMLHeadingElement>(null)
  const project = encodeURIComponent(projectId)
  const email = `runtz-security@${projectId}.iam.gserviceaccount.com`
  const payload = { workspaceId, projectId, name: name.trim() || projectId, credentials }

  React.useEffect(() => { headingRef.current?.focus() }, [step])

  function go(next: number) { setError(""); setStep(next) }
  async function upload(file?: File) {
    setValidated(false); setCredentials(null); setFileName(""); setError("")
    if (!file) return
    if (file.size > 32768) { setError("Choose a service account JSON key smaller than 32 KB."); return }
    try {
      const parsed = JSON.parse(await file.text())
      if (parsed.type !== "service_account" || typeof parsed.private_key !== "string" || typeof parsed.client_email !== "string" || typeof parsed.project_id !== "string") throw new Error()
      setCredentials(parsed); setFileName(file.name)
    } catch { setError("This file is not a valid GCP service account JSON key.") }
  }
  async function validate() {
    if (!credentials) return
    setBusy(true); setError("")
    try {
      await apiRequest("/api/v1/cloud/gcp/validate", { method: "POST", body: payload })
      setValidated(true); setName(current => current || projectId); go(5)
    } catch (err) { setError(cloudError(err)) }
    finally { setBusy(false) }
  }
  async function connect() {
    setBusy(true); setError("")
    try {
      const response = await apiRequest<{ connection: CloudConnection }>("/api/v1/cloud/gcp/connections", { method: "POST", body: payload })
      setCredentials(null); setConnection(response.connection); go(6)
      await onCreated()
    } catch (err) { setError(cloudError(err)) }
    finally { setBusy(false) }
  }

  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}>
    <DialogContent showCloseButton={!busy} className="max-h-[90dvh] gap-0 overflow-y-auto p-0 sm:max-w-3xl">
      <DialogHeader className="border-b px-6 py-5 pr-12 sm:px-8">
        <div className="mb-3 flex items-center gap-2 text-xs font-medium text-muted-foreground"><CloudIcon className="size-4" /> CLOUD INTEGRATIONS {step > 0 && <><span className="text-border">/</span><GCPIcon className="size-4" /><span>Google Cloud</span></>}</div>
        <DialogTitle className="text-xl">{step === 6 ? "Your cloud is connected" : step === 0 ? "Connect your cloud" : "Connect Google Cloud"}</DialogTitle>
        <DialogDescription>{step === 0 ? "Bring your cloud security findings into Runtz. One place to understand and prioritize risk." : "Read-only GCP configuration checks with findings grouped by severity."}</DialogDescription>
      </DialogHeader>
      {step > 0 && step < 6 && <div className="border-b bg-muted/30 px-6 py-4 sm:px-8">
        <ol aria-label="Connection progress" className="flex justify-between gap-2">
          {steps.map((label, index) => <li key={label} aria-current={step === index + 1 ? "step" : undefined} className={`flex flex-1 flex-col items-center gap-1.5 text-center text-[11px] sm:flex-row sm:text-xs ${step >= index + 1 ? "text-foreground" : "text-muted-foreground"}`}>
            <span className={`flex size-6 shrink-0 items-center justify-center rounded-full text-[11px] ${step > index + 1 ? "bg-primary/15 text-primary" : step === index + 1 ? "bg-primary text-primary-foreground" : "border bg-background"}`}>{step > index + 1 ? <CheckIcon className="size-3.5" /> : index + 1}</span><span className="hidden sm:inline">{label}</span><span className="sr-only sm:hidden">{label}</span>
          </li>)}
        </ol>
        <p className="mt-2 text-xs text-muted-foreground sm:hidden">Step {step} of 5 · {steps[step - 1]}</p>
      </div>}
      <div className="space-y-5 px-6 py-6 sm:px-8">
        {step === 0 && <>
          <button onClick={() => go(1)} className="group flex w-full items-center gap-4 rounded-xl border bg-card p-5 text-left transition-colors hover:border-primary/60 hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-primary">
            <div className="flex size-12 items-center justify-center rounded-xl border bg-background"><GCPIcon className="size-8" /></div>
            <div className="flex-1"><h3 className="font-medium">Google Cloud</h3><p className="mt-1 text-sm text-muted-foreground">32 configuration checks, organized by severity.</p></div><ArrowRightIcon className="size-5 text-muted-foreground group-hover:text-primary" />
          </button>
          <div className="grid gap-3 sm:grid-cols-2">{["Amazon Web Services", "Microsoft Azure"].map(provider => <div key={provider} aria-disabled="true" className="flex items-center gap-3 rounded-xl border bg-muted/20 p-4 text-muted-foreground"><CloudIcon className="size-5" /><span className="flex-1 text-sm">{provider}</span><Badge variant="secondary" className="text-[10px]">Coming soon</Badge></div>)}</div>
          <p className="flex items-center gap-2 text-xs text-muted-foreground"><LockKeyholeIcon className="size-3.5 shrink-0" />Read-only access. Credentials are encrypted before storage.</p>
        </>}
        {step === 1 && <>
          <h3 ref={headingRef} tabIndex={-1} className="text-base font-medium outline-none">Choose your GCP project</h3>
          <p className="text-sm text-muted-foreground">Connect one project at a time. You can add more GCP connections later, including projects accessed by the same service account.</p>
          <div className="grid gap-5 sm:grid-cols-2"><div className="space-y-2"><Label htmlFor="cloud-workspace">Runtz workspace</Label><select id="cloud-workspace" className="h-9 w-full rounded-lg border bg-background px-3 text-sm" value={workspaceId} onChange={event => { setWorkspaceId(event.target.value); setValidated(false) }}>{manageable.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}</select></div>
            <div className="space-y-2"><Label htmlFor="gcp-project">Project ID</Label><Input id="gcp-project" autoComplete="off" spellCheck={false} placeholder="my-production-project" value={projectId} onChange={event => { setProjectId(event.target.value.trim()); setValidated(false) }} /></div></div>
          <p className="text-xs text-muted-foreground">Copy the project ID from <ConsoleLink href="https://console.cloud.google.com/cloud-resource-manager">GCP project settings</ConsoleLink>. Use the ID, rather than its display name.</p>

        </>}
        {step === 2 && <>
          <h3 ref={headingRef} tabIndex={-1} className="text-base font-medium outline-none">Enable the resource APIs</h3>
          <p className="text-sm leading-relaxed text-muted-foreground">Runtz reads your project configuration directly. Enable the APIs for the resources you want to check from the Google Cloud Shell.</p>
          <Command value={`gcloud services enable cloudresourcemanager.googleapis.com compute.googleapis.com sqladmin.googleapis.com dns.googleapis.com container.googleapis.com storage.googleapis.com cloudkms.googleapis.com bigquery.googleapis.com logging.googleapis.com monitoring.googleapis.com --project=${projectId}`} />
          <p className="rounded-lg border bg-muted/30 p-3 text-xs leading-relaxed text-muted-foreground">Only enable the APIs you need. Unavailable APIs or missing read permissions appear as skipped checks, so incomplete coverage stays visible. These checks work with standalone GCP projects.</p>
        </>}
        {step === 3 && <>
          <h3 ref={headingRef} tabIndex={-1} className="text-base font-medium outline-none">Create a service account with read access</h3>
          <ol className="list-decimal space-y-3 pl-5 text-sm leading-relaxed"><li>Go to <ConsoleLink href={`https://console.cloud.google.com/iam-admin/serviceaccounts?project=${project}`}>IAM &amp; Admin → Service Accounts</ConsoleLink> and choose <strong>Create service account</strong>. Name it <code className="text-xs">runtz-security</code>.</li><li>On the target project, grant <strong>Security Reviewer</strong> (<code className="break-all text-xs">roles/iam.securityReviewer</code>). Start with this read-only role. Each scan reports any additional read permission required by an individual check.</li><li>Save the service account. Runtz does not need Owner or Editor access to your project.</li></ol>
          <Command value={`gcloud iam service-accounts create runtz-security --project=${projectId} --display-name="Runtz Security"\n\ngcloud projects add-iam-policy-binding ${projectId} \\\n  --member="serviceAccount:${email}" \\\n  --role="roles/iam.securityReviewer"`} />
          <p className="text-xs text-muted-foreground">Run these commands using your GCP administrator account. Runtz reads resource configurations without modifying your cloud resources.</p>
        </>}
        {step === 4 && <>
          <h3 ref={headingRef} tabIndex={-1} className="text-base font-medium outline-none">Create and upload the JSON key</h3>
          <ol className="list-decimal space-y-2 pl-5 text-sm leading-relaxed"><li>Open the service account you created in <ConsoleLink href={`https://console.cloud.google.com/iam-admin/serviceaccounts?project=${project}`}>Service Accounts</ConsoleLink>.</li><li>Select <strong>Keys → Add key → Create new key → JSON → Create</strong>.</li><li>Upload the downloaded JSON file below. Runtz will verify read access to the target project.</li></ol>
          <input ref={uploadRef} id="gcp-key" type="file" accept=".json,application/json" className="sr-only" disabled={busy} onChange={event => { void upload(event.target.files?.[0]); event.target.value = "" }} />
          <button type="button" disabled={busy} onClick={() => uploadRef.current?.click()} className="flex w-full flex-col items-center gap-2 rounded-xl border border-dashed bg-muted/20 px-5 py-6 transition-colors hover:border-primary hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-primary">
            {fileName ? <FileJsonIcon className="size-7 text-primary" /> : <UploadIcon className="size-7 text-muted-foreground" />}<span className="max-w-full truncate font-medium">{fileName || "Choose service account JSON"}</span><span className="text-xs text-muted-foreground">{fileName ? "Click to replace file" : "JSON key · up to 32 KB"}</span>
          </button>
          {credentials && <div className="space-y-1 rounded-lg border bg-muted/30 p-3 text-xs"><p className="break-all text-muted-foreground">Service account: <span className="text-foreground">{String(credentials.client_email)}</span></p><p className="text-muted-foreground">Target project: <span className="text-foreground">{projectId}</span></p></div>}
          <p className="flex items-start gap-2 text-xs leading-relaxed text-muted-foreground"><LockKeyholeIcon className="mt-0.5 size-3.5 shrink-0" />The key is encrypted on the server before being saved to MongoDB. It is never returned by the API. If your organization blocks service account keys, ask its administrator about the policy.</p>
        </>}
        {step === 5 && <>
          <h3 ref={headingRef} tabIndex={-1} className="text-base font-medium outline-none">Name your cloud connection</h3>
          <div className="flex items-center gap-3 rounded-lg border border-primary/30 bg-primary/5 p-4"><CheckCircle2Icon className="size-5 shrink-0 text-primary" /><div><p className="text-sm font-medium">GCP access verified</p><p className="mt-1 text-xs text-muted-foreground">{projectId}</p></div></div>
          <div className="space-y-2"><Label htmlFor="cloud-connection-name">Connection name</Label><Input id="cloud-connection-name" maxLength={80} placeholder="Production GCP" value={name} onChange={event => setName(event.target.value)} /></div>
          <p className="text-sm leading-relaxed text-muted-foreground">Connecting starts your first scan in the background. Findings and check coverage will appear in this project, and scans repeat automatically every 12 hours.</p>
          {busy && <p role="status" className="flex items-center gap-2 text-sm text-primary"><LoaderCircleIcon className="size-4 animate-spin" />Verifying the connection and queuing your first scan.</p>}
        </>}
        {step === 6 && connection && <div className="flex flex-col items-center gap-4 py-5 text-center">
          <div className="flex size-16 items-center justify-center rounded-full bg-primary/10"><CheckCircle2Icon className="size-8 text-primary" /></div>
          <div><h3 ref={headingRef} tabIndex={-1} className="text-lg font-medium outline-none">{connection.name}</h3><p className="mt-1 text-sm text-muted-foreground">{connection.projectId}</p></div>
          <p className="text-sm">Your first configuration scan is queued. Open the project to follow its progress and review findings as they become available.</p>
          <p className="max-w-md text-xs leading-relaxed text-muted-foreground">Checks report evidence, remediation and coverage. Add more projects from Google Cloud, or use + Cloud in the sidebar.</p>
        </div>}
        {error && <p role="alert" className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">{error}</p>}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t bg-muted/20 px-6 py-4 sm:px-8">
        {step === 0 ? <span className="text-xs text-muted-foreground">More cloud providers are on the way.</span> : step === 6 ? <Button variant="ghost" disabled={busy} onClick={onClose}>Close</Button> : <Button variant="ghost" disabled={busy} onClick={() => go(step - 1)}><ArrowLeftIcon />Back</Button>}
        {step > 0 && step < 4 && <Button disabled={step === 1 && (!workspaceId || !projectPattern.test(projectId))} onClick={() => go(step + 1)}>Continue<ArrowRightIcon /></Button>}
        {step === 4 && <Button disabled={!credentials || busy} onClick={() => void validate()}>{busy ? <LoaderCircleIcon className="animate-spin" /> : <LockKeyholeIcon />}{busy ? "Verifying access…" : "Verify & continue"}</Button>}
        {step === 5 && <Button disabled={!validated || !name.trim() || busy} onClick={() => void connect()}>{busy ? <LoaderCircleIcon className="animate-spin" /> : <GCPIcon className="size-4" />}{busy ? "Connecting…" : "Connect & start scan"}</Button>}
        {step === 6 && connection && <Button disabled={busy} onClick={async () => { setBusy(true); try { await onConnected(connection); onClose() } catch (err) { setError(cloudError(err)) } finally { setBusy(false) } }}>View Google Cloud<ArrowRightIcon /></Button>}
      </div>
    </DialogContent>
  </Dialog>
}
