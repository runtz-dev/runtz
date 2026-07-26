"use client"

import * as React from "react"

import type { Entitlement, User, Workspace } from "@/lib/api"

export type DeploymentMode = "cloud" | "self-hosted"

type WorkspaceContextValue = {
  currentUser: User
  deploymentMode: DeploymentMode
  entitlement: Entitlement
  workspaces: Workspace[]
  selectedWorkspaceId: string
  setSelectedWorkspaceId: (workspaceId: string) => void
  refreshWorkspaces: () => Promise<void>
}

const WorkspaceContext = React.createContext<WorkspaceContextValue | null>(null)

export function WorkspaceProvider({
  children,
  value,
}: {
  children: React.ReactNode
  value: WorkspaceContextValue
}) {
  return (
    <WorkspaceContext.Provider value={value}>
      {children}
    </WorkspaceContext.Provider>
  )
}

export function useWorkspace() {
  const context = React.useContext(WorkspaceContext)
  if (!context) {
    throw new Error("useWorkspace must be used inside WorkspaceProvider")
  }

  return context
}
