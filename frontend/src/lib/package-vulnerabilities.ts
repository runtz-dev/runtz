import type { Vulnerability } from "@/lib/api"
import type { CVEFixFilter } from "@/lib/dashboard"
import type { SeverityKey } from "@/lib/sca"

const SEVERITY_WEIGHT: Record<SeverityKey, number> = {
  critical: 5,
  high: 4,
  medium: 3,
  low: 2,
  unknown: 1,
}

export type PackageVulnerabilityGroup = {
  key: string
  name: string
  sourcePackages: string[]
  ecosystems: string[]
  installedVersions: string[]
  highestSeverity: SeverityKey
  fixableCount: number
  vulnerabilities: Vulnerability[]
}

export type RemediationContext = {
  targetName?: string
  targetKind?: "host" | "container" | "application"
  osName?: string
  osVersion?: string
  packageManager?: string
}

export function groupVulnerabilitiesByPackage(
  vulnerabilities: Vulnerability[]
): PackageVulnerabilityGroup[] {
  const groups = new Map<string, Vulnerability[]>()

  for (const vulnerability of vulnerabilities) {
    const name = vulnerabilityPackageName(vulnerability)
    const key = name.toLocaleLowerCase()
    const current = groups.get(key) ?? []
    current.push(vulnerability)
    groups.set(key, current)
  }

  return Array.from(groups.entries())
    .map(([key, packageVulnerabilities]) => {
      const deduplicated = deduplicateVulnerabilities(packageVulnerabilities)
      const highestSeverity = deduplicated.reduce<SeverityKey>(
        (highest, vulnerability) =>
          severityWeight(vulnerability.severity) > severityWeight(highest)
            ? normalizeSeverity(vulnerability.severity)
            : highest,
        "unknown"
      )

      return {
        key,
        name: vulnerabilityPackageName(deduplicated[0]),
        sourcePackages: uniqueValues(
          packageVulnerabilities.map(
            (vulnerability) => vulnerability.sourcePackage
          )
        ),
        ecosystems: uniqueValues(
          packageVulnerabilities.map(
            (vulnerability) => vulnerability.ecosystem
          )
        ),
        installedVersions: uniqueValues(
          packageVulnerabilities.map(
            (vulnerability) => vulnerability.installedVersion
          )
        ),
        highestSeverity,
        fixableCount: deduplicated.filter(vulnerabilityHasFix).length,
        vulnerabilities: deduplicated.sort(compareVulnerabilities),
      }
    })
    .sort((left, right) => {
      const severityDifference =
        severityWeight(right.highestSeverity) -
        severityWeight(left.highestSeverity)
      return severityDifference || left.name.localeCompare(right.name)
    })
}

export function filterVulnerabilitiesByFix(
  vulnerabilities: Vulnerability[],
  filter: CVEFixFilter
) {
  if (filter === "all") {
    return vulnerabilities
  }

  const wantsFix = filter === "with-fix"
  return vulnerabilities.filter(
    (vulnerability) => vulnerabilityHasFix(vulnerability) === wantsFix
  )
}

export function vulnerabilityHasFix(vulnerability: Vulnerability) {
  return Boolean(vulnerability.firstPatchedVersion?.trim())
}

export function vulnerabilityReferences(vulnerability: Vulnerability) {
  return uniqueValues([
    vulnerability.advisoryUrl,
    ...(vulnerability.references ?? []),
  ]).filter(isSafeExternalURL)
}

export function buildRemediationPrompt(
  group: PackageVulnerabilityGroup,
  context: RemediationContext = {}
) {
  const target = [context.targetKind, context.targetName]
    .filter(Boolean)
    .join(": ")
  const operatingSystem = [context.osName, context.osVersion]
    .filter(Boolean)
    .join(" ")
  const vulnerabilityLines = group.vulnerabilities
    .map((vulnerability) => {
      const references = vulnerabilityReferences(vulnerability)
      return [
        `- ${vulnerability.id} (${normalizeSeverity(vulnerability.severity)})`,
        `  Installed: ${vulnerability.installedVersion || "unknown"}`,
        `  Affected range: ${vulnerability.vulnerableRange || "not provided"}`,
        `  First patched version: ${vulnerability.firstPatchedVersion || "not published"}`,
        `  Summary: ${vulnerability.summary || "not provided"}`,
        `  References: ${references.length > 0 ? references.join(", ") : "not provided"}`,
      ].join("\n")
    })
    .join("\n")

  return `You are a senior security and platform engineer. Create a safe, minimal, production-ready remediation plan for the vulnerable package below.

Context
- Target: ${target || "not provided"}
- Operating system: ${operatingSystem || "not provided"}
- Package manager: ${context.packageManager || "not provided"}
- Package: ${group.name}
- Source package: ${group.sourcePackages.join(", ") || "not provided"}
- Ecosystem: ${group.ecosystems.join(", ") || "not provided"}
- Installed version(s): ${group.installedVersions.join(", ") || "unknown"}

Vulnerabilities
${vulnerabilityLines}

Requirements
1. Verify whether every vulnerability is applicable to this exact package, version and environment. State any assumption.
2. Recommend the smallest safe upgrade that remediates the maximum number of CVEs. Never invent a fixed version.
3. If no patched version exists, propose concrete compensating controls and how to validate them.
4. Provide exact commands or manifest/file changes appropriate to the package manager, but do not execute them.
5. Identify likely breaking changes, service restart requirements and operational risks.
6. Provide verification steps: package version check, focused regression tests and a vulnerability rescan.
7. Include a rollback plan and call out CVEs that remain unresolved.

Return: applicability assessment, recommended change, implementation steps, validation, rollback and residual risk.`
}

export function referenceLabel(reference: string) {
  try {
    const url = new URL(reference)
    const hostname = url.hostname.replace(/^www\./, "")
    if (url.pathname === "/") {
      return hostname
    }
    const path =
      url.pathname.length > 28
        ? `${url.pathname.slice(0, 27)}…`
        : url.pathname
    return `${hostname}${path}`
  } catch {
    return reference
  }
}

function vulnerabilityPackageName(vulnerability: Vulnerability) {
  return (
    vulnerability.installedPackage?.trim() ||
    vulnerability.packageName?.trim() ||
    "unknown"
  )
}

function deduplicateVulnerabilities(vulnerabilities: Vulnerability[]) {
  const unique = new Map<string, Vulnerability>()

  for (const vulnerability of vulnerabilities) {
    const key =
      vulnerability.id?.trim().toLocaleLowerCase() ||
      `${vulnerability.packageName}:${vulnerability.summary}`
    const current = unique.get(key)

    if (!current) {
      unique.set(key, vulnerability)
      continue
    }

    const currentReferences = vulnerabilityReferences(current)
    const nextReferences = vulnerabilityReferences(vulnerability)
    const higherSeverity =
      severityWeight(vulnerability.severity) > severityWeight(current.severity)
        ? vulnerability.severity
        : current.severity

    unique.set(key, {
      ...current,
      severity: higherSeverity,
      firstPatchedVersion:
        current.firstPatchedVersion || vulnerability.firstPatchedVersion,
      advisoryUrl: current.advisoryUrl || vulnerability.advisoryUrl,
      references: uniqueValues([...currentReferences, ...nextReferences]),
    })
  }

  return Array.from(unique.values())
}

function compareVulnerabilities(left: Vulnerability, right: Vulnerability) {
  const severityDifference =
    severityWeight(right.severity) - severityWeight(left.severity)
  return severityDifference || left.id.localeCompare(right.id)
}

function normalizeSeverity(severity: string): SeverityKey {
  const normalized = severity?.trim().toLocaleLowerCase()
  return normalized in SEVERITY_WEIGHT
    ? (normalized as SeverityKey)
    : "unknown"
}

function severityWeight(severity: string) {
  return SEVERITY_WEIGHT[normalizeSeverity(severity)]
}

function uniqueValues(values: Array<string | undefined>) {
  return Array.from(
    new Set(values.map((value) => value?.trim()).filter(Boolean) as string[])
  )
}

function isSafeExternalURL(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === "https:" || url.protocol === "http:"
  } catch {
    return false
  }
}
