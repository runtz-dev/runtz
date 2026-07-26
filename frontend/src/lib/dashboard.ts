import type { ScanSummary } from "@/lib/api"

export type TrendScan = {
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
