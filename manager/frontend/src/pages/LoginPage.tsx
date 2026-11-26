import { useState, type FormEvent } from "react"
import { Navigate, useLocation, useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth"

export function LoginPage() {
  const { authed, login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)

  const from =
    (location.state as { from?: { pathname?: string } } | null)?.from
      ?.pathname || "/"

  if (authed) {
    return <Navigate to={from} replace />
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError("")
    setBusy(true)
    try {
      await login(password)
      navigate(from, { replace: true })
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed")
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background p-6">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm space-y-4 rounded-xl border border-border p-6"
      >
        <div>
          <h1 className="text-lg font-medium">NFS Manager</h1>
          <p className="text-sm text-muted-foreground">
            Sign in with the panel password.
          </p>
        </div>
        <label className="block space-y-1.5 text-sm">
          <span>Password</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="h-9 w-full rounded-lg border border-input bg-background px-3 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
            autoFocus
            required
          />
        </label>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <Button type="submit" disabled={busy} className="w-full">
          {busy ? "Signing in…" : "Sign in"}
        </Button>
      </form>
    </div>
  )
}
