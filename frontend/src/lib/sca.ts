import type { SCAScan, ScanSummary } from "@/lib/api"

export type SeverityKey = "critical" | "high" | "medium" | "low" | "unknown"

export type SCAProject = {
  projectName: string
  workspaceNames: string[]
  scans: SCAScan[]
  latestScan: SCAScan
}

export type SCAOverviewTotals = {
  apps: number
  scans: number
  dependencies: number
  vulnerabilities: number
  critical: number
  high: number
  medium: number
  low: number
  unknown: number
}

export const SEVERITY_KEYS: SeverityKey[] = [
  "critical",
  "high",
  "medium",
  "low",
  "unknown",
]

const UNKNOWN_PROJECT = "unknown"

export function groupScansByProject(scans: SCAScan[]): SCAProject[] {
  const projects = new Map<string, SCAProject>()

  for (const scan of sortScansDesc(scans)) {
    const projectName = getScanProjectName(scan)
    const current = projects.get(projectName)

    if (current) {
      current.scans.push(scan)
      current.workspaceNames = uniqueSorted([
        ...current.workspaceNames,
        scan.workspaceName,
      ])
      continue
    }

    projects.set(projectName, {
      projectName,
      workspaceNames: uniqueSorted([scan.workspaceName]),
      scans: [scan],
      latestScan: scan,
    })
  }

  return Array.from(projects.values()).sort(
    (left, right) =>
      new Date(right.latestScan.createdAt).getTime() -
      new Date(left.latestScan.createdAt).getTime()
  )
}

export function filterScansByProject(scans: SCAScan[], projectName: string) {
  return sortScansDesc(
    scans.filter((scan) => getScanProjectName(scan) === projectName)
  )
}

export function summarizeProjects(projects: SCAProject[]): SCAOverviewTotals {
  return projects.reduce(
    (acc, project) => {
      const summary = project.latestScan.summary
      acc.scans += project.scans.length
      acc.dependencies += summary.totalDependencies
      acc.vulnerabilities += summary.vulnerabilities
      acc.critical += summary.critical
      acc.high += summary.high
      acc.medium += summary.medium
      acc.low += summary.low
      acc.unknown += summary.unknown
      return acc
    },
    {
      apps: projects.length,
      scans: 0,
      dependencies: 0,
      vulnerabilities: 0,
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      unknown: 0,
    }
  )
}

export function countSeverity(summary: ScanSummary, severity: SeverityKey) {
  return summary[severity] ?? 0
}

export function totalSeverity(summary: ScanSummary) {
  return SEVERITY_KEYS.reduce(
    (total, severity) => total + countSeverity(summary, severity),
    0
  )
}

export function getScanProjectName(scan: SCAScan) {
  return scan.projectName?.trim() || UNKNOWN_PROJECT
}

export function formatDate(value: string) {
  const date = new Date(value)

  if (Number.isNaN(date.getTime())) {
    return "-"
  }

  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(date)
}

export function sortScansDesc<TScan extends { createdAt: string }>(scans: TScan[]) {
  return [...scans].sort(
    (left, right) =>
      new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime()
  )
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) =>
    left.localeCompare(right)
  )
}
