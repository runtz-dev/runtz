"use client"

import * as React from "react"
import {
  ActivityIcon,
  ChartSplineIcon,
  PackageIcon,
  ShieldAlertIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import type { ScanSummary } from "@/lib/api"
import type { TrendScan } from "@/lib/dashboard"
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
  description,
}: {
  title: string
  value: number
  description: string
}) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{title}</CardDescription>
        <CardTitle className="text-3xl">{value}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  )
}

export function DashboardSummaryGrid({
  children,
  summary,
}: {
  children: React.ReactNode
  summary: ScanSummary
}) {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-[repeat(4,minmax(0,1fr))_minmax(18rem,1.15fr)]">
      {children}
      <SeverityCard
        summary={summary}
        className="md:col-span-2 xl:col-span-1"
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
      <TooltipTrigger render={children} aria-label={`${label}, ${value} vulnerabilidades`} />
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
}: {
  summary: ScanSummary
  className?: string
}) {
  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>Vulnerabilidades</CardTitle>
        <CardDescription>Distribution by severity.</CardDescription>
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
  { key: "unknown", label: "Desconhecida", color: "#8fa0b7" },
] as const

// Render order bottom to top so critical is the topmost visual layer.
const STACK_ORDER = ["unknown", "low", "medium", "high", "critical"] as const

export function VulnerabilityTrendChart({ scans }: { scans: TrendScan[] }) {
  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(22rem,2fr)]">
      <VulnerabilityChart scans={scans} />
      <DailyScansChart scans={scans} />
    </div>
  )
}

function VulnerabilityChart({ scans }: { scans: TrendScan[] }) {
  const [period, setPeriod] = React.useState<TrendPeriod>(30)
  const [hoveredIndex, setHoveredIndex] = React.useState<number | null>(null)
  const chart = React.useMemo(() => buildTrendChart(scans, period), [period, scans])
  const hoveredDay = hoveredIndex === null ? null : chart.days[hoveredIndex]

  return (
    <Card className="overflow-hidden">
      <CardHeader>
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
          <div>
            <CardTitle className="flex items-center gap-2">
              <ActivityIcon className="size-4 text-primary" />
              Vulnerability trend
            </CardTitle>
            <CardDescription>
              Severidades encontradas nos scans recebidos por dia.
            </CardDescription>
          </div>
          <TrendPeriodSelector
            period={period}
            onChange={(value) => {
              setHoveredIndex(null)
              setPeriod(value)
            }}
          />
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col px-2 pb-3 sm:px-6">
        <div className="min-w-0 flex-1 overflow-x-auto">
          <svg
            viewBox="0 0 920 340"
            className="h-[300px] min-w-[620px] w-full sm:h-[340px]"
            role="img"
            aria-label={`Vulnerability trend over the last ${period} days`}
            onMouseLeave={() => setHoveredIndex(null)}
          >
            {chart.gridLines.map((line) => (
              <g key={line.value}>
                <line
                  x1="42"
                  x2="904"
                  y1={line.y}
                  y2={line.y}
                  stroke="currentColor"
                  className="text-border"
                />
                <text
                  x="30"
                  y={line.y + 4}
                  textAnchor="end"
                  className="fill-muted-foreground text-[10px]"
                >
                  {line.value}
                </text>
              </g>
            ))}
            {/* Stacked filled areas rendered bottom to top. */}
            {chart.bands.map((band) => (
              <g key={band.key}>
                <polygon
                  points={band.fillPoints}
                  fill={band.color}
                  fillOpacity="0.20"
                />
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
            {hoveredDay ? (
              <>
                <line
                  x1={hoveredDay.x}
                  x2={hoveredDay.x}
                  y1={chart.top}
                  y2={chart.bottom}
                  stroke="currentColor"
                  strokeDasharray="4 4"
                  className="text-primary/70"
                />
                <circle
                  cx={hoveredDay.x}
                  cy={hoveredDay.y}
                  r="4"
                  className="fill-primary stroke-card"
                  strokeWidth="2"
                />
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
            {chart.labels.map((label) => (
              <text
                key={label.key}
                x={label.x}
                y="326"
                textAnchor={label.textAnchor}
                className="fill-muted-foreground text-[10px]"
              >
                {label.label}
              </text>
            ))}
            {chart.days.map((day, index) => (
              <rect
                key={day.key}
                x={day.hitboxX}
                y={chart.top}
                width={day.hitboxWidth}
                height={chart.bottom - chart.top}
                fill="transparent"
                className="cursor-crosshair"
                onMouseEnter={() => setHoveredIndex(index)}
              />
            ))}
          </svg>
        </div>
        <div className="mt-auto flex flex-wrap gap-x-4 gap-y-2 border-t pt-3 text-xs text-muted-foreground">
          {TREND_SERIES.map((series) => (
            <span key={series.key} className="inline-flex items-center gap-1.5">
              <span
                className="size-2.5 rounded-sm"
                style={{ backgroundColor: series.color }}
              />
              {series.label}
            </span>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function DailyScansChart({ scans }: { scans: TrendScan[] }) {
  const [period, setPeriod] = React.useState<TrendPeriod>(30)
  const [hoveredIndex, setHoveredIndex] = React.useState<number | null>(null)
  const chart = React.useMemo(() => buildDailyScansChart(scans, period), [period, scans])
  const hoveredDay = hoveredIndex === null ? null : chart.days[hoveredIndex]

  return (
    <Card className="overflow-hidden">
      <CardHeader>
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
          <div>
            <CardTitle className="flex items-center gap-2">
              <ChartSplineIcon className="size-4 text-primary" />
              Scans/dia
            </CardTitle>
            <CardDescription>Scans enviados por dia.</CardDescription>
          </div>
          <TrendPeriodSelector
            period={period}
            onChange={(value) => {
              setHoveredIndex(null)
              setPeriod(value)
            }}
          />
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col px-2 pb-3 sm:px-4">
        <div className="flex min-w-0 flex-1 items-center overflow-x-auto">
          <svg
            viewBox="0 0 520 340"
            className="h-[300px] min-w-[440px] w-full sm:h-[340px]"
            role="img"
            aria-label={`Scans submitted per day over the last ${period} days`}
            onMouseLeave={() => setHoveredIndex(null)}
          >
            {chart.gridLines.map((line) => (
              <g key={line.value}>
                <line
                  x1="36"
                  x2="504"
                  y1={line.y}
                  y2={line.y}
                  stroke="currentColor"
                  className="text-border"
                />
                <text
                  x="28"
                  y={line.y + 4}
                  textAnchor="end"
                  className="fill-muted-foreground text-[10px]"
                >
                  {line.value}
                </text>
              </g>
            ))}
            <polygon
              points={chart.fillPoints}
              className="fill-primary/20"
            />
            <polyline
              points={chart.topPoints}
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinejoin="round"
              strokeLinecap="round"
              className="text-primary"
            />
            {hoveredDay ? (
              <>
                <line
                  x1={hoveredDay.x}
                  x2={hoveredDay.x}
                  y1={chart.top}
                  y2={chart.bottom}
                  stroke="currentColor"
                  strokeDasharray="4 4"
                  className="text-primary/70"
                />
                <circle
                  cx={hoveredDay.x}
                  cy={hoveredDay.y}
                  r="4"
                  className="fill-primary stroke-card"
                  strokeWidth="2"
                />
                <SvgChartTooltip
                  x={hoveredDay.tooltipX}
                  y={chart.top + 8}
                  title={hoveredDay.label}
                  items={[{ label: "Scans", value: hoveredDay.scans, color: "#6db5ff" }]}
                />
              </>
            ) : null}
            {chart.labels.map((label) => (
              <text
                key={label.key}
                x={label.x}
                y="326"
                textAnchor={label.textAnchor}
                className="fill-muted-foreground text-[10px]"
              >
                {label.label}
              </text>
            ))}
            {chart.days.map((day, index) => (
              <rect
                key={day.key}
                x={day.hitboxX}
                y={chart.top}
                width={day.hitboxWidth}
                height={chart.bottom - chart.top}
                fill="transparent"
                className="cursor-crosshair"
                onMouseEnter={() => setHoveredIndex(index)}
              />
            ))}
          </svg>
        </div>
        <div className="mt-auto flex items-center justify-between gap-3 border-t pt-3 text-xs text-muted-foreground">
          <span>
            <strong className="font-semibold text-foreground">{chart.totalScans}</strong>{" "}
            scans
          </span>
          <span>{chart.dailyAverage}/day on average</span>
        </div>
      </CardContent>
    </Card>
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

function buildTrendChart(scans: TrendScan[], period: TrendPeriod) {
  const width = 862
  const height = 270
  const left = 42
  const top = 14
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
  const byDay = new Map(days.map((day) => [day.key, day]))

  for (const scan of scans) {
    const day = byDay.get(dayKey(new Date(scan.createdAt)))
    if (!day) {
      continue
    }
    day.scans += 1
    for (const series of TREND_SERIES) {
      day[series.key] += scan.summary[series.key]
    }
  }

  // Forward-fill: days with no scan carry the last known vulnerability state.
  // This shows "the vulnerability debt is still there even if we didn't scan today."
  const lastSeen: Record<string, number> = { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 }
  let seenAny = false
  for (const day of days) {
    if (day.scans > 0) {
      seenAny = true
      for (const s of TREND_SERIES) lastSeen[s.key] = day[s.key]
    } else if (seenAny) {
      for (const s of TREND_SERIES) day[s.key] = lastSeen[s.key]
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
        textAnchor: index === days.length - 1 ? "end" as const : "middle" as const,
        label: new Intl.DateTimeFormat("pt-BR", {
          day: "2-digit",
          month: "2-digit",
        }).format(day.date),
      }))
      .filter((_, index) => index % Math.max(1, Math.floor(period / 6)) === 0),
  }
}

function buildDailyScansChart(scans: TrendScan[], period: TrendPeriod) {
  const width = 468
  const height = 270
  const left = 36
  const top = 14
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
    }
  })
  const byDay = new Map(days.map((day) => [day.key, day]))

  for (const scan of scans) {
    const day = byDay.get(dayKey(new Date(scan.createdAt)))
    if (day) {
      day.scans += 1
    }
  }

  const totalScans = days.reduce((total, day) => total + day.scans, 0)
  const roundedMax = Math.max(
    4,
    Math.ceil(Math.max(...days.map((day) => day.scans)) / 4) * 4
  )
  const x = (index: number) => left + ((index + 0.5) / days.length) * width
  const y = (value: number) => bottom - (value / roundedMax) * height
  const hitboxWidth = width / days.length
  const labelStep = Math.ceil(period / 5)

  return {
    top,
    bottom,
    totalScans,
    dailyAverage: (totalScans / period).toLocaleString("pt-BR", {
      maximumFractionDigits: 1,
    }),
    gridLines: Array.from({ length: 5 }, (_, index) => {
      const value = (roundedMax / 4) * index
      return { value, y: y(value) }
    }),
    topPoints: days
      .map((day, index) => `${x(index).toFixed(1)},${y(day.scans).toFixed(1)}`)
      .join(" "),
    fillPoints: [
      `${x(0).toFixed(1)},${bottom}`,
      ...days.map((day, index) => `${x(index).toFixed(1)},${y(day.scans).toFixed(1)}`),
      `${x(days.length - 1).toFixed(1)},${bottom}`,
    ].join(" "),
    days: days.map((day, index) => {
      const pointX = x(index)
      return {
        ...day,
        x: pointX,
        y: y(day.scans),
        hitboxX: left + index * hitboxWidth,
        hitboxWidth,
        tooltipX: chartTooltipX(pointX, left, width),
        label: formatChartTooltipDate(day.date),
      }
    }),
    labels: days
      .map((day, index) => ({
        key: day.key,
        x: x(index),
        textAnchor: index === days.length - 1 ? "end" as const : "middle" as const,
        label: new Intl.DateTimeFormat("pt-BR", {
          day: "2-digit",
          month: "2-digit",
        }).format(day.date),
      }))
      .filter((_, index) => index === days.length - 1 || index % labelStep === 0),
  }
}

function chartTooltipX(pointX: number, left: number, width: number) {
  const gap = 10
  const right = left + width
  return pointX + SVG_CHART_TOOLTIP_WIDTH + gap > right
    ? pointX - SVG_CHART_TOOLTIP_WIDTH - gap
    : pointX + gap
}

function formatChartTooltipDate(date: Date) {
  return new Intl.DateTimeFormat("pt-BR", {
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
      aria-label="Carregando detalhes do scan"
    >
      <span className="sr-only">Carregando detalhes do scan</span>
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
        <EmptyTitle>Não foi possível carregar os detalhes</EmptyTitle>
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
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Últimos scans</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-3">
          {scans.slice(0, 8).map((scan) => (
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
                    {scan.summary.vulnerabilities} {findingLabel}
                  </Badge>
                </div>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
