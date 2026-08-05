"use client"

import Link from "next/link"
import {
  ArrowRightIcon,
  CodeIcon,
  ContainerIcon,
  LayoutDashboardIcon,
  ServerIcon,
  ShieldIcon,
  ShipWheelIcon,
} from "lucide-react"
import * as React from "react"

import {
  DashboardSummaryGrid,
  MetricCard,
  VulnerabilityTrendChart,
} from "@/components/runtz/sca-components"
import { usePlatform } from "@/components/runtz/platform-context"
import { useFindingScans } from "@/components/runtz/use-finding-scans"
import { usePackageScans } from "@/components/runtz/use-package-scans"
import { useSCAScans } from "@/components/runtz/use-sca-scans"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { aggregateSummaries } from "@/lib/dashboard"
import {
  getContainerTargetName,
  getHostTargetName,
  groupPackageScans,
} from "@/lib/package-scans"
import { groupFindingsScans } from "@/lib/finding-scans"
import { groupScansByProject } from "@/lib/sca"

export default function OverviewPage() {
  const { basePath } = usePlatform()
  const sca = useSCAScans()
  const sast = useFindingScans("sast")
  const containers = usePackageScans("container")
  const hosts = usePackageScans("host")
  const k8s = useFindingScans("k8s")
  const scaProjects = React.useMemo(() => groupScansByProject(sca.scans), [sca.scans])
  const sastProjects = React.useMemo(
    () => groupFindingsScans(sast.scans),
    [sast.scans]
  )
  const containerImages = React.useMemo(
    () => groupPackageScans(containers.scans, getContainerTargetName),
    [containers.scans]
  )
  const hostTargets = React.useMemo(
    () => groupPackageScans(hosts.scans, getHostTargetName),
    [hosts.scans]
  )
  const k8sTargets = React.useMemo(
    () => groupFindingsScans(k8s.scans),
    [k8s.scans]
  )
  const latestSummaries = React.useMemo(
    () => [
      ...scaProjects.map((project) => project.latestScan.summary),
      ...sastProjects.map((project) => project.latestScan.summary),
      ...containerImages.map((image) => image.latestScan.summary),
      ...hostTargets.map((host) => host.latestScan.summary),
      ...k8sTargets.map((target) => target.latestScan.summary),
    ],
    [containerImages, hostTargets, k8sTargets, sastProjects, scaProjects]
  )
  const totals = React.useMemo(
    () => aggregateSummaries(latestSummaries),
    [latestSummaries]
  )
  const scans = React.useMemo(
    () => [...sca.scans, ...sast.scans, ...containers.scans, ...hosts.scans, ...k8s.scans],
    [containers.scans, hosts.scans, k8s.scans, sast.scans, sca.scans]
  )
  const assets =
    scaProjects.length +
    sastProjects.length +
    containerImages.length +
    hostTargets.length +
    k8sTargets.length
  const loading =
    sca.loading || sast.loading || containers.loading || hosts.loading || k8s.loading
  const error = sca.error || sast.error || containers.error || hosts.error || k8s.error

  return (
    <div className="flex flex-col gap-6 p-4 md:p-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Badge variant="outline">PLATFORM</Badge>
          <Badge>Overview</Badge>
        </div>
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-semibold tracking-normal">
            <LayoutDashboardIcon className="size-5 text-primary" />
            Overview
          </h1>
          <p className="text-sm text-muted-foreground">
            Overview of scans and vulnerabilities across all assets.
          </p>
        </div>
      </div>

      {error ? (
        <Card>
          <CardHeader>
            <CardTitle>Failed to load part of the overview</CardTitle>
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
          <DashboardSummaryGrid summary={totals}>
            <MetricCard title="Assets" value={assets} description="apps, images, hosts, and K8s" />
            <MetricCard title="Scans" value={scans.length} description="runs received" />
            <MetricCard
              title="Vulnerabilities"
              value={totals.vulnerabilities}
              description="across the latest scans per asset"
            />
            <MetricCard
              title="Critical/High"
              value={totals.critical + totals.high}
              description="fix priority"
            />
          </DashboardSummaryGrid>

          <VulnerabilityTrendChart scans={scans} />

          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            <ScanFamilyCard
              href={`${basePath}/sca`}
              icon={ShieldIcon}
              title="SCA"
              description="Application dependencies"
              assets={scaProjects.length}
              scans={sca.scans.length}
            />
            <ScanFamilyCard
              href={`${basePath}/sast`}
              icon={CodeIcon}
              title="SAST"
              description="Static code findings"
              assets={sastProjects.length}
              scans={sast.scans.length}
            />
            <ScanFamilyCard
              href={`${basePath}/containers`}
              icon={ContainerIcon}
              title="Container scanning"
              description="Packages installed in images"
              assets={containerImages.length}
              scans={containers.scans.length}
            />
            <ScanFamilyCard
              href={`${basePath}/hosts`}
              icon={ServerIcon}
              title="Host scanning"
              description="Packages installed on hosts"
              assets={hostTargets.length}
              scans={hosts.scans.length}
            />
            <ScanFamilyCard
              href={`${basePath}/k8s`}
              icon={ShipWheelIcon}
              title="K8s scanning"
              description="Manifest posture"
              assets={k8sTargets.length}
              scans={k8s.scans.length}
            />
          </div>
        </>
      )}
    </div>
  )
}

function ScanFamilyCard({
  href,
  icon: Icon,
  title,
  description,
  assets,
  scans,
}: {
  href: string
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  title: string
  description: string
  assets: number
  scans: number
}) {
  return (
    <Link href={href} className="group">
      <Card className="h-full transition group-hover:-translate-y-0.5 group-hover:border-primary/45">
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div className="flex size-9 items-center justify-center rounded-lg bg-muted text-primary">
              <Icon className="size-4" />
            </div>
            <ArrowRightIcon className="size-4 text-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-primary" />
          </div>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardContent className="flex gap-2">
          <Badge variant="outline">{assets} assets</Badge>
          <Badge variant="secondary">{scans} scans</Badge>
        </CardContent>
      </Card>
    </Link>
  )
}
