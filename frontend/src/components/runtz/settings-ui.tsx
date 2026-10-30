"use client"

import * as React from "react"
import { AlertCircleIcon, CheckCircle2Icon, LoaderCircleIcon, SearchIcon } from "lucide-react"

import { Card, CardContent, CardFooter, CardHeader } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

type SettingsIcon = React.ComponentType<React.SVGProps<SVGSVGElement>>

export function SettingsSection({
  title,
  description,
  action,
  footer,
  children,
  className,
}: {
  title: string
  description: string
  action?: React.ReactNode
  footer?: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  const titleId = React.useId()

  return (
    <section aria-labelledby={titleId} className={cn("min-w-0", className)}>
      <Card className="gap-0 py-0">
        <CardHeader className="flex flex-col gap-4 border-b p-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <h2 id={titleId} className="text-base font-medium leading-snug">{title}</h2>
            <p className="max-w-xl text-sm text-muted-foreground">{description}</p>
          </div>
          {action ? <div className="shrink-0 self-start">{action}</div> : null}
        </CardHeader>
        <CardContent className="p-4">{children}</CardContent>
        {footer ? <CardFooter className="flex flex-wrap justify-between gap-3">{footer}</CardFooter> : null}
      </Card>
    </section>
  )
}

export function SettingsFeedback({
  children,
  error = false,
}: {
  children: React.ReactNode
  error?: boolean
}) {
  const Icon = error ? AlertCircleIcon : CheckCircle2Icon
  return (
    <div
      role={error ? "alert" : "status"}
      className={cn("flex items-start gap-2 rounded-lg border bg-muted/30 p-3 text-sm leading-6", error ? "border-destructive/25 text-destructive" : "text-foreground")}
    >
      <Icon aria-hidden="true" className={cn("mt-1 size-4 shrink-0", !error && "text-primary")} />
      <span>{children}</span>
    </div>
  )
}

export function SettingsEmpty({
  icon: Icon,
  title,
  description,
}: {
  icon: SettingsIcon
  title: string
  description: string
}) {
  return (
    <div className="flex flex-col items-start gap-2 rounded-lg border border-dashed p-4 text-left">
      <Icon aria-hidden="true" className="mb-1 size-5 text-muted-foreground" />
      <p className="font-medium">{title}</p>
      <p className="max-w-sm text-sm leading-6 text-muted-foreground">{description}</p>
    </div>
  )
}

export function SettingsSearch({ value, onChange, label }: {
  value: string
  onChange: (value: string) => void
  label: string
}) {
  return (
    <div className="relative mb-4 w-full sm:max-w-xs">
      <SearchIcon aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input type="search" aria-label={label} placeholder={label} value={value} onChange={(event) => onChange(event.target.value)} className="pl-9" />
    </div>
  )
}

export function SettingsPending({ children }: { children: React.ReactNode }) {
  return <><LoaderCircleIcon aria-hidden="true" className="size-4 animate-spin motion-reduce:animate-none" />{children}</>
}
