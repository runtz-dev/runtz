import { CloudProjectPage } from "@/components/runtz/cloud-pages"
export default async function AWSAccountPage({ params }: { params: Promise<{ connectionId: string }> }) {
  const { connectionId } = await params
  return <CloudProjectPage connectionId={connectionId} provider="aws" />
}
