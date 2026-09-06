"use client"

import * as React from "react"
import { UserPlusIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { apiRequest, type Workspace } from "@/lib/api"

export function WorkspaceSharingDialog({
  workspace,
  onAdded,
  disabled = false,
}: {
  workspace: Workspace
  onAdded: (email: string) => void
  disabled?: boolean
}) {
  const [open, setOpen] = React.useState(false)
  const [email, setEmail] = React.useState("")
  const [error, setError] = React.useState("")
  const [pending, setPending] = React.useState(false)

  function handleOpenChange(nextOpen: boolean) {
    if (pending) return
    setOpen(nextOpen)
    setEmail("")
    setError("")
  }

  async function shareWorkspace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (pending) return
    setPending(true)
    setError("")
    try {
      const address = email.trim().toLowerCase()
      await apiRequest(`/api/v1/workspaces/${workspace.id}/members`, {
        method: "POST",
        body: { email: address },
      })
      setOpen(false)
      setEmail("")
      onAdded(address)
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to add member")
    } finally {
      setPending(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button disabled={disabled} />}>
        <UserPlusIcon data-icon="inline-start" />
        Add member
      </DialogTrigger>
      <DialogContent className="sm:max-w-md" showCloseButton={!pending}>
        <DialogHeader>
          <DialogTitle>Add member</DialogTitle>
          <DialogDescription className="break-words">
            Give someone access to <strong>{workspace.name}</strong>.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={shareWorkspace} className="flex flex-col gap-5">
          <FieldGroup>
            <Field data-invalid={Boolean(error)}>
              <FieldLabel htmlFor={`share-email-${workspace.id}`}>Email address</FieldLabel>
              <Input
                id={`share-email-${workspace.id}`}
                type="email"
                autoComplete="email"
                placeholder="colleague@company.com"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
                disabled={pending}
                aria-invalid={Boolean(error)}
                aria-describedby={`share-help-${workspace.id}${error ? ` share-error-${workspace.id}` : ""}`}
              />
              <FieldDescription id={`share-help-${workspace.id}`}>
                Use an existing Runtz account. Access is granted immediately.
              </FieldDescription>
              {error ? <FieldError id={`share-error-${workspace.id}`} role="alert">{error}</FieldError> : null}
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" disabled={pending} onClick={() => handleOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={pending || !email.trim()}>{pending ? "Adding…" : "Add member"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
