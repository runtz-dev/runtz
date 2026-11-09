"use client"

import * as React from "react"
import Link from "next/link"
import { ExternalLinkIcon, LoaderCircleIcon, UnplugIcon } from "lucide-react"
import { AWSAccessKeyFields, validAWSAccessKeys } from "@/components/runtz/aws-access-key-fields"
import { useCloud } from "@/components/runtz/cloud-context"
import { SettingsFeedback } from "@/components/runtz/settings-ui"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { apiRequest } from "@/lib/api"
import { cloudConnectionPath, cloudError, type CloudConnection } from "@/lib/cloud"

export function AWSConnectionControls({ connection, canManage }: { connection: CloudConnection; canManage: boolean }) {
  const { refresh } = useCloud()
  const [confirm, setConfirm] = React.useState("")
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState("")
  const [accessKeyId, setAccessKeyId] = React.useState("")
  const [secretAccessKey, setSecretAccessKey] = React.useState("")
  const [saved, setSaved] = React.useState(false)
  const usesKeys = connection.authMethod === "access_key"
  async function rotate() {
    setBusy(true); setError(""); setSaved(false)
    try {
      await apiRequest(`/api/v1/cloud/connections/${connection.id}/credentials`, { method: "PATCH", body: { credentials: { authMethod: "access_key", accessKeyId, secretAccessKey, regions: connection.regions } } })
      setAccessKeyId(""); setSecretAccessKey(""); setSaved(true)
      await refresh()
    } catch (err) { setError(cloudError(err)) } finally { setBusy(false) }
  }
  const roleName = connection.roleArn?.split("/").at(-1) ?? ""
  return <div className="flex min-w-0 flex-col gap-5"><dl className="grid gap-4 text-sm sm:grid-cols-2">{[["Account", connection.accountId], ["Authentication", usesKeys ? "IAM user access key" : "IAM role"], ...(!usesKeys ? [["Role ARN", connection.roleArn]] : []), ["Regions", connection.regions?.join(", ")], ["Scan schedule", "Every 12 hours"]].map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 break-all">{value || "Not provided"}</dd></div>)}</dl><div className="flex flex-wrap gap-2"><Button variant="outline" size="sm" render={<Link href={cloudConnectionPath(connection)} />}>View findings</Button>{canManage && !usesKeys && <Button variant="outline" size="sm" render={<a href={`https://console.aws.amazon.com/iam/home#/roles/details/${encodeURIComponent(roleName)}`} target="_blank" rel="noopener noreferrer" />}>Manage IAM role<ExternalLinkIcon data-icon="inline-end" /></Button>}</div>{canManage && usesKeys && <FieldSet disabled={busy}>
    <FieldLegend>Replace access key</FieldLegend>
    <FieldDescription>Create a new key for the dedicated IAM user, save it here, and revoke the old key in AWS after verifying a scan. Account and regions stay the same.</FieldDescription>
    <FieldGroup><AWSAccessKeyFields id={`aws-rotate-${connection.id}`} accessKeyId={accessKeyId} secretAccessKey={secretAccessKey} onAccessKeyIdChange={value => { setAccessKeyId(value); setSaved(false) }} onSecretAccessKeyChange={value => { setSecretAccessKey(value); setSaved(false) }} /><Button variant="outline" className="w-fit" disabled={busy || !validAWSAccessKeys(accessKeyId, secretAccessKey)} onClick={() => void rotate()}>{busy && <LoaderCircleIcon className="animate-spin" data-icon="inline-start" />}Save replacement key</Button></FieldGroup>
    {saved && <SettingsFeedback>Access key replaced. Run a scan before revoking the old key in AWS.</SettingsFeedback>}
  </FieldSet>}{canManage && <FieldGroup><Field><FieldLabel htmlFor={`disconnect-${connection.id}`}>Disconnect AWS account</FieldLabel><FieldDescription>{usesKeys ? "Remove this connection, its stored credentials and results from Runtz. Revoke the access key separately in AWS. Reconnect to change authentication or regions." : "Remove this connection and its results from Runtz. Delete its CloudFormation stack separately to remove the IAM role. Reconnect to change authentication, role or regions."}</FieldDescription><Input id={`disconnect-${connection.id}`} autoComplete="off" placeholder={`Type ${connection.accountId} to confirm`} value={confirm} onChange={e => setConfirm(e.target.value)} disabled={busy} /></Field><Button variant="destructive" className="w-fit" disabled={busy || confirm !== connection.accountId} onClick={async () => { setBusy(true); setError(""); try { await apiRequest(`/api/v1/cloud/connections/${connection.id}`, { method: "DELETE" }); await refresh() } catch (err) { setError(cloudError(err)) } finally { setBusy(false) } }}>{busy ? <LoaderCircleIcon className="animate-spin" data-icon="inline-start" /> : <UnplugIcon data-icon="inline-start" />}Disconnect AWS</Button></FieldGroup>}{error && <SettingsFeedback error>{error}</SettingsFeedback>}</div>
}
