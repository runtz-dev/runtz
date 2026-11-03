"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { CloudIcon, PlusIcon } from "lucide-react"
import { CloudProviderIcon } from "@/components/runtz/cloud-icons"
import { cloudProviderNames, type CloudProvider } from "@/lib/cloud"
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
  const hasConnections = connections.length > 0
  const providers: CloudProvider[] = ["gcp", "azure"]
  const canAdd = workspaces.some(w => canManage(w.id))
  return (
    <SidebarGroup className="group/cloud">
      <SidebarGroupLabel>Cloud</SidebarGroupLabel>
      {hasConnections && canAdd && (
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
          {hasConnections ? providers.filter(provider => connections.some(c => c.provider === provider)).map(provider => (
            <SidebarMenuItem key={provider}>
              <SidebarMenuButton
                tooltip={cloudProviderNames[provider]}
                isActive={pathname.startsWith(`/cloud/${provider}`)}
                render={<Link href={`/cloud/${provider}`} />}
              >
                <CloudProviderIcon provider={provider} />
                <span>{cloudProviderNames[provider]}</span>
                <span className="ml-auto text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
                  {connections.filter(c => c.provider === provider).length}
                </span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )) : (
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
          {hasConnections && canAdd && (
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
