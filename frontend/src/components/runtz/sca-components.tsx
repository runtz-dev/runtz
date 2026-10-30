"use client"

import * as React from "react"
import {
  ChartSplineIcon,
  PackageIcon,
  ShieldAlertIcon,
  TrendingDownIcon,
  TrendingUpIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { useVulnerabilityFilter } from "@/components/runtz/vulnerability-filter"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import type { ScanSummary } from "@/lib/api"
import {
  aggregateSummaries,
  filterScanSummary,
  type CVEFixFilter,
  type TrendScan,
} from "@/lib/dashboard"
import {
  countSeverity,
  formatDate,
  SEVERITY_KEYS,
  totalSeverity,
  type SeverityKey,
} from "@/lib/sca"
import { cn } from "@/lib/utils"

const SEVERITY_META: Record<
  SeverityKey,
  {
    label: string
    segmentClassName: string
    textClassName: string
    badgeClassName: string
  }
> = {
  critical: {
    label: "critical",
    segmentClassName: "bg-severity-critical",
    textClassName: "text-severity-critical",
    badgeClassName: "border-severity-critical/40 text-severity-critical",
  },
  high: {
    label: "high",
    segmentClassName: "bg-severity-high",
    textClassName: "text-severity-high",
    badgeClassName: "border-severity-high/40 text-severity-high",
  },
  medium: {
    label: "medium",
    segmentClassName: "bg-severity-medium",
    textClassName: "text-severity-medium",
    badgeClassName: "border-severity-medium/40 text-severity-medium",
  },
  low: {
    label: "low",
    segmentClassName: "bg-severity-low",
    textClassName: "text-severity-low",
    badgeClassName: "border-severity-low/40 text-severity-low",
  },
  unknown: {
    label: "unknown",
    segmentClassName: "bg-severity-unknown",
    textClassName: "text-severity-unknown",
    badgeClassName: "border-severity-unknown/40 text-severity-unknown",
  },
}

export function MetricCard({
  title,
  value,
}: {
  title: string
  value: number
}) {
  return (
    <Card className="justify-center">
      <CardHeader>
        <CardDescription>{title}</CardDescription>
        <CardTitle className="text-3xl">{value}</CardTitle>
      </CardHeader>
    </Card>
  )
}

export function DashboardSummaryGrid({
  children,
  summary,
  trendScans,
  trendContext,
  trendFilter = "all",
}: {
  children: React.ReactNode
  summary: ScanSummary
  trendScans: TrendScan[]
  trendContext: string
  trendFilter?: CVEFixFilter
}) {
  return (
    <div className="grid grid-cols-2 gap-4 xl:grid-cols-[repeat(2,minmax(0,1fr))_minmax(18rem,1.15fr)]">
      {children}
      <SeverityCard
        summary={summary}
        className="col-span-2 xl:col-span-1"
        trendScans={trendScans}
        trendContext={trendContext}
        trendFilter={trendFilter}
      />
    </div>
  )
}

export function SeverityDistribution({
  summary,
  className,
}: {
  summary: ScanSummary
  className?: string
}) {
  const total = totalSeverity(summary)

  return (
    <div className={cn("flex w-full max-w-sm flex-col gap-2", className)}>
      <TooltipProvider>
      <div className="flex h-3 overflow-hidden rounded-full bg-muted ring-1 ring-border">
        {total > 0 ? (
          SEVERITY_KEYS.map((severity) => {
            const value = countSeverity(summary, severity)
            if (value === 0) {
              return null
            }

            return (
              <SeverityTooltip
                key={severity}
                severity={severity}
                total={total}
                value={value}
              >
                <span
                  className={cn(
                    "h-full cursor-default transition-[filter] hover:brightness-125 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-white/80",
                    SEVERITY_META[severity].segmentClassName
                  )}
                  style={{ flexGrow: value, flexBasis: 0 }}
                  tabIndex={0}
                />
              </SeverityTooltip>
            )
          })
        ) : (
          <div className="h-full w-full bg-muted-foreground/25" />
        )}
      </div>
      <div className="grid grid-cols-5 gap-2 text-center text-xs">
        {SEVERITY_KEYS.map((severity) => {
          const meta = SEVERITY_META[severity]
          const value = countSeverity(summary, severity)
          return (
            <SeverityTooltip
              key={severity}
              severity={severity}
              total={total}
              value={value}
            >
              <span
                className="flex min-w-0 cursor-default flex-col gap-1 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                tabIndex={0}
              >
                <span className={cn("h-1 rounded-full", meta.segmentClassName)} />
                <span className={cn("font-medium", meta.textClassName)}>
                  {value}
                </span>
              </span>
            </SeverityTooltip>
          )
        })}
      </div>
      </TooltipProvider>
    </div>
  )
}

function SeverityTooltip({
  severity,
  total,
  value,
  children,
}: {
  severity: SeverityKey
  total: number
  value: number
  children: React.ReactElement
}) {
  const percentage = total === 0 ? 0 : Math.round((value / total) * 100)
  const label = `${SEVERITY_META[severity].label} ${percentage}%`

  return (
    <Tooltip>
      <TooltipTrigger render={children} aria-label={`${label}, ${value} vulnerabilities`} />
      <TooltipContent>
        <span className="font-semibold">{label}</span>
        <span className="opacity-70">· {value} CVEs</span>
      </TooltipContent>
    </Tooltip>
  )
}

export function SeverityCard({
  summary,
  className,
  trendScans,
  trendContext,
  trendFilter,
}: {
  summary: ScanSummary
  className?: string
  trendScans: TrendScan[]
  trendContext: string
  trendFilter: CVEFixFilter
}) {
  return (
    <Card size="sm" className={className}>
      <CardHeader>
        <div className="flex items-center gap-1">
          <CardTitle>Vulnerabilities</CardTitle>
          <VulnerabilityTrendDialog
            scans={trendScans}
            context={trendContext}
            filter={trendFilter}
          />
        </div>
      </CardHeader>
      <CardContent>
        <SeverityDistribution summary={summary} className="max-w-none" />
      </CardContent>
    </Card>
  )
}

type TrendPeriod = 7 | 15 | 30

const TREND_PERIODS: TrendPeriod[] = [7, 15, 30]
const SVG_CHART_TOOLTIP_WIDTH = 176

const TREND_SERIES = [
  { key: "critical", label: "Critical", color: "#ff6b74" },
  { key: "high", label: "High", color: "#ff9a62" },
  { key: "medium", label: "Medium", color: "#ffd166" },
  { key: "low", label: "Low", color: "#6db5ff" },
  { key: "unknown", label: "Unknown", color: "#8fa0b7" },
] as const

// Render order bottom to top so critical is the topmost visual layer.
const STACK_ORDER = ["unknown", "low", "medium", "high", "critical"] as const

function VulnerabilityTrendDialog({
  scans,
  context,
  filter,
}: {
  scans: TrendScan[]
  context: string
  filter: CVEFixFilter
}) {
  return (
    <Dialog>
      <TooltipProvider delay={200}>
        <Tooltip>
          <TooltipTrigger
            render={
              <DialogTrigger
                render={<Button variant="ghost" size="icon-sm" />}
                aria-label={`View vulnerability trend: ${context}`}
              />
            }
          >
            <ChartSplineIcon aria-hidden="true" />
          </TooltipTrigger>
          <TooltipContent>View vulnerability trend</TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-4xl">
        <DialogHeader className="pr-8">
          <DialogTitle>Vulnerability trend</DialogTitle>
          <DialogDescription className="break-words">
            {context}
            {filter === "with-fix" ? " · With fixes available" : null}
            {filter === "without-fix" ? " · Without fixes available" : null}
          </DialogDescription>
        </DialogHeader>
        {scans.length > 0 ? (
          <VulnerabilityChart scans={scans} filter={filter} />
        ) : (
          <Empty className="min-h-48">
            <EmptyHeader>
              <EmptyMedia variant="icon"><ChartSplineIcon /></EmptyMedia>
              <EmptyTitle>No scan history yet</EmptyTitle>
              <EmptyDescription>
                The vulnerability trend will appear after the first scan.
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
      </DialogContent>
    </Dialog>
  )
}

function VulnerabilityChart({
  scans,
  filter,
}: {
  scans: TrendScan[]
  filter: CVEFixFilter
}) {
  const [period, setPeriod] = React.useState<TrendPeriod>(30)
  const [hoveredIndex, setHoveredIndex] = React.useState<number | null>(null)
  const [keyboardInspection, setKeyboardInspection] = React.useState(false)
  const [chartWidth, setChartWidth] = React.useState(800)
  const containerRef = React.useRef<HTMLDivElement>(null)
  const descriptionId = React.useId()

  React.useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const observer = new ResizeObserver(([entry]) => {
      if (entry.contentRect.width > 0) {
        setChartWidth(Math.round(entry.contentRect.width))
      }
    })
    observer.observe(container)
    return () => observer.disconnect()
  }, [])

  const chart = React.useMemo(
    () => buildTrendChart(scans, period, filter, chartWidth),
    [filter, period, scans, chartWidth]
  )
  const hoveredDay = hoveredIndex === null ? null : chart.days[hoveredIndex]
  const inspectPointer = (event: React.MouseEvent<SVGSVGElement>) => {
    const bounds = event.currentTarget.getBoundingClientRect()
    const x = ((event.clientX - bounds.left) / bounds.width) * chartWidth
    setKeyboardInspection(false)
    setHoveredIndex(Math.max(0, Math.min(period - 1,
      Math.floor(((x - chart.left) / chart.width) * period)
    )))
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          Latest known severity state per asset each day.
        </p>
        <TrendPeriodSelector
          period={period}
          onChange={(value) => {
            setHoveredIndex(null)
            setPeriod(value)
          }}
        />
      </div>
      <div ref={containerRef} className="min-w-0">
        <svg
          viewBox={`0 0 ${chartWidth} 320`}
          className="block h-80 w-full rounded-sm focus-visible:outline-2 focus-visible:outline-ring"
          role="img"
          tabIndex={0}
          aria-label={`Vulnerability trend over the last ${period} days. Use left and right arrow keys to inspect a date.`}
          aria-describedby={descriptionId}
          onPointerMove={inspectPointer}
          onClick={inspectPointer}
          onPointerLeave={(event) => {
            if (event.pointerType === "mouse") setHoveredIndex(null)
          }}
          onFocus={(event) => {
            if (event.currentTarget.matches(":focus-visible")) {
              setKeyboardInspection(true)
              setHoveredIndex(period - 1)
            }
          }}
          onBlur={() => setHoveredIndex(null)}
          onKeyDown={(event) => {
            if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return
            event.preventDefault()
            setKeyboardInspection(true)
            setHoveredIndex((current) => {
              if (event.key === "Home") return 0
              if (event.key === "End") return period - 1
              return Math.max(0, Math.min(period - 1,
                (current ?? period - 1) + (event.key === "ArrowLeft" ? -1 : 1)
              ))
            })
          }}
        >
          <text x={chart.left} y="12" className="fill-muted-foreground text-xs">
            Vulnerabilities
          </text>
          {chart.gridLines.map((line) => (
            <g key={line.value}>
              <line
                x1={chart.left}
                x2={chart.left + chart.width}
                y1={line.y}
                y2={line.y}
                stroke="currentColor"
                className="text-border"
              />
              <text x={chart.left - 10} y={line.y + 4} textAnchor="end" className="fill-muted-foreground text-xs">
                {line.value}
              </text>
            </g>
          ))}
          {chart.bands.map((band) => (
            <g key={band.key}>
              <polygon points={band.fillPoints} fill={band.color} fillOpacity="0.20" />
              <polyline
                points={band.topPoints}
                fill="none"
                stroke={band.color}
                strokeWidth="1.5"
                strokeLinejoin="round"
                strokeLinecap="round"
              />
            </g>
          ))}
          {chart.labels.map((label) => (
            <text key={label.key} x={label.x} y="308" textAnchor={label.textAnchor} className="fill-muted-foreground text-xs">
              {label.label}
            </text>
          ))}
          {hoveredDay ? (
            <>
              <line x1={hoveredDay.x} x2={hoveredDay.x} y1={chart.top} y2={chart.bottom} stroke="currentColor" strokeDasharray="4 4" className="text-primary/70" />
              <circle cx={hoveredDay.x} cy={hoveredDay.y} r="4" className="fill-primary stroke-popover" strokeWidth="2" />
              <SvgChartTooltip
                x={hoveredDay.tooltipX}
                y={chart.top + 8}
                title={hoveredDay.label}
                totalLabel="Total"
                totalValue={hoveredDay.total}
                items={hoveredDay.items}
              />
            </>
          ) : null}
        </svg>
        <p id={descriptionId} className="sr-only" aria-live={keyboardInspection ? "polite" : "off"}>
          {hoveredDay
            ? `${hoveredDay.label}: ${hoveredDay.total} vulnerabilities. ${hoveredDay.items.map((item) => `${item.label}: ${item.value}`).join(", ")}.`
            : "Daily counts by severity based on the latest scan available for each asset."}
        </p>
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-2 border-t pt-3 text-xs text-muted-foreground">
        {TREND_SERIES.map((series) => (
          <span key={series.key} className="inline-flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm" style={{ backgroundColor: series.color }} />
            {series.label}
          </span>
        ))}
      </div>
    </div>
  )
}

function SvgChartTooltip({
  x,
  y,
  title,
  totalLabel,
  totalValue,
  items,
}: {
  x: number
  y: number
  title: string
  totalLabel?: string
  totalValue?: number
  items: Array<{ label: string; value: number; color: string }>
}) {
  const height = 44 + items.length * 20 + (totalLabel ? 26 : 0)

  return (
    <foreignObject
      x={x}
      y={y}
      width={SVG_CHART_TOOLTIP_WIDTH}
      height={height}
      className="pointer-events-none overflow-visible"
    >
      <div className="rounded-lg border bg-popover/95 p-2.5 text-xs leading-tight text-popover-foreground shadow-lg">
        <div className="mb-2 font-semibold">{title}</div>
        <div className="flex flex-col gap-1.5">
          {items.map((item) => (
            <div key={item.label} className="flex items-center justify-between gap-3">
              <span className="flex items-center gap-1.5 text-muted-foreground">
                <span
                  className="size-2 rounded-full"
                  style={{ backgroundColor: item.color }}
                />
                {item.label}
              </span>
              <strong className="font-semibold text-foreground">{item.value}</strong>
            </div>
          ))}
        </div>
        {totalLabel ? (
          <div className="mt-2 flex items-center justify-between border-t pt-2">
            <span className="text-muted-foreground">{totalLabel}</span>
            <strong className="font-semibold text-foreground">{totalValue}</strong>
          </div>
        ) : null}
      </div>
    </foreignObject>
  )
}

function TrendPeriodSelector({
  period,
  onChange,
}: {
  period: TrendPeriod
  onChange: (period: TrendPeriod) => void
}) {
  return (
    <div className="flex w-fit rounded-lg border bg-muted/35 p-1">
      {TREND_PERIODS.map((value) => (
        <button
          key={value}
          type="button"
          aria-label={`Show last ${value} days`}
          aria-pressed={period === value}
          className={cn(
            "rounded-md px-3 py-1.5 text-xs font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            period === value
              ? "bg-primary text-primary-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          )}
          onClick={() => onChange(value)}
        >
          {value}d
        </button>
      ))}
    </div>
  )
}

function buildTrendChart(
  scans: TrendScan[],
  period: TrendPeriod,
  filter: CVEFixFilter,
  chartWidth: number
) {
  const left = 42
  const width = Math.max(1, chartWidth - left - 16)
  const height = 250
  const top = 24
  const bottom = top + height
  const now = new Date()
  now.setHours(0, 0, 0, 0)
  const days = Array.from({ length: period }, (_, index) => {
    const date = new Date(now)
    date.setDate(date.getDate() - (period - index - 1))
    return {
      date,
      key: dayKey(date),
      scans: 0,
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      unknown: 0,
    }
  })
  const orderedScans = scans
    .map((scan, index) => ({
      scan,
      index,
      timestamp: new Date(scan.createdAt).getTime(),
    }))
    .filter(({ timestamp }) => Number.isFinite(timestamp))
    .sort((left, right) => left.timestamp - right.timestamp)
  const latestByAsset = new Map<string, ScanSummary>()
  let scanIndex = 0

  for (const day of days) {
    const nextDay = new Date(day.date)
    nextDay.setDate(nextDay.getDate() + 1)

    while (
      scanIndex < orderedScans.length &&
      orderedScans[scanIndex].timestamp < nextDay.getTime()
    ) {
      const current = orderedScans[scanIndex]
      latestByAsset.set(
        trendAssetKey(current.scan, current.index),
        filterScanSummary(current.scan.summary, filter)
      )
      scanIndex++
    }

    const summary = aggregateSummaries(Array.from(latestByAsset.values()))
    day.scans = latestByAsset.size
    for (const series of TREND_SERIES) {
      day[series.key] = summary[series.key]
    }
  }

  // Compute per-day cumulative stacks (bottom → top: unknown, low, medium, high, critical)
  type DayStack = { base: number; top: number }
  const dayStacks: Array<Record<string, DayStack>> = days.map((day) => {
    let cum = 0
    const stacks: Record<string, DayStack> = {}
    for (const key of STACK_ORDER) {
      stacks[key] = { base: cum, top: cum + day[key] }
      cum += day[key]
    }
    return stacks
  })
  const dailyTotals = days.map((day) =>
    TREND_SERIES.reduce((s, series) => s + day[series.key], 0)
  )
  const maxTotal = Math.max(1, ...dailyTotals)
  const roundedMax = Math.max(4, Math.ceil(maxTotal / 4) * 4)
  const x = (index: number) =>
    left + ((index + 0.5) / days.length) * width
  const y = (value: number) => bottom - (value / roundedMax) * height
  const hitboxWidth = width / days.length

  const bands = STACK_ORDER.map((key) => {
    const meta = TREND_SERIES.find((s) => s.key === key)!
    const fwd = days
      .map((_, i) => `${x(i).toFixed(1)},${y(dayStacks[i][key].top).toFixed(1)}`)
      .join(" ")
    const rev = [...days]
      .reverse()
      .map((_, ri) => {
        const i = days.length - 1 - ri
        return `${x(i).toFixed(1)},${y(dayStacks[i][key].base).toFixed(1)}`
      })
      .join(" ")
    return { key, color: meta.color, topPoints: fwd, fillPoints: `${fwd} ${rev}` }
  })

  return {
    left,
    width,
    top,
    bottom,
    gridLines: Array.from({ length: 5 }, (_, index) => {
      const value = Math.round((roundedMax / 4) * index)
      return { value, y: y(value) }
    }),
    bands,
    days: days.map((day, index) => {
      const total = dailyTotals[index]
      const pointX = x(index)
      return {
        key: day.key,
        x: pointX,
        y: y(total),
        hitboxX: left + index * hitboxWidth,
        hitboxWidth,
        tooltipX: chartTooltipX(pointX, left, width),
        label: formatChartTooltipDate(day.date),
        total,
        items: TREND_SERIES.map((series) => ({
          label: series.label,
          value: day[series.key],
          color: series.color,
        })),
      }
    }),
    labels: days
      .map((day, index) => ({
        key: day.key,
        x: x(index),
        textAnchor: index === 0 ? "start" as const : index === days.length - 1 ? "end" as const : "middle" as const,
        label: new Intl.DateTimeFormat("en-US", {
          day: "2-digit",
          month: "2-digit",
        }).format(day.date),
      }))
      .filter((_, index) => {
        const labelCount = chartWidth < 480 ? 3 : 6
        const step = Math.ceil((period - 1) / (labelCount - 1))
        return index === 0 || index === days.length - 1 ||
          (index % step === 0 && days.length - 1 - index >= step / 2)
      }),
  }
}

function trendAssetKey(scan: TrendScan, index: number) {
  const targetName =
    scan.hostname?.trim() ||
    scan.imageName?.trim() ||
    scan.targetName?.trim() ||
    scan.imageRef?.trim() ||
    scan.projectName?.trim()

  if (targetName) {
    return [scan.type || "scan", scan.workspaceId || "workspace", targetName].join(
      ":"
    )
  }

  return scan.id || `scan-${index}`
}

function chartTooltipX(pointX: number, left: number, width: number) {
  const gap = 10
  const right = left + width
  const preferred = pointX + SVG_CHART_TOOLTIP_WIDTH + gap > right
    ? pointX - SVG_CHART_TOOLTIP_WIDTH - gap
    : pointX + gap
  return Math.max(0, Math.min(preferred, right - SVG_CHART_TOOLTIP_WIDTH))
}

function formatChartTooltipDate(date: Date) {
  return new Intl.DateTimeFormat("en-US", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  }).format(date)
}

function dayKey(date: Date) {
  if (Number.isNaN(date.getTime())) {
    return ""
  }
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`
}

export function SeverityBadge({ severity }: { severity: string }) {
  const normalized = severity.toLowerCase() as SeverityKey
  const meta = SEVERITY_META[normalized] ?? SEVERITY_META.unknown

  return (
    <Badge variant="outline" className={meta.badgeClassName}>
      {severity || "unknown"}
    </Badge>
  )
}

export function ScanDetailSkeleton() {
  return (
    <div
      className="flex flex-col gap-3"
      role="status"
      aria-label="Loading scan details"
    >
      <span className="sr-only">Loading scan details</span>
      {Array.from({ length: 5 }).map((_, index) => (
        <div
          key={index}
          className="grid items-center gap-3 border-b pb-3 last:border-b-0 md:grid-cols-[1.2fr_0.35fr_0.55fr_0.8fr]"
        >
          <Skeleton className="h-5 w-3/4" />
          <Skeleton className="h-5 w-12" />
          <Skeleton className="h-5 w-24" />
          <Skeleton className="h-5 w-full" />
        </div>
      ))}
    </div>
  )
}

export function ScanDetailError({ message }: { message: string }) {
  return (
    <Empty className="min-h-48 border" role="alert">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <ShieldAlertIcon />
        </EmptyMedia>
        <EmptyTitle>Could not load scan details</EmptyTitle>
        <EmptyDescription>{message}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

export function LatestScansCard({
  scans,
  description,
  getTitle = (scan) => scan.projectName || scan.targetName || "scan",
  packageLabel = "deps",
  findingLabel = "vulns",
  supportsFixFilter = false,
}: {
  scans: Array<{
    id: string
    projectName?: string
    targetName?: string
    createdAt: string
    summary: ScanSummary
  }>
  description: string
  getTitle?: (scan: {
    projectName?: string
    targetName?: string
    createdAt: string
    summary: ScanSummary
  }) => string
  packageLabel?: string
  findingLabel?: string
  supportsFixFilter?: boolean
}) {
  const { filter } = useVulnerabilityFilter()
  const activeFilter = supportsFixFilter ? filter : "all"
  const visibleScans = scans.slice(0, 8).map((scan) => ({
    ...scan,
    filteredSummary: filterScanSummary(scan.summary, activeFilter),
  }))

  return (
    <Card>
      <CardHeader>
        <CardTitle>Latest scans</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-3">
          {visibleScans.map((scan, index) => {
            const previousSummary = visibleScans[index + 1]?.filteredSummary
            const delta = previousSummary
              ? scan.filteredSummary.vulnerabilities -
                previousSummary.vulnerabilities
              : null

            return (
              <div
                key={scan.id}
                className="flex items-start gap-3 rounded-lg border p-3"
              >
                <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted">
                  <PackageIcon />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium">
                    {getTitle(scan)}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {formatDate(scan.createdAt)}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Badge variant="outline">
                      {scan.summary.totalDependencies} {packageLabel}
                    </Badge>
                    <Badge variant="secondary">
                      {scan.filteredSummary.vulnerabilities} {findingLabel}
                    </Badge>
                    <VulnerabilityDelta delta={delta} />
                  </div>
                  <SeverityDistribution
                    summary={scan.filteredSummary}
                    className="mt-3 max-w-none"
                  />
                </div>
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

function VulnerabilityDelta({ delta }: { delta: number | null }) {
  if (delta === null) {
    return null
  }

  if (delta > 0) {
    return (
      <Badge variant="destructive" aria-label={`${delta} more vulnerabilities`}>
        <TrendingUpIcon data-icon="inline-start" />+{delta}
      </Badge>
    )
  }

  if (delta < 0) {
    return (
      <Badge variant="secondary" aria-label={`${Math.abs(delta)} fewer vulnerabilities`}>
        <TrendingDownIcon data-icon="inline-start" />
        {delta}
      </Badge>
    )
  }

  return (
    <Badge variant="outline" aria-label="No vulnerability count change">
      −
    </Badge>
  )
}
