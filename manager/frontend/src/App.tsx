import { Navigate, Route, Routes } from "react-router-dom"
import { AppLayout } from "@/components/layout"
import { RequireAuth } from "@/components/RequireAuth"
import { DashboardPage } from "@/pages/DashboardPage"
import { LoginPage } from "@/pages/LoginPage"
import { LogsPage } from "@/pages/LogsPage"
import { McpPage } from "@/pages/McpPage"
import { ReportsPage } from "@/pages/ReportsPage"
import { SettingsPage } from "@/pages/SettingsPage"
import { StorageCreatePage } from "@/pages/StorageCreatePage"
import { StorageEditPage } from "@/pages/StorageEditPage"
import { StoragesPage } from "@/pages/StoragesPage"
import { TokensPage } from "@/pages/TokensPage"
import { UsersPage } from "@/pages/UsersPage"

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<RequireAuth />}>
        <Route element={<AppLayout />}>
          <Route index element={<DashboardPage />} />
          <Route path="storages" element={<StoragesPage />} />
          <Route path="storages/new" element={<StorageCreatePage />} />
          <Route path="storages/:id" element={<StorageEditPage />} />
          <Route path="reports" element={<ReportsPage />} />
          <Route path="logs" element={<LogsPage />} />
          <Route path="tokens" element={<TokensPage />} />
          <Route path="mcp-admin" element={<McpPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default App
