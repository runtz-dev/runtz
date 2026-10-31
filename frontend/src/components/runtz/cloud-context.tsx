"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { CloudConnectDialog } from "@/components/runtz/cloud-connect-dialog"
import { cloudError, listCloudConnections, type CloudConnection } from "@/lib/cloud"

type CloudContextValue = {
  connections: CloudConnection[]; loading: boolean; error: string; configured: boolean
  refresh: () => Promise<void>; openConnect: () => void
  canManage: (workspaceId: string) => boolean
}
const CloudContext = React.createContext<CloudContextValue | null>(null)

export function CloudProvider({ children, disabled = false }: { children: React.ReactNode; disabled?: boolean }) {
  const { selectedWorkspaceId, currentUser, deploymentMode, workspaces } = useWorkspace()
  const router = useRouter()
  const [connections, setConnections] = React.useState<CloudConnection[]>([])
  const [loading, setLoading] = React.useState(!disabled)
  const [error, setError] = React.useState("")
  const [configured, setConfigured] = React.useState(false)
  const [open, setOpen] = React.useState(false)
  const requestSequence = React.useRef(0)
  const foregroundSequence = React.useRef(0)
  const backgroundInFlight = React.useRef(false)
  const refresh = React.useCallback(async (signal?: AbortSignal, quiet = false) => {
    if (disabled) return
    if (quiet && (foregroundSequence.current !== 0 || backgroundInFlight.current)) return
    if (quiet) backgroundInFlight.current = true
    const sequence = ++requestSequence.current
    if (!quiet) { foregroundSequence.current = sequence; setLoading(true) }
    try {
      const response = await listCloudConnections(selectedWorkspaceId, signal)
      if (sequence !== requestSequence.current || signal?.aborted) return
      setConnections(response.connections); setConfigured(response.configured); setError("")
    } catch (err) { if (sequence === requestSequence.current && !signal?.aborted) setError(cloudError(err)) }
    finally {
      if (quiet) backgroundInFlight.current = false
      if (!quiet && foregroundSequence.current === sequence) { foregroundSequence.current = 0; setLoading(false) }
    }
  }, [disabled, selectedWorkspaceId])
  React.useEffect(() => {
    setConnections([])
    const controller = new AbortController()
    void refresh(controller.signal)
    return () => controller.abort()
  }, [refresh])
  const hasConnections = connections.length > 0
  const hasActiveImport = connections.some(c => c.status === "syncing" || c.status === "connecting")
  React.useEffect(() => {
    if (!hasConnections) return
    const timer = setInterval(() => { void refresh(undefined, true) }, hasActiveImport ? 3000 : 30000)
    return () => clearInterval(timer)
  }, [hasConnections, hasActiveImport, refresh])
  const canManage = React.useCallback((workspaceId: string) => {
    if (disabled) return false
    return deploymentMode === "self-hosted" ? currentUser.role === "admin" : workspaces.some(w => w.id === workspaceId && w.createdBy === currentUser.id)
  }, [disabled, deploymentMode, currentUser, workspaces])
  return <CloudContext.Provider value={{ connections, loading, error, configured, refresh, openConnect: () => setOpen(true), canManage }}>
    {children}
    {open && !disabled && <CloudConnectDialog onClose={() => setOpen(false)} onCreated={refresh} onConnected={async connection => {
      await refresh()
      router.push(`/cloud/gcp/${connection.id}`)
    }} />}
  </CloudContext.Provider>
}
export function useCloud() {
  const context = React.useContext(CloudContext)
  if (!context) throw new Error("useCloud must be used inside CloudProvider")
  return context
}
