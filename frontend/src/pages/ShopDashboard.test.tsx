import { render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ShopDashboard from './ShopDashboard'

const METRICS = {
  open_orders: 7,
  finished_today: 4,
  revenue_month_cents: 1248000,
}

const ORDERS = [
  {
    order_number: 'AW-2025-000123',
    status: 'in_progress',
    desired_date: '2025-03-07',
    vehicle_plate: 'M-AB 1234',
    make: 'BMW',
    model: '320d',
    customer_name: 'Anna Berger',
  },
  {
    order_number: 'AW-2025-000122',
    status: 'confirmed',
    desired_date: '2025-03-08',
    vehicle_plate: 'M-CD 9911',
    make: 'VW',
    model: 'Golf 7',
    customer_name: 'Carl Dorn',
  },
  {
    order_number: 'AW-2025-000121',
    status: 'requested',
    desired_date: '2025-03-09',
    vehicle_plate: 'M-EF 2210',
    make: 'Audi',
    model: 'A4 Avant',
    customer_name: 'Eva Falk',
  },
  {
    order_number: 'AW-2025-000120',
    status: 'done',
    desired_date: '2025-03-06',
    vehicle_plate: 'M-GH 3344',
    make: 'Opel',
    model: 'Astra',
    customer_name: 'Gerd Holm',
  },
] as const

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => JSON.stringify(body),
  } as unknown as Response
}

function stubApi(handler?: (url: string) => Response | Promise<Response>) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = String(input)
    if (handler) {
      return Promise.resolve(handler(url))
    }
    if (url.includes('/api/shop/dashboard')) {
      return Promise.resolve(jsonResponse(METRICS))
    }
    if (url.includes('/api/shop/orders')) {
      return Promise.resolve(jsonResponse({ orders: ORDERS }))
    }
    return Promise.reject(new Error(`unexpected request: ${url}`))
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function renderDashboard() {
  return render(
    <MemoryRouter
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <ShopDashboard />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ShopDashboard', () => {
  it('renders the three figures from the dashboard endpoint', async () => {
    stubApi()
    renderDashboard()

    await waitFor(() =>
      expect(screen.getByTestId('stat-open-orders')).toHaveTextContent('7'),
    )
    expect(screen.getByTestId('stat-finished-today')).toHaveTextContent('4')
    expect(screen.getByTestId('stat-revenue-month')).toHaveTextContent(
      '12.480,00 €',
    )

    expect(screen.getByText('Offene Aufträge')).toBeInTheDocument()
    expect(screen.getByText('Heute fertig')).toBeInTheDocument()
    expect(screen.getByText(/^Umsatz /)).toBeInTheDocument()
    expect(
      screen.getByText('aus Rechnungen des laufenden Monats'),
    ).toBeInTheDocument()
  })

  it('lists the newest orders and counts them by status', async () => {
    stubApi()
    renderDashboard()

    await waitFor(() =>
      expect(screen.getAllByText('AW-2025-000123').length).toBeGreaterThan(0),
    )

    expect(screen.getAllByText('M-AB 1234').length).toBeGreaterThan(0)
    expect(screen.getAllByText('BMW 320d').length).toBeGreaterThan(0)

    const statusSection = screen
      .getByText('Aufträge nach Status')
      .closest('section') as HTMLElement
    expect(
      within(statusSection).getByTestId('status-count-requested'),
    ).toHaveTextContent('1')
    expect(
      within(statusSection).getByTestId('status-count-confirmed'),
    ).toHaveTextContent('1')
    expect(
      within(statusSection).getByTestId('status-count-in_progress'),
    ).toHaveTextContent('1')
    expect(
      within(statusSection).getByTestId('status-count-done'),
    ).toHaveTextContent('1')
    expect(
      within(statusSection).getByTestId('status-count-picked_up'),
    ).toHaveTextContent('0')
  })

  it('shows a readable German alert when the metrics request fails', async () => {
    stubApi((url) => {
      if (url.includes('/api/shop/dashboard')) {
        return jsonResponse(
          { error: { code: 'unauthorized', message: 'Bitte melden Sie sich an.' } },
          401,
        )
      }
      return jsonResponse({ orders: [] })
    })
    renderDashboard()

    await waitFor(() =>
      expect(screen.getByRole('alert')).toHaveTextContent(
        'Bitte melden Sie sich an.',
      ),
    )
  })

  it('renders an empty state when there are no orders', async () => {
    stubApi((url) => {
      if (url.includes('/api/shop/dashboard')) {
        return jsonResponse(METRICS)
      }
      return jsonResponse({ orders: [] })
    })
    renderDashboard()

    await waitFor(() =>
      expect(
        screen.getByText('Noch keine Aufträge vorhanden.'),
      ).toBeInTheDocument(),
    )
  })
})
