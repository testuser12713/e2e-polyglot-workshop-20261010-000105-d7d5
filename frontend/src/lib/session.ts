/**
 * Workshop session handling.
 *
 * A successful `POST /api/shop/login` returns a bearer token plus the
 * signed-in employee. This module keeps that session in module state so the
 * whole app (TopNav, route guard, API client) sees the same instance, attaches
 * the token to every authenticated workshop request through the `api` auth
 * hook, and persists it so a reload does not throw the employee back to the
 * login.
 */

import { useSyncExternalStore } from 'react'
import { request, setAuthToken } from './api'

export interface Employee {
  id: number
  email: string
  name: string
}

export interface Session {
  token: string
  employee: Employee
}

export interface LoginResponse {
  token: string
  employee: Employee
}

const STORAGE_KEY = 'werkstatt.session'

type Listener = () => void

const listeners = new Set<Listener>()
let current: Session | null = null

function emit(): void {
  for (const listener of listeners) {
    listener()
  }
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** The current session snapshot; stable reference until the session changes. */
export function getSession(): Session | null {
  return current
}

function isEmployee(value: unknown): value is Employee {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const candidate = value as Record<string, unknown>
  return (
    typeof candidate.id === 'number' &&
    typeof candidate.email === 'string' &&
    typeof candidate.name === 'string'
  )
}

function isSession(value: unknown): value is Session {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const candidate = value as Record<string, unknown>
  return typeof candidate.token === 'string' && isEmployee(candidate.employee)
}

function readStoredSession(): Session | null {
  if (typeof localStorage === 'undefined') {
    return null
  }
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    return isSession(parsed) ? parsed : null
  } catch {
    return null
  }
}

function storeSession(session: Session): void {
  if (typeof localStorage === 'undefined') {
    return
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session))
  } catch {
    // Storage may be unavailable (private mode, quota); the in-memory session
    // still works for the current page.
  }
}

function removeStoredSession(): void {
  if (typeof localStorage === 'undefined') {
    return
  }
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Ignore unavailable storage.
  }
}

/**
 * Set (or clear) the session, keep the API auth hook and storage in sync and
 * notify every subscriber.
 */
export function setSession(session: Session | null): void {
  current = session
  if (session) {
    setAuthToken(session.token)
    storeSession(session)
  } else {
    setAuthToken(null)
    removeStoredSession()
  }
  emit()
}

/** Clear the session and forget the stored token. */
export function clearSession(): void {
  setSession(null)
}

/**
 * Restore a persisted session and attach its token to the API client.
 * Idempotent and safe to call repeatedly — it simply reloads what is stored.
 */
export function restoreSession(): Session | null {
  const stored = readStoredSession()
  current = stored
  setAuthToken(stored?.token ?? null)
  return stored
}

/**
 * Authenticate against the workshop API and open a session on success.
 *
 * @throws {ApiError} with status 401 for wrong credentials.
 */
export async function login(email: string, password: string): Promise<Session> {
  const response = await request<LoginResponse>('POST', '/api/shop/login', {
    email,
    password,
  })
  const session: Session = {
    token: response.token,
    employee: response.employee,
  }
  setSession(session)
  return session
}

/** Log out: drop the session, the token and the persisted copy. */
export function logout(): void {
  setSession(null)
}

/** React hook: re-renders the caller whenever the session changes. */
export function useSession(): Session | null {
  return useSyncExternalStore(subscribe, getSession, getSession)
}

/** The signed-in employee, or null when there is no session. */
export function getEmployee(): Employee | null {
  return current?.employee ?? null
}

/** Whether a session (and therefore a bearer token) is present. */
export function isAuthenticated(): boolean {
  return current !== null
}

// Restore any persisted session as soon as the module is loaded, so the token
// is on the API client before the first request — also after a full reload.
restoreSession()
