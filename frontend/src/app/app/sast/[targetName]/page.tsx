"use client"

import { CodeIcon } from "lucide-react"

import { FindingsTargetPage } from "@/components/runtz/findings-target-page"

export default function SASTTargetPage() {
  return (
    <FindingsTargetPage
      scanType="sast"
      section="CODE"
      badge="SAST"
      backLabel="Apps SAST"
      targetTitle="App"
      targetScope="for this app"
      description="Findings from this app's latest SAST scan."
      inspectedTitle="Arquivos"
      emptyTitle="App not found"
      emptyDescription="No SAST scans for this app in the selected workspace."
      icon={CodeIcon}
    />
  )
}
