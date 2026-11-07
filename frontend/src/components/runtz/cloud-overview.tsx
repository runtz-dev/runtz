"use client"

import Link from "next/link"
import { ArrowRightIcon, CloudIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { cloudProviderNames, type CloudProvider } from "@/lib/cloud"

export function CloudOverviewCard() {
  const { connections, loading, error } = useCloud()
  if (!connections.length && !error) return null
  const totals = connections.reduce((sum, c) => ({ total: sum.total + c.summary.total, urgent: sum.urgent + c.summary.critical + c.summary.high }), { total: 0, urgent: 0 })
  const providers: CloudProvider[] = ["gcp", "azure", "aws"]
  return <Card><CardHeader><div className="flex flex-wrap items-center justify-between gap-3"><CardTitle className="flex items-center gap-2"><CloudIcon className="size-4 text-primary" />Cloud security</CardTitle><div className="flex flex-wrap gap-1">{providers.filter(provider => connections.some(c => c.provider === provider)).map(provider => <Button key={provider} variant="ghost" size="sm" render={<Link href={`/cloud/${provider}`} />}>{cloudProviderNames[provider]}<ArrowRightIcon /></Button>)}</div></div><CardDescription>Findings from Runtz configuration checks across connected accounts, projects and subscriptions.</CardDescription></CardHeader><CardContent>{error ? <p className="text-sm text-destructive">Cloud findings are unavailable. Retry loading your cloud connections.</p> : <div className="flex flex-wrap gap-x-8 gap-y-3 text-sm"><span><strong className="mr-2 tabular-nums">{connections.length}</strong><span className="text-muted-foreground">cloud connections</span></span><span><strong className="mr-2 tabular-nums">{totals.total}</strong><span className="text-muted-foreground">active findings</span></span><span><strong className="mr-2 text-severity-high tabular-nums">{totals.urgent}</strong><span className="text-muted-foreground">critical / high</span></span>{loading && <span className="text-xs text-muted-foreground">Refreshing…</span>}</div>}</CardContent></Card>
}
