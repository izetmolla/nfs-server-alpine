import axios, { type AxiosError } from "axios"

export type Export = {
  id: number
  path: string
  description?: string
  clients: string
  accessMode: string
  commitMode: string
  writeDelay: string
  subtreeCheck: string
  portMode: string
  lockMode: string
  hideMode: string
  crossmnt: boolean
  rootSquash: string
  allSquash: string
  security: string
  fsid: number
  anonUid?: number | null
  anonGid?: number | null
  enabled: boolean
  quotaBytes?: number
  quotaPostbackUrl?: string
  quotaNotifiedAt?: string | null
  exportLine?: string
  createdAt?: string
  updatedAt?: string
}

export type ExportInput = Omit<
  Export,
  "id" | "exportLine" | "createdAt" | "updatedAt" | "quotaNotifiedAt"
>

export type QuotaUsage = {
  exportId: number
  path: string
  enabled: boolean
  exists: boolean
  unlimited: boolean
  quotaBytes: number
  usedBytes: number
  remainingBytes?: number | null
  usedPct: number
  full: boolean
  quotaPostbackUrl?: string
  quotaNotifiedAt?: string | null
  fsTotalBytes: number
  fsUsedBytes: number
  fsFreeBytes: number
  error?: string
}

export type QuotaReport = {
  exports: QuotaUsage[]
  checkedAt: string
}

export type Status = {
  mountdRunning: boolean
  exportsLive: string
  lastApplyAt: string
  lastApplyOK: boolean
  lastOutput: string
  lastError: string
}

export type AppSettings = {
  defaultClients: string
  defaultReadOnly: boolean
  defaultSync: boolean
  defaultExtraOptions: string
  globalExtraOptions: string
  swaggerEnabled: boolean
  panelAllowedCidrs: string
  hasCustomPassword: boolean
}

export type PublicConfig = {
  swaggerEnabled: boolean
  panelAllowedCidrs: string
  mcpEnabled?: boolean
}

export type MCPSettings = {
  enabled: boolean
  allowWrite: boolean
  publicBaseUrl: string
}

export type MCPSession = {
  id: number
  clientName: string
  clientId: string
  scope: string
  createdAt: string
  expiresAt: string
  lastUsedAt?: string | null
  revoked: boolean
}

export type MCPClient = {
  id: number
  clientId: string
  clientName: string
  redirectUris?: string[] | string
  createdAt: string
}

export type DashboardRangePreset =
  | "today"
  | "yesterday"
  | "last_7_days"
  | "this_month"
  | "last_month"
  | "this_year"
  | "custom"

export type DashboardSnapshot = {
  status: Status
  storage: StorageReport
  connections: {
    activeClients: number
    addresses: string[]
    source: string
    detail?: string
  }
  metrics: {
    authOk: number
    authFail: number
    applyOk: number
    applyFail: number
    apiRequests: number
    firewallDeny: number
    startedAt: string
    series: Array<{
      at: string
      minute?: string
      authOk: number
      authFail: number
      applyOk: number
      applyFail: number
      apiRequests: number
      firewallDeny: number
    }>
    range: {
      preset: DashboardRangePreset | string
      from: string
      to: string
      granularity: "hour" | "day" | "week" | string
    }
    partial?: boolean
    coverageFrom?: string | null
    note?: string
  }
  logLevels: Record<string, number>
  exportsTotal: number
  exportsEnabled: number
  tokens: number
  uptimeSeconds: number
}

export type LogsQueryResult = {
  entries: LogEntry[]
  total: number
  page: number
  pageSize: number
  levels: Record<string, number>
  sources: string[]
}

export type LogEntry = {
  id: number
  time: string
  level: string
  source: string
  message: string
  detail?: string
}

export type NetworkInfo = {
  interfaces: Array<{
    name: string
    up: boolean
    loopback: boolean
    addresses: Array<{ ip: string; cidr: string; family: string }>
  }>
  suggestions: string[]
}

export type CatalogField = {
  key: string
  label: string
  help?: string
  summary?: string
  examples?: string[]
  cases?: string[]
  type: string
  choices?: Array<{
    value: string
    label: string
    help?: string
    warning?: string
  }>
  default?: unknown
  advanced?: boolean
  risk?: string
}

export type OptionCatalog = {
  fields: CatalogField[]
  default: ExportInput
  notes: string[]
}

export type FsListing = {
  path: string
  parent?: string
  exists: boolean
  isDir: boolean
  entries: Array<{ name: string; path: string; isDir: boolean }>
  allowedRoots: string[]
  writable: boolean
}

export type StorageReport = {
  paths: Array<{
    exportId: number
    path: string
    enabled: boolean
    exists: boolean
    /** Directory contents size (not whole disk). */
    usedBytes: number
    totalBytes: number
    freeBytes: number
    usedPct: number
    fsTotalBytes?: number
    fsUsedBytes?: number
    fsFreeBytes?: number
    error?: string
  }>
  /** Sum of per-export folder sizes. */
  usedBytes: number
  /** Deduped filesystem capacity behind the exports. */
  totalBytes: number
  freeBytes: number
  usedPct: number
}

export type APIToken = {
  id: number
  name: string
  tokenPrefix: string
  allowedCidrs: string
  scopes: string
  enabled: boolean
  expiresAt?: string | null
  lastUsedAt?: string | null
  createdAt?: string
}

const TOKEN_KEY = "nfs_panel_token"

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export const http = axios.create({
  baseURL: "/",
  withCredentials: true,
  headers: { "Content-Type": "application/json" },
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (r) => r,
  (error: AxiosError<{ error?: string }>) => {
    const message =
      error.response?.data?.error ||
      error.message ||
      error.response?.statusText ||
      "request failed"
    return Promise.reject(new Error(message))
  }
)

export const api = {
  login(password: string) {
    return http
      .post<{ ok: boolean; token: string }>("/api/login", { password })
      .then((r) => r.data)
  },
  logout() {
    return http.post("/api/logout").then((r) => r.data)
  },
  status() {
    return http.get<Status>("/api/status").then((r) => r.data)
  },
  publicConfig() {
    return http.get<PublicConfig>("/api/public-config").then((r) => r.data)
  },
  dashboard(params?: {
    range?: DashboardRangePreset | string
    from?: string
    to?: string
  }) {
    return http
      .get<DashboardSnapshot>("/api/dashboard", { params })
      .then((r) => r.data)
  },
  catalog() {
    return http.get<OptionCatalog>("/api/exports/option-catalog").then((r) => r.data)
  },
  previewExport(input: Partial<ExportInput>) {
    return http
      .post<{ exportLine: string; export: Export }>("/api/exports/preview", input)
      .then((r) => r.data)
  },
  listExports() {
    return http.get<Export[]>("/api/exports").then((r) => r.data)
  },
  getExport(id: number) {
    return http.get<Export>(`/api/exports/${id}`).then((r) => r.data)
  },
  createExport(input: Partial<ExportInput>) {
    return http.post<Export>("/api/exports", input).then((r) => r.data)
  },
  updateExport(id: number, input: Partial<ExportInput>) {
    return http.put<Export>(`/api/exports/${id}`, input).then((r) => r.data)
  },
  cloneExport(
    id: number,
    body: { path: string; description?: string; apply?: boolean }
  ) {
    return http
      .post<{
        export: Export
        applied?: boolean
        output?: string
        status?: Status
      }>(`/api/exports/${id}/clone`, body)
      .then((r) => r.data)
  },
  patchExport(
    id: number,
    body: {
      enabled?: boolean
      accessMode?: "rw" | "ro"
      readOnly?: boolean
      description?: string
      apply?: boolean
    }
  ) {
    return http
      .patch<{
        export: Export
        applied?: boolean
        output?: string
        status?: Status
      }>(`/api/exports/${id}`, body)
      .then((r) => r.data)
  },
  deleteExport(id: number) {
    return http.delete(`/api/exports/${id}`).then(() => undefined)
  },
  applyExports() {
    return http
      .post<{ ok: boolean; output: string; status: Status }>("/api/exports/apply")
      .then((r) => r.data)
  },
  network() {
    return http.get<NetworkInfo>("/api/network/interfaces").then((r) => r.data)
  },
  validateClients(clients: string) {
    return http
      .post<{ valid: boolean; clients: string }>("/api/network/validate-clients", {
        clients,
      })
      .then((r) => r.data)
  },
  logs(after = 0, limit = 200) {
    return http
      .get<{ entries: LogEntry[] }>("/api/logs", { params: { after, limit } })
      .then((r) => r.data)
  },
  queryLogs(params: {
    page?: number
    pageSize?: number
    level?: string
    source?: string
    q?: string
    sort?: string
    order?: string
  }) {
    return http.get<LogsQueryResult>("/api/logs", { params }).then((r) => r.data)
  },
  clearLogs() {
    return http.delete("/api/logs").then((r) => r.data)
  },
  getSettings() {
    return http.get<AppSettings>("/api/settings").then((r) => r.data)
  },
  saveSettings(settings: Omit<AppSettings, "hasCustomPassword">) {
    return http.put<AppSettings>("/api/settings", settings).then((r) => r.data)
  },
  applySettings() {
    return http.post("/api/settings/apply").then((r) => r.data)
  },
  changePassword(currentPassword: string, newPassword: string) {
    return http
      .post<{ ok: boolean; token: string }>("/api/settings/password", {
        currentPassword,
        newPassword,
      })
      .then((r) => r.data)
  },
  storageReport() {
    return http.get<StorageReport>("/api/reports/storage").then((r) => r.data)
  },
  quotaReport() {
    return http.get<QuotaReport>("/api/reports/quotas").then((r) => r.data)
  },
  quotaCheck() {
    return http
      .post<{ ok: boolean; report: QuotaReport }>("/api/reports/quotas/check")
      .then((r) => r.data)
  },
  exportUsage(id: number) {
    return http.get<QuotaUsage>(`/api/exports/${id}/usage`).then((r) => r.data)
  },
  updateExportQuota(
    id: number,
    body: {
      quotaBytes?: number
      quotaPostbackUrl?: string
      unlimited?: boolean
    }
  ) {
    return http
      .put<{ export: Export; usage: QuotaUsage }>(`/api/exports/${id}/quota`, body)
      .then((r) => r.data)
  },
  listFilesystem(path: string) {
    return http
      .get<FsListing>("/api/filesystem", { params: { path } })
      .then((r) => r.data)
  },
  mkdirFilesystem(path: string) {
    return http
      .post<FsListing>("/api/filesystem/mkdir", { path })
      .then((r) => r.data)
  },
  listTokens() {
    return http.get<APIToken[]>("/api/tokens").then((r) => r.data)
  },
  createToken(body: {
    name: string
    allowedCidrs: string
    scopes: string[]
    expiresAt?: string | null
  }) {
    return http
      .post<{ token: APIToken; rawToken: string }>("/api/tokens", body)
      .then((r) => r.data)
  },
  updateToken(
    id: number,
    body: Partial<{
      name: string
      allowedCidrs: string
      scopes: string[]
      enabled: boolean
      expiresAt: string | null
    }>
  ) {
    return http.patch<APIToken>(`/api/tokens/${id}`, body).then((r) => r.data)
  },
  deleteToken(id: number) {
    return http.delete(`/api/tokens/${id}`).then(() => undefined)
  },
  getMCPSettings() {
    return http.get<MCPSettings>("/api/mcp/settings").then((r) => r.data)
  },
  saveMCPSettings(settings: MCPSettings) {
    return http.put<MCPSettings>("/api/mcp/settings", settings).then((r) => r.data)
  },
  listMCPSessions() {
    return http.get<MCPSession[]>("/api/mcp/sessions").then((r) => r.data)
  },
  revokeMCPSession(id: number) {
    return http.delete(`/api/mcp/sessions/${id}`).then(() => undefined)
  },
  listMCPClients() {
    return http.get<MCPClient[]>("/api/mcp/clients").then((r) => r.data)
  },
  deleteMCPClient(id: number) {
    return http.delete(`/api/mcp/clients/${id}`).then(() => undefined)
  },
}

export function emptyExportInput(defaults?: Partial<ExportInput>): ExportInput {
  return {
    path: "/mnt",
    description: "",
    clients: "*",
    accessMode: "rw",
    commitMode: "async",
    writeDelay: "wdelay",
    subtreeCheck: "no_subtree_check",
    portMode: "insecure",
    lockMode: "insecure_locks",
    hideMode: "hide",
    crossmnt: false,
    rootSquash: "no_root_squash",
    allSquash: "no_all_squash",
    security: "sys",
    fsid: 0,
    anonUid: null,
    anonGid: null,
    enabled: true,
    quotaBytes: 0,
    quotaPostbackUrl: "",
    ...defaults,
  }
}

export function formatBytes(n: number): string {
  if (!n) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}
