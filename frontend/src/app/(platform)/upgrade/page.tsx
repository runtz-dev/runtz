import { redirect } from "next/navigation"

export default function UpgradePage() {
  redirect("/settings?tab=billing")
}
