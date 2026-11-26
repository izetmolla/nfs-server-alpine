import { useEffect, useMemo, useState, type KeyboardEvent } from "react"
import Select from "react-select"
import CreatableSelect from "react-select/creatable"
import { Button } from "@/components/ui/button"
import { Dialog } from "@/components/Dialog"
import { FieldLabel } from "@/components/FieldLabel"
import { PathBrowser } from "@/components/PathBrowser"
import {
  api,
  emptyExportInput,
  type CatalogField,
  type ExportInput,
  type OptionCatalog,
} from "@/lib/api"
import { validateClientsInput } from "@/lib/ip"
import {
  warningForApply,
  warningForClients,
  warningForEnumChange,
  type PendingChange,
} from "@/lib/optionWarnings"
import {
  bytesToUnitAmount,
  preferUnit,
  unitToBytes,
  QUOTA_UNIT_OPTIONS,
  type QuotaUnit,
} from "@/lib/quotaUnits"
import { selectStyles, selectTheme, type SelectOption } from "@/lib/selectStyles"
import { cn } from "@/lib/utils"

type Props = {
  initial?: Partial<ExportInput>
  submitLabel: string
  onSubmit: (input: ExportInput, apply: boolean) => Promise<void>
}

type FieldErrors = Partial<Record<keyof ExportInput | "form", string>>

const ENUM_KEYS: Array<keyof ExportInput> = [
  "accessMode",
  "commitMode",
  "writeDelay",
  "subtreeCheck",
  "portMode",
  "lockMode",
  "hideMode",
  "rootSquash",
  "allSquash",
  "security",
]

function blockEnterSubmit(e: KeyboardEvent) {
  if (e.key !== "Enter") return
  const target = e.target as HTMLElement
  if (target.tagName === "TEXTAREA") return
  e.preventDefault()
}

function validateForm(form: ExportInput): FieldErrors {
  const errors: FieldErrors = {}
  const path = form.path.trim()
  if (!path) {
    errors.path = "Export path is required"
  } else if (!path.startsWith("/")) {
    errors.path = "Path must be absolute (start with /)"
  }

  const clients = validateClientsInput(form.clients)
  if (!clients.ok) {
    errors.clients = clients.error || "Invalid clients"
  } else if (!clients.normalized) {
    errors.clients = "At least one client is required"
  }

  if (form.fsid == null || Number.isNaN(Number(form.fsid)) || form.fsid < 0) {
    errors.fsid = "Fsid must be 0 or a positive integer"
  }

  for (const key of ENUM_KEYS) {
    const value = String(form[key] ?? "").trim()
    if (!value) {
      errors[key] = "This field is required"
    }
  }

  if (form.anonUid != null && (Number.isNaN(form.anonUid) || form.anonUid < 0)) {
    errors.anonUid = "anonuid must be a non-negative integer"
  }
  if (form.anonGid != null && (Number.isNaN(form.anonGid) || form.anonGid < 0)) {
    errors.anonGid = "anongid must be a non-negative integer"
  }

  return errors
}

function inputClass(invalid?: boolean) {
  return cn(
    "h-9 w-full rounded-lg border bg-background px-3 outline-none focus-visible:ring-3",
    invalid
      ? "border-destructive focus-visible:ring-destructive/30"
      : "border-input focus-visible:ring-ring/50"
  )
}

export function StorageForm({ initial, submitLabel, onSubmit }: Props) {
  const [form, setForm] = useState<ExportInput>(emptyExportInput(initial))
  const [catalog, setCatalog] = useState<OptionCatalog | null>(null)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [preview, setPreview] = useState("")
  const [error, setError] = useState("")
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [busy, setBusy] = useState(false)
  const [pending, setPending] = useState<PendingChange | null>(null)
  const [pendingApply, setPendingApply] = useState(false)
  const [touched, setTouched] = useState(false)
  const [quotaUnit, setQuotaUnit] = useState<QuotaUnit>(() =>
    preferUnit(initial?.quotaBytes ?? 0)
  )
  const [quotaAmount, setQuotaAmount] = useState<string>(() => {
    const bytes = initial?.quotaBytes ?? 0
    if (!bytes) return ""
    return String(bytesToUnitAmount(bytes, preferUnit(bytes)))
  })

  useEffect(() => {
    void Promise.all([api.catalog(), api.network()]).then(([cat, net]) => {
      setCatalog(cat)
      setSuggestions(net.suggestions || [])
      if (!initial) {
        setForm(emptyExportInput(cat.default))
      }
    })
  }, [initial])

  useEffect(() => {
    const bytes = initial?.quotaBytes ?? 0
    const unit = preferUnit(bytes)
    setQuotaUnit(unit)
    setQuotaAmount(bytes ? String(bytesToUnitAmount(bytes, unit)) : "")
  }, [initial?.quotaBytes])

  function applyQuota(amountRaw: string, unit: QuotaUnit, unlimited: boolean) {
    if (unlimited) {
      setQuotaAmount("")
      setForm((prev) => ({ ...prev, quotaBytes: 0 }))
      return
    }
    const amount = Number(amountRaw)
    const bytes = unitToBytes(Number.isFinite(amount) ? amount : 0, unit)
    setForm((prev) => ({ ...prev, quotaBytes: bytes }))
  }

  useEffect(() => {
    const handle = window.setTimeout(() => {
      api
        .previewExport(form)
        .then((r) => {
          setPreview(r.exportLine)
          if (!touched) setError("")
        })
        .catch((err: Error) => {
          if (!touched) setError(err.message)
        })
    }, 250)
    return () => window.clearTimeout(handle)
  }, [form, touched])

  useEffect(() => {
    if (!touched) return
    setFieldErrors(validateForm(form))
  }, [form, touched])

  const fieldsByKey = useMemo(() => {
    const map = new Map<string, CatalogField>()
    for (const f of catalog?.fields || []) map.set(f.key, f)
    return map
  }, [catalog])

  const clientOptions = useMemo(() => {
    const values = new Set<string>(["*", ...suggestions])
    for (const part of form.clients.replaceAll(",", " ").split(/\s+/)) {
      if (part) values.add(part)
    }
    return [...values].map((v) => ({ value: v, label: v }))
  }, [form.clients, suggestions])

  const selectedClients = useMemo(
    () =>
      form.clients
        .replaceAll(",", " ")
        .split(/\s+/)
        .filter(Boolean)
        .map((v) => ({ value: v, label: v })),
    [form.clients]
  )

  function field(key: string) {
    return fieldsByKey.get(key)
  }

  function clearFieldError(key: keyof FieldErrors) {
    setFieldErrors((prev) => {
      if (!prev[key]) return prev
      const next = { ...prev }
      delete next[key]
      return next
    })
  }

  function requestEnumChange(key: keyof ExportInput, value: string) {
    const warn = warningForEnumChange(key, value)
    if (warn && String(form[key]) !== value) {
      setPending(warn)
      return
    }
    setForm({ ...form, [key]: value })
    clearFieldError(key)
  }

  function applyPending() {
    if (!pending) return
    if (pending.key === "apply") {
      setPending(null)
      void doSubmit(true)
      return
    }
    if (pending.key === "clients") {
      setForm({ ...form, clients: String(pending.next) })
      clearFieldError("clients")
    } else {
      setForm({ ...form, [pending.key]: pending.next as never })
      clearFieldError(pending.key as keyof FieldErrors)
    }
    setPending(null)
  }

  function enumField(key: keyof ExportInput, label: string) {
    const meta = field(String(key))
    const options: SelectOption[] =
      meta?.choices?.map((c) => ({ value: c.value, label: c.label })) || []
    const invalid = Boolean(fieldErrors[key])
    return (
      <div key={String(key)} className="space-y-1.5 text-sm">
        <FieldLabel label={label} field={meta} error={fieldErrors[key]} />
        <Select
          classNamePrefix="rs"
          styles={selectStyles({ invalid })}
          theme={selectTheme}
          options={options}
          value={options.find((o) => o.value === String(form[key])) || null}
          onChange={(opt) => {
            const v = (opt as SelectOption | null)?.value
            if (v) requestEnumChange(key, v)
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.stopPropagation()
          }}
          aria-invalid={invalid || undefined}
        />
      </div>
    )
  }

  async function doSubmit(apply: boolean) {
    setTouched(true)
    const errors = validateForm(form)
    setFieldErrors(errors)
    const keys = Object.keys(errors)
    if (keys.length > 0) {
      setError("Fix the highlighted fields before saving")
      return
    }

    const check = validateClientsInput(form.clients)
    setBusy(true)
    setError("")
    try {
      const validated = await api.validateClients(check.normalized)
      await onSubmit({ ...form, clients: validated.clients }, apply)
    } catch (err) {
      const message = err instanceof Error ? err.message : "save failed"
      setError(message)
      const lower = message.toLowerCase()
      if (lower.includes("path") || lower.includes("mkdir")) {
        setFieldErrors((prev) => ({ ...prev, path: message }))
      } else if (lower.includes("client")) {
        setFieldErrors((prev) => ({ ...prev, clients: message }))
      } else if (lower.includes("fsid")) {
        setFieldErrors((prev) => ({ ...prev, fsid: message }))
      }
    } finally {
      setBusy(false)
    }
  }

  function startSave(apply: boolean) {
    setTouched(true)
    const errors = validateForm(form)
    setFieldErrors(errors)
    if (Object.keys(errors).length > 0) {
      setError("Fix the highlighted fields before saving")
      return
    }
    if (apply) {
      setPending(warningForApply(form))
      setPendingApply(true)
      return
    }
    setPendingApply(false)
    void doSubmit(false)
  }

  return (
    <div
      className="space-y-6"
      onKeyDown={blockEnterSubmit}
      role="presentation"
    >
      {catalog?.notes?.length ? (
        <ul className="list-disc space-y-1 rounded-xl border border-border p-4 text-sm text-muted-foreground ps-8">
          {catalog.notes.map((n) => (
            <li key={n}>{n}</li>
          ))}
        </ul>
      ) : null}

      {error ? (
        <div className="rounded-lg border border-destructive/40 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <section className="grid gap-4 rounded-xl border border-border p-4 sm:grid-cols-2">
        <h3 className="font-medium sm:col-span-2">Path</h3>
        <div className="space-y-1.5 text-sm sm:col-span-2">
          <FieldLabel
            label="Export path"
            field={field("path")}
            error={fieldErrors.path}
          />
          <div
            className={cn(
              fieldErrors.path && "rounded-lg ring-2 ring-destructive/30"
            )}
          >
            <PathBrowser
              value={form.path}
              onChange={(path) => {
                setForm({ ...form, path })
                clearFieldError("path")
              }}
            />
          </div>
        </div>
        <div className="space-y-1.5 text-sm">
          <FieldLabel
            label="Fsid"
            field={field("fsid")}
            htmlFor="export-fsid"
            error={fieldErrors.fsid}
          />
          <input
            id="export-fsid"
            type="number"
            min={0}
            value={form.fsid}
            onChange={(e) => {
              setForm({ ...form, fsid: Number(e.target.value) || 0 })
              clearFieldError("fsid")
            }}
            className={inputClass(Boolean(fieldErrors.fsid))}
            aria-invalid={Boolean(fieldErrors.fsid)}
          />
        </div>
        <div className="flex items-center gap-2 text-sm">
          <input
            id="export-enabled"
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
          />
          <FieldLabel
            label="Enabled"
            field={field("enabled")}
            htmlFor="export-enabled"
          />
        </div>
        <div className="space-y-1.5 text-sm sm:col-span-2">
          <FieldLabel
            label="Description"
            htmlFor="export-description"
            fallbackHelp="Optional note shown in the storages list and included in manager logs when this export is toggled or updated."
          />
          <textarea
            id="export-description"
            rows={2}
            value={form.description ?? ""}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder="e.g. media share for LAN clients"
            className={cn(
              "w-full rounded-lg border border-input bg-background px-3 py-2 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            )}
          />
        </div>
      </section>

      <section className="space-y-1.5 rounded-xl border border-border p-4">
        <FieldLabel
          label="Clients"
          field={field("clients")}
          error={fieldErrors.clients}
        />
        <CreatableSelect
          isMulti
          classNamePrefix="rs"
          styles={selectStyles({ invalid: Boolean(fieldErrors.clients) })}
          theme={selectTheme}
          options={clientOptions}
          value={selectedClients}
          onChange={(opts) => {
            const list = opts ? [...opts] : []
            const next = list.map((o) => o.value).join(" ") || "*"
            const warn = warningForClients(next)
            if (warn && next !== form.clients) {
              setPending(warn)
              return
            }
            setForm({ ...form, clients: next })
            clearFieldError("clients")
          }}
          onCreateOption={(input) => {
            const next = [...selectedClients.map((c) => c.value), input]
              .filter(Boolean)
              .join(" ")
            const warn = warningForClients(next)
            if (warn) {
              setPending(warn)
              return
            }
            setForm({ ...form, clients: next })
            clearFieldError("clients")
          }}
          placeholder="Type IP/CIDR and press Enter to add"
          formatCreateLabel={(input) => `Add client “${input}”`}
          onKeyDown={(e) => {
            if (e.key === "Enter") e.stopPropagation()
          }}
          aria-invalid={Boolean(fieldErrors.clients) || undefined}
        />
      </section>

      <section className="grid gap-4 rounded-xl border border-border p-4 sm:grid-cols-2">
        <h3 className="font-medium sm:col-span-2">Access & performance</h3>
        {enumField("accessMode", "Access")}
        {enumField("commitMode", "Commit")}
        {enumField("writeDelay", "Write delay")}
        {enumField("subtreeCheck", "Subtree check")}
      </section>

      <section className="grid gap-4 rounded-xl border border-border p-4 sm:grid-cols-2">
        <h3 className="font-medium sm:col-span-2">Security & squashing</h3>
        {enumField("security", "Security")}
        {enumField("rootSquash", "Root squash")}
        {enumField("allSquash", "All squash")}
        {enumField("portMode", "Ports")}
        <div className="space-y-1.5 text-sm">
          <FieldLabel
            label="anonuid"
            field={field("anonUid")}
            htmlFor="export-anonuid"
            error={fieldErrors.anonUid}
          />
          <input
            id="export-anonuid"
            type="number"
            value={form.anonUid ?? ""}
            onChange={(e) => {
              setForm({
                ...form,
                anonUid: e.target.value === "" ? null : Number(e.target.value),
              })
              clearFieldError("anonUid")
            }}
            className={inputClass(Boolean(fieldErrors.anonUid))}
            aria-invalid={Boolean(fieldErrors.anonUid)}
          />
        </div>
        <div className="space-y-1.5 text-sm">
          <FieldLabel
            label="anongid"
            field={field("anonGid")}
            htmlFor="export-anongid"
            error={fieldErrors.anonGid}
          />
          <input
            id="export-anongid"
            type="number"
            value={form.anonGid ?? ""}
            onChange={(e) => {
              setForm({
                ...form,
                anonGid: e.target.value === "" ? null : Number(e.target.value),
              })
              clearFieldError("anonGid")
            }}
            className={inputClass(Boolean(fieldErrors.anonGid))}
            aria-invalid={Boolean(fieldErrors.anonGid)}
          />
        </div>
      </section>

      <section className="grid gap-4 rounded-xl border border-border p-4 sm:grid-cols-2">
        <h3 className="font-medium sm:col-span-2">Storage quota</h3>
        <p className="text-xs text-muted-foreground sm:col-span-2">
          Default is unlimited. Values use binary units (1 GB = 1024³ bytes).
          When directory usage reaches the quota, the manager POSTs a JSON{" "}
          <code className="font-mono">quota.full</code> event to the postback
          URL (once until usage drops below the limit).
        </p>
        <div className="flex items-center gap-2 text-sm sm:col-span-2">
          <input
            id="export-unlimited"
            type="checkbox"
            checked={(form.quotaBytes ?? 0) === 0}
            onChange={(e) => {
              if (e.target.checked) {
                applyQuota("", quotaUnit, true)
                return
              }
              const nextAmount = quotaAmount || "1"
              setQuotaAmount(nextAmount)
              applyQuota(nextAmount, quotaUnit, false)
            }}
          />
          <label htmlFor="export-unlimited">Unlimited quota</label>
        </div>
        <div className="space-y-1.5 text-sm sm:col-span-2">
          <FieldLabel
            label="Quota size"
            fallbackHelp="Enter an amount and choose MB, GB, or TB. Stored as bytes on the server (binary: 1 MB = 1048576 bytes)."
            error={fieldErrors.quotaBytes}
          />
          <div className="grid gap-2 sm:grid-cols-[1fr_160px]">
            <input
              type="number"
              min={0}
              step="any"
              disabled={(form.quotaBytes ?? 0) === 0}
              value={(form.quotaBytes ?? 0) === 0 ? "" : quotaAmount}
              onChange={(e) => {
                const next = e.target.value
                setQuotaAmount(next)
                if (next.trim() === "" || Number(next) <= 0) {
                  applyQuota("", quotaUnit, true)
                  return
                }
                applyQuota(next, quotaUnit, false)
              }}
              className={inputClass()}
              placeholder="e.g. 50"
            />
            <Select
              classNamePrefix="rs"
              styles={selectStyles()}
              theme={selectTheme}
              isDisabled={(form.quotaBytes ?? 0) === 0}
              options={QUOTA_UNIT_OPTIONS}
              value={
                QUOTA_UNIT_OPTIONS.find((o) => o.value === quotaUnit) || null
              }
              onChange={(opt) => {
                const unit = ((opt as SelectOption | null)?.value ||
                  "GB") as QuotaUnit
                setQuotaUnit(unit)
                if ((form.quotaBytes ?? 0) === 0 && !quotaAmount) return
                const amount = quotaAmount || "1"
                setQuotaAmount(amount)
                applyQuota(amount, unit, false)
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") e.stopPropagation()
              }}
            />
          </div>
          <p className="text-xs text-muted-foreground">
            {(form.quotaBytes ?? 0) > 0
              ? `Stored as ${(form.quotaBytes ?? 0).toLocaleString()} bytes`
              : "No quota limit (unlimited)"}
          </p>
          <div className="flex flex-wrap gap-2 text-xs">
            {[
              { label: "500 MB", amount: "500", unit: "MB" as QuotaUnit },
              { label: "1 GB", amount: "1", unit: "GB" as QuotaUnit },
              { label: "10 GB", amount: "10", unit: "GB" as QuotaUnit },
              { label: "100 GB", amount: "100", unit: "GB" as QuotaUnit },
              { label: "1 TB", amount: "1", unit: "TB" as QuotaUnit },
            ].map((s) => (
              <button
                key={s.label}
                type="button"
                className="rounded-md border border-border px-2 py-1 hover:bg-muted"
                onClick={() => {
                  setQuotaUnit(s.unit)
                  setQuotaAmount(s.amount)
                  applyQuota(s.amount, s.unit, false)
                }}
              >
                {s.label}
              </button>
            ))}
          </div>
        </div>
        <div className="space-y-1.5 text-sm sm:col-span-2">
          <FieldLabel
            label="Quota full postback URL"
            fallbackHelp="HTTPS endpoint that receives POST application/json when this export hits its quota."
          />
          <input
            type="url"
            value={form.quotaPostbackUrl ?? ""}
            onChange={(e) =>
              setForm({ ...form, quotaPostbackUrl: e.target.value })
            }
            className={inputClass()}
            placeholder="https://hooks.example.com/nfs-quota-full"
          />
        </div>
      </section>

      <section className="grid gap-4 rounded-xl border border-border p-4 sm:grid-cols-2">
        <h3 className="font-medium sm:col-span-2">Topology / advanced</h3>
        {enumField("hideMode", "Hide")}
        {enumField("lockMode", "Locks")}
        <div className="flex items-center gap-2 text-sm sm:col-span-2">
          <input
            id="export-crossmnt"
            type="checkbox"
            checked={form.crossmnt}
            onChange={(e) => setForm({ ...form, crossmnt: e.target.checked })}
          />
          <FieldLabel
            label="crossmnt (expose child mounts)"
            field={field("crossmnt")}
            htmlFor="export-crossmnt"
          />
        </div>
      </section>

      <section className="rounded-xl border border-border p-4">
        <h3 className="mb-2 font-medium">Export line preview</h3>
        <pre className="overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs whitespace-pre-wrap">
          {preview || "…"}
        </pre>
      </section>

      <div className="flex flex-wrap gap-2">
        <Button type="button" disabled={busy} onClick={() => startSave(false)}>
          {busy && !pendingApply ? "Saving…" : submitLabel}
        </Button>
        <Button
          type="button"
          variant="secondary"
          disabled={busy}
          onClick={() => startSave(true)}
        >
          {busy && pendingApply ? "Applying…" : "Save & Apply"}
        </Button>
      </div>

      <Dialog
        open={Boolean(pending)}
        title={pending?.title || ""}
        tone={pending?.tone || "warning"}
        confirmLabel="Confirm"
        cancelLabel="Cancel"
        onClose={() => setPending(null)}
        onConfirm={applyPending}
      >
        <ul className="list-disc space-y-2 ps-5">
          {(pending?.bullets || []).map((b) => (
            <li key={b}>{b}</li>
          ))}
        </ul>
      </Dialog>
    </div>
  )
}
