"use client"

import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import * as React from "react"
import {
  BoxesIcon,
  CodeIcon,
  ContainerIcon,
  KeyRoundIcon,
  LogOutIcon,
  ScanLineIcon,
  ServerIcon,
  SettingsIcon,
  ShieldIcon,
  ShipWheelIcon,
  UserIcon,
} from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Skeleton } from "@/components/ui/skeleton"
import {
  apiRequest,
  clearToken,
  getStoredToken,
  type Entitlement,
  type User,
  type Workspace,
} from "@/lib/api"
import {
  PlatformProvider,
  type PlatformMode,
} from "@/components/runtz/platform-context"
import { RuntzMark, RuntzWordmark } from "@/components/runtz/logo"
import { ThemeToggle } from "@/components/runtz/theme-provider"
import { WorkspaceProvider } from "@/components/runtz/workspace-context"
import type { DeploymentMode } from "@/components/runtz/workspace-context"

type MeResponse = {
  user: User
  workspaces: Workspace[]
  deploymentMode: DeploymentMode
  entitlement: Entitlement
}

const ALL_WORKSPACES = "all"
const WORKSPACE_FILTER_KEY = "runtz_workspace_filter"

type NavItem = {
  label: string
  href?: string
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  comingSoon?: boolean
}

const PLAYGROUND_WORKSPACE_ID = "playground"
const PLAYGROUND_CREATED_AT = "2026-01-01T00:00:00Z"
const PLAYGROUND_WORKSPACES: Workspace[] = [
  {
    id: PLAYGROUND_WORKSPACE_ID,
    name: "Playground",
    slug: "playground",
    createdAt: PLAYGROUND_CREATED_AT,
    updatedAt: PLAYGROUND_CREATED_AT,
  },
]
const PLAYGROUND_USER: User = {
  id: "playground",
  username: "playground",
  displayName: "Playground",
  authProvider: "password",
  role: "member",
  workspaceIds: [PLAYGROUND_WORKSPACE_ID],
  requirePasswordChange: false,
  onboardingCompleted: true,
  createdAt: PLAYGROUND_CREATED_AT,
  updatedAt: PLAYGROUND_CREATED_AT,
}
const FREE_SELF_HOSTED_ENTITLEMENT: Entitlement = {
  plan: "free",
  deploymentMode: "self-hosted",
  status: "free",
  features: [],
}

const buildCodeItems = (basePath: string): NavItem[] => [
  { label: "SCA", href: `${basePath}/sca`, icon: BoxesIcon },
  { label: "SAST", href: `${basePath}/sast`, icon: CodeIcon },
  { label: "DAST", icon: ScanLineIcon, comingSoon: true },
]

const buildHostItems = (basePath: string): NavItem[] => [
  {
    label: "Container scanning",
    href: `${basePath}/containers`,
    icon: ContainerIcon,
  },
  { label: "Host scanning", href: `${basePath}/hosts`, icon: ServerIcon },
  { label: "K8s scanning", href: `${basePath}/k8s`, icon: ShipWheelIcon },
]

export function AppShell({
  children,
  mode = "app",
}: {
  children: React.ReactNode
  mode?: PlatformMode
}) {
  const router = useRouter()
  const pathname = usePathname()
  const isPlayground = mode === "playground"
  const basePath = isPlayground ? "/playground" : "/app"
  const codeItems = React.useMemo(() => buildCodeItems(basePath), [basePath])
  const hostItems = React.useMemo(() => buildHostItems(basePath), [basePath])
  const [token, setToken] = React.useState<string | null>(null)
  const [user, setUser] = React.useState<User | null>(null)
  const [workspaces, setWorkspaces] = React.useState<Workspace[]>([])
  const [deploymentMode, setDeploymentMode] =
    React.useState<DeploymentMode>("self-hosted")
  const [entitlement, setEntitlement] = React.useState<Entitlement>(
    FREE_SELF_HOSTED_ENTITLEMENT
  )
  const [selectedWorkspaceId, setSelectedWorkspaceIdState] =
    React.useState(ALL_WORKSPACES)
  const [loading, setLoading] = React.useState(true)

  const refreshWorkspaces = React.useCallback(async () => {
    if (isPlayground) {
      setWorkspaces(PLAYGROUND_WORKSPACES)
      setDeploymentMode("self-hosted")
      setEntitlement(FREE_SELF_HOSTED_ENTITLEMENT)
      setSelectedWorkspaceIdState(ALL_WORKSPACES)
      return
    }

    const activeToken = getStoredToken()
    if (!activeToken) {
      return
    }

    const response = await apiRequest<{ workspaces: Workspace[] }>(
      "/api/v1/workspaces",
      { token: activeToken }
    )
    const workspaces = response.workspaces ?? []
    setWorkspaces(workspaces)
    setSelectedWorkspaceIdState((current) => {
      if (current === ALL_WORKSPACES) {
        return ALL_WORKSPACES
      }
      if (current && workspaces.some((item) => item.id === current)) {
        return current
      }
      return ALL_WORKSPACES
    })
  }, [isPlayground])

  React.useEffect(() => {
    if (isPlayground) {
      setToken("playground")
      setUser(PLAYGROUND_USER)
      setWorkspaces(PLAYGROUND_WORKSPACES)
      setEntitlement(FREE_SELF_HOSTED_ENTITLEMENT)
      setSelectedWorkspaceIdState(ALL_WORKSPACES)
      setLoading(false)
      return
    }

    const activeToken = getStoredToken()
    if (!activeToken) {
      router.replace("/login")
      return
    }

    setToken(activeToken)
    apiRequest<MeResponse>("/api/v1/me", { token: activeToken })
      .then((response) => {
        setUser(response.user)
        setDeploymentMode(response.deploymentMode)
        setEntitlement(response.entitlement)
        const workspaces = response.workspaces ?? []
        setWorkspaces(workspaces)
        const storedWorkspaceId = window.localStorage.getItem(
          WORKSPACE_FILTER_KEY
        )
        const storedWorkspaceExists =
          storedWorkspaceId === ALL_WORKSPACES ||
          workspaces.some((workspace) => workspace.id === storedWorkspaceId)
        const nextWorkspaceId =
          storedWorkspaceId && storedWorkspaceExists
            ? storedWorkspaceId
            : ALL_WORKSPACES
        setSelectedWorkspaceIdState(nextWorkspaceId)
        if (
          response.deploymentMode === "cloud" &&
          !response.user.onboardingCompleted &&
          !sessionStorage.getItem("runtz_onboarding_seen")
        ) {
          sessionStorage.setItem("runtz_onboarding_seen", "1")
          router.replace("/app/onboarding")
        }
      })
      .catch(() => {
        clearToken()
        router.replace("/login")
      })
      .finally(() => setLoading(false))
  }, [isPlayground, pathname, router])

  function setSelectedWorkspaceId(workspaceId: string) {
    setSelectedWorkspaceIdState(workspaceId)
    window.localStorage.setItem(WORKSPACE_FILTER_KEY, workspaceId)
  }

  function logout() {
    clearToken()
    router.replace("/login")
  }

  if (loading || !token || !user) {
    return (
      <div className="flex min-h-svh items-center justify-center bg-background p-6">
        <div className="flex w-full max-w-sm flex-col gap-4">
          <Skeleton className="h-8 w-28" />
          <Skeleton className="h-44 w-full" />
        </div>
      </div>
    )
  }

  return (
    <PlatformProvider mode={mode}>
      <WorkspaceProvider
        value={{
          currentUser: user,
          deploymentMode,
          entitlement,
          workspaces,
          selectedWorkspaceId,
          setSelectedWorkspaceId,
          refreshWorkspaces,
        }}
      >
        <SidebarProvider className="runtz-app-surface">
          <Sidebar collapsible="icon" className="border-transparent">
            <div aria-hidden="true" className="runtz-dot-map pointer-events-none absolute inset-0 z-0 opacity-[0.10]" />
            <SidebarHeader className="relative z-10">
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    size="lg"
                    className="rounded-xl hover:bg-sidebar-accent data-active:bg-sidebar-accent"
                    render={<Link href={`${basePath}/overview`} />}
                  >
                    <div className="flex size-8 items-center justify-center overflow-hidden rounded-xl">
                      <RuntzMark className="size-full" />
                    </div>
                    <div className="flex min-w-0 flex-col gap-0.5 group-data-[collapsible=icon]:hidden">
                      <RuntzWordmark className="self-start text-sm" />
                      <span className="truncate text-xs text-muted-foreground">
                        DevSecOps Platform
                      </span>
                    </div>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarHeader>
            <SidebarContent className="relative z-10">
              <SidebarGroup>
                <SidebarGroupContent>
                  <SidebarMenu>
                    <SidebarItem
                      item={{
                        label: "Overview",
                        href: `${basePath}/overview`,
                        icon: ShieldIcon,
                      }}
                      active={pathname === `${basePath}/overview`}
                    />
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
              <SidebarGroup>
                <SidebarGroupLabel>CODE</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {codeItems.map((item) => (
                      <SidebarItem
                        key={item.label}
                        item={item}
                        active={item.href ? pathname.startsWith(item.href) : false}
                      />
                    ))}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
              <SidebarGroup>
                <SidebarGroupLabel>Hosts</SidebarGroupLabel>
                <SidebarGroupContent>
                  <SidebarMenu>
                    {hostItems.map((item) => (
                      <SidebarItem
                        key={item.label}
                        item={item}
                        active={item.href ? pathname.startsWith(item.href) : false}
                      />
                    ))}
                  </SidebarMenu>
                </SidebarGroupContent>
              </SidebarGroup>
            </SidebarContent>
            <SidebarFooter className="relative z-10">
              <SidebarMenu>
                <SidebarItem
                  item={{
                    label: "API Keys",
                    href: isPlayground ? undefined : "/app/api-keys",
                    icon: KeyRoundIcon,
                  }}
                  active={!isPlayground && pathname === "/app/api-keys"}
                />
                <SidebarItem
                  item={{
                    label: "Settings",
                    href: isPlayground ? undefined : "/app/settings",
                    icon: SettingsIcon,
                  }}
                  active={!isPlayground && pathname === "/app/settings"}
                />
              </SidebarMenu>
              <SidebarMenu>
                <SidebarMenuItem>
                  <AccountMenu
                    user={user}
                    isPlayground={isPlayground}
                    onProfile={() => router.push("/app/settings")}
                    onLogout={logout}
                  />
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarFooter>
          </Sidebar>
          <SidebarInset>
          <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background px-4 text-[#071222] backdrop-blur dark:border-[#213047] dark:bg-[#050912]/88 dark:text-[#eaf4ff]">
            <SidebarTrigger />
            <Separator orientation="vertical" className="h-5" />
            <div className="flex min-w-0 flex-1 items-center gap-3">
              <span className="text-sm font-medium text-muted-foreground">
                Workspace
              </span>
              <Select
                value={selectedWorkspaceId}
                onValueChange={(value) => {
                  if (value) {
                    setSelectedWorkspaceId(value)
                  }
                }}
                disabled={isPlayground}
              >
                <SelectTrigger className="w-56">
                  <SelectValue placeholder="Select" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value={ALL_WORKSPACES}>
                      All workspaces
                    </SelectItem>
                    {workspaces.map((workspace) => (
                      <SelectItem key={workspace.id} value={workspace.id}>
                        {workspace.name}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <Badge
              variant="outline"
              className="border-[#2f7eff]/20 bg-[#dcecff]/70 text-[#1d5fc7] dark:border-[#6db5ff]/24 dark:bg-[#101827] dark:text-[#b8cbe4]"
            >
              {isPlayground ? "playground" : deploymentMode}
            </Badge>
            <ThemeToggle />
          </header>
          <main className="min-h-0 flex-1 bg-transparent">{children}</main>
          </SidebarInset>
        </SidebarProvider>
      </WorkspaceProvider>
    </PlatformProvider>
  )
}

function AccountMenu({
  user,
  isPlayground,
  onProfile,
  onLogout,
}: {
  user: User
  isPlayground: boolean
  onProfile: () => void
  onLogout: () => void
}) {
  const [open, setOpen] = React.useState(false)
  const menuRef = React.useRef<HTMLDivElement | null>(null)

  React.useEffect(() => {
    if (!open) {
      return
    }

    function closeOnOutsideClick(event: MouseEvent) {
      if (
        menuRef.current &&
        !menuRef.current.contains(event.target as Node)
      ) {
        setOpen(false)
      }
    }

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false)
      }
    }

    document.addEventListener("mousedown", closeOnOutsideClick)
    document.addEventListener("keydown", closeOnEscape)
    return () => {
      document.removeEventListener("mousedown", closeOnOutsideClick)
      document.removeEventListener("keydown", closeOnEscape)
    }
  }, [open])

  return (
    <div ref={menuRef} className="relative">
      {open ? (
        <div
          role="menu"
          className="absolute bottom-[calc(100%+0.5rem)] left-0 z-50 w-60 overflow-hidden rounded-2xl border border-[#071222]/10 bg-[#f7fbff] p-2 text-[#071222] shadow-[0_18px_48px_rgb(32_64_96/0.18)] dark:border-[#6db5ff]/18 dark:bg-[#0d1420] dark:text-[#eaf4ff] dark:shadow-[0_24px_80px_rgb(0_0_0/0.42)]"
        >
          <div className="px-3 py-2">
            <div className="text-xs font-medium uppercase tracking-wide text-[#1d5fc7] dark:text-[#6db5ff]">
              Conta
            </div>
            <div className="mt-1 truncate text-sm font-semibold">
              {user.username}
            </div>
            <div className="truncate text-xs text-muted-foreground">{user.role}</div>
          </div>
          <div className="my-1 h-px bg-[#071222]/10 dark:bg-[#6db5ff]/14" />
          {isPlayground ? null : (
            <button
              type="button"
              role="menuitem"
              className="flex h-9 w-full items-center gap-2 rounded-xl px-3 text-sm text-[#102238] transition hover:bg-[#d8ebff] hover:text-[#071222] dark:text-[#c9dbf2] dark:hover:bg-[#172844] dark:hover:text-[#eaf4ff]"
              onClick={() => {
                setOpen(false)
                onProfile()
              }}
            >
              <UserIcon className="size-4 text-[#1d5fc7] dark:text-[#6db5ff]" />
              Profile
            </button>
          )}
          <button
            type="button"
            role="menuitem"
            className="flex h-9 w-full items-center gap-2 rounded-xl px-3 text-sm text-[#102238] transition hover:bg-[#d8ebff] hover:text-[#071222] dark:text-[#c9dbf2] dark:hover:bg-[#172844] dark:hover:text-[#eaf4ff]"
            onClick={() => {
              setOpen(false)
              onLogout()
            }}
          >
            <LogOutIcon className="size-4 text-[#1d5fc7] dark:text-[#6db5ff]" />
            {isPlayground ? "Login" : "Logout"}
          </button>
        </div>
      ) : null}
      <SidebarMenuButton
        aria-expanded={open}
        aria-haspopup="menu"
        className="h-12 rounded-xl border border-[#071222]/10 bg-[#f7fbff]/78 hover:bg-[#d8ebff] aria-expanded:bg-[#d8ebff] dark:border-[#6db5ff]/12 dark:bg-[#101827]/78 dark:hover:bg-[#172844] dark:aria-expanded:bg-[#172844]"
        size="lg"
        type="button"
        onClick={() => setOpen((current) => !current)}
      >
        <Avatar className="size-8 border border-[#2f7eff]/20 dark:border-[#6db5ff]/20">
          <AvatarFallback className="bg-[#dcecff] text-xs text-[#102238] dark:bg-[#050912] dark:text-[#d9e9ff]">
            {user.username.slice(0, 2).toUpperCase()}
          </AvatarFallback>
        </Avatar>
        <div className="flex min-w-0 flex-col text-left">
          <span className="truncate font-medium text-[#071222] dark:text-[#eaf4ff]">
            {user.username}
          </span>
          <span className="truncate text-xs text-muted-foreground">{user.role}</span>
        </div>
      </SidebarMenuButton>
    </div>
  )
}

function SidebarItem({ item, active }: { item: NavItem; active: boolean }) {
  const Icon = item.icon
  const content = (
    <>
      <Icon />
      <span>{item.label}</span>
      {item.comingSoon ? (
        <Badge variant="secondary" className="ml-auto">
          coming soon
        </Badge>
      ) : null}
    </>
  )

  return (
    <SidebarMenuItem>
      {item.href ? (
        <SidebarMenuButton
          tooltip={item.label}
          isActive={active}
          render={<Link href={item.href} />}
        >
          {content}
        </SidebarMenuButton>
      ) : (
        <SidebarMenuButton disabled tooltip={item.label}>
          {content}
        </SidebarMenuButton>
      )}
    </SidebarMenuItem>
  )
}
