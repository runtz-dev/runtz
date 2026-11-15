"use client"

import * as React from "react"
import Link from "next/link"
import { ArrowLeftIcon, ArrowRightIcon, CheckCircle2Icon, CheckIcon, ExternalLinkIcon, LoaderCircleIcon, ShieldCheckIcon, TerminalIcon } from "lucide-react"
import { CloudProviderIcon } from "@/components/runtz/cloud-icons"
import { CloudAccessGuide } from "@/components/runtz/cloud-access-guide"
import { CloudCommand } from "@/components/runtz/cloud-command"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cloudError, cloudProviderNames, cloudScope, listCloudConnections, type CloudConnection, type CloudProvider } from "@/lib/cloud"

const steps = ["Prepare access", "Sign in to Runtz", "Configure locally", "Send your scan"]
const expectedFields = { gcp: ["Project ID", "Path to your service account JSON"], aws: ["12-digit AWS account ID", "Access key ID and secret access key", "Regions to scan"], azure: ["Subscription ID", "Tenant ID and application (client) ID", "Client secret value"] }

export function CloudCLIDialog({ provider, onBack, onClose, onCreated, onConnected }: { provider: CloudProvider; onBack: () => void; onClose: () => void; onCreated: () => Promise<void>; onConnected: (connection: CloudConnection) => Promise<void> }) {
  const { workspaces, selectedWorkspaceId } = useWorkspace()
  const [workspaceId, setWorkspaceId] = React.useState(workspaces.some(w => w.id === selectedWorkspaceId) ? selectedWorkspaceId : workspaces[0]?.id ?? "")
  const [scope, setScope] = React.useState("")
  const [step, setStep] = React.useState(0)
  const [waiting, setWaiting] = React.useState(false)
  const [error, setError] = React.useState("")
  const [connection, setConnection] = React.useState<CloudConnection | null>(null)
  const [since] = React.useState(() => Date.now())
  const heading = React.useRef<HTMLHeadingElement>(null)
  const validScope = provider === "gcp" ? /^[a-z][a-z0-9-]{4,61}[a-z0-9]$/.test(scope) : provider === "aws" ? /^\d{12}$/.test(scope) : /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(scope)
  React.useEffect(() => { heading.current?.focus() }, [step])
  React.useEffect(() => {
    if (!waiting) return
    const abort = new AbortController()
    let timer: ReturnType<typeof setTimeout>
    async function check() {
      try {
        const response = await listCloudConnections(workspaceId, abort.signal)
        if (abort.signal.aborted) return
        const match = response.connections.find(c => c.provider === provider && c.source === "cli" && cloudScope(c).toLowerCase() === scope.toLowerCase() && c.lastScanAt && new Date(c.updatedAt).getTime() >= since)
        if (match) { setConnection(match); setWaiting(false); setError(""); await onCreated(); return }
        setError("")
      } catch (err) { if (!abort.signal.aborted) setError(cloudError(err)) }
      if (!abort.signal.aborted) timer = setTimeout(() => void check(), 4000)
    }
    void check()
    return () => { abort.abort(); clearTimeout(timer) }
  }, [waiting, workspaceId, provider, scope, since, onCreated])
  return <Dialog open onOpenChange={open => { if (!open) onClose() }}><DialogContent className="max-h-[90dvh] min-w-0 grid-cols-[minmax(0,1fr)] gap-0 overflow-x-hidden overflow-y-auto p-0 sm:max-w-4xl">
    <DialogHeader className="border-b px-6 py-5 pr-12 sm:px-8"><div className="mb-3 flex items-center gap-2 text-xs text-muted-foreground"><CloudProviderIcon provider={provider} className="size-5" />{cloudProviderNames[provider]}<span className="text-border">/</span><TerminalIcon className="size-3.5" />RUNTZ CLI</div><DialogTitle className="text-xl">Your cloud. Your terminal.</DialogTitle><DialogDescription>Start read-only checks on Runtz Security from your terminal.</DialogDescription></DialogHeader>
    <div className="grid min-w-0 md:grid-cols-[210px_minmax(0,1fr)]">
      <nav aria-label="CLI setup steps" className="flex gap-1 overflow-x-auto border-b bg-muted/15 p-3 md:flex-col md:overflow-visible md:border-r md:border-b-0 md:p-5">{steps.map((label, index) => <button key={label} aria-current={step === index ? "step" : undefined} disabled={waiting} onClick={() => setStep(index)} className={`flex shrink-0 items-center gap-2.5 rounded-lg px-3 py-3 text-left text-xs transition-colors focus-visible:outline-2 focus-visible:outline-primary ${step === index ? "bg-primary/10 font-medium text-primary" : "text-muted-foreground hover:bg-muted"}`}><span className="flex size-5 shrink-0 items-center justify-center rounded-full border text-[10px]">{index < step ? <CheckIcon className="size-3" /> : index + 1}</span>{label}</button>)}</nav>
      <div className="min-w-0 space-y-5 p-6 sm:p-8">
        <div><p className="mb-2 text-xs font-medium text-primary">STEP {step + 1} OF 4</p><h3 ref={heading} tabIndex={-1} className="text-lg font-medium outline-none">{steps[step]}</h3></div>
        {step === 0 && <><div className="space-y-2"><Label htmlFor="cli-cloud-scope">{provider === "gcp" ? "GCP project ID" : provider === "aws" ? "AWS account ID" : "Azure subscription ID"}</Label><Input id="cli-cloud-scope" autoComplete="off" spellCheck={false} value={scope} onChange={e => setScope(e.target.value.trim())} placeholder={provider === "gcp" ? "my-production-project" : provider === "aws" ? "123456789012" : "00000000-0000-0000-0000-000000000000"} /></div><CloudAccessGuide provider={provider} scope={scope} /></>}
        {step === 1 && <><p className="text-sm leading-6 text-muted-foreground">Install or update Runtz CLI, then sign in with a token for the workspace that should receive your results.</p><CloudCommand label="Update Runtz CLI" value="runtz update" /><a href="https://runtz.dev/docs/install-cli" target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">First time? Install the CLI<ExternalLinkIcon className="size-3.5" /></a><div className="space-y-2"><Label htmlFor="cli-cloud-workspace">Destination workspace</Label><select id="cli-cloud-workspace" value={workspaceId} onChange={e => setWorkspaceId(e.target.value)} className="h-10 w-full rounded-lg border bg-background px-3 text-sm">{workspaces.filter(w => w.kind !== "playground").map(w => <option key={w.id} value={w.id}>{w.name}</option>)}</select><p className="text-xs leading-5 text-muted-foreground">Use a token issued for this workspace. The token determines the upload destination.</p></div><CloudCommand label="Sign in to your workspace" value="runtz login" /><Button variant="outline" size="sm" render={<Link href="/api-keys" target="_blank" />}>Open API tokens<ExternalLinkIcon /></Button></>}
        {step === 2 && <><CloudCommand label={`Configure ${cloudProviderNames[provider]}`} value={`runtz ${provider} config`} /><p className="text-sm text-muted-foreground">The guided prompt asks for:</p><ul className="space-y-3 text-sm">{expectedFields[provider].map(field => <li key={field} className="flex items-center gap-2"><CheckCircle2Icon className="size-4 shrink-0 text-primary" />{field}</li>)}</ul><div className="rounded-lg border bg-muted/20 p-4 text-xs leading-6 text-muted-foreground">Saved locally in <code className="break-all text-foreground">~/.config/runtz/config.json</code> on Linux. macOS and Windows use the native user config directory. GCP saves the JSON file path; AWS and Azure save their credentials with owner-only file access.</div><p className="flex items-start gap-2 text-xs leading-5 text-muted-foreground"><ShieldCheckIcon className="mt-0.5 size-4 shrink-0" />Validation and scans send cloud credentials over HTTPS to Runtz Security. This CLI API uses them for the request without saving them. Only evaluated checks, findings and coverage go to your workspace.</p></>}
        {step === 3 && <><CloudCommand label="Run and send your cloud scan" value={`runtz ${provider}`} /><p className="text-sm leading-6 text-muted-foreground">The CLI sends cloud authorization to Runtz Security, where the private checks run, then uploads the results to your workspace. The terminal shows progress, a summary and the result link. Missing configuration opens setup automatically in an interactive terminal.</p>{!validScope && <div className="space-y-2"><Label htmlFor="cli-wait-scope">Enter the cloud scope to find your scan</Label><Input id="cli-wait-scope" value={scope} onChange={e => setScope(e.target.value.trim())} placeholder={provider === "gcp" ? "Project ID" : provider === "aws" ? "12-digit account ID" : "Subscription ID"} /></div>}{connection ? <div role="status" className="space-y-3 rounded-xl border border-primary/30 bg-primary/5 p-5"><p className="flex items-center gap-2 font-medium"><CheckCircle2Icon className="size-5 text-primary" />Your scan has arrived</p><p className="text-sm text-muted-foreground">{connection.name} · {cloudScope(connection)}</p><Button onClick={async () => { try { await onConnected(connection) } catch (err) { setError(cloudError(err)) } }}>View findings<ArrowRightIcon /></Button></div> : <div className="space-y-3 rounded-xl border bg-muted/15 p-4"><Button variant="outline" disabled={!validScope || !workspaceId} onClick={() => setWaiting(!waiting)}>{waiting ? <LoaderCircleIcon className="animate-spin" /> : <TerminalIcon />}{waiting ? "Stop waiting" : "Wait for my scan"}</Button><p role="status" className="text-xs leading-5 text-muted-foreground">{waiting ? "Listening for a new CLI result in your selected workspace. Keep this window open while the terminal finishes." : "After running the command, wait here to open your findings as soon as they arrive."}</p></div>}<p className="text-xs leading-5 text-muted-foreground">Run again whenever you need fresh results, or schedule the command in your CI pipeline. CLI scans start only when you run or schedule the command.</p></>}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <a href={`https://runtz.dev/docs/scans/${provider === "gcp" ? "google-cloud" : provider}#scan-with-runtz-cli`} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-xs text-primary hover:underline">Full setup guide and permissions<ExternalLinkIcon className="size-3.5" /></a>
      </div>
    </div>
    <div className="flex justify-between gap-3 border-t px-6 py-4 sm:px-8"><Button variant="ghost" disabled={waiting} onClick={() => step === 0 ? onBack() : setStep(step - 1)}><ArrowLeftIcon />Back</Button>{step < 3 ? <Button onClick={() => setStep(step + 1)}>Continue<ArrowRightIcon /></Button> : <Button variant="ghost" onClick={onClose}>Close</Button>}</div>
  </DialogContent></Dialog>
}
