"use client"

import {
  CheckIcon,
  CopyIcon,
  ShieldCheckIcon,
  TerminalIcon,
} from "lucide-react"
import type { LucideIcon } from "lucide-react"
import * as React from "react"

import { Button } from "@/components/ui/button"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"

export function FirstScanEmptyState({
  title,
  description,
  command,
  icon: Icon,
}: {
  title: string
  description: string
  command: string
  icon: LucideIcon
}) {
  const [copied, setCopied] = React.useState(false)
  const resetTimeout = React.useRef<ReturnType<typeof setTimeout>>(undefined)

  React.useEffect(
    () => () => {
      if (resetTimeout.current) {
        clearTimeout(resetTimeout.current)
      }
    },
    []
  )

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(command)
      setCopied(true)
      if (resetTimeout.current) {
        clearTimeout(resetTimeout.current)
      }
      resetTimeout.current = setTimeout(() => setCopied(false), 2000)
    } catch {
      setCopied(false)
    }
  }

  return (
    <Empty className="min-h-80 border bg-card/35">
      <EmptyHeader className="max-w-md">
        <EmptyMedia variant="icon">
          <Icon />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      <EmptyContent className="max-w-lg">
        <div className="flex w-full flex-col gap-2 text-left">
          <span className="text-xs font-medium text-foreground">
            Run your first scan
          </span>
          <div className="flex w-full min-w-0 items-center gap-2 rounded-lg border bg-background/70 p-1.5 pl-3 shadow-sm">
            <TerminalIcon className="size-4 shrink-0 text-primary" />
            <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap py-1 font-mono text-xs text-foreground sm:text-sm">
              {command}
            </code>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={copyCommand}
              aria-label={copied ? "Command copied" : "Copy command"}
            >
              {copied ? (
                <CheckIcon data-icon="inline-start" />
              ) : (
                <CopyIcon data-icon="inline-start" />
              )}
              <span aria-live="polite">{copied ? "Copied" : "Copy"}</span>
            </Button>
          </div>
        </div>
      </EmptyContent>
    </Empty>
  )
}

export function CleanScanState({
  title = "Great news — no vulnerabilities detected",
}: {
  title?: string
}) {
  return (
    <Empty className="min-h-48 border bg-[#34c68a]/[0.04]">
      <EmptyHeader className="max-w-md">
        <EmptyMedia
          variant="icon"
          className="bg-[#34c68a]/10 text-[#198754] dark:text-[#80d673]"
        >
          <ShieldCheckIcon />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>
          The latest scan found no security issues in its scope. Keep scanning
          regularly to catch new risks as your code, dependencies, and
          environments evolve.
        </EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}
