import { SetupLogin } from "@/components/runtz/setup-login"
import * as React from "react"

export const dynamic = "force-dynamic"

export default function LoginPage() {
  return (
    <React.Suspense fallback={null}>
      <SetupLogin />
    </React.Suspense>
  )
}
