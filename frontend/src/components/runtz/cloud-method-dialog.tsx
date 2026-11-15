"use client"

import { ArrowLeftIcon, ArrowRightIcon, Clock3Icon, MonitorIcon, ShieldCheckIcon, TerminalIcon } from "lucide-react"
import { CloudProviderIcon } from "@/components/runtz/cloud-icons"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { cloudProviderNames, type CloudProvider } from "@/lib/cloud"

export function CloudMethodDialog({ provider, onSelect, onBack, onClose }: { provider: CloudProvider; onSelect: (method: "site" | "cli") => void; onBack: () => void; onClose: () => void }) {
  return <Dialog open onOpenChange={open => { if (!open) onClose() }}><DialogContent className="max-h-[90dvh] gap-0 overflow-y-auto p-0 sm:max-w-3xl">
    <DialogHeader className="border-b px-6 py-6 pr-12 sm:px-8"><div className="mb-4 flex items-center gap-2 text-xs text-muted-foreground"><CloudProviderIcon provider={provider} className="size-5" />{cloudProviderNames[provider]}</div><DialogTitle className="text-xl">How would you like to scan?</DialogTitle><DialogDescription>Choose where Runtz runs your cloud checks. Both paths bring findings and coverage into the same workspace.</DialogDescription></DialogHeader>
    <div className="grid gap-4 p-6 sm:grid-cols-2 sm:p-8">{([
      { id: "site", icon: MonitorIcon, title: "Connect through the site", body: "Follow the guided setup and let Runtz scan your cloud automatically.", detail: "Automatic scans every 12 hours", detailIcon: Clock3Icon },
      { id: "cli", icon: TerminalIcon, title: "Scan with Runtz CLI", body: "Start a cloud scan from your terminal or CI pipeline. Send results to your workspace.", detail: "Private checks run on Runtz Security", detailIcon: ShieldCheckIcon },
    ] as const).map(option => <button key={option.id} onClick={() => onSelect(option.id)} className="group flex min-w-0 flex-col items-start rounded-xl border bg-card p-5 text-left transition-colors hover:border-primary/60 hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"><span className="mb-5 flex size-11 items-center justify-center rounded-lg border bg-background text-primary"><option.icon className="size-5" /></span><span className="font-medium">{option.title}</span><span className="mt-2 text-sm leading-6 text-muted-foreground">{option.body}</span><span className="mt-6 flex items-start gap-2 text-xs text-muted-foreground"><option.detailIcon className="size-4 shrink-0" />{option.detail}</span><span className="mt-6 inline-flex items-center gap-2 text-sm font-medium text-primary">Continue<ArrowRightIcon className="size-4 transition-transform group-hover:translate-x-1" /></span></button>)}</div>
    <div className="border-t px-6 py-4 sm:px-8"><Button variant="ghost" onClick={onBack}><ArrowLeftIcon />Choose another cloud</Button></div>
  </DialogContent></Dialog>
}
