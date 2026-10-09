import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import ShopOrders, { type OrderSummary } from './ShopOrders'
import { setAuthToken } from '../lib/api'

const ORDERS: OrderSummary[] = [
  {
    order_number: 'AW-2025-000123',
    status: 'in_progress',
    desired_date: '2025-03-07',
    vehicle_plate: 'M-AB 1234',
    make: 'BMW',
    model: '320d',
    customer_name: 'Anna Muster',
  },
  {
    order_number: 'AW-2025-000122',
    status: 'confirmed',
    desired_date: '2025-03-10',
    vehicle_plate: 'M-CD 9911',
    make: 'VW',
    model: 'Golf 7',
    customer_name: 'Berta Beispiel',
  },
  {
    order_number: 'AW-2025-000121',
    status: 'requested',
    desired_date: '2025-03-11',
    vehicle_plate: 'M-EF 2210',
    make: 'Audi',
    model: 'A4 Avant',
    customer_name: 'Carl Probe',
  },
]

/** Fetch stub that filters like the real API (`?status=…&plate=…`). */
function installFetch() {
  const fn = vi.fn((input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input.toString()
    const url = new URL(raw, 'http://localhost:8000')
    const status = url.searchParams.get('status')
    const plate = url.searchParams.get('plate')

    let result = ORDERS
    if (status) {
      result = result.filter((order) => order.status === status)
    }
    if (plate) {
      result = result.filter((order) => order.vehicle_plate.includes(plate))
    }

    return Promise.resolve({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ orders: result }),
    } as unknown as Response)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

function renderShopOrders(initialPath = '/shop/orders') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <ShopOrders />
    </MemoryRouter>,
  )
}

function lastRequestUrl(fn: ReturnType<typeof installFetch>): URL {
  const calls = fn.mock.calls
  const call = calls[calls.length - 1]
  const input = call?.[0] as string
  return new URL(input, 'http://localhost:8000')
}

beforeEach(() => {
  setAuthToken(null)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ShopOrders order list', () => {
  it('renders order number, plate, vehicle, desired date and status from the API', async () => {
    installFetch()
    renderShopOrders()

    const table = await screen.findByRole('table')
    const row = within(table).getByText('AW-2025-000123').closest('tr')
    expect(row).not.toBeNull()
    const cells = within(row as HTMLElement)

    expect(cells.getByText('M-AB 1234')).toBeInTheDocument()
    expect(cells.getByText('BMW 320d')).toBeInTheDocument()
    expect(cells.getByText('07.03.2025')).toBeInTheDocument()
    expect(cells.getByText('in Arbeit')).toBeInTheDocument()
  })

  it('links every row action to that order detail page', async () => {
    installFetch()
    renderShopOrders()

    const table = await screen.findByRole('table')
    const row = within(table).getByText('AW-2025-000122').closest('tr') as HTMLElement
    const link = within(row).getByRole('link', { name: 'Öffnen' })
    expect(link).toHaveAttribute('href', '/shop/orders/AW-2025-000122')
  })

  it('shows the empty state instead of a blank table when nothing matches', async () => {
    installFetch()
    renderShopOrders('/shop/orders?status=done')

    expect(
      await screen.findByText('Noch keine Aufträge in diesem Status.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('shows a readable error instead of throwing when the request fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.reject(new TypeError('fetch failed'))),
    )
    renderShopOrders()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Verbindung zum Server')
  })
})

describe('ShopOrders status filter', () => {
  it('passes the status as a query parameter and only shows that status', async () => {
    const fetchMock = installFetch()
    const user = userEvent.setup()
    renderShopOrders()

    await screen.findByRole('table')
    await user.click(screen.getByRole('button', { name: 'bestätigt' }))

    await waitFor(() => {
      expect(lastRequestUrl(fetchMock).searchParams.get('status')).toBe('confirmed')
    })

    const table = screen.getByRole('table')
    expect(within(table).getByText('AW-2025-000122')).toBeInTheDocument()
    expect(within(table).queryByText('AW-2025-000123')).not.toBeInTheDocument()
    expect(within(table).queryByText('AW-2025-000121')).not.toBeInTheDocument()
  })

  it('marks the active segment and returns to all with "Alle"', async () => {
    const fetchMock = installFetch()
    const user = userEvent.setup()
    renderShopOrders()

    await screen.findByRole('table')
    await user.click(screen.getByRole('button', { name: 'fertig' }))
    expect(screen.getByRole('button', { name: 'fertig' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )

    await user.click(screen.getByRole('button', { name: 'Alle' }))
    await waitFor(() => {
      expect(lastRequestUrl(fetchMock).searchParams.get('status')).toBeNull()
    })
    expect(screen.getByRole('button', { name: 'Alle' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
  })
})

describe('ShopOrders plate search', () => {
  it('trims and uppercases the input and passes it as a query parameter', async () => {
    const fetchMock = installFetch()
    const user = userEvent.setup()
    renderShopOrders()

    await screen.findByRole('table')
    await user.type(screen.getByLabelText('Nach Kennzeichen suchen'), '  m-ab  ')

    await waitFor(() => {
      expect(lastRequestUrl(fetchMock).searchParams.get('plate')).toBe('M-AB')
    })

    const table = screen.getByRole('table')
    expect(within(table).getByText('M-AB 1234')).toBeInTheDocument()
    expect(within(table).queryByText('M-CD 9911')).not.toBeInTheDocument()
  })

  it('restores the filter state from the URL on reload', async () => {
    const fetchMock = installFetch()
    renderShopOrders('/shop/orders?status=confirmed&plate=M-CD')

    await screen.findAllByText('AW-2025-000122')

    const url = lastRequestUrl(fetchMock)
    expect(url.searchParams.get('status')).toBe('confirmed')
    expect(url.searchParams.get('plate')).toBe('M-CD')
    expect(screen.getByLabelText('Nach Kennzeichen suchen')).toHaveValue('M-CD')
    expect(screen.getByRole('button', { name: 'bestätigt' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(screen.queryByRole('table')?.textContent).not.toContain('AW-2025-000123')
  })

  it('clears the search with the reset button', async () => {
    const fetchMock = installFetch()
    const user = userEvent.setup()
    renderShopOrders('/shop/orders?plate=M-AB')

    await screen.findAllByText('AW-2025-000123')
    await user.click(screen.getByRole('button', { name: 'Suche zurücksetzen' }))

    await waitFor(() => {
      expect(lastRequestUrl(fetchMock).searchParams.get('plate')).toBeNull()
    })
  })
})
