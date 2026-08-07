"use client"

import { ShipWheelIcon } from "lucide-react"

import { FindingsScanPage } from "@/components/runtz/findings-scan-page"

export default function KubernetesPage() {
  return (
    <FindingsScanPage
      scanType="k8s"
      section="HOSTS"
      badge="K8s scanning"
      title="K8s scanning"
      description="Kubernetes cluster posture submitted by the CLI."
      targetTitle="Clusters"
      targetDescription="distinct clusters"
      targetColumnTitle="Cluster name"
      targetListDescription="Click a cluster name to see the findings from its latest scan."
      inspectedTitle="Resources"
      emptyTitle="No clusters scanned"
      emptyDescription="Scan your cluster posture to start tracking Kubernetes security findings."
      commandLabel="runtz k8s"
      icon={ShipWheelIcon}
    />
  )
}
