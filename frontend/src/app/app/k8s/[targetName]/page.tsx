"use client"

import { ShipWheelIcon } from "lucide-react"

import { FindingsTargetPage } from "@/components/runtz/findings-target-page"

export default function KubernetesTargetPage() {
  return (
    <FindingsTargetPage
      scanType="k8s"
      section="HOSTS"
      badge="K8s scanning"
      backLabel="Clusters K8s"
      targetTitle="Cluster"
      targetScope="for this cluster"
      description="Findings encontrados in the latest scan Kubernetes deste cluster."
      inspectedTitle="Recursos"
      emptyTitle="Cluster not found"
      emptyDescription="No Kubernetes scans for this cluster in the selected workspace."
      icon={ShipWheelIcon}
    />
  )
}
