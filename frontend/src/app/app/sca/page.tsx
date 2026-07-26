"use client"

import Link from "next/link"
import { ArrowRightIcon, RadarIcon } from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  MetricCard,
  SeverityDistribution,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { usePlatform } from "@/components/runtz/platform-context"
import { useSCAScans } from "@/components/runtz/use-sca-scans"
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
import { formatDate, groupScansByProject, summarizeProjects } from "@/lib/sca"
import { aggregateSummaries } from "@/lib/dashboard"

export default function SCAPage() {
  const { basePath } = usePlatform()
  const { scans, loading, error } = useSCAScans()
  const apps = React.useMemo(() => groupScansByProject(scans), [scans])
  const totals = React.useMemo(() => summarizeProjects(apps), [apps])
  const severitySummary = React.useMemo(
    () => aggregateSummaries(apps.map((app) => app.latestScan.summary)),
    [apps]
  )

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Badge variant="outline">CODE</Badge>
          <Badge>SCA</Badge>
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">SCA</h1>
          <p className="text-sm text-muted-foreground">
            Overview of scanned apps and their npm vulnerabilities.
          </p>
        </div>
      </div>

      {error ? (
        <Card>
          <CardHeader>
            <CardTitle>Erro</CardTitle>
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
              title="Apps scanneadas"
              value={totals.apps}
              description="projetos diferentes"
            />
            <MetricCard
              title="Scans"
              value={totals.scans}
              description="runs received"
            />
            <MetricCard
              title="CVEs/GHSAs"
              value={totals.vulnerabilities}
              description="across the latest scans per app"
            />
            <MetricCard
              title="Critical/High"
              value={totals.critical + totals.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={scans} />

          {apps.length === 0 ? (
            <Empty className="min-h-80 border">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <RadarIcon />
                </EmptyMedia>
                <EmptyTitle>No apps scanned</EmptyTitle>
                <EmptyDescription>
                  Execute o CLI apontando para o backend local para preencher este
                  painel.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Apps</CardTitle>
                <CardDescription>
                  Click an app name to see the CVE list from its latest scan.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>App</TableHead>
                      <TableHead>Vulnerabilidades</TableHead>
                      <TableHead>Dependencies</TableHead>
                      <TableHead>Scans</TableHead>
                      <TableHead>Último scan</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {apps.map((app) => (
                      <TableRow key={app.projectName}>
                        <TableCell className="min-w-64">
                          <div className="flex min-w-0 flex-col gap-1">
                            <Link
                              className="inline-flex items-center gap-2 font-medium text-primary underline-offset-4 hover:underline"
                              href={`${basePath}/sca/${encodeURIComponent(
                                app.projectName
                              )}`}
                            >
                              <span className="truncate">{app.projectName}</span>
                              <ArrowRightIcon />
                            </Link>
                            <span className="truncate text-xs text-muted-foreground">
                              {app.workspaceNames.join(", ")}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="min-w-72">
                          <div className="flex items-center gap-4">
                            <Badge
                              variant={
                                app.latestScan.summary.vulnerabilities > 0
                                  ? "secondary"
                                  : "outline"
                              }
                            >
                              {app.latestScan.summary.vulnerabilities} vulns
                            </Badge>
                            <SeverityDistribution
                              summary={app.latestScan.summary}
                              className="min-w-56"
                            />
                          </div>
                        </TableCell>
                        <TableCell>
                          {app.latestScan.summary.totalDependencies}
                        </TableCell>
                        <TableCell>{app.scans.length}</TableCell>
                        <TableCell>{formatDate(app.latestScan.createdAt)}</TableCell>
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
