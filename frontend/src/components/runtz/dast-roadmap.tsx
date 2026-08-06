import { ArrowUpRightIcon, ScanLineIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { buttonVariants } from "@/components/ui/button"
import { Empty, EmptyContent, EmptyHeader, EmptyMedia } from "@/components/ui/empty"

export default function DastRoadmap() {
  return (
    <main className="flex min-h-[calc(100svh-3.5rem)] items-center justify-center p-4 md:p-6">
      <Empty className="max-w-2xl border bg-card/80 py-16 shadow-sm md:py-20">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ScanLineIcon />
          </EmptyMedia>
          <Badge variant="secondary">Coming soon</Badge>
        </EmptyHeader>
        <EmptyContent>
          <a href="/home/roadmap" className={buttonVariants({ size: "lg" })}>
            View roadmap
            <ArrowUpRightIcon data-icon="inline-end" />
          </a>
        </EmptyContent>
      </Empty>
    </main>
  )
}
