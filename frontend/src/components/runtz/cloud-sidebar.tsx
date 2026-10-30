"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { CloudIcon, PlusIcon } from "lucide-react"
import { GCPIcon } from "@/components/runtz/cloud-icons"
import { useCloud } from "@/components/runtz/cloud-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import {
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from "@/components/ui/sidebar"

export function CloudSidebar() {
  const { connections, openConnect, loading, error, refresh, canManage } = useCloud()
  const { workspaces } = useWorkspace()
  const pathname = usePathname()
  const hasGCP = connections.length > 0
  const canAdd = workspaces.some(w => canManage(w.id))
  return (
    <SidebarGroup className="group/cloud">
      <SidebarGroupLabel>Cloud</SidebarGroupLabel>
      {hasGCP && canAdd && (
        <SidebarGroupAction
          type="button"
          aria-label="Add cloud"
          title="Add cloud"
          onClick={openConnect}
          className="aspect-auto w-auto gap-1 px-1.5 text-xs text-muted-foreground opacity-0 transition-opacity group-hover/cloud:opacity-100 group-focus-within/cloud:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100"
        >
          <PlusIcon />
          <span>Cloud</span>
        </SidebarGroupAction>
      )}
      <SidebarGroupContent>
        <SidebarMenu>
          {hasGCP ? (
            <SidebarMenuItem>
              <SidebarMenuButton
                tooltip="Google Cloud"
                isActive={pathname.startsWith("/cloud/gcp")}
                render={<Link href="/cloud/gcp" />}
              >
                <GCPIcon />
                <span>Google Cloud</span>
                <span className="ml-auto text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
                  {connections.length}
                </span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ) : (
            <SidebarMenuItem>
              <SidebarMenuButton
                tooltip={canAdd ? "Add cloud" : "Ask your workspace owner to connect a cloud"}
                onClick={openConnect}
                disabled={!canAdd || loading}
                className="text-muted-foreground"
              >
                <CloudIcon />
                <span>+ Add cloud</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )}
          {hasGCP && canAdd && (
            <SidebarMenuItem className="hidden group-data-[collapsible=icon]:block">
              <SidebarMenuButton tooltip="Add cloud" onClick={openConnect}>
                <PlusIcon />
                <span>Add cloud</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )}
          {error && !loading && (
            <SidebarMenuItem>
              <SidebarMenuButton
                onClick={() => void refresh()}
                tooltip="Retry cloud connections"
                className="text-xs text-muted-foreground"
              >
                <CloudIcon />
                <span>Cloud unavailable · Retry</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}
