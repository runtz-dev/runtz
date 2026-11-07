"use client"

import * as React from "react"
import Link from "next/link"
import { ExternalLinkIcon, LoaderCircleIcon, UnplugIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { SettingsFeedback } from "@/components/runtz/settings-ui"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { apiRequest } from "@/lib/api"
import { cloudConnectionPath, cloudError, type CloudConnection } from "@/lib/cloud"

export function AWSConnectionControls({ connection, canManage }: { connection: CloudConnection; canManage: boolean }) {
  const { refresh } = useCloud()
  const [confirm, setConfirm] = React.useState("")
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const roleName = connection.roleArn?.split("/").at(-1) ?? ""
  return <div className="flex min-w-0 flex-col gap-5"><dl className="grid gap-4 text-sm sm:grid-cols-2">{[["Account", connection.accountId], ["Role ARN", connection.roleArn], ["Regions", connection.regions?.join(", ")], ["Scan schedule", "Every 12 hours"]].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl><div className="flex flex-wrap gap-2"><Button variant="outline" size="sm" render={<Link href={cloudConnectionPath(connection)} />}>View findings</Button>{canManage && <Button variant="outline" size="sm" render={<a href={`https://console.aws.amazon.com/iam/home#/roles/details/${encodeURIComponent(roleName)}`} target="_blank" rel="noopener noreferrer" />}>Manage IAM role<ExternalLinkIcon data-icon="inline-end" /></Button>}</div>{canManage && <FieldGroup><Field><FieldLabel htmlFor={`disconnect-${connection.id}`}>Disconnect AWS account</FieldLabel><FieldDescription>Remove this connection and its results from Runtz. Delete its CloudFormation stack separately to remove the IAM role. To change the role or regions, reconnect the account.</FieldDescription><Input id={`disconnect-${connection.id}`} autoComplete="off" placeholder={`Type ${connection.accountId} to confirm`} value={confirm} onChange={e => setConfirm(e.target.value)} disabled={busy} /></Field><Button variant="destructive" className="w-fit" disabled={busy || confirm !== connection.accountId} onClick={async () => { setBusy(true); setError(""); try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh() } catch (err) { setError(cloudError(err)) } finally { setBusy(false) } }}>{busy ? <LoaderCircleIcon className="animate-spin" data-icon="inline-start" /> : <UnplugIcon data-icon="inline-start" />}Disconnect AWS</Button></FieldGroup>}{error && <SettingsFeedback error>{error}</SettingsFeedback>}</div>
}
