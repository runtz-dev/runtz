"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeftIcon, RadarIcon } from "lucide-react"
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
import { useScanDetail } from "@/components/runtz/use-scan-detail"
import { useSCAScans } from "@/components/runtz/use-sca-scans"
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
import { filterScansByProject, formatDate } from "@/lib/sca"
import { filterScanSummary } from "@/lib/dashboard"

export default function SCAProjectPage() {
  const { basePath } = usePlatform()
  const { filter } = useVulnerabilityFilter()
  const params = useParams<{ projectName: string }>()
  const projectName = React.useMemo(
    () => decodeProjectName(params.projectName),
    [params.projectName]
  )
  const { scans, loading, error } = useSCAScans()
  const projectScans = React.useMemo(
    () => filterScansByProject(scans, projectName),
    [projectName, scans]
  )
  const latestScan = projectScans[0]
  const latestSummary = React.useMemo(
    () =>
      latestScan ? filterScanSummary(latestScan.summary, filter) : undefined,
    [filter, latestScan]
  )
  const scanDetail = useScanDetail("sca", latestScan)

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-3">
        <div>
          <Button
            variant="outline"
            size="sm"
            render={<Link href={`${basePath}/sca`} />}
            nativeButton={false}
          >
            <ArrowLeftIcon data-icon="inline-start" />
            Apps SCA
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Badge variant="outline">CODE</Badge>
            <Badge>SCA</Badge>
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">
              {projectName}
            </h1>
            <p className="text-sm text-muted-foreground">
              Vulnerability results from this app&apos;s latest SCA scan.
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
              <RadarIcon />
            </EmptyMedia>
            <EmptyTitle>App not found</EmptyTitle>
            <EmptyDescription>
              No SCA scans for this app in the selected workspace.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <DashboardSummaryGrid summary={latestSummary}>
            <MetricCard
              title="Scans"
              value={projectScans.length}
              description="runs for this app"
            />
            <MetricCard
              title="Dependencies"
              value={latestSummary.totalDependencies}
              description="in the latest scan"
            />
            <MetricCard
              title="CVEs/GHSAs"
              value={latestSummary.vulnerabilities}
              description="in the latest scan"
            />
            <MetricCard
              title="Critical/High"
              value={latestSummary.critical + latestSummary.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={projectScans} filter={filter} />

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
                    targetKind="application"
                    targetName={projectName}
                  />
                ) : null}
              </CardContent>
            </Card>

            <div className="flex flex-col gap-6">
              <LatestScansCard
                scans={projectScans}
                description="Recent history for this app."
                supportsFixFilter
              />
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function decodeProjectName(value: string) {
  try {
    return decodeURIComponent(value)
  } catch {
    return value
  }
}
