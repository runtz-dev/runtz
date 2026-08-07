"use client"

import Link from "next/link"
import { ArrowRightIcon, ServerIcon } from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  MetricCard,
  SeverityDistribution,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { FirstScanEmptyState } from "@/components/runtz/scan-empty-state"
import { usePlatform } from "@/components/runtz/platform-context"
import { usePackageScans } from "@/components/runtz/use-package-scans"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  getHostTargetName,
  groupPackageScans,
  summarizePackageTargets,
} from "@/lib/package-scans"
import { formatDate } from "@/lib/sca"
import { aggregateSummaries } from "@/lib/dashboard"

export default function HostsPage() {
  const { basePath } = usePlatform()
  const { scans, loading, error } = usePackageScans("host")
  const hosts = React.useMemo(
    () => groupPackageScans(scans, getHostTargetName),
    [scans]
  )
  const totals = React.useMemo(() => summarizePackageTargets(hosts), [hosts])
  const severitySummary = React.useMemo(
    () => aggregateSummaries(hosts.map((host) => host.latestScan.summary)),
    [hosts]
  )

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Badge variant="outline">HOSTS</Badge>
          <Badge>Host scanning</Badge>
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">
            Host scanning
          </h1>
          <p className="text-sm text-muted-foreground">
            Hosts scanned by installed-package inventory.
          </p>
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
              title="Hosts"
              value={totals.targets}
              description="distinct hostnames"
            />
            <MetricCard
              title="Scans"
              value={totals.scans}
              description="runs received"
            />
            <MetricCard
              title="CVEs"
              value={totals.vulnerabilities}
              description="across the latest scans per host"
            />
            <MetricCard
              title="Critical/High"
              value={totals.critical + totals.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={scans} />

          {hosts.length === 0 ? (
            <FirstScanEmptyState
              title="No hosts scanned"
              description="Scan this host to start tracking vulnerable operating-system packages."
              command="runtz host"
              icon={ServerIcon}
            />
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Hosts</CardTitle>
                <CardDescription>
                  Click a hostname to see the CVE list from its latest scan.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Hostname</TableHead>
                      <TableHead>Vulnerabilities</TableHead>
                      <TableHead>Packages</TableHead>
                      <TableHead>OS</TableHead>
                      <TableHead>Scans</TableHead>
                      <TableHead>Latest scan</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {hosts.map((host) => (
                      <TableRow key={host.targetName}>
                        <TableCell className="min-w-64">
                          <div className="flex min-w-0 flex-col gap-1">
                            <Link
                              className="inline-flex items-center gap-2 font-medium text-primary underline-offset-4 hover:underline"
                              href={`${basePath}/hosts/${encodeURIComponent(
                                host.targetName
                              )}`}
                            >
                              <span className="truncate">{host.targetName}</span>
                              <ArrowRightIcon />
                            </Link>
                            <span className="truncate text-xs text-muted-foreground">
                              {host.workspaceNames.join(", ")}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="min-w-72">
                          <div className="flex items-center gap-4">
                            <Badge
                              variant={
                                host.latestScan.summary.vulnerabilities > 0
                                  ? "secondary"
                                  : "outline"
                              }
                            >
                              {host.latestScan.summary.vulnerabilities} vulns
                            </Badge>
                            <SeverityDistribution
                              summary={host.latestScan.summary}
                              className="min-w-56"
                            />
                          </div>
                        </TableCell>
                        <TableCell>
                          {host.latestScan.summary.totalDependencies}
                        </TableCell>
                        <TableCell>
                          {host.latestScan.osName || host.latestScan.osId || "-"}
                        </TableCell>
                        <TableCell>{host.scans.length}</TableCell>
                        <TableCell>
                          {formatDate(host.latestScan.createdAt)}
                        </TableCell>
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
