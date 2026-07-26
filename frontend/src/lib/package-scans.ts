import type { PackageScan, ScanSummary } from "@/lib/api"
import { SEVERITY_KEYS, countSeverity, sortScansDesc, type SeverityKey } from "@/lib/sca"

export type PackageScanTarget<TScan extends PackageScan = PackageScan> = {
  targetName: string
  workspaceNames: string[]
  scans: TScan[]
  latestScan: TScan
}

export type PackageScanTotals = {
  targets: number
  scans: number
  packages: number
  vulnerabilities: number
  critical: number
  high: number
  medium: number
  low: number
  unknown: number
}

export function groupPackageScans<TScan extends PackageScan>(
  scans: TScan[],
  getTargetName: (scan: TScan) => string
): PackageScanTarget<TScan>[] {
  const targets = new Map<string, PackageScanTarget<TScan>>()

  for (const scan of sortPackageScansDesc(scans)) {
    const targetName = getTargetName(scan).trim() || "unknown"
    const current = targets.get(targetName)

    if (current) {
      current.scans.push(scan)
      current.workspaceNames = uniqueSorted([
        ...current.workspaceNames,
        scan.workspaceName,
      ])
      continue
    }

    targets.set(targetName, {
      targetName,
      workspaceNames: uniqueSorted([scan.workspaceName]),
      scans: [scan],
      latestScan: scan,
    })
  }

  return Array.from(targets.values()).sort(
    (left, right) =>
      new Date(right.latestScan.createdAt).getTime() -
      new Date(left.latestScan.createdAt).getTime()
  )
}

export function filterPackageScansByTarget<TScan extends PackageScan>(
  scans: TScan[],
  targetName: string,
  getTargetName: (scan: TScan) => string
) {
  return sortPackageScansDesc(
    scans.filter((scan) => getTargetName(scan).trim() === targetName)
  )
}

export function summarizePackageTargets(
  targets: PackageScanTarget[]
): PackageScanTotals {
  return targets.reduce(
    (acc, target) => {
      const summary = target.latestScan.summary
      acc.scans += target.scans.length
      acc.packages += summary.totalDependencies
      acc.vulnerabilities += summary.vulnerabilities
      acc.critical += countSeverity(summary, "critical")
      acc.high += countSeverity(summary, "high")
      acc.medium += countSeverity(summary, "medium")
      acc.low += countSeverity(summary, "low")
      acc.unknown += countSeverity(summary, "unknown")
      return acc
    },
    {
      targets: targets.length,
      scans: 0,
      packages: 0,
      vulnerabilities: 0,
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      unknown: 0,
    }
  )
}

export function getHostTargetName(scan: PackageScan) {
  return scan.hostname?.trim() || scan.targetName?.trim() || scan.projectName
}

export function getContainerTargetName(scan: PackageScan) {
  return scan.imageName?.trim() || scan.targetName?.trim() || scan.imageRef?.trim() || scan.projectName
}

export function packageCount(summary: ScanSummary) {
  return summary.totalDependencies
}

export function sortPackageScansDesc<TScan extends PackageScan>(scans: TScan[]) {
  return sortScansDesc(scans) as TScan[]
}

export function severityKeys(): SeverityKey[] {
  return SEVERITY_KEYS
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) =>
    left.localeCompare(right)
  )
}
