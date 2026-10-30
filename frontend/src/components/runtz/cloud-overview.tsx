"use client"

import Link from "next/link"
import { ArrowRightIcon, CloudIcon } from "lucide-react"
import { useCloud } from "@/components/runtz/cloud-context"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

export function CloudOverviewCard() {
  const { connections, loading, error } = useCloud()
  if (!connections.length && !error) return null
  const totals = connections.reduce((sum, c) => ({ total: sum.total + c.summary.total, urgent: sum.urgent + c.summary.critical + c.summary.high }), { total: 0, urgent: 0 })
  return <Card><CardHeader><div className="flex flex-wrap items-center justify-between gap-3"><CardTitle className="flex items-center gap-2"><CloudIcon className="size-4 text-primary" />Cloud security</CardTitle><Button variant="ghost" size="sm" render={<Link href="/cloud/gcp" />}>Google Cloud<ArrowRightIcon /></Button></div><CardDescription>Native Security Command Center findings from your connected GCP projects.</CardDescription></CardHeader><CardContent>{error ? <p className="text-sm text-destructive">Cloud findings are unavailable. Open Google Cloud to retry.</p> : <div className="flex flex-wrap gap-x-8 gap-y-3 text-sm"><span><strong className="mr-2 tabular-nums">{connections.length}</strong><span className="text-muted-foreground">projects connected</span></span><span><strong className="mr-2 tabular-nums">{totals.total}</strong><span className="text-muted-foreground">active findings</span></span><span><strong className="mr-2 text-severity-high tabular-nums">{totals.urgent}</strong><span className="text-muted-foreground">critical / high</span></span>{loading && <span className="text-xs text-muted-foreground">Refreshing…</span>}</div>}</CardContent></Card>
}
