"use client"

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? ""

export type Workspace = {
  id: string
  name: string
  slug: string
  createdAt: string
  updatedAt: string
}

export type User = {
  id: string
  username: string
  email?: string
  displayName?: string
  avatarUrl?: string
  authProvider?: "password" | "email" | "google" | "github"
  role: "admin" | "member"
  workspaceIds: string[]
  requirePasswordChange: boolean
  onboardingCompleted: boolean
  lastLoginAt?: string
  createdAt: string
  updatedAt: string
}

export type Entitlement = {
  plan: "free" | "pro" | "enterprise"
  deploymentMode: "cloud" | "self-hosted"
  status: string
  features: string[]
  licenseKeyPrefix?: string
  installationId?: string
  expiresAt?: string
  currentPeriodEnd?: string
  cancelAtPeriodEnd?: boolean
}

export type ApiKey = {
  id: string
  workspaceId: string
  name: string
  prefix: string
  scopes?: string[]
  createdBy: string
  lastUsedAt?: string
  revokedAt?: string
  expiresAt?: string
  createdAt: string
  updatedAt: string
}

export type Dependency = {
  name: string
  requestedRange: string
  resolvedVersion: string
  scope: string
  ecosystem: string
}

export type Vulnerability = {
  id: string
  ghsaId?: string
  cveId?: string
  packageName: string
  installedPackage?: string
  sourcePackage?: string
  ecosystem: string
  installedVersion: string
  vulnerableRange: string
  firstPatchedVersion?: string
  severity: string
  summary: string
  advisoryUrl: string
  cvssScore?: number
  references?: string[]
  publishedAt?: string
  updatedAt?: string
}

export type ScanSummary = {
  totalDependencies: number
  vulnerabilities: number
  critical: number
  high: number
  medium: number
  low: number
  unknown: number
}

export type ScanPackage = {
  name: string
  version: string
  architecture?: string
  sourceName?: string
  sourceVersion?: string
  manager: string
}

export type Finding = {
  id: string
  title: string
  description?: string
  severity: string
  category?: string
  file?: string
  line?: number
  column?: number
  resourceKind?: string
  resourceName?: string
  namespace?: string
  remediation?: string
}

export type SCAScan = {
  id: string
  type: "sca"
  workspaceId: string
  workspaceName: string
  projectName: string
  source: string
  targetFile: string
  status: string
  scannerVersion: string
  summary: ScanSummary
  dependencies: Dependency[]
  vulnerabilities: Vulnerability[]
  createdAt: string
}

export type FindingsScan = {
  id: string
  type: "sast" | "k8s"
  workspaceId: string
  workspaceName: string
  projectName: string
  targetName?: string
  source: string
  status: string
  scannerVersion: string
  filesScanned?: number
  resourcesScanned?: number
  summary: ScanSummary
  findings: Finding[]
  createdAt: string
}

export type SASTScan = FindingsScan & {
  type: "sast"
}

export type KubernetesScan = FindingsScan & {
  type: "k8s"
}

export type PackageScan = {
  id: string
  type: "host" | "container"
  workspaceId: string
  workspaceName: string
  projectName: string
  targetName?: string
  hostname?: string
  imageName?: string
  imageRef?: string
  imageDigest?: string
  source: string
  status: string
  osId?: string
  osName?: string
  osVersion?: string
  osCodename?: string
  packageManager?: string
  scannerVersion: string
  summary: ScanSummary
  packages: ScanPackage[]
  vulnerabilities: Vulnerability[]
  createdAt: string
}

export type HostScan = PackageScan & {
  type: "host"
  hostname?: string
}

export type ContainerScan = PackageScan & {
  type: "container"
  imageName?: string
  imageRef?: string
  imageDigest?: string
}

type RequestOptions = {
  method?: "GET" | "POST" | "PATCH"
  body?: unknown
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {}
): Promise<T> {
  // The session rides in an HttpOnly cookie the browser attaches on its own —
  // there is no token for this code to read, store or forward, which is the
  // point: a script injected into the dashboard cannot reach the credential.
  const response = await fetch(`${API_URL}${path}`, {
    method: options.method ?? "GET",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  })

  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError(
      response.status,
      typeof payload.error === "string" ? payload.error : "Request failed"
    )
  }

  return payload as T
}

// clearClientState drops the local UI preferences tied to a signed-in user.
// The session itself is not here to clear — it is an HttpOnly cookie only the
// engine can set or expire, which is why signing out has to go through the API.
export function clearClientState() {
  if (typeof window === "undefined") {
    return
  }

  window.localStorage.removeItem("runtz_workspace_id")
  window.localStorage.removeItem("runtz_workspace_filter")
}

export async function signOut() {
  try {
    await apiRequest("/api/v1/auth/logout", { method: "POST" })
  } catch {
    // The local state has to go even if the engine is unreachable; the cookie
    // expires on its own and the session row with it.
  }

  clearClientState()
}
