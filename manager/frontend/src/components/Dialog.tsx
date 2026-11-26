import { useEffect, type ReactNode } from "react"
import { Button } from "@/components/ui/button"

type DialogProps = {
  open: boolean
  title: string
  children: ReactNode
  onClose: () => void
  tone?: "info" | "warning" | "danger"
  confirmLabel?: string
  cancelLabel?: string
  onConfirm?: () => void
  hideCancel?: boolean
}

export function Dialog({
  open,
  title,
  children,
  onClose,
  tone = "info",
  confirmLabel = "OK",
  cancelLabel = "Cancel",
  onConfirm,
  hideCancel,
}: DialogProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return
      e.stopImmediatePropagation()
      onClose()
    }
    // Capture so the topmost dialog (last registered) handles Escape first.
    window.addEventListener("keydown", onKey, true)
    return () => window.removeEventListener("keydown", onKey, true)
  }, [open, onClose])

  if (!open) return null

  const border =
    tone === "danger"
      ? "border-destructive/50"
      : tone === "warning"
        ? "border-amber-500/50"
        : "border-border"

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/55 p-4"
      role="dialog"
      aria-modal="true"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className={`max-h-[85vh] w-full max-w-lg overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-xl ${border}`}
      >
        <div className="border-b border-border px-4 py-3">
          <h3 className="font-medium">{title}</h3>
        </div>
        <div className="max-h-[60vh] overflow-y-auto px-4 py-3 text-sm leading-relaxed">
          {children}
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          {!hideCancel ? (
            <Button type="button" variant="outline" onClick={onClose}>
              {cancelLabel}
            </Button>
          ) : null}
          <Button
            type="button"
            variant={tone === "danger" ? "destructive" : "default"}
            onClick={() => {
              onConfirm?.()
              if (!onConfirm) onClose()
            }}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}
