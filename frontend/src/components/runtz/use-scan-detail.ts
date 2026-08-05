"use client"

import * as React from "react"

import { usePlatform } from "@/components/runtz/platform-context"
import {
  apiRequest,
  type FindingsScan,
  type PackageScan,
  type SCAScan,
} from "@/lib/api"

type Scan = SCAScan | FindingsScan | PackageScan

export function useScanDetail<TScan extends Scan>(
  scanType: TScan["type"],
  scanSummary?: TScan
) {
  const { isPlayground } = usePlatform()
  const [scan, setScan] = React.useState<TScan>()
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState("")

  React.useEffect(() => {
    if (!scanSummary) {
      setScan(undefined)
      setLoading(false)
      setError("")
      return
    }

    if (isPlayground) {
      setScan(scanSummary)
      setLoading(false)
      setError("")
      return
    }

    let active = true
    setScan(undefined)
    setLoading(true)
    setError("")

    apiRequest<{ scan: TScan }>(
      `/api/v1/scans/${scanType}/${encodeURIComponent(scanSummary.id)}?view=results`
    )
      .then((response) => {
        if (active) {
          setScan(response.scan)
        }
      })
      .catch((requestError) => {
        if (active) {
          setError(
            requestError instanceof Error
              ? requestError.message
              : "Failed to load scan details"
          )
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [isPlayground, scanSummary, scanType])

  return { scan, loading, error }
}
