import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { api, getToken, setToken } from "@/lib/api"

type AuthContextValue = {
  authed: boolean
  login: (password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [authed, setAuthed] = useState(Boolean(getToken()))

  const login = useCallback(async (password: string) => {
    const res = await api.login(password)
    setToken(res.token)
    setAuthed(true)
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      // ignore
    }
    setToken(null)
    setAuthed(false)
  }, [])

  const value = useMemo(
    () => ({ authed, login, logout }),
    [authed, login, logout]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}
