import { useCallback, useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"
import {
  api,
  formatBytes,
  type QuotaReport,
  type StorageReport,
} from "@/lib/api"
import { cn } from "@/lib/utils"

export function ReportsPage() {
  const [report, setReport] = useState<StorageReport | null>(null)
  const [quotas, setQuotas] = useState<QuotaReport | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState("")

  const refresh = useCallback(async () => {
    setBusy(true)
    setError("")
    try {
      const [storage, quota] = await Promise.all([
        api.storageReport(),
        api.quotaReport(),
      ])
      setReport(storage)
      setQuotas(quota)
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load report")
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  async function runCheck() {
    setBusy(true)
    setMessage("")
    setError("")
    try {
      const res = await api.quotaCheck()
      setQuotas(res.report)
      setMessage("Quota check completed (postbacks fired for newly full exports).")
    } catch (err) {
      setError(err instanceof Error ? err.message : "quota check failed")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-medium">Storage reports</h2>
          <p className="text-sm text-muted-foreground">
            Filesystem usage plus per-export directory quotas (unlimited by
            default).
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => void refresh()} disabled={busy}>
            Refresh
          </Button>
          <Button variant="secondary" onClick={() => void runCheck()} disabled={busy}>
            Check quotas now
          </Button>
        </div>
      </div>

      {(message || error) && (
        <div
          className={cn(
            "rounded-lg border px-3 py-2 text-sm",
            error
              ? "border-destructive/40 text-destructive"
              : "border-border text-foreground"
          )}
        >
          {error || message}
        </div>
      )}

      <section className="grid gap-3 sm:grid-cols-4">
        <Card label="Disk total" value={formatBytes(report?.totalBytes || 0)} />
        <Card
          label="Folder used"
          value={formatBytes(report?.usedBytes || 0)}
        />
        <Card label="Disk free" value={formatBytes(report?.freeBytes || 0)} />
        <Card
          label="Folders / disk"
          value={`${(report?.usedPct || 0).toFixed(1)}%`}
        />
      </section>

      <section className="space-y-3">
        <h3 className="font-medium">Export quotas</h3>
        {(quotas?.exports || []).length === 0 ? (
          <p className="text-sm text-muted-foreground">No exports configured.</p>
        ) : (
          quotas?.exports.map((q) => (
            <div key={q.exportId} className="rounded-xl border border-border p-4">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div>
                  <div className="font-mono text-sm">{q.path}</div>
                  <div className="text-xs text-muted-foreground">
                    export #{q.exportId}
                    {q.enabled ? "" : " · disabled"}
                    {q.exists ? "" : " · missing"}
                    {q.unlimited ? " · unlimited" : ""}
                    {q.full ? " · FULL" : ""}
                  </div>
                </div>
                <div className="text-right text-sm">
                  <div>
                    {formatBytes(q.usedBytes)}
                    {q.unlimited
                      ? " / unlimited"
                      : ` / ${formatBytes(q.quotaBytes)}`}
                  </div>
                  {!q.unlimited ? (
                    <div className="text-xs text-muted-foreground">
                      {q.usedPct.toFixed(1)}% used
                      {q.remainingBytes != null
                        ? ` · ${formatBytes(q.remainingBytes)} left`
                        : ""}
                    </div>
                  ) : null}
                </div>
              </div>
              {!q.unlimited && q.exists ? (
                <div className="mt-3 h-2 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn(
                      "h-full",
                      q.full ? "bg-destructive" : "bg-primary"
                    )}
                    style={{ width: `${Math.min(100, q.usedPct)}%` }}
                  />
                </div>
              ) : null}
              <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                <span>
                  Postback:{" "}
                  {q.quotaPostbackUrl ? (
                    <span className="font-mono">{q.quotaPostbackUrl}</span>
                  ) : (
                    "not set"
                  )}
                </span>
                <Link
                  to={`/storages/${q.exportId}`}
                  className="hover:underline"
                >
                  Edit storage
                </Link>
              </div>
              {q.error ? (
                <p className="mt-2 text-xs text-destructive">{q.error}</p>
              ) : null}
            </div>
          ))
        )}
      </section>

      <section className="space-y-3">
        <h3 className="font-medium">Filesystem paths</h3>
        {(report?.paths || []).map((p) => (
          <div key={p.exportId} className="rounded-xl border border-border p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <div className="font-mono text-sm">{p.path}</div>
                <div className="text-xs text-muted-foreground">
                  export #{p.exportId}
                  {p.enabled ? "" : " · disabled"}
                  {p.exists ? "" : " · missing"}
                </div>
              </div>
              <div className="text-sm">
                {p.exists
                  ? `${formatBytes(p.usedBytes)} folder · ${formatBytes(p.freeBytes)} disk free`
                  : p.error || "n/a"}
              </div>
            </div>
            {p.exists ? (
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full bg-primary"
                  style={{ width: `${Math.min(100, p.usedPct)}%` }}
                />
              </div>
            ) : null}
          </div>
        ))}
      </section>
    </div>
  )
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border p-4">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-medium">{value}</div>
    </div>
  )
}
