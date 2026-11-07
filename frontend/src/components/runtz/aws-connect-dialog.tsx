"use client"

import * as React from "react"
import { ArrowLeftIcon, ArrowRightIcon, CheckCircle2Icon, DownloadIcon, ExternalLinkIcon, LoaderCircleIcon, ShieldCheckIcon } from "lucide-react"
import { AWSIcon } from "@/components/runtz/cloud-icons"
import { CloudCommand } from "@/components/runtz/cloud-command"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel, FieldSet } from "@/components/ui/field"
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { apiRequest } from "@/lib/api"
import { awsSetupDocumentation, cloudError, safeCloudLink, type CloudConnection } from "@/lib/cloud"
import { cn } from "@/lib/utils"

type AWSSetup = { accountId: string; principalArn: string; externalId: string; roleArn: string; roleName: string; template: Record<string, unknown>; quickCreateUrl?: string }
const steps = ["Account", "Authorize access", "Connect"]
const regionPattern = /^(af|ap|ca|eu|il|me|mx|sa|us)-(central|east|west|north|south|northeast|southeast)-[1-9][0-9]?$/

export function AWSConnectDialog({ onClose, onBack, onCreated, onConnected }: { onClose: () => void; onBack: () => void; onCreated: () => Promise<void>; onConnected: (connection: CloudConnection) => Promise<void> }) {
  const { selectedWorkspaceId, workspaces, currentUser, deploymentMode } = useWorkspace()
  const manageable = workspaces.filter(w => deploymentMode === "self-hosted" ? currentUser.role === "admin" : w.createdBy === currentUser.id)
  const [workspaceId, setWorkspaceId] = React.useState(manageable.some(w => w.id === selectedWorkspaceId) ? selectedWorkspaceId : manageable[0]?.id ?? "")
  const [accountId, setAccountId] = React.useState("")
  const [regionInput, setRegionInput] = React.useState("")
  const [roleArn, setRoleArn] = React.useState("")
  const [name, setName] = React.useState("")
  const [setup, setSetup] = React.useState<AWSSetup | null>(null)
  const [step, setStep] = React.useState(1)
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [connection, setConnection] = React.useState<CloudConnection | null>(null)
  const heading = React.useRef<HTMLHeadingElement>(null)
  React.useEffect(() => { heading.current?.focus() }, [step])
  const regions = regionInput.split(/[\s,]+/).filter(Boolean)
  const validRegions = regions.length > 0 && regions.length <= 32 && new Set(regions).size === regions.length && regions.every(r => regionPattern.test(r))
  const validRole = new RegExp(`^arn:aws:iam::${accountId}:role/[a-zA-Z0-9+=,.@_/-]+$`).test(roleArn)
  const payload = { workspaceId, accountId, credentials: { roleArn, externalId: setup?.externalId, regions } }
  function go(next: number) { setStep(next); setError("") }
  async function prepare() {
    setBusy(true); setError("")
    try { const result = await apiRequest<AWSSetup>("/api/v1/cloud/aws/onboarding", { method: "POST", body: { workspaceId, accountId } }); setSetup(result); setRoleArn(result.roleArn); setName(current => current || accountId); go(2) }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  async function verify() {
    setBusy(true); setError("")
    try { await apiRequest("/api/v1/cloud/aws/validate", { method: "POST", body: payload }); go(3) }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  async function connect() {
    setBusy(true); setError("")
    try { const result = await apiRequest<{ connection: CloudConnection }>("/api/v1/cloud/aws/connections", { method: "POST", body: { ...payload, name: name.trim() } }); setConnection(result.connection); setSetup(null); go(4); await onCreated() }
    catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  function download() {
    if (!setup) return
    const url = URL.createObjectURL(new Blob([JSON.stringify(setup.template, null, 2)], { type: "application/json" }))
    const link = document.createElement("a"); link.href = url; link.download = `${setup.roleName}.json`; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  const quickCreate = setup?.quickCreateUrl && safeCloudLink(setup.quickCreateUrl)
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose() }}><DialogContent showCloseButton={!busy} className="max-h-[90dvh] min-w-0 gap-0 overflow-y-auto p-0 sm:max-w-3xl">
    <DialogHeader className="border-b px-6 py-5 pr-12 sm:px-8"><p className="flex items-center gap-2 text-xs text-muted-foreground"><AWSIcon className="size-5" />CLOUD INTEGRATIONS / AMAZON WEB SERVICES</p><DialogTitle>{connection ? "Your cloud is connected" : "Connect Amazon Web Services"}</DialogTitle><DialogDescription>Authorize read-only configuration checks with an IAM role. Your AWS access keys stay with you.</DialogDescription></DialogHeader>
    {!connection && <ol aria-label="Connection progress" className="flex justify-between gap-3 border-b bg-muted/20 px-6 py-4 text-xs sm:px-8">{steps.map((label, index) => <li key={label} aria-current={step === index + 1 ? "step" : undefined} className={cn("text-muted-foreground", step === index + 1 && "font-medium text-primary")}>{index + 1}. {label}</li>)}</ol>}
    <FieldSet disabled={busy} className="px-6 py-6 sm:px-8">
      {step === 1 && <><h3 ref={heading} tabIndex={-1} className="font-medium outline-none">Choose an account and the regions to scan</h3><FieldGroup>
        <Field><FieldLabel htmlFor="aws-workspace">Workspace</FieldLabel><Select value={workspaceId} items={manageable.map(w => ({ value: w.id, label: w.name }))} onValueChange={value => setWorkspaceId(value ?? "")}><SelectTrigger id="aws-workspace" className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectGroup>{manageable.map(w => <SelectItem key={w.id} value={w.id}>{w.name}</SelectItem>)}</SelectGroup></SelectContent></Select></Field>
        <Field><FieldLabel htmlFor="aws-account">AWS account ID</FieldLabel><Input id="aws-account" inputMode="numeric" autoComplete="off" maxLength={12} placeholder="123456789012" value={accountId} onChange={e => setAccountId(e.target.value.trim())} /><FieldDescription>Copy the 12-digit account ID from the AWS console. Connect each account separately.</FieldDescription></Field>
        <Field><FieldLabel htmlFor="aws-regions">Regions</FieldLabel><Input id="aws-regions" autoComplete="off" spellCheck={false} placeholder="us-east-1, sa-east-1" value={regionInput} onChange={e => setRegionInput(e.target.value)} /><FieldDescription>Enter the regions you use, separated by commas. Global IAM and S3 account checks run once. Resources in other regions are outside scan coverage.</FieldDescription></Field>
      </FieldGroup></>}
      {step === 2 && setup && <><h3 ref={heading} tabIndex={-1} className="font-medium outline-none">Authorize Runtz in your AWS account</h3><ol className="flex list-decimal flex-col gap-3 pl-5 text-sm leading-relaxed"><li>Open CloudFormation in account <strong>{accountId}</strong> with permission to create IAM roles.</li><li>Review the template, acknowledge the named IAM resources, and create the stack.</li><li>Wait for <strong>CREATE_COMPLETE</strong>, then copy <strong>RoleArn</strong> from Outputs below.</li></ol><div className="flex flex-wrap gap-2">{quickCreate && <Button render={<a href={quickCreate} target="_blank" rel="noopener noreferrer" />}>Open AWS setup<ExternalLinkIcon data-icon="inline-end" /></Button>}<Button variant="outline" onClick={download}><DownloadIcon data-icon="inline-start" />Download CloudFormation template</Button>{!quickCreate && <Button variant="outline" render={<a href="https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/create/template" target="_blank" rel="noopener noreferrer" />}>Open CloudFormation<ExternalLinkIcon data-icon="inline-end" /></Button>}</div>{!quickCreate && <FieldDescription>Choose Create stack → With new resources → Upload a template file, then upload the downloaded JSON. Its parameters are already filled in.</FieldDescription>}<CloudCommand label="Show external ID" value={setup.externalId} /><FieldGroup><Field><FieldLabel htmlFor="aws-role">Role ARN from stack Outputs</FieldLabel><Input id="aws-role" autoComplete="off" spellCheck={false} value={roleArn} onChange={e => setRoleArn(e.target.value.trim())} /><FieldDescription>The role must belong to account {accountId} and require this connection&apos;s External ID.</FieldDescription></Field></FieldGroup><p className="flex items-center gap-2 text-xs text-muted-foreground"><ShieldCheckIcon className="size-4 shrink-0" />The template grants configuration reads for 18 checks across IAM, S3, EC2/EBS, RDS, CloudTrail and EKS.</p></>}
      {step === 3 && <><h3 ref={heading} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-5 text-primary" />AWS role and account verified</h3><p className="text-sm text-muted-foreground">Account {accountId} · {regions.join(", ")}. The first scan will report permissions and incomplete coverage.</p><FieldGroup><Field><FieldLabel htmlFor="aws-name">Connection name</FieldLabel><Input id="aws-name" maxLength={80} placeholder="Production AWS" value={name} onChange={e => setName(e.target.value)} /></Field></FieldGroup></>}
      {connection && <><h3 ref={heading} tabIndex={-1} className="flex items-center gap-2 font-medium outline-none"><CheckCircle2Icon className="size-5 text-primary" />Your first scan is running</h3><p className="text-sm text-muted-foreground">Results refresh automatically. Scans repeat every 12 hours.</p><p className="text-sm">{connection.name} · {connection.accountId}</p></>}
      {error && <FieldError role="alert">{error}</FieldError>}
      <a href={awsSetupDocumentation} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-xs text-primary hover:underline">AWS setup and required permissions<ExternalLinkIcon className="size-3.5" /></a>
    </FieldSet>
    <div className="flex justify-between gap-3 border-t px-6 py-4 sm:px-8">{connection ? <><Button variant="ghost" onClick={onClose}>Close</Button><Button disabled={busy} onClick={() => void onConnected(connection)}>View AWS findings<ArrowRightIcon data-icon="inline-end" /></Button></> : <><Button variant="ghost" disabled={busy} onClick={() => step === 1 ? onBack() : go(step - 1)}><ArrowLeftIcon data-icon="inline-start" />Back</Button><Button disabled={busy || (step === 1 ? !workspaceId || !/^\d{12}$/.test(accountId) || !validRegions : step === 2 ? !validRole : !name.trim())} onClick={() => void (step === 1 ? prepare() : step === 2 ? verify() : connect())}>{busy && <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />}{step === 1 ? "Continue" : step === 2 ? "Verify access" : "Connect and start scan"}</Button></>}</div>
  </DialogContent></Dialog>
}
