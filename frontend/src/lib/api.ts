/**
 * The one API client for the whole web app.
 *
 * Every request goes through `request<T>(method, path, body?)`. The base URL is
 * read lazily from `import.meta.env.VITE_API_BASE_URL` (declared in RUN.json as
 * the origin of the `api` service), the bearer token is attached through the
 * small auth hook below, and a failed response is turned into an `ApiError`
 * carrying the German, human-readable message from the shared error body:
 *   {"error":{"code":"snake_case","message":"German user text"}}
 * Pages therefore always show a readable German message (AC-22) — never a
 * technical error page.
 */

export interface ApiErrorBody {
  error: {
    code: string
    message: string
  }
}

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/** Error raised for any non-2xx response or a failed network call. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const NETWORK_ERROR_MESSAGE =
  'Die Verbindung zum Server ist fehlgeschlagen. Bitte prüfen Sie Ihre Internetverbindung.'

const FALLBACK_MESSAGE =
  'Es ist ein unerwarteter Fehler aufgetreten. Bitte versuchen Sie es erneut.'

/** German fallbacks for the standard status codes when the body carries no message. */
const STATUS_MESSAGES: Record<number, string> = {
  400: 'Die Anfrage ist ungültig. Bitte prüfen Sie Ihre Eingaben.',
  401: 'Sie sind nicht angemeldet oder Ihre Sitzung ist abgelaufen.',
  403: 'Sie haben keine Berechtigung für diese Aktion.',
  404: 'Die angeforderte Ressource wurde nicht gefunden.',
  409: 'Die Angaben stehen in Konflikt mit einem bestehenden Eintrag.',
  422: 'Die übermittelten Angaben sind ungültig.',
  429: 'Zu viele Versuche. Bitte versuchen Sie es später erneut.',
  500: 'Der Server hat einen Fehler festgestellt. Bitte versuchen Sie es erneut.',
  503: 'Der Dienst ist vorübergehend nicht verfügbar. Bitte versuchen Sie es später erneut.',
}

const CODE_MESSAGES: Record<string, string> = {
  not_found: STATUS_MESSAGES[404],
  unauthorized: STATUS_MESSAGES[401],
  forbidden: STATUS_MESSAGES[403],
  conflict: STATUS_MESSAGES[409],
  validation_failed: STATUS_MESSAGES[422],
  rate_limited: STATUS_MESSAGES[429],
  internal_error: STATUS_MESSAGES[500],
  service_unavailable: STATUS_MESSAGES[503],
}

let authToken: string | null = null

/**
 * Auth hook: the session module calls this after a successful login, on
 * restore, and with `null` on logout. `request` reads it per call.
 */
export function setAuthToken(token: string | null): void {
  authToken = token
}

export function getAuthToken(): string | null {
  return authToken
}

/** The configured API origin, with a dev default when RUN.json leaves it unset. */
export function apiBaseUrl(): string {
  const configured = import.meta.env.VITE_API_BASE_URL
  if (typeof configured === 'string' && configured.trim().length > 0) {
    return configured.trim().replace(/\/+$/, '')
  }
  return 'http://localhost:8000'
}

export function apiUrl(path: string): string {
  const suffix = path.startsWith('/') ? path : `/${path}`
  return `${apiBaseUrl()}${suffix}`
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null || !('error' in value)) {
    return false
  }
  const error = (value as { error: unknown }).error
  return typeof error === 'object' && error !== null
}

function messageFor(status: number, code: string, bodyMessage?: unknown): string {
  if (typeof bodyMessage === 'string' && bodyMessage.trim().length > 0) {
    return bodyMessage
  }
  return CODE_MESSAGES[code] ?? STATUS_MESSAGES[status] ?? FALLBACK_MESSAGE
}

async function readBody(response: Response): Promise<unknown> {
  const text = await response.text()
  if (!text) {
    return null
  }
  try {
    return JSON.parse(text) as unknown
  } catch {
    return null
  }
}

/**
 * Perform an API request and return the parsed JSON body.
 *
 * @throws {ApiError} on a non-2xx status or a failed network call.
 */
export async function request<T>(
  method: HttpMethod,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (authToken) {
    headers['Authorization'] = `Bearer ${authToken}`
  }

  let response: Response
  try {
    response = await fetch(apiUrl(path), {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network_error', NETWORK_ERROR_MESSAGE)
  }

  const payload = await readBody(response)

  if (!response.ok) {
    const errorBody = isErrorBody(payload) ? payload.error : null
    const code =
      errorBody && typeof errorBody.code === 'string'
        ? errorBody.code
        : `http_${response.status}`
    throw new ApiError(
      response.status,
      code,
      messageFor(response.status, code, errorBody?.message),
    )
  }

  return payload as T
}
