"use client"

import * as React from "react"

export type PlatformMode = "app" | "playground"

type PlatformContextValue = {
  mode: PlatformMode
  basePath: "/app" | "/playground"
  isPlayground: boolean
}

const DEFAULT_PLATFORM: PlatformContextValue = {
  mode: "app",
  basePath: "/app",
  isPlayground: false,
}

const PlatformContext =
  React.createContext<PlatformContextValue>(DEFAULT_PLATFORM)

export function PlatformProvider({
  children,
  mode,
}: {
  children: React.ReactNode
  mode: PlatformMode
}) {
  const value = React.useMemo<PlatformContextValue>(
    () => ({
      mode,
      basePath: mode === "playground" ? "/playground" : "/app",
      isPlayground: mode === "playground",
    }),
    [mode]
  )

  return (
    <PlatformContext.Provider value={value}>
      {children}
    </PlatformContext.Provider>
  )
}

export function usePlatform() {
  return React.useContext(PlatformContext)
}
