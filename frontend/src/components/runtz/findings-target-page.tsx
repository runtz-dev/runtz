"use client"

import Link from "next/link"
import { useParams, useSearchParams } from "next/navigation"
import { ArrowLeftIcon } from "lucide-react"
import type { LucideIcon } from "lucide-react"
import * as React from "react"

import { FindingsTable } from "@/components/runtz/findings-scan-page"
import { ScanTypeFilter } from "@/components/runtz/scan-type-filter"
import {
  DashboardSummaryGrid,
  LatestScansCard,
  MetricCard,
  ScanDetailError,
  ScanDetailSkeleton,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { usePlatform } from "@/components/runtz/platform-context"
import { useFindingScans } from "@/components/runtz/use-finding-scans"
import { useScanDetail } from "@/components/runtz/use-scan-detail"
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
  filterFindingsScansByTarget,
  inspectedLabel,
} from "@/lib/finding-scans"
import { formatDate } from "@/lib/sca"

type FindingsTargetPageProps = {
  scanType: "sast" | "k8s"
  section: "CODE" | "HOSTS"
  badge: string
  backLabel: string
  targetTitle: string
  targetScope: string
  description: string
  inspectedTitle: string
  emptyTitle: string
  emptyDescription: string
  icon: LucideIcon
}

export function FindingsTargetPage({
  scanType,
  section,
  badge,
  backLabel,
  targetTitle,
  targetScope,
  description,
  inspectedTitle,
  emptyTitle,
  emptyDescription,
  icon: Icon,
}: FindingsTargetPageProps) {
  const { basePath } = usePlatform()
  const params = useParams<{ targetName: string }>()
  const searchParams = useSearchParams()
  const requestedScanId = searchParams.get("scanId")
  const targetName = React.useMemo(
    () => decodeParam(params.targetName),
    [params.targetName]
  )
  const { scans, loading, error } = useFindingScans(scanType)
  const targetScans = React.useMemo(
    () => filterFindingsScansByTarget(scans, targetName),
    [scans, targetName]
  )
  const selectedScan = React.useMemo(
    () =>
      targetScans.find((scan) => scan.id === requestedScanId) ?? targetScans[0],
    [requestedScanId, targetScans]
  )
  const selectedSummary = selectedScan?.summary
  const scanDetail = useScanDetail(scanType, selectedScan)
  const selectedFindings = React.useMemo(
    () =>
      scanDetail.scan
        ? (scanDetail.scan.findings ?? []).map((finding) => ({
            scan: scanDetail.scan!,
            finding,
          }))
        : [],
    [scanDetail.scan]
  )
  const unit = inspectedLabel(scanType)

  const [disabledCategories, setDisabledCategories] = React.useState<
    Set<string>
  >(new Set())
  const categoryCounts = React.useMemo(() => {
    const counts: Record<string, number> = {}
    for (const { finding } of selectedFindings) {
      const category = finding.category || "uncategorized"
      counts[category] = (counts[category] ?? 0) + 1
    }
    return counts
  }, [selectedFindings])
  const visibleFindings = React.useMemo(
    () =>
      selectedFindings.filter(
        ({ finding }) =>
          !disabledCategories.has(finding.category || "uncategorized")
      ),
    [selectedFindings, disabledCategories]
  )

  const toggleCategory = React.useCallback(
    (category: string, enabled: boolean) => {
      setDisabledCategories((current) => {
        const next = new Set(current)
        if (enabled) {
          next.delete(category)
        } else {
          next.add(category)
        }
        return next
      })
    },
    []
  )

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-3">
        <div>
          <Button
            variant="outline"
            size="sm"
            render={<Link href={`${basePath}/${scanType}`} />}
            nativeButton={false}
          >
            <ArrowLeftIcon data-icon="inline-start" />
            {backLabel}
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Badge variant="outline">{section}</Badge>
            <Badge>{badge}</Badge>
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">
              {targetName}
            </h1>
            <p className="text-sm text-muted-foreground">{description}</p>
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
      ) : !selectedScan || !selectedSummary ? (
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
        <>
          <DashboardSummaryGrid summary={selectedSummary}>
            <MetricCard
              title="Scans"
              value={targetScans.length}
              description={`runs ${targetScope}`}
            />
            <MetricCard
              title={inspectedTitle}
              value={selectedSummary.totalDependencies}
              description="in the selected scan"
            />
            <MetricCard
              title="Findings"
              value={selectedSummary.vulnerabilities}
              description="in the selected scan"
            />
            <MetricCard
              title="Critical/High"
              value={selectedSummary.critical + selectedSummary.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={targetScans} />

          <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
            {scanDetail.loading ? (
              <Card>
                <CardHeader>
                  <CardTitle>Findings found</CardTitle>
                  <CardDescription>Loading the selected scan…</CardDescription>
                </CardHeader>
                <CardContent>
                  <ScanDetailSkeleton />
                </CardContent>
              </Card>
            ) : scanDetail.error ? (
              <ScanDetailError message={scanDetail.error} />
            ) : scanDetail.scan ? (
              <FindingsTable
                scans={[scanDetail.scan]}
                findings={visibleFindings}
                scanIsClean={selectedFindings.length === 0}
                title="Security findings"
                description={
                  disabledCategories.size > 0
                    ? `Showing ${visibleFindings.length} of ${selectedFindings.length} findings · Selected scan on ${formatDate(selectedScan.createdAt)}`
                    : `Selected scan on ${formatDate(selectedScan.createdAt)}`
                }
              />
            ) : null}

            <div className="flex flex-col gap-6">
              <ScanTypeFilter
                scanType={scanType}
                counts={categoryCounts}
                disabledCategories={disabledCategories}
                onToggle={toggleCategory}
              />

              <Card>
                <CardHeader>
                  <CardTitle>{targetTitle}</CardTitle>
                  <CardDescription>{selectedScan.workspaceName}</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-2 text-sm">
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Source</span>
                    <span className="truncate text-right">
                      {selectedScan.source || "-"}
                    </span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Status</span>
                    <span>{selectedScan.status || "-"}</span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Scanner</span>
                    <span>{selectedScan.scannerVersion || "-"}</span>
                  </div>
                </CardContent>
              </Card>

              <LatestScansCard
                scans={targetScans}
                description={`Recent history ${targetScope}.`}
                getTitle={(scan) => scan.targetName || scan.projectName || targetName}
                packageLabel={unit}
                findingLabel="findings"
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
