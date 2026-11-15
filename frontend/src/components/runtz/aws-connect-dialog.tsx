"use client"

import * as React from "react"
import { ArrowLeftIcon, ArrowRightIcon, CheckCircle2Icon, ExternalLinkIcon, LoaderCircleIcon } from "lucide-react"
import { AWSAccessKeyFields, validAWSAccessKeys } from "@/components/runtz/aws-access-key-fields"
import { CloudAccessGuide } from "@/components/runtz/cloud-access-guide"
import { AWSIcon } from "@/components/runtz/cloud-icons"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldSet } from "@/components/ui/field"
import { apiRequest } from "@/lib/api"
import { awsSetupDocumentation, cloudError, type CloudConnection } from "@/lib/cloud"

const steps = ["Account", "Read-only access", "Connect"]
const regionPattern = /^(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|southeast)-[1-9][0-9]?$/

export function AWSConnectDialog({ onClose, onBack, onCreated, onConnected }: { onClose: () => void; onBack: () => void; onCreated: () => Promise<void>; onConnected: (connection: CloudConnection) => Promise<void> }) {
  const { selectedWorkspaceId, workspaces, currentUser, deploymentMode } = useWorkspace()
  const manageable = workspaces.filter(w => deploymentMode === "self-hosted" ? currentUser.role === "admin" : w.createdBy === currentUser.id)
  const [workspaceId, setWorkspaceId] = React.useState(manageable.some(w => w.id === selectedWorkspaceId) ? selectedWorkspaceId : manageable[0]?.id ?? "")
  const [accountId, setAccountId] = React.useState("")
  const [regionInput, setRegionInput] = React.useState("")
  const [accessKeyId, setAccessKeyId] = React.useState("")
  const [secretAccessKey, setSecretAccessKey] = React.useState("")
  const [name, setName] = React.useState("")
  const [step, setStep] = React.useState(1)
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [connection, setConnection] = React.useState<CloudConnection | null>(null)
  const heading = React.useRef<HTMLHeadingElement>(null)
  React.useEffect(() => { heading.current?.focus() }, [step])
  const regions = regionInput.split(/[\s,]+/).filter(Boolean)
  const validRegions = regions.length > 0 && regions.length <= 32 && new Set(regions).size === regions.length && regions.every(r => regionPattern.test(r))
  const payload = { workspaceId, accountId, credentials: { authMethod: "access_key", accessKeyId, secretAccessKey, regions } }
  function go(next: number) { setStep(next); setError("") }
  async function verify() {
    setBusy(true); setError("")
    try { await apiRequest("/api/v1/cloud/aws/validate", { method: "POST", body: payload }); setName(current => current || accountId); go(3) }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  async function connect() {
    setBusy(true); setError("")
    try { const result = await apiRequest<{ connection: CloudConnection }>("/api/v1/cloud/aws/connections", { method: "POST", body: { ...payload, name: name.trim() } }); setConnection(result.connection); setAccessKeyId(""); setSecretAccessKey(""); go(4); await onCreated() }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}><DialogContent showCloseButton={!busy} className="max-h-[90dvh] min-w-0 grid-cols-[minmax(0,1fr)] gap-0 overflow-x-hidden overflow-y-auto p-0 sm:max-w-3xl">
    <DialogHeader className="border-b px-6 py-5 pr-12 sm:px-8"><p className="mb-3 flex items-center gap-2 text-xs text-muted-foreground"><AWSIcon className="size-5" />CLOUD INTEGRATIONS / AMAZON WEB SERVICES</p><DialogTitle>{connection ? "Your cloud is connected" : "Connect Amazon Web Services"}</DialogTitle><DialogDescription>Connect a dedicated IAM user&apos;s access key for read-only configuration checks.</DialogDescription></DialogHeader>
    {!connection && <ol aria-label="Connection progress" className="flex justify-between gap-3 border-b bg-muted/20 px-6 py-4 text-xs sm:px-8">{steps.map((label, index) => <li key={label} aria-current={step === index + 1 ? "step" : undefined} className={step === index + 1 ? "font-medium text-primary" : "text-muted-foreground"}>{index + 1}. {label}</li>)}</ol>}
    <FieldSet disabled={busy} className="min-w-0 px-6 py-6 sm:px-8">
      {step === 1 && <><h3 ref={heading} tabIndex={-1} className="font-medium outline-none">Choose an account and the regions to scan</h3><FieldGroup>
        <Field><FieldLabel htmlFor="aws-workspace">Workspace</FieldLabel><select id="aws-workspace" value={workspaceId} onChange={e => setWorkspaceId(e.target.value)} className="h-10 w-full rounded-lg border bg-background px-3 text-sm">{manageable.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}</select></Field>
        <Field><FieldLabel htmlFor="aws-account">AWS account ID</FieldLabel><Input id="aws-account" inputMode="numeric" autoComplete="off" maxLength={12} placeholder="123456789012" value={accountId} onChange={e => setAccountId(e.target.value.trim())} /><FieldDescription>Copy the 12-digit account ID from the AWS console. Connect each account separately.</FieldDescription></Field>
        <Field><FieldLabel htmlFor="aws-regions">Regions</FieldLabel><Input id="aws-regions" autoComplete="off" spellCheck={false} placeholder="us-east-1, sa-east-1" value={regionInput} onChange={e => setRegionInput(e.target.value)} /><FieldDescription>Enter your regions, separated by commas. Global IAM and S3 account checks run once. Other regions remain outside coverage.</FieldDescription></Field>
      </FieldGroup></>}
      {step === 2 && <><h3 ref={heading} tabIndex={-1} className="font-medium outline-none">Create a dedicated read-only IAM user</h3><CloudAccessGuide provider="aws" scope={accountId} /><FieldGroup><AWSAccessKeyFields id="aws-connect" accessKeyId={accessKeyId} secretAccessKey={secretAccessKey} onAccessKeyIdChange={setAccessKeyId} onSecretAccessKeyChange={setSecretAccessKey} /></FieldGroup><FieldDescription>Use a dedicated IAM user key. Root and temporary session credentials are not supported. Runtz encrypts the key before storing it.</FieldDescription></>}
      {step === 3 && <><h3 ref={heading} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-5 text-primary" />AWS identity and account verified</h3><p className="text-sm text-muted-foreground">Account {accountId} · {regions.join(", ")}. The first scan reports permissions and incomplete coverage.</p><FieldGroup><Field><FieldLabel htmlFor="aws-name">Connection name</FieldLabel><Input id="aws-name" maxLength={80} placeholder="Production AWS" value={name} onChange={e => setName(e.target.value)} /></Field></FieldGroup></>}
      {connection && <><h3 ref={heading} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-5 text-primary" />Your first scan is running</h3><p className="text-sm text-muted-foreground">Results refresh automatically. Scans repeat every 12 hours.</p><p className="text-sm">{connection.name} · {connection.accountId}</p></>}
      {error && <FieldError role="alert">{error}</FieldError>}
      <a href={awsSetupDocumentation} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-xs text-primary hover:underline">AWS setup and required permissions<ExternalLinkIcon className="size-3.5" /></a>
    </FieldSet>
    <div className="flex justify-between gap-3 border-t px-6 py-4 sm:px-8">{connection ? <><Button variant="ghost" onClick={onClose}>Close</Button><Button disabled={busy} onClick={async () => { try { await onConnected(connection) } catch (err) { setError(cloudError(err)) } }}>View AWS findings<ArrowRightIcon /></Button></> : <><Button variant="ghost" disabled={busy} onClick={() => step === 1 ? onBack() : go(step - 1)}><ArrowLeftIcon />Back</Button><Button disabled={busy || (step === 1 ? !workspaceId || !/^\d{12}$/.test(accountId) || !validRegions : step === 2 ? !validAWSAccessKeys(accessKeyId, secretAccessKey) : !name.trim())} onClick={() => { if (step === 1) go(2); else void (step === 2 ? verify() : connect()) }}>{busy && <LoaderCircleIcon className="animate-spin" />}{step === 1 ? "Continue" : step === 2 ? "Verify access" : "Connect and start scan"}</Button></>}</div>
  </DialogContent></Dialog>
}
