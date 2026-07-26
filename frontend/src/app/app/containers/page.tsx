"use client"

import Link from "next/link"
import { ArrowRightIcon, ContainerIcon } from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  MetricCard,
  SeverityDistribution,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
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
import {
  getContainerTargetName,
  groupPackageScans,
  summarizePackageTargets,
} from "@/lib/package-scans"
import { formatDate } from "@/lib/sca"
import { aggregateSummaries } from "@/lib/dashboard"

export default function ContainersPage() {
  const { basePath } = usePlatform()
  const { scans, loading, error } = usePackageScans("container")
  const images = React.useMemo(
    () => groupPackageScans(scans, getContainerTargetName),
    [scans]
  )
  const totals = React.useMemo(() => summarizePackageTargets(images), [images])
  const severitySummary = React.useMemo(
    () => aggregateSummaries(images.map((image) => image.latestScan.summary)),
    [images]
  )

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Badge variant="outline">HOSTS</Badge>
          <Badge>Container scanning</Badge>
        </div>
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">
            Container scanning
          </h1>
          <p className="text-sm text-muted-foreground">
            Images scanned by installed-package inventory.
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
              title="Imagens"
              value={totals.targets}
              description="imagens diferentes"
            />
            <MetricCard
              title="Scans"
              value={totals.scans}
              description="runs received"
            />
            <MetricCard
              title="CVEs"
              value={totals.vulnerabilities}
              description="across the latest scans per image"
            />
            <MetricCard
              title="Critical/High"
              value={totals.critical + totals.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={scans} />

          {images.length === 0 ? (
            <Empty className="min-h-80 border">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <ContainerIcon />
                </EmptyMedia>
                <EmptyTitle>No images scanned</EmptyTitle>
                <EmptyDescription>
                  Execute o CLI de container apontando para o backend para
                  preencher este painel.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>Imagens</CardTitle>
                <CardDescription>
                  Click an image name to see the CVE list from its latest
                  scan.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Imagem</TableHead>
                      <TableHead>Vulnerabilidades</TableHead>
                      <TableHead>Pacotes</TableHead>
                      <TableHead>OS</TableHead>
                      <TableHead>Scans</TableHead>
                      <TableHead>Último scan</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {images.map((image) => (
                      <TableRow key={image.targetName}>
                        <TableCell className="min-w-72">
                          <div className="flex min-w-0 flex-col gap-1">
                            <Link
                              className="inline-flex items-center gap-2 font-medium text-primary underline-offset-4 hover:underline"
                              href={`${basePath}/containers/${encodeURIComponent(
                                image.targetName
                              )}`}
                            >
                              <span className="truncate">{image.targetName}</span>
                              <ArrowRightIcon />
                            </Link>
                            <span className="truncate text-xs text-muted-foreground">
                              {image.workspaceNames.join(", ")}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="min-w-72">
                          <div className="flex items-center gap-4">
                            <Badge
                              variant={
                                image.latestScan.summary.vulnerabilities > 0
                                  ? "secondary"
                                  : "outline"
                              }
                            >
                              {image.latestScan.summary.vulnerabilities} vulns
                            </Badge>
                            <SeverityDistribution
                              summary={image.latestScan.summary}
                              className="min-w-56"
                            />
                          </div>
                        </TableCell>
                        <TableCell>
                          {image.latestScan.summary.totalDependencies}
                        </TableCell>
                        <TableCell>
                          {image.latestScan.osName || image.latestScan.osId || "-"}
                        </TableCell>
                        <TableCell>{image.scans.length}</TableCell>
                        <TableCell>
                          {formatDate(image.latestScan.createdAt)}
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
