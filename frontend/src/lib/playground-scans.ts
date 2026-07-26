"use client"

import {
  apiRequest,
  type FindingsScan,
  type PackageScan,
  type SCAScan,
} from "@/lib/api"

export type PlaygroundScan = SCAScan | FindingsScan | PackageScan

let playgroundScansCache: PlaygroundScan[] | null = null
let playgroundScansRequest: Promise<PlaygroundScan[]> | null = null
let playgroundScansCacheDay = ""

export function loadPlaygroundScans() {
  const today = playgroundDayKey()
  if (playgroundScansCache && playgroundScansCacheDay === today) {
    return Promise.resolve(playgroundScansCache)
  }

  if (!playgroundScansRequest) {
    playgroundScansRequest = apiRequest<{ scans: PlaygroundScan[] }>(
      "/api/v1/playground/scans"
    )
      .then((response) => {
        playgroundScansCache = response.scans ?? []
        playgroundScansCacheDay = today
        playgroundScansRequest = null
        return playgroundScansCache
      })
      .catch((error) => {
        playgroundScansRequest = null
        throw error
      })
  }

  return playgroundScansRequest
}

export function playgroundScansOfType<TScan extends PlaygroundScan>(
  scans: PlaygroundScan[],
  scanType: TScan["type"]
) {
  return scans.filter((scan): scan is TScan => scan.type === scanType)
}

function playgroundDayKey() {
  return new Date().toISOString().slice(0, 10)
}
