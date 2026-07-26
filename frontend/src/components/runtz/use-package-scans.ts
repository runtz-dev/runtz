"use client"

import * as React from "react"

import { usePlatform } from "@/components/runtz/platform-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { apiRequest, getStoredToken, type PackageScan } from "@/lib/api"
import {
  loadPlaygroundScans,
  playgroundScansOfType,
} from "@/lib/playground-scans"

export function usePackageScans(scanType: "host" | "container") {
  const { isPlayground } = usePlatform()
  const { selectedWorkspaceId } = useWorkspace()
  const [scans, setScans] = React.useState<PackageScan[]>([])
  const [loading, setLoading] = React.useState(true)
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    if (isPlayground) {
      setLoading(true)
      setError("")
      loadPlaygroundScans()
        .then((responseScans) => {
          setScans(playgroundScansOfType<PackageScan>(responseScans, scanType))
        })
        .catch((error) => {
          setError(
            error instanceof Error
              ? error.message
              : `Falha ao carregar ${scanType}`
          )
          setScans([])
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

    setLoading(true)
    setError("")
    const path =
      selectedWorkspaceId && selectedWorkspaceId !== "all"
        ? `/api/v1/scans/${scanType}?workspaceId=${selectedWorkspaceId}`
        : `/api/v1/scans/${scanType}`

    apiRequest<{ scans: PackageScan[] }>(path, { token })
      .then((response) => {
        setScans(response.scans ?? [])
      })
      .catch((error) => {
        setError(
          error instanceof Error
            ? error.message
            : `Falha ao carregar ${scanType}`
        )
        setScans([])
      })
      .finally(() => setLoading(false))
  }, [isPlayground, scanType, selectedWorkspaceId])

  return { scans, loading, error }
}
