"use client"

import * as React from "react"
import { gcpSetupDocumentation } from "@/lib/cloud"

export function CloudSetupGuideLink(props: Omit<React.ComponentProps<"a">, "href">) {
  const [href, setHref] = React.useState(gcpSetupDocumentation)

  React.useEffect(() => {
    // Hosted dev has its own docs deployment. Self-hosted installs use the public guide.
    if (window.location.hostname === "runtz-dev.runtz.dev") {
      const url = new URL(gcpSetupDocumentation)
      url.hostname = window.location.hostname
      setHref(url.href)
    }
  }, [])

  return <a {...props} href={href} />
}
