"use client"

import * as React from "react"

import { usePlatform } from "@/components/runtz/platform-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest, type FindingsScan } from "@/lib/api"
import {
  loadPlaygroundScans,
  playgroundScansOfType,
} from "@/lib/playground-scans"

export function useFindingScans(scanType: "sast" | "k8s") {
  const { isPlayground } = usePlatform()
  const { selectedWorkspaceId } = useWorkspace()
  const [scans, setScans] = React.useState<FindingsScan[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    if (isPlayground) {
      setLoading(true)
      setError("")
      loadPlaygroundScans()
        .then((responseScans) => {
          setScans(playgroundScansOfType<FindingsScan>(responseScans, scanType))
        })
        .catch((error) => {
          setError(
            error instanceof Error
              ? error.message
              : `Failed to load ${scanType}`
          )
          setScans([])
        })
        .finally(() => setLoading(false))
      return
    }


    setLoading(true)
    setError("")
    const path =
      selectedWorkspaceId && selectedWorkspaceId !== "all"
        ? `/api/v1/scans/${scanType}?workspaceId=${selectedWorkspaceId}`
        : `/api/v1/scans/${scanType}`

    apiRequest<{ scans: FindingsScan[] }>(path)
      .then((response) => {
        setScans(response.scans ?? [])
      })
      .catch((error) => {
        setError(
          error instanceof Error
            ? error.message
            : `Failed to load ${scanType}`
        )
        setScans([])
      })
      .finally(() => setLoading(false))
  }, [isPlayground, scanType, selectedWorkspaceId])

  return { scans, loading, error }
}
