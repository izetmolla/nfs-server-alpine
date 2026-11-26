export type QuotaUnit = "MB" | "GB" | "TB"

export const QUOTA_UNIT_OPTIONS: Array<{ value: QuotaUnit; label: string }> = [
  { value: "MB", label: "MB" },
  { value: "GB", label: "GB" },
  { value: "TB", label: "TB" },
]

const UNIT_BYTES: Record<QuotaUnit, number> = {
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
}

export function unitToBytes(amount: number, unit: QuotaUnit): number {
  if (!Number.isFinite(amount) || amount <= 0) return 0
  return Math.round(amount * UNIT_BYTES[unit])
}

export function bytesToUnitAmount(
  bytes: number,
  unit: QuotaUnit
): number {
  if (!bytes || bytes <= 0) return 0
  const raw = bytes / UNIT_BYTES[unit]
  // Keep readable precision without floating noise.
  return Math.round(raw * 1000) / 1000
}

/** Pick the largest unit that keeps a clean-ish display value. */
export function preferUnit(bytes: number): QuotaUnit {
  if (!bytes || bytes <= 0) return "GB"
  if (bytes % UNIT_BYTES.TB === 0 || bytes >= UNIT_BYTES.TB) return "TB"
  if (bytes % UNIT_BYTES.GB === 0 || bytes >= UNIT_BYTES.GB) return "GB"
  return "MB"
}

export function formatQuotaBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "Unlimited"
  const unit = preferUnit(bytes)
  const amount = bytesToUnitAmount(bytes, unit)
  return `${amount} ${unit} (${bytes.toLocaleString()} bytes)`
}
