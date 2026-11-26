import { cn } from "@/lib/utils"

type Props = {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  label: string
  id?: string
  title?: string
}

export function Toggle({
  checked,
  onChange,
  disabled,
  label,
  id,
  title,
}: Props) {
  return (
    <div
      title={title || label}
      className={cn(
        "inline-flex items-center gap-2 text-xs select-none",
        disabled ? "opacity-50" : ""
      )}
    >
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={cn(
          "relative inline-flex h-5 w-9 shrink-0 rounded-full border transition-colors",
          checked ? "border-primary bg-primary" : "border-border bg-muted",
          disabled ? "cursor-not-allowed" : "cursor-pointer"
        )}
      >
        <span
          className={cn(
            "pointer-events-none absolute top-0.5 left-0.5 size-4 rounded-full bg-background shadow transition-transform",
            checked && "translate-x-4"
          )}
        />
      </button>
      <span className="text-muted-foreground">{label}</span>
    </div>
  )
}
