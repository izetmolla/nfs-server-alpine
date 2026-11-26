import { Navigate, Outlet, useLocation } from "react-router-dom"
import { useAuth } from "@/lib/auth"

export function RequireAuth() {
  const { authed } = useAuth()
  const location = useLocation()

  if (!authed) {
    return <Navigate to="/login" replace state={{ from: location }} />
  }

  return <Outlet />
}
