"use client"

import * as React from "react"
import { CheckIcon, ChevronDownIcon, CopyIcon, TerminalIcon } from "lucide-react"
import { Button } from "@/components/ui/button"

export function CloudCommand({ value, label = "Show gcloud commands" }: { value: string; label?: string }) {
  const [expanded, setExpanded] = React.useState(false)
  const [copied, setCopied] = React.useState(false)
  const [failed, setFailed] = React.useState(false)
  const id = React.useId()

  React.useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 2500)
    return () => clearTimeout(timer)
  }, [copied])

  async function copy() {
    setExpanded(true)
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true); setFailed(false)
    } catch { setFailed(true) }
  }

  return <div className="min-w-0 rounded-lg border bg-background">
    <div className="flex min-w-0 items-center gap-2 p-2">
      <Button variant="ghost" className="h-auto min-w-0 flex-1 justify-start whitespace-normal text-left" aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(!expanded)}>
        <TerminalIcon className="shrink-0" /><span>{label}</span><ChevronDownIcon className={`ml-auto shrink-0 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </Button>
      <Button variant="outline" size="sm" onClick={() => void copy()} aria-label="Copy and show gcloud commands">{copied ? <CheckIcon /> : <CopyIcon />}{copied ? "Copied" : "Copy"}</Button>
    </div>
    <div id={id} hidden={!expanded} className="min-w-0 border-t p-3">
      <pre className="whitespace-pre-wrap break-all text-xs leading-relaxed"><code>{value}</code></pre>
      <span role="status" className="text-xs text-muted-foreground">{failed ? "Select and copy the commands manually." : copied ? "Commands copied to clipboard." : ""}</span>
    </div>
  </div>
}
