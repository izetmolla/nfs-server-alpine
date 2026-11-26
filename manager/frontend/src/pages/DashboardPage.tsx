import { useCallback, useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"
import { ChevronDown } from "lucide-react"
import { Popover } from "@base-ui/react/popover"
import { Button } from "@/components/ui/button"
import { BarMeter, Sparkline } from "@/components/Sparkline"
import {
  api,
  formatBytes,
  type DashboardRangePreset,
  type DashboardSnapshot,
} from "@/lib/api"
import { cn } from "@/lib/utils"

const RANGE_OPTIONS: Array<{ value: DashboardRangePreset; label: string }> = [
  { value: "today", label: "Today" },
  { value: "yesterday", label: "Yesterday" },
  { value: "last_7_days", label: "Last 7 days" },
  { value: "this_month", label: "This month" },
  { value: "last_month", label: "Last month" },
  { value: "this_year", label: "This year" },
  { value: "custom", label: "Custom range" },
]

const AUTO_REFRESH_OPTIONS = [
  { value: 0, label: "Off" },
  { value: 5, label: "5s" },
  { value: 10, label: "10s" },
  { value: 15, label: "15s" },
  { value: 30, label: "30s" },
  { value: 60, label: "60s" },
] as const

function formatUptime(sec: number) {
  if (!sec || sec < 0) return "—"
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function toDateInputValue(d: Date) {
  return d.toISOString().slice(0, 10)
}

export function DashboardPage() {
  const [data, setData] = useState<DashboardSnapshot | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [preset, setPreset] = useState<DashboardRangePreset>("last_7_days")
  const [customFrom, setCustomFrom] = useState(() =>
    toDateInputValue(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000))
  )
  const [customTo, setCustomTo] = useState(() => toDateInputValue(new Date()))
  const [autoRefreshSec, setAutoRefreshSec] = useState(5)
  const [periodOpen, setPeriodOpen] = useState(false)

  const refresh = useCallback(async (opts?: { silent?: boolean }) => {
    if (!opts?.silent) setBusy(true)
    setError("")
    try {
      setData(
        await api.dashboard({
          range: preset,
          from: preset === "custom" ? `${customFrom}T00:00:00Z` : undefined,
          to: preset === "custom" ? `${customTo}T23:59:59Z` : undefined,
        })
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load dashboard")
    } finally {
      if (!opts?.silent) setBusy(false)
    }
  }, [preset, customFrom, customTo])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (autoRefreshSec <= 0) return
    const t = window.setInterval(
      () => void refresh({ silent: true }),
      autoRefreshSec * 1000
    )
    return () => window.clearInterval(t)
  }, [autoRefreshSec, refresh])

  const series = data?.metrics.series || []
  const chartSeries = useMemo(
    () => [
      {
        label: "Auth OK",
        color: "#10b981",
        values: series.map((b) => b.authOk),
      },
      {
        label: "Auth fail",
        color: "#ef4444",
        values: series.map((b) => b.authFail),
      },
      {
        label: "API",
        color: "#38bdf8",
        values: series.map((b) => b.apiRequests),
      },
    ],
    [series]
  )

  const applySeries = useMemo(
    () => [
      {
        label: "Apply OK",
        color: "#22c55e",
        values: series.map((b) => b.applyOk),
      },
      {
        label: "Apply fail",
        color: "#f59e0b",
        values: series.map((b) => b.applyFail),
      },
    ],
    [series]
  )

  const status = data?.status
  const storage = data?.storage
  const m = data?.metrics
  const logs = data?.logLevels || {}
  const granularity = m?.range?.granularity || "day"
  const periodLabel =
    RANGE_OPTIONS.find((o) => o.value === (m?.range?.preset || preset))
      ?.label || "Selected period"

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="font-medium">Dashboard</h2>
          <p className="text-sm text-muted-foreground">
            Live NFS health plus activity for the selected period.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="whitespace-nowrap">Auto-refresh</span>
            <select
              value={autoRefreshSec}
              onChange={(e) => setAutoRefreshSec(Number(e.target.value))}
              className="h-8 rounded-lg border border-border bg-background px-2 text-sm text-foreground outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:border-input dark:bg-input/30"
              aria-label="Auto-refresh interval"
            >
              {AUTO_REFRESH_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </label>
          <Button
            variant="outline"
            onClick={() => void refresh()}
            disabled={busy}
          >
            {busy ? "Refreshing…" : "Refresh"}
          </Button>
          <Popover.Root open={periodOpen} onOpenChange={setPeriodOpen}>
            <Popover.Trigger
              render={
                <Button
                  variant="outline"
                  className="gap-1.5"
                  aria-label="Activity period"
                />
              }
            >
              {periodLabel}
              <ChevronDown className="size-3.5 opacity-70" />
            </Popover.Trigger>
            <Popover.Portal>
              <Popover.Positioner side="bottom" align="end" sideOffset={8}>
                <Popover.Popup className="z-50 w-[min(100vw-2rem,22rem)] origin-[var(--transform-origin)] rounded-xl border border-border bg-popover p-4 text-popover-foreground shadow-lg outline-none data-[ending-style]:scale-95 data-[ending-style]:opacity-0 data-[starting-style]:scale-95 data-[starting-style]:opacity-0">
                  <div className="mb-3 space-y-1">
                    <Popover.Title className="font-medium">
                      Activity period
                    </Popover.Title>
                    <Popover.Description className="text-xs text-muted-foreground">
                      Bucketed by {granularity}
                      {m?.range?.from
                        ? ` · ${new Date(m.range.from).toLocaleString()} → ${new Date(m.range.to).toLocaleString()}`
                        : null}
                    </Popover.Description>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {RANGE_OPTIONS.map((opt) => (
                      <button
                        key={opt.value}
                        type="button"
                        onClick={() => {
                          setPreset(opt.value)
                          if (opt.value !== "custom") setPeriodOpen(false)
                        }}
                        className={cn(
                          "rounded-lg border px-3 py-1.5 text-xs transition-colors",
                          preset === opt.value
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-border text-muted-foreground hover:bg-muted hover:text-foreground"
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                  {preset === "custom" ? (
                    <div className="mt-3 flex flex-wrap items-end gap-3">
                      <label className="space-y-1 text-xs">
                        <span className="text-muted-foreground">From</span>
                        <input
                          type="date"
                          value={customFrom}
                          onChange={(e) => setCustomFrom(e.target.value)}
                          className="block h-9 rounded-lg border border-input bg-background px-3 text-sm"
                        />
                      </label>
                      <label className="space-y-1 text-xs">
                        <span className="text-muted-foreground">To</span>
                        <input
                          type="date"
                          value={customTo}
                          onChange={(e) => setCustomTo(e.target.value)}
                          className="block h-9 rounded-lg border border-input bg-background px-3 text-sm"
                        />
                      </label>
                      <Button
                        type="button"
                        size="sm"
                        onClick={() => {
                          void refresh()
                          setPeriodOpen(false)
                        }}
                      >
                        Apply range
                      </Button>
                    </div>
                  ) : null}
                  {m?.note ? (
                    <p className="mt-3 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-200">
                      {m.note}
                    </p>
                  ) : null}
                </Popover.Popup>
              </Popover.Positioner>
            </Popover.Portal>
          </Popover.Root>
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/40 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="NFS mountd"
          value={status?.mountdRunning ? "Running" : "Down"}
          hint={`Uptime ${formatUptime(data?.uptimeSeconds || 0)}`}
          ok={Boolean(status?.mountdRunning)}
        />
        <MetricCard
          label="Active NFS clients"
          value={String(data?.connections.activeClients ?? 0)}
          hint={data?.connections.source || "—"}
          ok
        />
        <MetricCard
          label="Exports enabled"
          value={`${data?.exportsEnabled ?? 0}/${data?.exportsTotal ?? 0}`}
          hint={`${data?.tokens ?? 0} API tokens`}
          ok
        />
        <MetricCard
          label="Last apply"
          value={
            status?.lastApplyAt && status.lastApplyAt !== "0001-01-01T00:00:00Z"
              ? status.lastApplyOK
                ? "Success"
                : "Failed"
              : "None"
          }
          hint={
            status?.lastApplyAt && status.lastApplyAt !== "0001-01-01T00:00:00Z"
              ? new Date(status.lastApplyAt).toLocaleString()
              : "No apply yet"
          }
          ok={status?.lastApplyOK ?? true}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Auth success"
          value={String(m?.authOk ?? 0)}
          hint={`${periodLabel} totals`}
          ok
          accent="emerald"
        />
        <MetricCard
          label="Auth failures"
          value={String(m?.authFail ?? 0)}
          hint="Bad password attempts"
          ok={(m?.authFail ?? 0) === 0}
          accent="rose"
        />
        <MetricCard
          label="Apply success"
          value={String(m?.applyOk ?? 0)}
          hint="exportfs apply OK"
          ok
          accent="emerald"
        />
        <MetricCard
          label="Apply / firewall fails"
          value={`${m?.applyFail ?? 0} / ${m?.firewallDeny ?? 0}`}
          hint="Apply errors · panel IP denies"
          ok={(m?.applyFail ?? 0) === 0 && (m?.firewallDeny ?? 0) === 0}
          accent="amber"
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-xl border border-border p-4">
          <div className="mb-2 flex items-center justify-between gap-2">
            <h3 className="font-medium">Panel activity</h3>
            <span className="text-xs text-muted-foreground">
              {m?.apiRequests ?? 0} API calls · {periodLabel}
            </span>
          </div>
          <Sparkline series={chartSeries} className="w-full" />
          <Legend
            items={[
              { color: "#10b981", label: "Auth OK" },
              { color: "#ef4444", label: "Auth fail" },
              { color: "#38bdf8", label: "API" },
            ]}
          />
        </section>

        <section className="rounded-xl border border-border p-4">
          <div className="mb-2 flex items-center justify-between gap-2">
            <h3 className="font-medium">NFS apply activity</h3>
            <Link
              to="/logs"
              className="text-xs text-muted-foreground hover:underline"
            >
              View logs
            </Link>
          </div>
          <Sparkline series={applySeries} className="w-full" />
          <Legend
            items={[
              { color: "#22c55e", label: "Apply OK" },
              { color: "#f59e0b", label: "Apply fail" },
            ]}
          />
        </section>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="space-y-4 rounded-xl border border-border p-4">
          <div className="flex items-center justify-between gap-2">
            <h3 className="font-medium">Storage usage</h3>
            <Link
              to="/reports"
              className="text-xs text-muted-foreground hover:underline"
            >
              Full reports
            </Link>
          </div>
          <BarMeter
            pct={storage?.usedPct || 0}
            label="Export folders vs disk"
          />
          <dl className="grid grid-cols-3 gap-2 text-sm">
            <div>
              <dt className="text-xs text-muted-foreground">Disk total</dt>
              <dd className="font-medium">
                {formatBytes(storage?.totalBytes || 0)}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Folder used</dt>
              <dd className="font-medium">
                {formatBytes(storage?.usedBytes || 0)}
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Disk free</dt>
              <dd className="font-medium">
                {formatBytes(storage?.freeBytes || 0)}
              </dd>
            </div>
          </dl>
          <ul className="space-y-2">
            {(storage?.paths || []).slice(0, 4).map((p) => (
              <li key={p.exportId} className="space-y-1">
                <div className="flex justify-between gap-2 font-mono text-xs">
                  <span className="truncate">{p.path}</span>
                  <span className="text-muted-foreground">
                    {p.exists ? formatBytes(p.usedBytes) : "missing"}
                  </span>
                </div>
                <BarMeter pct={p.exists ? p.usedPct : 0} />
              </li>
            ))}
          </ul>
        </section>

        <section className="space-y-4 rounded-xl border border-border p-4">
          <h3 className="font-medium">Event mix & clients</h3>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {(["success", "info", "warn", "error"] as const).map((level) => (
              <div
                key={level}
                className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-center"
              >
                <div className="text-xs uppercase text-muted-foreground">
                  {level}
                </div>
                <div className="mt-1 text-lg font-medium">
                  {logs[level] || 0}
                </div>
              </div>
            ))}
          </div>
          <div>
            <p className="mb-2 text-xs text-muted-foreground">
              Connected client addresses ({data?.connections.source})
            </p>
            {(data?.connections.addresses || []).length === 0 ? (
              <p className="text-sm text-muted-foreground">No clients detected.</p>
            ) : (
              <ul className="max-h-36 space-y-1 overflow-auto font-mono text-xs">
                {data?.connections.addresses.map((addr) => (
                  <li
                    key={addr}
                    className="rounded-md border border-border px-2 py-1"
                  >
                    {addr}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      </div>

      <section className="rounded-xl border border-border p-4 text-sm">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h3 className="font-medium">Live exports</h3>
          <Link
            to="/storages"
            className="inline-flex h-7 items-center rounded-lg border border-border px-2.5 text-[0.8rem] hover:bg-muted"
          >
            Manage storages
          </Link>
        </div>
        {status?.exportsLive ? (
          <pre className="overflow-x-auto rounded-lg bg-muted p-3 text-xs whitespace-pre-wrap">
            {status.exportsLive}
          </pre>
        ) : (
          <p className="text-muted-foreground">No live export output yet.</p>
        )}
      </section>
    </div>
  )
}

function MetricCard({
  label,
  value,
  hint,
  ok,
  accent,
}: {
  label: string
  value: string
  hint?: string
  ok: boolean
  accent?: "emerald" | "rose" | "amber"
}) {
  const dot =
    accent === "rose"
      ? "bg-rose-500"
      : accent === "amber"
        ? "bg-amber-500"
        : accent === "emerald"
          ? "bg-emerald-500"
          : ok
            ? "bg-emerald-500"
            : "bg-destructive"
  return (
    <div className="rounded-xl border border-border p-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span className={`inline-block size-2.5 rounded-full ${dot}`} />
        {label}
      </div>
      <div className="mt-2 text-xl font-medium tracking-tight">{value}</div>
      {hint ? <p className="mt-1 text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}

function Legend({ items }: { items: Array<{ color: string; label: string }> }) {
  return (
    <div className="mt-3 flex flex-wrap gap-3 text-xs text-muted-foreground">
      {items.map((item) => (
        <span key={item.label} className="inline-flex items-center gap-1.5">
          <span
            className="inline-block size-2 rounded-full"
            style={{ background: item.color }}
          />
          {item.label}
        </span>
      ))}
    </div>
  )
}
