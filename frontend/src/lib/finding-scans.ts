import type { FindingsScan, ScanSummary } from "@/lib/api"
import { SEVERITY_KEYS, countSeverity, sortScansDesc } from "@/lib/sca"

export type FindingsTarget<TScan extends FindingsScan = FindingsScan> = {
  targetName: string
  workspaceNames: string[]
  scans: TScan[]
  latestScan: TScan
}

export type FindingsTotals = {
  targets: number
  scans: number
  inspected: number
  findings: number
  critical: number
  high: number
  medium: number
  low: number
  unknown: number
}

export function groupFindingsScans<TScan extends FindingsScan>(
  scans: TScan[]
): FindingsTarget<TScan>[] {
  const targets = new Map<string, FindingsTarget<TScan>>()

  for (const scan of sortScansDesc(scans)) {
    const targetName = getFindingsTargetName(scan)
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

export function summarizeFindingsTargets(
  targets: FindingsTarget[]
): FindingsTotals {
  return targets.reduce(
    (acc, target) => {
      const summary = target.latestScan.summary
      acc.scans += target.scans.length
      acc.inspected += summary.totalDependencies
      acc.findings += summary.vulnerabilities
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
      inspected: 0,
      findings: 0,
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      unknown: 0,
    }
  )
}

export function latestFindings(scans: FindingsScan[], limit = 12) {
  return sortScansDesc(scans)
    .flatMap((scan) =>
      (scan.findings ?? []).map((finding) => ({
        scan,
        finding,
      }))
    )
    .slice(0, limit)
}

export function filterFindingsScansByTarget<TScan extends FindingsScan>(
  scans: TScan[],
  targetName: string
) {
  return sortScansDesc(
    scans.filter((scan) => getFindingsTargetName(scan) === targetName)
  )
}

export function inspectedLabel(scanType: FindingsScan["type"]) {
  return scanType === "k8s" ? "resources" : "files"
}

export function getFindingsTargetName(scan: FindingsScan) {
  return scan.targetName?.trim() || scan.projectName?.trim() || "unknown"
}

export function hasFindings(summary: ScanSummary) {
  return SEVERITY_KEYS.some((severity) => summary[severity] > 0)
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) =>
    left.localeCompare(right)
  )
}
