"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { CloudIcon, PlusIcon } from "lucide-react"
import { GCPIcon } from "@/components/runtz/cloud-icons"
import { useCloud } from "@/components/runtz/cloud-context"
import { useWorkspace } from "@/components/runtz/workspace-context"
import { SidebarGroup, SidebarGroupLabel, SidebarGroupContent, SidebarMenu, SidebarMenuItem, SidebarMenuButton } from "@/components/ui/sidebar"


export function CloudSidebar() {
  const { connections, openConnect, loading, error, refresh, canManage } = useCloud()
  const { workspaces } = useWorkspace()
  const pathname = usePathname()
  const hasGCP = connections.length > 0
  const canAdd = workspaces.some(w => canManage(w.id))
  return <SidebarGroup>
    <SidebarGroupLabel>Cloud</SidebarGroupLabel>
    <SidebarGroupContent><SidebarMenu>
      <SidebarMenuItem><SidebarMenuButton tooltip={hasGCP ? "Google Cloud" : "Cloud security"} isActive={pathname.startsWith("/cloud")} render={<Link href="/cloud/gcp" />}>
        {hasGCP ? <GCPIcon /> : <CloudIcon />}<span>{hasGCP ? "Google Cloud" : "Cloud security"}</span>
        {hasGCP && <span className="ml-auto text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">{connections.length}</span>}
      </SidebarMenuButton></SidebarMenuItem>
      {canAdd && <SidebarMenuItem><SidebarMenuButton tooltip="Add cloud" onClick={openConnect} className="text-muted-foreground"><PlusIcon /><span>Add cloud</span></SidebarMenuButton></SidebarMenuItem>}
      {error && !loading && <SidebarMenuItem><SidebarMenuButton onClick={() => void refresh()} tooltip="Retry cloud connections" className="text-xs text-muted-foreground"><CloudIcon /><span>Cloud unavailable · Retry</span></SidebarMenuButton></SidebarMenuItem>}
    </SidebarMenu></SidebarGroupContent>
  </SidebarGroup>
}
