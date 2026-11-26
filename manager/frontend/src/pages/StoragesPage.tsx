import { useCallback, useEffect, useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { DataTable, Td, Th } from "@/components/DataTable"
import { Dialog } from "@/components/Dialog"
import { PathBrowser } from "@/components/PathBrowser"
import { RowMenu } from "@/components/RowMenu"
import { Toggle } from "@/components/Toggle"
import { api, formatBytes, type Export, type Status, type StorageReport } from "@/lib/api"

export function StoragesPage() {
  const navigate = useNavigate()
  const [exportsList, setExportsList] = useState<Export[]>([])
  const [status, setStatus] = useState<Status | null>(null)
  const [report, setReport] = useState<StorageReport | null>(null)
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const [rowBusy, setRowBusy] = useState<number | null>(null)
  const [cloneSource, setCloneSource] = useState<Export | null>(null)
  const [clonePath, setClonePath] = useState("")
  const [cloneBusy, setCloneBusy] = useState(false)
  const [cloneError, setCloneError] = useState("")

  const refresh = useCallback(async () => {
    const [list, st, rep] = await Promise.all([
      api.listExports(),
      api.status(),
      api.storageReport(),
    ])
    setExportsList(list)
    setStatus(st)
    setReport(rep)
  }, [])

  useEffect(() => {
    refresh().catch((err: Error) => setError(err.message))
  }, [refresh])

  async function handleApply() {
    setBusy(true)
    setError("")
    setMessage("")
    try {
      const res = await api.applyExports()
      setStatus(res.status)
      setMessage(res.output || "Exports applied")
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "apply failed")
    } finally {
      setBusy(false)
    }
  }

  async function handleDelete(id: number) {
    if (!confirm("Delete this storage export?")) return
    setBusy(true)
    try {
      await api.deleteExport(id)
      setMessage("Deleted")
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed")
    } finally {
      setBusy(false)
    }
  }

  function openClone(exp: Export) {
    const base = exp.path.replace(/\/$/, "")
    setCloneSource(exp)
    setClonePath(`${base}-copy`)
    setCloneError("")
  }

  async function confirmClone(apply: boolean) {
    if (!cloneSource) return
    const path = clonePath.trim()
    if (!path.startsWith("/")) {
      setCloneError("Path must be absolute (start with /)")
      return
    }
    if (path === cloneSource.path) {
      setCloneError("Clone path must differ from the source")
      return
    }
    setCloneBusy(true)
    setCloneError("")
    try {
      const res = await api.cloneExport(cloneSource.id, { path, apply })
      setCloneSource(null)
      setMessage(
        apply
          ? `Cloned to ${res.export.path} and applied`
          : `Cloned to ${res.export.path}`
      )
      await refresh()
      navigate(`/storages/${res.export.id}`)
    } catch (err) {
      setCloneError(err instanceof Error ? err.message : "clone failed")
    } finally {
      setCloneBusy(false)
    }
  }

  async function toggleEnabled(exp: Export, enabled: boolean) {
    setRowBusy(exp.id)
    setError("")
    setMessage("")
    try {
      const res = await api.patchExport(exp.id, { enabled, apply: true })
      setExportsList((prev) =>
        prev.map((e) => (e.id === exp.id ? { ...e, ...res.export } : e))
      )
      if (res.status) setStatus(res.status)
      setMessage(
        enabled
          ? `Enabled ${exp.path} and applied`
          : `Disabled ${exp.path} and applied`
      )
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "toggle failed")
      await refresh().catch(() => undefined)
    } finally {
      setRowBusy(null)
    }
  }

  async function toggleReadOnly(exp: Export, readOnly: boolean) {
    setRowBusy(exp.id)
    setError("")
    setMessage("")
    try {
      const res = await api.patchExport(exp.id, { readOnly, apply: true })
      setExportsList((prev) =>
        prev.map((e) => (e.id === exp.id ? { ...e, ...res.export } : e))
      )
      if (res.status) setStatus(res.status)
      setMessage(
        readOnly
          ? `Set ${exp.path} read-only and applied`
          : `Set ${exp.path} read-write and applied`
      )
      await refresh()
    } catch (err) {
      setError(err instanceof Error ? err.message : "toggle failed")
      await refresh().catch(() => undefined)
    } finally {
      setRowBusy(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-medium">Storages</h2>
          <p className="text-sm text-muted-foreground">
            NFSv4 exports with full selectable options.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => void refresh()} disabled={busy}>
            Refresh
          </Button>
          <Button variant="secondary" onClick={() => void handleApply()} disabled={busy}>
            Apply
          </Button>
          <Link
            to="/storages/new"
            className="inline-flex h-8 items-center rounded-lg bg-primary px-3 text-sm text-primary-foreground"
          >
            New storage
          </Link>
        </div>
      </div>

      <section className="grid gap-3 sm:grid-cols-3">
        <Stat
          label="mountd"
          value={status?.mountdRunning ? "running" : "down"}
        />
        <Stat
          label="Folder used"
          value={report ? formatBytes(report.usedBytes) : "—"}
        />
        <Stat
          label="Disk free"
          value={report ? formatBytes(report.freeBytes) : "—"}
        />
      </section>

      {(message || error) && (
        <div
          className={`rounded-lg border px-3 py-2 text-sm ${
            error ? "border-destructive/40 text-destructive" : "border-border"
          }`}
        >
          {error || message}
        </div>
      )}

      <DataTable>
        <thead>
          <tr>
            <Th style={{ width: "22%" }}>Path</Th>
            <Th style={{ width: "14%" }}>Clients</Th>
            <Th style={{ width: "12%" }}>Options</Th>
            <Th style={{ width: "8%" }}>Fsid</Th>
            <Th style={{ width: "12%" }}>Folder size</Th>
            <Th style={{ width: "10%" }}>On</Th>
            <Th style={{ width: "10%" }}>Read-only</Th>
            <Th style={{ width: "12%" }}>Actions</Th>
          </tr>
        </thead>
        <tbody>
          {exportsList.map((exp) => {
            const usage = report?.paths.find((p) => p.exportId === exp.id)
            const busyRow = rowBusy === exp.id || busy || cloneBusy
            return (
              <tr key={exp.id} className="border-b border-border last:border-0">
                <Td>
                  <div className="font-mono text-xs">{exp.path}</div>
                  {exp.description ? (
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {exp.description}
                    </div>
                  ) : null}
                </Td>
                <Td className="font-mono text-xs">{exp.clients}</Td>
                <Td className="text-xs">
                  {exp.accessMode}/{exp.commitMode}
                  {exp.enabled ? "" : " (disabled)"}
                </Td>
                <Td>{exp.fsid}</Td>
                <Td className="text-xs">
                  {usage?.exists
                    ? formatBytes(usage.usedBytes)
                    : usage?.error || "n/a"}
                </Td>
                <Td>
                  <Toggle
                    id={`enabled-${exp.id}`}
                    label={exp.enabled ? "On" : "Off"}
                    title="Enable or disable this export and apply immediately"
                    checked={exp.enabled}
                    disabled={busyRow}
                    onChange={(next) => void toggleEnabled(exp, next)}
                  />
                </Td>
                <Td>
                  <Toggle
                    id={`ro-${exp.id}`}
                    label={exp.accessMode === "ro" ? "RO" : "RW"}
                    title="Toggle read-only and apply immediately"
                    checked={exp.accessMode === "ro"}
                    disabled={busyRow || !exp.enabled}
                    onChange={(next) => void toggleReadOnly(exp, next)}
                  />
                </Td>
                <Td>
                  <div className="flex justify-end">
                    <RowMenu
                      disabled={busyRow}
                      items={[
                        {
                          label: "Edit",
                          onSelect: () => navigate(`/storages/${exp.id}`),
                        },
                        {
                          label: "Clone",
                          onSelect: () => openClone(exp),
                        },
                        {
                          label: "Delete",
                          tone: "danger",
                          onSelect: () => void handleDelete(exp.id),
                        },
                      ]}
                    />
                  </div>
                </Td>
              </tr>
            )
          })}
        </tbody>
      </DataTable>

      {status?.exportsLive ? (
        <pre className="overflow-x-auto rounded-xl border border-border bg-muted p-3 text-xs whitespace-pre-wrap">
          {status.exportsLive}
        </pre>
      ) : null}

      <Dialog
        open={Boolean(cloneSource)}
        title="Clone storage"
        tone="info"
        confirmLabel={cloneBusy ? "Cloning…" : "Clone & apply"}
        cancelLabel="Cancel"
        onClose={() => {
          if (!cloneBusy) setCloneSource(null)
        }}
        onConfirm={() => {
          if (!cloneBusy) void confirmClone(true)
        }}
      >
        <div className="space-y-3">
          <p className="text-muted-foreground">
            Copy options from{" "}
            <span className="font-mono text-foreground">
              {cloneSource?.path}
            </span>{" "}
            to a new export path. A new fsid is assigned automatically.
          </p>
          <div className="space-y-1.5">
            <div className="text-sm font-medium">New path</div>
            <PathBrowser value={clonePath} onChange={setClonePath} />
          </div>
          {cloneError ? (
            <p className="text-sm text-destructive">{cloneError}</p>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={cloneBusy}
              onClick={() => void confirmClone(false)}
            >
              Clone without apply
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-border p-4">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-medium">{value}</div>
    </div>
  )
}
