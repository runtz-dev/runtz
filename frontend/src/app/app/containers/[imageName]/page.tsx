"use client"

import Link from "next/link"
import { useParams } from "next/navigation"
import { ArrowLeftIcon, ContainerIcon } from "lucide-react"
import * as React from "react"

import {
  CVETable,
  DashboardSummaryGrid,
  LatestScansCard,
  MetricCard,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { usePlatform } from "@/components/runtz/platform-context"
import { usePackageScans } from "@/components/runtz/use-package-scans"
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
  getContainerTargetName,
} from "@/lib/package-scans"
import { formatDate } from "@/lib/sca"

export default function ContainerDetailPage() {
  const { basePath } = usePlatform()
  const params = useParams<{ imageName: string }>()
  const imageName = React.useMemo(
    () => decodeParam(params.imageName),
    [params.imageName]
  )
  const { scans, loading, error } = usePackageScans("container")
  const imageScans = React.useMemo(
    () => filterPackageScansByTarget(scans, imageName, getContainerTargetName),
    [imageName, scans]
  )
  const latestScan = imageScans[0]
  const latestSummary = latestScan?.summary

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-3">
        <div>
          <Button
            variant="outline"
            size="sm"
            render={<Link href={`${basePath}/containers`} />}
            nativeButton={false}
          >
            <ArrowLeftIcon data-icon="inline-start" />
            Imagens
          </Button>
        </div>
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Badge variant="outline">HOSTS</Badge>
            <Badge>Container scanning</Badge>
          </div>
          <div>
            <h1 className="text-2xl font-semibold tracking-normal">
              {imageName}
            </h1>
            <p className="text-sm text-muted-foreground">
              CVEs found in the image&apos;s latest package scan.
            </p>
          </div>
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
      ) : !latestScan || !latestSummary ? (
        <Empty className="min-h-80 border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <ContainerIcon />
            </EmptyMedia>
            <EmptyTitle>Image not found</EmptyTitle>
            <EmptyDescription>
              No scans for this image in the selected workspace.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <DashboardSummaryGrid summary={latestSummary}>
            <MetricCard
              title="Scans"
              value={imageScans.length}
              description="runs for this image"
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

          <VulnerabilityTrendChart scans={imageScans} />

          <div className="grid gap-6 xl:grid-cols-[1fr_360px]">
            <Card>
              <CardHeader>
                <CardTitle>CVEs encontradas</CardTitle>
                <CardDescription>
                  Último scan em {formatDate(latestScan.createdAt)}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <CVETable vulnerabilities={latestScan.vulnerabilities} />
              </CardContent>
            </Card>

            <div className="flex flex-col gap-6">
              <Card>
                <CardHeader>
                  <CardTitle>Imagem</CardTitle>
                  <CardDescription>
                    {latestScan.osName || latestScan.osId || "-"}
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-2 text-sm">
                  <div className="flex justify-between gap-4">
                    <span className="text-muted-foreground">Digest</span>
                    <span className="truncate text-right">
                      {latestScan.imageDigest || "-"}
                    </span>
                  </div>
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
                scans={imageScans}
                description="Recent history for this image."
                getTitle={(scan) => scan.targetName || scan.projectName || imageName}
                packageLabel="packages"
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
