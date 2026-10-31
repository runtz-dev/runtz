import { apiRequest } from "@/lib/api"

export type CloudCounts = { total: number; critical: number; high: number; medium: number; low: number; unknown: number }
export type CloudCoverage = { passed: number; failed: number; errors: number; skipped: number; notApplicable: number; critical: number; high: number; medium: number; low: number }
export type CloudConnection = {
  id: string; workspaceId: string; provider: "gcp"; name: string; projectId: string
  location: string; serviceAccount: string; status: string; lastError?: string
  summary: CloudCounts; lastScanAt?: string; createdAt: string; updatedAt: string
  coverage?: CloudCoverage
}
export type CloudFinding = {
  id: string; title: string; category: string; description: string
  severity: string; state: string; mute: string; resourceName: string
  resourceType: string; projectName: string; location: string; source: string
  externalUri: string; eventTime: string; remediation: string
  ruleId?: string; evidence?: string
}
export type CloudFindingResponse = { findings: CloudFinding[]; total: number; page: number; pageSize: number; resultAvailable?: boolean; coverage?: CloudCoverage }
export type CloudCheck = { ruleId: string; title: string; status: string; severity: string; resource: string; evidence: string }
export type CloudCheckResponse = { checks: CloudCheck[]; total: number; page: number; pageSize: number; resultAvailable: boolean; coverage?: CloudCoverage }
export const cloudSeverities = ["critical", "high", "medium", "low", "unknown"] as const
export const emptyCloudCounts: CloudCounts = { total: 0, critical: 0, high: 0, medium: 0, low: 0, unknown: 0 }
export const cloudSeverityColors: Record<string, string> = {
  critical: "text-severity-critical", high: "text-severity-high", medium: "text-severity-medium",
  low: "text-severity-low", unknown: "text-severity-unknown",
}
export function listCloudConnections(workspaceId: string, signal?: AbortSignal) {
  const query = workspaceId === "all" ? "" : `?workspaceId=${encodeURIComponent(workspaceId)}`
  return apiRequest<{ connections: CloudConnection[]; configured: boolean }>(`/api/v1/cloud/connections${query}`, { signal })
}
export function cloudError(error: unknown) { return error instanceof Error ? error.message : "Something went wrong. Please retry." }
export function safeCloudLink(uri: string) {
  try { const url = new URL(uri); return url.protocol === "https:" ? url.href : undefined } catch { return undefined }
}
