import { CloudProjectPage } from "@/components/runtz/cloud-pages"

export default async function AzureSubscriptionPage({ params }: { params: Promise<{ connectionId: string }> }) {
  const { connectionId } = await params
  return <CloudProjectPage connectionId={connectionId} provider="azure" />
}
