import { AppShell } from "@/components/runtz/app-shell"

export default function OnboardingLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return <AppShell>{children}</AppShell>
}
