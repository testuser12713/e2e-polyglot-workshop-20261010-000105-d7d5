import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import OrderTracking from './OrderTracking'

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === null ? '' : JSON.stringify(body)),
  } as unknown as Response
}

function stubFetch(status: number, body: unknown) {
  const fn = vi.fn(async () => jsonResponse(status, body))
  vi.stubGlobal('fetch', fn)
  return fn
}

function renderPage(initialEntries: string[] = ['/track']) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <OrderTracking />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

const orderResponse = {
  order: {
    order_number: 'AW-2025-000123',
    status: 'in_progress',
    desired_date: '2025-03-07',
    vehicle_plate: 'M-AB 1234',
    make: 'BMW',
    model: '320d',
    customer_name: 'Anna Beispiel',
    problem: 'Bremsen quietschen',
    customer: {
      id: 1,
      name: 'Anna Beispiel',
      email: 'anna@example.com',
      phone: '0170 000000',
    },
    vehicle: {
      id: 1,
      plate: 'M-AB 1234',
      make: 'BMW',
      model: '320d',
      mileage: 120000,
    },
    items: [],
    history: [
      { at: '2025-03-05T17:48:00Z', from_status: null, to_status: 'requested' },
      {
        at: '2025-03-06T09:15:00Z',
        from_status: 'requested',
        to_status: 'confirmed',
      },
      {
        at: '2025-03-07T14:32:00Z',
        from_status: 'confirmed',
        to_status: 'in_progress',
      },
    ],
  },
  invoice: null,
}

describe('OrderTracking', () => {
  it('shows the status, the vehicle and the history newest first on success', async () => {
    stubFetch(200, orderResponse)
    const user = userEvent.setup()
    const { container } = renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'aw-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'm-ab 1234')
    await user.click(
      screen.getByRole('button', { name: 'Status abfragen' }),
    )

    expect((await screen.findAllByText('in Arbeit')).length).toBeGreaterThan(0)
    expect(screen.getByText('AW-2025-000123')).toBeInTheDocument()
    expect(screen.getByText(/BMW 320d/)).toBeInTheDocument()
    expect(screen.getByText('M-AB 1234')).toBeInTheDocument()
    expect(screen.getByText('07.03.2025')).toBeInTheDocument()

    const timeline = container.querySelectorAll('ol li')
    expect(timeline).toHaveLength(3)
    expect(timeline[0].textContent).toContain('07.03.2025, 14:32 (UTC)')
    expect(timeline[0].textContent).toContain('in Arbeit')
    expect(timeline[2].textContent).toContain('05.03.2025, 17:48 (UTC)')
    expect(timeline[2].textContent).toContain('angefragt')
  })

  it('shows one German mismatch alert and no order data for a wrong plate', async () => {
    stubFetch(404, {
      error: {
        code: 'not_found',
        message: 'Auftrag oder Kennzeichen nicht gefunden.',
      },
    })
    const user = userEvent.setup()
    const { container } = renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'AW-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'B-XY 9999')
    await user.click(
      screen.getByRole('button', { name: 'Status abfragen' }),
    )

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(
      'Zu diesen Angaben wurde kein Auftrag gefunden.',
    )
    expect(screen.queryByText('in Arbeit')).not.toBeInTheDocument()
    expect(screen.queryByText('AW-2025-000123')).not.toBeInTheDocument()
    expect(container.querySelectorAll('ol li')).toHaveLength(0)
  })

  it('calls the public tracking endpoint with the entered order number and plate', async () => {
    const fetchMock = stubFetch(200, orderResponse)
    const user = userEvent.setup()
    renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'aw-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'm-ab 1234')
    await user.click(
      screen.getByRole('button', { name: 'Status abfragen' }),
    )

    await screen.findAllByText('in Arbeit')
    const [url] = fetchMock.mock.calls[0] as unknown as [string]
    expect(url).toContain('/api/public/orders/AW-2025-000123')
    expect(url).toContain('plate=M-AB%201234')
  })
})
