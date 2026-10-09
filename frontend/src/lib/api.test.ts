import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  ApiError,
  apiBaseUrl,
  apiUrl,
  getAuthToken,
  request,
  setAuthToken,
} from './api'

function fakeResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === null || body === undefined ? '' : JSON.stringify(body)),
  } as unknown as Response
}

function mockFetch(impl: (url: string, init?: RequestInit) => Promise<Response> | Response) {
  const fn = vi.fn(impl)
  vi.stubGlobal('fetch', fn)
  return fn
}

beforeEach(() => {
  setAuthToken(null)
  vi.stubEnv('VITE_API_BASE_URL', 'http://api.test')
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

describe('apiBaseUrl', () => {
  it('uses the configured VITE_API_BASE_URL when it is set', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.test/')

    expect(apiBaseUrl()).toBe('https://api.example.test')
    expect(apiUrl('/api/shop/login')).toBe('https://api.example.test/api/shop/login')
  })

  it('falls back to the current origin when VITE_API_BASE_URL is absent', () => {
    vi.stubEnv('VITE_API_BASE_URL', '')

    expect(apiBaseUrl()).toBe(window.location.origin)
    expect(apiUrl('/api/shop/login')).toBe(
      `${window.location.origin}/api/shop/login`,
    )
  })
})

describe('request', () => {
  it('calls the configured base URL and returns the parsed body', async () => {
    const fetchMock = mockFetch(() => fakeResponse(200, { status: 'ok' }))

    const result = await request<{ status: string }>('GET', '/api/health')

    expect(result).toEqual({ status: 'ok' })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://api.test/api/health')
    expect(init.method).toBe('GET')
    expect(init.body).toBeUndefined()
  })

  it('serialises a body and sets the JSON content type', async () => {
    const fetchMock = mockFetch(() => fakeResponse(201, { id: 1 }))

    await request('POST', '/api/public/customers', { name: 'Anna' })

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(init.body).toBe(JSON.stringify({ name: 'Anna' }))
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
  })

  it('throws an ApiError carrying the German message from the standard error body', async () => {
    mockFetch(() =>
      fakeResponse(409, {
        error: {
          code: 'email_taken',
          message: 'Diese E-Mail-Adresse ist bereits vergeben.',
        },
      }),
    )

    const error = (await request('POST', '/api/public/customers', {}).catch(
      (e: unknown) => e,
    )) as ApiError

    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({
      status: 409,
      code: 'email_taken',
      message: 'Diese E-Mail-Adresse ist bereits vergeben.',
    })
  })

  it('maps a non-standard error body to a German message by status', async () => {
    mockFetch(() => fakeResponse(404, null))

    await expect(request('GET', '/api/public/orders/AW-1')).rejects.toMatchObject({
      status: 404,
      message: 'Die angeforderte Ressource wurde nicht gefunden.',
    })
  })

  it('maps an unknown error body to a generic German message', async () => {
    mockFetch(() => fakeResponse(500, { detail: 'boom' }))

    const error = (await request('GET', '/api/health').catch((e: unknown) => e)) as ApiError
    expect(error).toBeInstanceOf(ApiError)
    expect(error.message).toBe(
      'Der Server hat einen Fehler festgestellt. Bitte versuchen Sie es erneut.',
    )
  })

  it('attaches the bearer token when one is set', async () => {
    setAuthToken('secret-token')
    const fetchMock = mockFetch(() => fakeResponse(200, {}))

    await request('GET', '/api/shop/dashboard')

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBe(
      'Bearer secret-token',
    )
    expect(getAuthToken()).toBe('secret-token')
  })

  it('omits the bearer token when none is set', async () => {
    const fetchMock = mockFetch(() => fakeResponse(200, {}))

    await request('GET', '/api/health')

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBeUndefined()
  })

  it('reports a failed network call as a German ApiError', async () => {
    mockFetch(() => Promise.reject(new TypeError('fetch failed')))

    const error = (await request('GET', '/api/health').catch((e: unknown) => e)) as ApiError

    expect(error).toBeInstanceOf(ApiError)
    expect(error.status).toBe(0)
    expect(error.message).toContain('Verbindung zum Server')
  })
})
