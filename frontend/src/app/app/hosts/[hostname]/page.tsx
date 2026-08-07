"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeftIcon, ServerIcon } from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  LatestScansCard,
  MetricCard,
  ScanDetailError,
  ScanDetailSkeleton,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { PackageVulnerabilityExplorer } from "@/components/runtz/package-vulnerability-explorer"
import { usePlatform } from "@/components/runtz/platform-context"
import { usePackageScans } from "@/components/runtz/use-package-scans"
import { useScanDetail } from "@/components/runtz/use-scan-detail"
import {
  CVEFixFilterSwitches,
  useVulnerabilityFilter,
} from "@/components/runtz/vulnerability-filter"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
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
  filterPackageScansByTarget,
  getHostTargetName,
} from "@/lib/package-scans"
import { formatDate } from "@/lib/sca"
import { filterScanSummary } from "@/lib/dashboard"

export default function HostDetailPage() {
  const { basePath } = usePlatform()
  const { filter } = useVulnerabilityFilter()
  const params = useParams<{ hostname: string }>()
  const hostname = React.useMemo(
    () => decodeParam(params.hostname),
    [params.hostname]
  )
  const { scans, loading, error } = usePackageScans("host")
  const hostScans = React.useMemo(
    () => filterPackageScansByTarget(scans, hostname, getHostTargetName),
    [hostname, scans]
  )
  const latestScan = hostScans[0]
  const latestSummary = React.useMemo(
    () =>
      latestScan ? filterScanSummary(latestScan.summary, filter) : undefined,
    [filter, latestScan]
  )
  const scanDetail = useScanDetail("host", latestScan)

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-3">
        <div>
          <Button
            variant="outline"
            size="sm"
            render={<Link href={`${basePath}/hosts`} />}
            nativeButton={false}
          >
            <ArrowLeftIcon data-icon="inline-start" />
            Hosts
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Badge variant="outline">HOSTS</Badge>
            <Badge>Host scanning</Badge>
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">
              {hostname}
            </h1>
            <p className="text-sm text-muted-foreground">
              Vulnerability results from the host&apos;s latest package scan.
            </p>
          </div>
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
      ) : !latestScan || !latestSummary ? (
        <Empty className="min-h-80 border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <ServerIcon />
            </EmptyMedia>
            <EmptyTitle>Host not found</EmptyTitle>
            <EmptyDescription>
              No scans for this hostname in the selected workspace.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <DashboardSummaryGrid summary={latestSummary}>
            <MetricCard
              title="Scans"
              value={hostScans.length}
              description="runs for this host"
            />
            <MetricCard
              title="Packages"
              value={latestSummary.totalDependencies}
              description="in the latest scan"
            />
            <MetricCard
              title="CVEs"
              value={latestSummary.vulnerabilities}
              description="in the latest scan"
            />
            <MetricCard
              title="Critical/High"
              value={latestSummary.critical + latestSummary.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={hostScans} filter={filter} />

          <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
            <Card>
              <CardHeader className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
                <div>
                  <CardTitle>Vulnerability results</CardTitle>
                  <CardDescription>
                    Latest scan on {formatDate(latestScan.createdAt)}
                  </CardDescription>
                </div>
                <CVEFixFilterSwitches />
              </CardHeader>
              <CardContent>
                {scanDetail.loading ? (
                  <ScanDetailSkeleton />
                ) : scanDetail.error ? (
                  <ScanDetailError message={scanDetail.error} />
                ) : scanDetail.scan ? (
                  <PackageVulnerabilityExplorer
                    vulnerabilities={scanDetail.scan.vulnerabilities ?? []}
                    targetKind="host"
                    targetName={hostname}
                    osName={latestScan.osName}
                    osVersion={latestScan.osVersion}
                    packageManager={latestScan.packageManager}
                  />
                ) : null}
              </CardContent>
            </Card>

            <div className="flex flex-col gap-6">
              <Card>
                <CardHeader>
                  <CardTitle>Host</CardTitle>
                  <CardDescription>
                    {latestScan.osName || latestScan.osId || "-"}
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-2 text-sm">
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">OS version</span>
                    <span>{latestScan.osVersion || "-"}</span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Package manager</span>
                    <span>{latestScan.packageManager || "-"}</span>
                  </div>
                </CardContent>
              </Card>

              <LatestScansCard
                scans={hostScans}
                description="Recent history for this host."
                getTitle={(scan) => scan.targetName || scan.projectName || hostname}
                packageLabel="packages"
                supportsFixFilter
              />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function decodeParam(value: string) {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}
