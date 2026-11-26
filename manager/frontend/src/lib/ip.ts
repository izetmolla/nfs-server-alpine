export function splitClients(raw: string): string[] {
  return raw
    .replaceAll(",", " ")
    .split(/\s+/)
    .map((p) => p.trim())
    .filter(Boolean)
}

function isIPv4(value: string): boolean {
  const parts = value.split(".")
  if (parts.length !== 4) return false
  return parts.every((part) => {
    if (!/^\d{1,3}$/.test(part)) return false
    const n = Number(part)
    return n >= 0 && n <= 255
  })
}

function isIPv4CIDR(value: string): boolean {
  const [ip, prefix] = value.split("/")
  if (!ip || prefix == null) return false
  if (!isIPv4(ip)) return false
  if (!/^\d{1,2}$/.test(prefix)) return false
  const n = Number(prefix)
  return n >= 0 && n <= 32
}

function isIPv6Like(value: string): boolean {
  // Accept parsed-looking IPv6 / IPv6 CIDR loosely; server does strict check.
  if (!value.includes(":")) return false
  const [addr, prefix] = value.split("/")
  if (prefix != null) {
    if (!/^\d{1,3}$/.test(prefix)) return false
    const n = Number(prefix)
    if (n < 0 || n > 128) return false
  }
  return /^[0-9a-fA-F:.]+$/.test(addr)
}

function isHostname(value: string): boolean {
  if (value.length === 0 || value.length > 253) return false
  if (value.startsWith(".") || value.endsWith(".")) return false
  return value.split(".").every((label) => {
    if (label.length === 0 || label.length > 63) return false
    if (label.startsWith("-") || label.endsWith("-")) return false
    return /^[a-zA-Z0-9-]+$/.test(label)
  })
}

export function validateClientSpec(value: string): string | null {
  const trimmed = value.trim()
  if (!trimmed) return "Client value is empty"
  if (trimmed === "*") return null
  if (isIPv4(trimmed) || isIPv4CIDR(trimmed) || isIPv6Like(trimmed) || isHostname(trimmed)) {
    return null
  }
  return `Invalid client: ${trimmed} (use *, IP, CIDR, or hostname)`
}

export function validateClientsInput(raw: string): {
  ok: boolean
  error?: string
  normalized: string
} {
  const parts = splitClients(raw || "*")
  if (parts.length === 0) {
    return { ok: true, normalized: "*" }
  }
  for (const part of parts) {
    const err = validateClientSpec(part)
    if (err) return { ok: false, error: err, normalized: raw }
  }
  return { ok: true, normalized: parts.join(" ") }
}
