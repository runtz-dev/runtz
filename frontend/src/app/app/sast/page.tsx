"use client"

import { CodeIcon } from "lucide-react"

import { FindingsScanPage } from "@/components/runtz/findings-scan-page"

export default function SASTPage() {
  return (
    <FindingsScanPage
      scanType="sast"
      section="CODE"
      badge="SAST"
      title="SAST"
      description="Static code findings submitted by the CLI."
      targetTitle="Apps"
      targetDescription="distinct apps"
      targetColumnTitle="App"
      targetListDescription="Click an app name to see the findings from its latest scan."
      inspectedTitle="Files"
      emptyTitle="No apps scanned"
      emptyDescription="Scan your source code to start tracking security findings in this workspace."
      commandLabel="runtz sast ./"
      icon={CodeIcon}
    />
  )
}
