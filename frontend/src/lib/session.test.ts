import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getAuthToken } from './api'
import {
  clearSession,
  getEmployee,
  getSession,
  isAuthenticated,
  login,
  restoreSession,
  setSession,
} from './session'

function fakeResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () =>
      body === null || body === undefined ? '' : JSON.stringify(body),
  } as unknown as Response
}

function mockFetch(
  impl: (url: string, init?: RequestInit) => Promise<Response> | Response,
) {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

const EMPLOYEE = { id: 7, email: 'anna@werkstatt-berger.de', name: 'Anna Berger' }

beforeEach(() => {
  localStorage.clear()
  clearSession()
  vi.stubEnv('VITE_API_BASE_URL', 'http://api.test')
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
  clearSession()
})

describe('workshop session', () => {
  it('stores the session and attaches the bearer token after a successful login', async () => {
    const fetchMock = mockFetch(() =>
      fakeResponse(200, { token: 'tok-123', employee: EMPLOYEE }),
    )

    const session = await login('anna@werkstatt-berger.de', 'geheim')

    expect(session.token).toBe('tok-123')
    expect(session.employee).toEqual(EMPLOYEE)
    expect(isAuthenticated()).toBe(true)
    expect(getEmployee()).toEqual(EMPLOYEE)
    expect(getAuthToken()).toBe('tok-123')

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://api.test/api/shop/login')
    expect(init.method).toBe('POST')
    expect(init.body).toBe(
      JSON.stringify({ email: 'anna@werkstatt-berger.de', password: 'geheim' }),
    )

    expect(localStorage.getItem('werkstatt.session')).toContain('tok-123')
  })

  it('does not open a session when the credentials are rejected', async () => {
    mockFetch(() =>
      fakeResponse(401, {
        error: { code: 'unauthorized', message: 'Anmeldung fehlgeschlagen.' },
      }),
    )

    const error = (await login('anna@werkstatt-berger.de', 'falsch').catch(
      (caught: unknown) => caught,
    )) as ApiError

    expect(error).toBeInstanceOf(ApiError)
    expect(error.status).toBe(401)
    expect(getSession()).toBeNull()
    expect(isAuthenticated()).toBe(false)
    expect(getAuthToken()).toBeNull()
    expect(localStorage.getItem('werkstatt.session')).toBeNull()
  })

  it('restores a persisted session and its token on a fresh load', () => {
    localStorage.setItem(
      'werkstatt.session',
      JSON.stringify({ token: 'stored-token', employee: EMPLOYEE }),
    )

    const restored = restoreSession()

    expect(restored?.token).toBe('stored-token')
    expect(getAuthToken()).toBe('stored-token')
    expect(getEmployee()?.email).toBe('anna@werkstatt-berger.de')
  })

  it('ignores corrupt persisted data', () => {
    localStorage.setItem('werkstatt.session', 'not-json')

    expect(restoreSession()).toBeNull()
    expect(isAuthenticated()).toBe(false)
    expect(getAuthToken()).toBeNull()
  })

  it('logs out and clears the token and the stored session', () => {
    setSession({ token: 'tok-123', employee: EMPLOYEE })
    expect(getAuthToken()).toBe('tok-123')

    clearSession()

    expect(getSession()).toBeNull()
    expect(isAuthenticated()).toBe(false)
    expect(getAuthToken()).toBeNull()
    expect(localStorage.getItem('werkstatt.session')).toBeNull()
  })
})
