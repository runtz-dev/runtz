"use client"

import * as React from "react"

import { usePlatform } from "@/components/runtz/platform-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest, getStoredToken, type SCAScan } from "@/lib/api"
import {
  loadPlaygroundScans,
  playgroundScansOfType,
} from "@/lib/playground-scans"

export function useSCAScans() {
  const { isPlayground } = usePlatform()
  const { selectedWorkspaceId } = useWorkspace()
  const [scans, setScans] = React.useState<SCAScan[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    if (isPlayground) {
      setLoading(true)
      setError("")
      loadPlaygroundScans()
        .then((responseScans) => {
          setScans(playgroundScansOfType<SCAScan>(responseScans, "sca"))
        })
        .catch((error) => {
          setScans([])
          setError(
            error instanceof Error ? error.message : "Failed to load SCA"
          )
        })
        .finally(() => setLoading(false))
      return
    }

    const token = getStoredToken()
    if (!token) {
      setScans([])
      setLoading(false)
      return
    }

    const path =
      selectedWorkspaceId && selectedWorkspaceId !== "all"
        ? `/api/v1/scans/sca?workspaceId=${selectedWorkspaceId}`
        : "/api/v1/scans/sca"

    setLoading(true)
    apiRequest<{ scans: SCAScan[] }>(path, { token })
      .then((response) => {
        setScans(response.scans ?? [])
        setError("")
      })
      .catch((error) => {
        setScans([])
        setError(error instanceof Error ? error.message : "Failed to load SCA")
      })
      .finally(() => setLoading(false))
  }, [isPlayground, selectedWorkspaceId])

  return { scans, loading, error }
}
