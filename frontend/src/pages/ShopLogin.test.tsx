import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { clearSession } from '../lib/session'

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

function renderApp(initialPath: string) {
  return render(
    <MemoryRouter
      initialEntries={[initialPath]}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <App />
    </MemoryRouter>,
  )
}

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

describe('ShopLogin', () => {
  it('redirects an unauthenticated workshop page to the login', () => {
    renderApp('/shop/orders')

    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung' }),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('E-Mail')).toBeInTheDocument()
    expect(screen.getByLabelText('Passwort')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Anmelden' }),
    ).toBeInTheDocument()
    expect(
      screen.getByText('Zugang für Werkstattmitarbeiter'),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('heading', { level: 1, name: 'Aufträge' }),
    ).not.toBeInTheDocument()
  })

  it('opens the requested workshop page after a successful login', async () => {
    const user = userEvent.setup()
    const employee = {
      id: 7,
      email: 'anna@werkstatt-berger.de',
      name: 'Anna Berger',
    }
    const fetchMock = mockFetch(() =>
      fakeResponse(200, { token: 'tok-123', employee }),
    )

    renderApp('/shop/orders')

    await user.type(screen.getByLabelText('E-Mail'), 'anna@werkstatt-berger.de')
    await user.type(screen.getByLabelText('Passwort'), 'geheim')
    await user.click(screen.getByRole('button', { name: 'Anmelden' }))

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Aufträge' }),
    ).toBeInTheDocument()
    expect(screen.getByText('anna@werkstatt-berger.de')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeInTheDocument()

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('http://api.test/api/shop/login')
    expect(init.method).toBe('POST')
    expect(localStorage.getItem('werkstatt.session')).toContain('tok-123')
  })

  it('shows exactly one generic alert on a 401 without revealing the wrong part', async () => {
    const user = userEvent.setup()
    mockFetch(() =>
      fakeResponse(401, {
        error: { code: 'unauthorized', message: 'Anmeldung fehlgeschlagen.' },
      }),
    )

    renderApp('/shop/login')

    await user.type(screen.getByLabelText('E-Mail'), 'anna@werkstatt-berger.de')
    await user.type(screen.getByLabelText('Passwort'), 'falsch')
    await user.click(screen.getByRole('button', { name: 'Anmelden' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('E-Mail oder Passwort ist falsch.')
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung' }),
    ).toBeInTheDocument()
    expect(localStorage.getItem('werkstatt.session')).toBeNull()
  })

  it('disables the button and shows the loading state while sending', async () => {
    const user = userEvent.setup()
    let resolveResponse: (response: Response) => void = () => {}
    const pending = new Promise<Response>((resolve) => {
      resolveResponse = resolve
    })
    mockFetch(() => pending)

    renderApp('/shop/login')

    await user.type(screen.getByLabelText('E-Mail'), 'anna@werkstatt-berger.de')
    await user.type(screen.getByLabelText('Passwort'), 'geheim')
    await user.click(screen.getByRole('button', { name: 'Anmelden' }))

    const loadingButton = screen.getByRole('button', { name: /Wird gesendet/ })
    expect(loadingButton).toBeDisabled()

    resolveResponse(
      fakeResponse(200, {
        token: 'tok-123',
        employee: { id: 1, email: 'a@b.de', name: 'A' },
      }),
    )

    await waitFor(() => {
      expect(
        screen.getByRole('heading', { level: 1, name: 'Aufträge' }),
      ).toBeInTheDocument()
    })
  })
})
