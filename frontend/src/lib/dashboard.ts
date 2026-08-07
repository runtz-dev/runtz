import type { ScanSummary, VulnerabilityCounts } from "@/lib/api"

export type CVEFixFilter = "all" | "with-fix" | "without-fix"

export type TrendScan = {
  id?: string
  type?: string
  workspaceId?: string
  projectName?: string
  targetName?: string
  hostname?: string
  imageName?: string
  imageRef?: string
  createdAt: string
  summary: ScanSummary
}

export function emptyScanSummary(): ScanSummary {
  return {
    totalDependencies: 0,
    vulnerabilities: 0,
    critical: 0,
    high: 0,
    medium: 0,
    low: 0,
    unknown: 0,
  }
}

export function aggregateSummaries(summaries: ScanSummary[]): ScanSummary {
  return summaries.reduce((total, summary) => {
    total.totalDependencies += summary.totalDependencies
    total.vulnerabilities += summary.vulnerabilities
    total.critical += summary.critical
    total.high += summary.high
    total.medium += summary.medium
    total.low += summary.low
    total.unknown += summary.unknown
    return total
  }, emptyScanSummary())
}

export function filterScanSummary(
  summary: ScanSummary,
  filter: CVEFixFilter
): ScanSummary {
  if (filter === "all") {
    return summary
  }

  const counts = filter === "with-fix" ? summary.withFix : summary.withoutFix
  return summaryFromCounts(summary.totalDependencies, counts)
}

function summaryFromCounts(
  totalDependencies: number,
  counts?: VulnerabilityCounts
): ScanSummary {
  return {
    totalDependencies,
    vulnerabilities: counts?.vulnerabilities ?? 0,
    critical: counts?.critical ?? 0,
    high: counts?.high ?? 0,
    medium: counts?.medium ?? 0,
    low: counts?.low ?? 0,
    unknown: counts?.unknown ?? 0,
  }
}
