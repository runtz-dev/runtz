"use client"

import Link from "next/link"
import { ArrowRightIcon } from "lucide-react"
import type { LucideIcon } from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  MetricCard,
  SeverityBadge,
  SeverityDistribution,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { usePlatform } from "@/components/runtz/platform-context"
import { useFindingScans } from "@/components/runtz/use-finding-scans"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { aggregateSummaries } from "@/lib/dashboard"
import {
  groupFindingsScans,
  inspectedLabel,
  summarizeFindingsTargets,
} from "@/lib/finding-scans"
import { formatDate } from "@/lib/sca"
import type { Finding, FindingsScan } from "@/lib/api"

type FindingsScanPageProps = {
  scanType: "sast" | "k8s"
  section: "CODE" | "HOSTS"
  badge: string
  title: string
  description: string
  targetTitle: string
  targetDescription: string
  targetColumnTitle?: string
  targetListDescription?: string
  inspectedTitle: string
  emptyTitle: string
  emptyDescription: string
  commandLabel: string
  icon: LucideIcon
}

export function FindingsScanPage({
  scanType,
  section,
  badge,
  title,
  description,
  targetTitle,
  targetDescription,
  targetColumnTitle = "Target",
  targetListDescription,
  inspectedTitle,
  emptyTitle,
  emptyDescription,
  commandLabel,
  icon: Icon,
}: FindingsScanPageProps) {
  const { basePath } = usePlatform()
  const { scans, loading, error } = useFindingScans(scanType)
  const targets = React.useMemo(() => groupFindingsScans(scans), [scans])
  const totals = React.useMemo(() => summarizeFindingsTargets(targets), [targets])
  const severitySummary = React.useMemo(
    () => aggregateSummaries(targets.map((target) => target.latestScan.summary)),
    [targets]
  )
  const unit = inspectedLabel(scanType)

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Badge variant="outline">{section}</Badge>
          <Badge>{badge}</Badge>
        </div>
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-normal">
            <Icon className="size-5 text-primary" />
            {title}
          </h1>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
      </div>

      {error ? (
        <Card>
          <CardHeader>
            <CardTitle>Error</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {loading ? (
        <div className="grid gap-4 md:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-28" />
          ))}
        </div>
      ) : (
        <>
          <DashboardSummaryGrid summary={severitySummary}>
            <MetricCard
              title={targetTitle}
              value={totals.targets}
              description={targetDescription}
            />
            <MetricCard
              title="Scans"
              value={totals.scans}
              description="runs received"
            />
            <MetricCard
              title="Findings"
              value={totals.findings}
              description="across the latest scans per target"
            />
            <MetricCard
              title="Critical/High"
              value={totals.critical + totals.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={scans} />

          {targets.length === 0 ? (
            <Empty className="min-h-80 border">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <Icon />
                </EmptyMedia>
                <EmptyTitle>{emptyTitle}</EmptyTitle>
                <EmptyDescription>{emptyDescription}</EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{targetTitle}</CardTitle>
                <CardDescription>
                  {targetListDescription ??
                    `Latest reports received from ${commandLabel}.`}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{targetColumnTitle}</TableHead>
                      <TableHead>Findings</TableHead>
                      <TableHead>{inspectedTitle}</TableHead>
                      <TableHead>Scans</TableHead>
                      <TableHead>Latest scan</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {targets.map((target) => (
                      <TableRow key={target.targetName}>
                        <TableCell className="min-w-64">
                          <div className="flex min-w-0 flex-col gap-1">
                            <Link
                              className="inline-flex items-center gap-2 font-medium text-primary underline-offset-4 hover:underline"
                              href={`${basePath}/${scanType}/${encodeURIComponent(
                                target.targetName
                              )}`}
                            >
                              <span className="truncate">{target.targetName}</span>
                              <ArrowRightIcon />
                            </Link>
                            <span className="truncate text-xs text-muted-foreground">
                              {target.workspaceNames.join(", ")}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="min-w-72">
                          <div className="flex items-center gap-4">
                            <Badge
                              variant={
                                target.latestScan.summary.vulnerabilities > 0
                                  ? "secondary"
                                  : "outline"
                              }
                            >
                              {target.latestScan.summary.vulnerabilities} findings
                            </Badge>
                            <SeverityDistribution
                              summary={target.latestScan.summary}
                              className="min-w-56"
                            />
                          </div>
                        </TableCell>
                        <TableCell>
                          {target.latestScan.summary.totalDependencies} {unit}
                        </TableCell>
                        <TableCell>{target.scans.length}</TableCell>
                        <TableCell>{formatDate(target.latestScan.createdAt)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  )
}

export function FindingsTable({
  scans,
  findings,
  title = "Latest findings",
  description,
}: {
  scans: FindingsScan[]
  findings: Array<{ scan: FindingsScan; finding: Finding }>
  title?: string
  description?: string
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ??
            `${scans.length} scans received, showing the latest findings.`}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Severity</TableHead>
              <TableHead>Finding</TableHead>
              <TableHead>Location</TableHead>
              <TableHead>Remediation</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {findings.map(({ scan, finding }, index) => (
              <TableRow
                key={`${scan.id}-${finding.id}-${finding.file ?? ""}-${
                  finding.line ?? 0
                }-${finding.resourceName ?? ""}-${index}`}
              >
                <TableCell>
                  <SeverityBadge severity={finding.severity} />
                </TableCell>
                <TableCell className="min-w-72">
                  <div className="flex flex-col gap-1">
                    <span className="font-medium">{finding.title}</span>
                    <span className="text-xs text-muted-foreground">
                      {finding.id}
                      {finding.category ? ` · ${finding.category}` : ""}
                    </span>
                    {finding.description ? (
                      <span className="text-xs text-muted-foreground">
                        {finding.description}
                      </span>
                    ) : null}
                  </div>
                </TableCell>
                <TableCell className="min-w-64">
                  <span className="text-sm">{findingLocation(finding)}</span>
                </TableCell>
                <TableCell className="min-w-72 text-sm text-muted-foreground">
                  {finding.remediation || "-"}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function findingLocation(finding: Finding) {
  if (finding.resourceKind || finding.resourceName) {
    return [
      finding.namespace,
      finding.resourceKind,
      finding.resourceName,
      finding.file,
    ]
      .filter(Boolean)
      .join(" / ")
  }

  if (finding.file && finding.line) {
    return `${finding.file}:${finding.line}`
  }

  return finding.file || "-"
}
