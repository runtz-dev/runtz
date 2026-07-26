import { AppShell } from "@/components/runtz/app-shell"

export default function PlaygroundLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return <AppShell mode="playground">{children}</AppShell>
}
