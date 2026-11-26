import { useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { api, type LogEntry, type LogsQueryResult } from "@/lib/api"
import { cn } from "@/lib/utils"

const LEVELS = ["success", "info", "warn", "error"] as const

type SortKey = "time" | "level" | "source" | "id"

export function LogsPage() {
  const [result, setResult] = useState<LogsQueryResult | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [live, setLive] = useState(true)
  const [search, setSearch] = useState("")
  const [draftSearch, setDraftSearch] = useState("")
  const [levels, setLevels] = useState<string[]>([])
  const [source, setSource] = useState("")
  const [sort, setSort] = useState<SortKey>("time")
  const [order, setOrder] = useState<"asc" | "desc">("desc")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(25)

  const load = useCallback(async () => {
    setBusy(true)
    try {
      const data = await api.queryLogs({
        page,
        pageSize,
        q: search || undefined,
        level: levels.length ? levels.join(",") : undefined,
        source: source || undefined,
        sort,
        order,
      })
      setResult(data)
      setError("")
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load logs")
    } finally {
      setBusy(false)
    }
  }, [page, pageSize, search, levels, source, sort, order])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!live) return
    const t = window.setInterval(() => void load(), 2500)
    return () => window.clearInterval(t)
  }, [live, load])

  const totalPages = Math.max(1, Math.ceil((result?.total || 0) / pageSize))

  const levelCounts = result?.levels || {}
  const sources = result?.sources || []

  function toggleLevel(level: string) {
    setPage(1)
    setLevels((prev) =>
      prev.includes(level) ? prev.filter((l) => l !== level) : [...prev, level]
    )
  }

  function toggleSort(key: SortKey) {
    if (sort === key) {
      setOrder((o) => (o === "asc" ? "desc" : "asc"))
    } else {
      setSort(key)
      setOrder(key === "time" || key === "id" ? "desc" : "asc")
    }
  }

  async function handleClear() {
    if (!confirm("Clear the in-memory log buffer?")) return
    await api.clearLogs()
    setPage(1)
    await load()
  }

  const rows = useMemo(() => result?.entries || [], [result])

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-medium">Logs</h2>
          <p className="text-sm text-muted-foreground">
            Searchable event table with level filters, sorting, and pagination.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => setLive((v) => !v)}>
            Live: {live ? "on" : "off"}
          </Button>
          <Button variant="outline" onClick={() => void load()} disabled={busy}>
            Refresh
          </Button>
          <Button variant="destructive" onClick={() => void handleClear()}>
            Clear
          </Button>
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/40 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="flex flex-wrap gap-2">
        {LEVELS.map((level) => {
          const active = levels.includes(level)
          return (
            <button
              key={level}
              type="button"
              onClick={() => toggleLevel(level)}
              className={cn(
                "rounded-lg border px-3 py-1.5 text-xs capitalize transition-colors",
                active
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-border text-muted-foreground hover:bg-muted"
              )}
            >
              {level}
              <span className="ms-1 opacity-70">({levelCounts[level] || 0})</span>
            </button>
          )
        })}
        {levels.length > 0 ? (
          <button
            type="button"
            className="rounded-lg border border-border px-3 py-1.5 text-xs text-muted-foreground hover:bg-muted"
            onClick={() => {
              setLevels([])
              setPage(1)
            }}
          >
            Clear levels
          </button>
        ) : null}
      </div>

      <div className="flex flex-wrap gap-2">
        <input
          value={draftSearch}
          onChange={(e) => setDraftSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault()
              setSearch(draftSearch.trim())
              setPage(1)
            }
          }}
          placeholder="Search message, detail, source…"
          className="h-9 min-w-[220px] flex-1 rounded-lg border border-input bg-background px-3 text-sm"
        />
        <Button
          type="button"
          variant="secondary"
          onClick={() => {
            setSearch(draftSearch.trim())
            setPage(1)
          }}
        >
          Search
        </Button>
        <select
          value={source}
          onChange={(e) => {
            setSource(e.target.value)
            setPage(1)
          }}
          className="h-9 rounded-lg border border-input bg-background px-3 text-sm"
        >
          <option value="">All sources</option>
          {sources.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
        <select
          value={pageSize}
          onChange={(e) => {
            setPageSize(Number(e.target.value))
            setPage(1)
          }}
          className="h-9 rounded-lg border border-input bg-background px-3 text-sm"
        >
          {[10, 25, 50, 100].map((n) => (
            <option key={n} value={n}>
              {n} / page
            </option>
          ))}
        </select>
      </div>

      <div className="overflow-x-auto rounded-xl border border-border">
        <table className="w-full min-w-[820px] text-left text-sm">
          <thead className="border-b border-border bg-muted/50 text-muted-foreground">
            <tr>
              <SortTh
                label="Time"
                active={sort === "time"}
                order={order}
                onClick={() => toggleSort("time")}
              />
              <SortTh
                label="Level"
                active={sort === "level"}
                order={order}
                onClick={() => toggleSort("level")}
              />
              <SortTh
                label="Source"
                active={sort === "source"}
                order={order}
                onClick={() => toggleSort("source")}
              />
              <th className="px-3 py-2 font-medium">Message</th>
              <th className="px-3 py-2 font-medium">Detail</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-3 py-8 text-center text-muted-foreground"
                >
                  {busy ? "Loading…" : "No log events match these filters."}
                </td>
              </tr>
            ) : (
              rows.map((entry) => <LogRow key={entry.id} entry={entry} />)
            )}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
        <p className="text-muted-foreground">
          {result?.total || 0} events · page {page} of {totalPages}
        </p>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  )
}

function SortTh({
  label,
  active,
  order,
  onClick,
}: {
  label: string
  active: boolean
  order: "asc" | "desc"
  onClick: () => void
}) {
  return (
    <th className="px-3 py-2">
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "inline-flex items-center gap-1 font-medium",
          active ? "text-foreground" : "text-muted-foreground hover:text-foreground"
        )}
      >
        {label}
        {active ? <span className="text-xs">{order === "asc" ? "↑" : "↓"}</span> : null}
      </button>
    </th>
  )
}

function LogRow({ entry }: { entry: LogEntry }) {
  return (
    <tr className="border-b border-border last:border-0 align-top">
      <td className="whitespace-nowrap px-3 py-2 text-xs text-muted-foreground">
        {new Date(entry.time).toLocaleString()}
      </td>
      <td className="px-3 py-2">
        <span
          className={cn(
            "inline-flex rounded-md px-2 py-0.5 text-xs uppercase",
            entry.level === "error" && "bg-destructive/15 text-destructive",
            entry.level === "warn" && "bg-amber-500/15 text-amber-700 dark:text-amber-300",
            entry.level === "success" &&
              "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300",
            entry.level === "info" && "bg-sky-500/15 text-sky-700 dark:text-sky-300"
          )}
        >
          {entry.level}
        </span>
      </td>
      <td className="px-3 py-2 font-mono text-xs">{entry.source}</td>
      <td className="px-3 py-2">{entry.message}</td>
      <td className="max-w-[280px] px-3 py-2">
        {entry.detail ? (
          <pre className="overflow-x-auto whitespace-pre-wrap font-mono text-xs text-muted-foreground">
            {entry.detail}
          </pre>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>
    </tr>
  )
}
