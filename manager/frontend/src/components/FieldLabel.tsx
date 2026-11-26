import { Info } from "lucide-react"
import { useState, type ReactNode } from "react"
import { Dialog } from "@/components/Dialog"
import type { CatalogField } from "@/lib/api"
import { cn } from "@/lib/utils"

type Props = {
  label: string
  field?: CatalogField
  fallbackHelp?: string
  htmlFor?: string
  error?: string
  className?: string
}

export function FieldLabel({
  label,
  field,
  fallbackHelp,
  htmlFor,
  error,
  className,
}: Props) {
  const [open, setOpen] = useState(false)
  const hasHelp = Boolean(
    field?.help ||
      field?.summary ||
      field?.examples?.length ||
      field?.cases?.length ||
      fallbackHelp
  )

  const title: ReactNode = htmlFor ? (
    <label htmlFor={htmlFor} className="cursor-default">
      {label}
    </label>
  ) : (
    <span>{label}</span>
  )

  return (
    <>
      <div className={cn("flex w-fit max-w-full items-center gap-1.5", className)}>
        {title}
        {hasHelp ? (
          <button
            type="button"
            className="inline-flex size-5 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground"
            aria-label={`Help for ${label}`}
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setOpen(true)
            }}
            onMouseDown={(e) => {
              // Prevent parent <label> activation / select focus steals.
              e.preventDefault()
              e.stopPropagation()
            }}
          >
            <Info className="size-3.5" aria-hidden />
          </button>
        ) : null}
      </div>
      {error ? (
        <p className="text-xs text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <Dialog
        open={open}
        title={field?.label || label}
        onClose={() => setOpen(false)}
        hideCancel
        confirmLabel="Close"
        tone={
          field?.risk === "high"
            ? "danger"
            : field?.risk === "medium"
              ? "warning"
              : "info"
        }
      >
        <div className="space-y-3">
          {field?.summary ? <p className="font-medium">{field.summary}</p> : null}
          <p className="text-muted-foreground">
            {field?.help || fallbackHelp || "No additional details."}
          </p>
          {field?.choices?.length ? (
            <div>
              <p className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                Choices
              </p>
              <ul className="space-y-1.5">
                {field.choices.map((c) => (
                  <li key={c.value} className="rounded-lg bg-muted/60 px-2.5 py-1.5">
                    <span className="font-mono text-xs">{c.value}</span>
                    {c.help ? (
                      <span className="text-muted-foreground"> — {c.help}</span>
                    ) : null}
                    {c.warning ? (
                      <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                        Warning: {c.warning}
                      </p>
                    ) : null}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          {field?.examples?.length ? (
            <div>
              <p className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                Examples
              </p>
              <ul className="list-disc space-y-1 ps-5 text-muted-foreground">
                {field.examples.map((ex) => (
                  <li key={ex}>{ex}</li>
                ))}
              </ul>
            </div>
          ) : null}
          {field?.cases?.length ? (
            <div>
              <p className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                What happens
              </p>
              <ul className="list-disc space-y-1 ps-5 text-muted-foreground">
                {field.cases.map((c) => (
                  <li key={c}>{c}</li>
                ))}
              </ul>
            </div>
          ) : null}
          {field?.risk ? (
            <p className="text-xs text-muted-foreground">
              Risk level: <span className="font-medium">{field.risk}</span>
            </p>
          ) : null}
        </div>
      </Dialog>
    </>
  )
}
