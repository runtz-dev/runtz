"use client"

import * as React from "react"
import { DownloadIcon, ExternalLinkIcon, MonitorIcon, TerminalIcon } from "lucide-react"
import { CloudCommand } from "@/components/runtz/cloud-command"
import { Button } from "@/components/ui/button"
import { awsReadOnlyPolicy, cloudSetupCommands } from "@/lib/cloud-guide"
import type { CloudProvider } from "@/lib/cloud"

const consoleInstructions: Record<CloudProvider, React.ReactNode[]> = {
  gcp: [<>Open <strong>IAM &amp; Admin → Service Accounts</strong> in the target project and create a dedicated account named <code>runtz-security</code>.</>, <>Grant <strong>Security Reviewer</strong> on the target project. Additional read permissions needed by a check appear in its coverage report.</>, <>Open the account&apos;s <strong>Keys → Add key → Create new key → JSON</strong>. Keep the downloaded file on your machine.</>],
  aws: [<>Open <strong>IAM → Users → Create user</strong> in the target account. Create a dedicated user without console access.</>, <>In <strong>Permissions → Add permissions → Create inline policy → JSON</strong>, paste the read-only policy below and save it as <code>RuntzReadOnly</code>.</>, <>Open <strong>Security credentials → Create access key</strong>. Choose the use case matching where you run Runtz, then save the access key ID and secret access key.</>],
  azure: [<>In <strong>Microsoft Entra ID → App registrations</strong>, register a dedicated application. Copy its application ID and directory ID.</>, <>In <strong>Subscriptions → Access control (IAM)</strong>, assign the application&apos;s service principal the <strong>Reader</strong> role on the target subscription.</>, <>Open <strong>Certificates &amp; secrets → New client secret</strong> and copy the secret&apos;s <strong>value</strong>. You will also need the subscription ID.</>],
}
const consoleURLs: Record<CloudProvider, string> = { gcp: "https://console.cloud.google.com/iam-admin/serviceaccounts", aws: "https://console.aws.amazon.com/iam/home#/users", azure: "https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" }

export function CloudAccessGuide({ provider, scope }: { provider: CloudProvider; scope: string }) {
  const [method, setMethod] = React.useState<"console" | "cli">("console")
  // Only validated IDs reach shell snippets; placeholders are safe literals.
  const safeScope = provider === "gcp" ? (/^[a-z][a-z0-9-]{4,61}[a-z0-9]$/.test(scope) ? scope : "YOUR_PROJECT_ID") : provider === "aws" ? (/^\d{12}$/.test(scope) ? scope : "YOUR_ACCOUNT_ID") : (/^[a-f0-9-]{36}$/i.test(scope) ? scope : "YOUR_SUBSCRIPTION_ID")
  const cliName = { gcp: "gcloud", aws: "AWS CLI", azure: "Azure CLI" }[provider]
  function downloadPolicy() {
    const url = URL.createObjectURL(new Blob([JSON.stringify(awsReadOnlyPolicy, null, 2)], { type: "application/json" }))
    const link = document.createElement("a"); link.href = url; link.download = "runtz-readonly-policy.json"; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  return <div className="min-w-0 space-y-4">
    <div className="flex flex-wrap gap-1 rounded-lg border bg-muted/20 p-1" aria-label="Cloud access setup method">
      <Button variant={method === "console" ? "secondary" : "ghost"} size="sm" aria-pressed={method === "console"} onClick={() => setMethod("console")}><MonitorIcon />Via console</Button>
      <Button variant={method === "cli" ? "secondary" : "ghost"} size="sm" aria-pressed={method === "cli"} onClick={() => setMethod("cli")}><TerminalIcon />Via {cliName}</Button>
    </div>
    {method === "console" ? <><ol className="list-decimal space-y-3 pl-5 text-sm leading-6">{consoleInstructions[provider].map((instruction, index) => <li key={index}>{instruction}</li>)}</ol><a href={consoleURLs[provider]} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline">Open cloud console<ExternalLinkIcon className="size-3.5" /></a></> : <><p className="text-sm leading-6 text-muted-foreground">Run these setup commands with an administrator identity authorized to create the account and grant read access. The Runtz scanner itself only reads configuration.</p>{provider === "aws" && <p className="text-xs text-muted-foreground">First confirm that <code>aws sts get-caller-identity</code> returns account {safeScope}. Download the policy to your current directory before running <code>put-user-policy</code>.</p>}<CloudCommand label={`Show ${cliName} setup commands`} value={cloudSetupCommands(provider, safeScope)} />{provider === "azure" && <p className="text-xs leading-5 text-muted-foreground">Use a unique application name if runtz-security already exists. From the output, <code>appId</code> is the client ID, <code>tenant</code> is the tenant ID and <code>password</code> is the client secret value.</p>}</>}
    {provider === "aws" && <div className="space-y-2"><Button variant="outline" size="sm" onClick={downloadPolicy}><DownloadIcon />Download read-only policy</Button><CloudCommand label="Review AWS read-only policy" value={JSON.stringify(awsReadOnlyPolicy, null, 2)} /></div>}
  </div>
}
