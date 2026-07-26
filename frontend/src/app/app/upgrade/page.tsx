import { redirect } from "next/navigation"

export default function UpgradePage() {
  redirect("/app/settings?tab=billing")
}
