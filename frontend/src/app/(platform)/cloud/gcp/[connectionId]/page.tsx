import { GCPProjectPage } from "@/components/runtz/cloud-pages"

export default async function GoogleCloudProjectPage({ params }: { params: Promise<{ connectionId: string }> }) {
  const { connectionId } = await params
  return <GCPProjectPage connectionId={connectionId} />
}
