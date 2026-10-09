import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppointmentRequest from './AppointmentRequest'

function fakeResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () =>
      body === null || body === undefined ? '' : JSON.stringify(body),
  } as unknown as Response
}

async function fillValidForm(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.type(screen.getByLabelText(/Name/), 'Anna Beispiel')
  await user.type(screen.getByLabelText(/E-Mail/), 'anna@example.com')
  await user.type(screen.getByLabelText(/Telefon/), '030 123456')
  await user.type(screen.getByLabelText(/Kennzeichen/), 'M-AB 1234')
  await user.type(screen.getByLabelText(/Marke/), 'BMW')
  await user.type(screen.getByLabelText(/Modell/), '320d')
  await user.type(screen.getByLabelText(/Kilometerstand/), '87450')
  fireEvent.change(screen.getByLabelText(/Wunschtermin/), {
    target: { value: '2025-06-01' },
  })
  await user.type(screen.getByLabelText(/Problembeschreibung/), 'Bremsen quietschen.')
}

beforeEach(() => {
  vi.stubEnv('VITE_API_BASE_URL', 'http://api.test')
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.unstubAllEnvs()
})

describe('AppointmentRequest (AC-13)', () => {
  it('shows no error on a fresh, untouched form', () => {
    render(<AppointmentRequest />)

    expect(screen.queryAllByRole('alert')).toHaveLength(0)
    expect(screen.queryByText('Bitte geben Sie Ihren Namen ein.')).not.toBeInTheDocument()
    expect(
      screen.queryByText('Bitte geben Sie eine gültige E-Mail-Adresse ein.'),
    ).not.toBeInTheDocument()
    expect(document.querySelector('.field--error')).toBeNull()
    expect(screen.getByLabelText(/E-Mail/)).not.toHaveAttribute('aria-invalid', 'true')
  })

  it('flags every missing required field and a bad e-mail address on submit', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    render(<AppointmentRequest />)

    await user.type(screen.getByLabelText(/E-Mail/), 'keine-adresse')
    await user.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(screen.getByText('Bitte geben Sie Ihren Namen ein.')).toBeInTheDocument()
    expect(
      screen.getByText('Bitte geben Sie eine gültige E-Mail-Adresse ein.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Bitte geben Sie eine Telefonnummer ein.')).toBeInTheDocument()
    expect(screen.getByText('Bitte geben Sie das Kennzeichen ein.')).toBeInTheDocument()
    expect(screen.getByText('Bitte geben Sie die Marke ein.')).toBeInTheDocument()
    expect(screen.getByText('Bitte geben Sie das Modell ein.')).toBeInTheDocument()
    expect(screen.getByText('Bitte geben Sie den Kilometerstand an.')).toBeInTheDocument()
    expect(screen.getByText('Bitte wählen Sie einen Wunschtermin.')).toBeInTheDocument()
    expect(screen.getByText('Bitte beschreiben Sie das Problem.')).toBeInTheDocument()
    expect(document.querySelectorAll('.field--error')).toHaveLength(9)

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('does not flag a field before it was touched', async () => {
    const user = userEvent.setup()
    render(<AppointmentRequest />)

    await user.click(screen.getByLabelText(/Name/))
    await user.tab()

    expect(screen.getByText('Bitte geben Sie Ihren Namen ein.')).toBeInTheDocument()
    expect(
      screen.queryByText('Bitte geben Sie eine gültige E-Mail-Adresse ein.'),
    ).not.toBeInTheDocument()
  })

  it('posts a valid request and shows the created order number', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn(async () =>
      fakeResponse(201, { order_number: 'AW-2025-000124', status: 'requested' }),
    )
    vi.stubGlobal('fetch', fetchMock)
    render(<AppointmentRequest />)

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText(/AW-2025-000124/)).toBeInTheDocument()

    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('http://api.test/api/public/orders')
    expect(init.method).toBe('POST')
    const body = JSON.parse(init.body as string) as {
      customer: { name: string; email: string; phone: string }
      vehicle: { plate: string; make: string; model: string; mileage: number }
      desired_date: string
      problem: string
      items: unknown[]
    }
    expect(body.customer).toEqual({
      name: 'Anna Beispiel',
      email: 'anna@example.com',
      phone: '030 123456',
    })
    expect(body.vehicle).toEqual({
      plate: 'M-AB 1234',
      make: 'BMW',
      model: '320d',
      mileage: 87450,
    })
    expect(body.desired_date).toBe('2025-06-01')
    expect(body.items).toEqual([])
  })

  it('shows a readable error when the request fails', async () => {
    const user = userEvent.setup()
    const fetchMock = vi.fn(async () =>
      fakeResponse(422, {
        error: { code: 'validation_failed', message: 'Die Angaben sind ungültig.' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)
    render(<AppointmentRequest />)

    await fillValidForm(user)
    await user.click(screen.getByRole('button', { name: 'Termin anfragen' }))

    expect(await screen.findByText('Die Angaben sind ungültig.')).toBeInTheDocument()
  })

  it('adds a position row and updates the summary', async () => {
    const user = userEvent.setup()
    render(<AppointmentRequest />)

    await user.click(screen.getByRole('button', { name: 'Position hinzufügen' }))
    expect(screen.getAllByLabelText(/Beschreibung/)).toHaveLength(2)

    const amountInputs = screen.getAllByLabelText(/Menge\/Stunden/)
    const priceInputs = screen.getAllByLabelText(/Einzelpreis/)
    await user.type(amountInputs[0], '2')
    await user.type(priceInputs[0], '89,00')

    expect(screen.getByText(/^178,00\s€$/)).toBeInTheDocument()
    expect(screen.getByText(/^211,82\s€$/)).toBeInTheDocument()
  })
})
