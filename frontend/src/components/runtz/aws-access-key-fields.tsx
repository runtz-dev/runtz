"use client"

import { Input } from "@/components/ui/input"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"

export function validAWSAccessKeys(accessKeyId: string, secretAccessKey: string) {
  return /^AKIA[A-Z0-9]{16}$/.test(accessKeyId) && /^[A-Za-z0-9/+=]{40}$/.test(secretAccessKey)
}

export function AWSAccessKeyFields({ id, accessKeyId, secretAccessKey, onAccessKeyIdChange, onSecretAccessKeyChange }: {
  id: string
  accessKeyId: string
  secretAccessKey: string
  onAccessKeyIdChange: (value: string) => void
  onSecretAccessKeyChange: (value: string) => void
}) {
  return <>
    <Field>
      <FieldLabel htmlFor={`${id}-key-id`}>Access key ID</FieldLabel>
      <Input id={`${id}-key-id`} type="password" autoComplete="new-password" spellCheck={false} maxLength={20} value={accessKeyId} onChange={e => onAccessKeyIdChange(e.target.value.trim())} />
      <FieldDescription>Use a dedicated IAM user with the Runtz read-only policy. Root access keys are not accepted.</FieldDescription>
    </Field>
    <Field>
      <FieldLabel htmlFor={`${id}-key-secret`}>Secret access key</FieldLabel>
      <Input id={`${id}-key-secret`} type="password" autoComplete="new-password" spellCheck={false} maxLength={40} value={secretAccessKey} onChange={e => onSecretAccessKeyChange(e.target.value.trim())} />
      <FieldDescription>Stored encrypted and never displayed again. Replace this key in Settings when you rotate it in AWS.</FieldDescription>
    </Field>
  </>
}
