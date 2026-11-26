type Series = { label?: string; values: number[]; color: string }

type Props = {
  series: Series[]
  height?: number
  className?: string
}

export function Sparkline({ series, height = 96, className }: Props) {
  const width = 320
  const pad = 8
  const all = series.flatMap((s) => s.values)
  const max = Math.max(1, ...all)
  const len = Math.max(1, ...series.map((s) => s.values.length))

  function path(values: number[]) {
    if (values.length === 0) return ""
    return values
      .map((v, i) => {
        const x = pad + (i * (width - pad * 2)) / Math.max(1, len - 1)
        const y = height - pad - (v / max) * (height - pad * 2)
        return `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`
      })
      .join(" ")
  }

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={className}
      role="img"
      aria-label="activity chart"
    >
      <line
        x1={pad}
        y1={height - pad}
        x2={width - pad}
        y2={height - pad}
        className="stroke-border"
        strokeWidth="1"
      />
      {series.map((s) => (
        <path
          key={s.color + (s.label || "")}
          d={path(s.values)}
          fill="none"
          stroke={s.color}
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      ))}
    </svg>
  )
}

export function BarMeter({
  pct,
  label,
}: {
  pct: number
  label?: string
}) {
  const clamped = Math.max(0, Math.min(100, pct))
  const tone =
    clamped >= 90
      ? "bg-destructive"
      : clamped >= 75
        ? "bg-amber-500"
        : "bg-emerald-500"
  return (
    <div className="space-y-1.5">
      {label ? (
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>{label}</span>
          <span>{clamped.toFixed(1)}%</span>
        </div>
      ) : null}
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div className={`h-full ${tone}`} style={{ width: `${clamped}%` }} />
      </div>
    </div>
  )
}
