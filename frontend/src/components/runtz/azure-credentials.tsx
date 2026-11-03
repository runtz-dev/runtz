"use client"

import { useId } from "react"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

export type AzureCredentials = { tenantId: string; clientId: string; clientSecret: string }
export const azureIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
export function validAzureCredentials(value: AzureCredentials) { return azureIDPattern.test(value.tenantId) && azureIDPattern.test(value.clientId) && value.clientSecret.trim().length > 0 && value.clientSecret.length <= 4096 }

export function AzureCredentialFields({ value, onChange, lockIdentity = false }: { value: AzureCredentials; onChange: (value: AzureCredentials) => void; lockIdentity?: boolean }) {
  const id = useId()
  return <div className="space-y-4">
    {([['tenantId', 'Directory (tenant) ID'], ['clientId', 'Application (client) ID']] as const).map(([key, label]) => <div key={key} className="space-y-2"><Label htmlFor={`${id}-${key}`}>{label}</Label><Input id={`${id}-${key}`} value={value[key]} maxLength={36} readOnly={lockIdentity} autoComplete="off" spellCheck={false} placeholder="00000000-0000-0000-0000-000000000000" onChange={event => onChange({ ...value, [key]: event.target.value.trim() })} /></div>)}
    <div className="space-y-2"><Label htmlFor={`${id}-secret`}>Client secret value</Label><Input id={`${id}-secret`} type="password" value={value.clientSecret} maxLength={4096} autoComplete="new-password" onChange={event => onChange({ ...value, clientSecret: event.target.value })} /><p className="text-xs text-muted-foreground">Use the secret value from Certificates &amp; secrets, rather than its secret ID.</p></div>
  </div>
}
