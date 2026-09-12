import { AppShell } from "@/components/runtz/app-shell"

export default function PlatformLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return <AppShell>{children}</AppShell>
}

