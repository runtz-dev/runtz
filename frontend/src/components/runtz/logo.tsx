/**
 * runtz wordmark — official lockup from the "Runtz Logo" Claude Design project
 * (assets/README.md): JetBrains Mono 700, tracking -0.05em, terminal block
 * cursor after the z (0.44em x 0.82em, gap 0.2em, 0.07em below the baseline,
 * always a pure rectangle — no radius).
 *
 * Rules: the wordmark is the logo everywhere; the cursor block alone is
 * reserved for favicons and tiny app icons. Text is white on dark surfaces,
 * #071222 on light; cursor #6db5ff (dark) / #2f7eff (light).
 */

export function RuntzWordmark({
  className = "",
  cursorClassName = "bg-[#2f7eff] dark:bg-[#6db5ff]",
}: {
  /** Sets the size via font-size utilities (e.g. "text-[17px]") and may override text color. */
  className?: string
  /** Tailwind background classes for the cursor block. */
  cursorClassName?: string
}) {
  return (
    <span
      className={`inline-flex items-center gap-[0.2em] font-bold leading-none tracking-[-0.05em] text-[#071222] dark:text-white ${className}`}
      style={{ fontFamily: "var(--font-brand), 'JetBrains Mono', ui-monospace, monospace" }}
      aria-label="runtz"
    >
      runtz
      <span
        aria-hidden="true"
        className={`inline-block h-[0.82em] w-[0.44em] translate-y-[0.07em] ${cursorClassName}`}
      />
    </span>
  )
}

/** Favicon tile: night rounded square with the cursor block (official favicon.svg). */
export function RuntzMark({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 64 64"
      className={className}
      role="img"
      aria-label="runtz"
    >
      <rect width="64" height="64" rx="14" fill="#050912" />
      <rect x="22" y="16" width="20" height="34" fill="#6db5ff" />
    </svg>
  )
}
