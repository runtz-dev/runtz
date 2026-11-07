import type * as React from "react"
import { cn } from "@/lib/utils"
import type { CloudProvider } from "@/lib/cloud"

export function AzureIcon({ className, ...props }: React.SVGProps<SVGSVGElement>) {
  return <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="currentColor" stroke="none" aria-hidden="true" focusable="false" className={cn("shrink-0", className)} {...props}><path d="M9.5 2H16L8.9 22H2.4L9.5 2Zm7.1 0 7 20H9.8l5.3-3.1h4.1L13.7 2h2.9Z" /></svg>
}

export function CloudProviderIcon({ provider, ...props }: React.SVGProps<SVGSVGElement> & { provider: CloudProvider }) {
  return provider === "aws" ? <AWSIcon {...props} /> : provider === "azure" ? <AzureIcon {...props} /> : <GCPIcon {...props} />
}

// Google Cloud symbol: https://www.gstatic.com/pantheon/images/welcome/supercloud.svg
// Preserve the official geometry and inherit the surrounding UI's icon color.
export function GCPIcon({ className, ...props }: React.SVGProps<SVGSVGElement>) {
  return <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 97.75 78.62" fill="currentColor" stroke="none" aria-hidden="true" focusable="false" className={cn("shrink-0", className)} {...props}>
    <path d="M62.21 21.73l3.13.06 8.53-8.51.42-3.6a38.2 38.2 0 0 0-62.3 18.57c.88-.64 2.88-.16 2.88-.16l17-2.8s.88-1.44 1.31-1.35a21.19 21.19 0 0 1 29-2.21z" />
    <path d="M85.79 28.27a38.32 38.32 0 0 0-11.54-18.6l-12 12a21.2 21.2 0 0 1 7.92 16.53v2.19a10.63 10.63 0 1 1 0 21.25h-21.3l-2.12 2.14v12.73l2.12 2.12h21.25a27.61 27.61 0 0 0 15.67-50.36z" />
    <path d="M27.62 78.63h21.25v-17H27.62a10.54 10.54 0 0 1-4.38-1l-3.06.94-8.51 8.55-.74 2.88a27.47 27.47 0 0 0 16.69 5.63z" />
    <path d="M27.62 23.39A27.61 27.61 0 0 0 10.94 73l12.32-12.32a10.62 10.62 0 1 1 14-14l12.36-12.36a27.6 27.6 0 0 0-22-10.93z" />
  </svg>
}

export function AWSIcon({ className, ...props }: React.SVGProps<SVGSVGElement>) {
  return <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 32 24" fill="none" aria-hidden="true" focusable="false" className={cn("shrink-0", className)} {...props}><text x="1" y="14" fill="currentColor" fontFamily="Arial, sans-serif" fontSize="16" fontWeight="600">aws</text><path d="M3 18c7 5 17 5 25-1m-5-1 6 .2-1.8 5" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" /></svg>
}
