"use client"

import { InfoIcon, SlidersHorizontalIcon } from "lucide-react"
import * as React from "react"

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Switch } from "@/components/ui/switch"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

type ScanTypeInfo = {
  label: string
  description: string
}

// Catalog of check types per scanner. New types added in the CLI
// show up automatically with a generic description until they get an entry here.
const scanTypeCatalog: Record<"sast" | "k8s", Record<string, ScanTypeInfo>> = {
  sast: {
    secret: {
      label: "Secrets",
      description:
        "Private keys, cloud credentials and hardcoded secrets committed to the source code.",
    },
    injection: {
      label: "Dynamic execution",
      description:
        "Use of eval and dynamic code execution, which can run attacker-controlled input.",
    },
    "command-injection": {
      label: "Command injection",
      description:
        "Process execution with shell enabled, vulnerable to command injection.",
    },
    "transport-security": {
      label: "Transport security",
      description:
        "TLS certificate verification disabled in network calls.",
    },
    cryptography: {
      label: "Criptografia fraca",
      description:
        "Weak hash functions such as MD5 and SHA-1 in security-sensitive contexts.",
    },
  },
  k8s: {
    "pod-security": {
      label: "Pod security",
      description:
        "hostNetwork, hostPID, hostIPC and hostPath volumes, which expose the node to the workload.",
    },
    "container-security": {
      label: "Container security",
      description:
        "Privileged containers, running as root, broad capabilities and a writable root filesystem.",
    },
    rbac: {
      label: "RBAC",
      description:
        "cluster-admin bindings, wildcard rules and service accounts with more permissions than needed.",
    },
    network: {
      label: "Network",
      description:
        "Publicly exposed Services (LoadBalancer/NodePort) and Ingress without TLS.",
    },
    "supply-chain": {
      label: "Supply chain",
      description:
        "Images without an immutable tag (latest or untagged), subject to silent content swaps.",
    },
    resilience: {
      label: "Resilience",
      description:
        "Requests/limits e probes ausentes, que aumentam o raio de dano de incidentes.",
    },
  },
}

export function scanTypeCategories(
  scanType: "sast" | "k8s",
  presentCategories: Iterable<string>
): string[] {
  const known = Object.keys(scanTypeCatalog[scanType])
  const extra = [...new Set(presentCategories)].filter(
    (category) => !known.includes(category)
  )
  return [...known, ...extra.sort()]
}

export function ScanTypeFilter({
  scanType,
  counts,
  disabledCategories,
  onToggle,
}: {
  scanType: "sast" | "k8s"
  counts: Record<string, number>
  disabledCategories: Set<string>
  onToggle: (category: string, enabled: boolean) => void
}) {
  const categories = scanTypeCategories(scanType, Object.keys(counts))

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <SlidersHorizontalIcon className="size-4 text-primary" />
          Tipos de scan
        </CardTitle>
        <CardDescription>
          Filter findings by check type. All are enabled by default.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-1">
        <TooltipProvider>
          {categories.map((category) => {
            const info = scanTypeCatalog[scanType][category] ?? {
              label: category,
              description: "New check type from this scanner.",
            }
            const enabled = !disabledCategories.has(category)
            const count = counts[category] ?? 0

            return (
              <div
                key={category}
                className="flex items-center justify-between gap-3 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/60"
              >
                <div className="flex min-w-0 items-center gap-1.5">
                  <span
                    className={
                      enabled
                        ? "truncate text-sm"
                        : "truncate text-sm text-muted-foreground line-through decoration-muted-foreground/50"
                    }
                  >
                    {info.label}
                  </span>
                  <Tooltip>
                    <TooltipTrigger
                      aria-label={`Sobre ${info.label}`}
                      className="flex shrink-0 items-center text-muted-foreground outline-none hover:text-foreground focus-visible:text-foreground"
                    >
                      <InfoIcon className="size-3.5" />
                    </TooltipTrigger>
                    <TooltipContent side="left" className="max-w-64">
                      {info.description}
                    </TooltipContent>
                  </Tooltip>
                </div>
                <div className="flex shrink-0 items-center gap-2.5">
                  <span className="text-xs tabular-nums text-muted-foreground">
                    {count}
                  </span>
                  <Switch
                    checked={enabled}
                    onCheckedChange={(checked) => onToggle(category, checked)}
                    aria-label={`Exibir findings de ${info.label}`}
                  />
                </div>
              </div>
            )
          })}
        </TooltipProvider>
      </CardContent>
    </Card>
  )
}
