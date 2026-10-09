import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import ShopOrderDetail, {
  type OrderDetail,
  type OrderItem,
  type OrderStatus,
  formatMoney,
  formatTimestamp,
  parseDecimal,
  parseEurosToCents,
} from './ShopOrderDetail'
import { setAuthToken } from '../lib/api'

const ORDER_NUMBER = 'AW-2025-000123'

function makeOrder(status: OrderStatus): OrderDetail {
  const history = [
    { at: '2025-03-05T17:48:00Z', from_status: null, to_status: 'requested' as OrderStatus },
    { at: '2025-03-06T09:15:00Z', from_status: 'requested' as OrderStatus, to_status: 'confirmed' as OrderStatus },
  ]
  if (status !== 'requested' && status !== 'confirmed') {
    history.push({ at: '2025-03-07T14:32:00Z', from_status: 'confirmed' as OrderStatus, to_status: 'in_progress' as OrderStatus })
  }
  return {
    order_number: ORDER_NUMBER,
    status,
    desired_date: '2025-03-07',
    vehicle_plate: 'M-AB 1234',
    make: 'BMW',
    model: '320d',
    customer_name: 'Thomas Berger',
    problem: 'Quietschende Geräusche beim Bremsen.',
    customer: {
      id: 1,
      name: 'Thomas Berger',
      email: 'thomas@example.de',
      phone: '0170 000000',
    },
    vehicle: {
      id: 1,
      plate: 'M-AB 1234',
      make: 'BMW',
      model: '320d',
      mileage: 87450,
    },
    items: [
      {
        id: 1,
        kind: 'labor',
        description: 'Arbeitszeit — Inspektion',
        quantity: 0,
        hours: 2.5,
        unit_price_cents: 8900,
        amount_cents: 22250,
      },
    ],
    history,
  }
}

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => JSON.stringify(body),
  } as unknown as Response
}

interface MockResult {
  fn: ReturnType<typeof vi.fn>
  order: OrderDetail
}

/** Fetch stub that behaves like the real API for one order. */
function installFetch(status: OrderStatus = 'requested'): MockResult {
  let order = makeOrder(status)
  const fn = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const raw = typeof input === 'string' ? input : input.toString()
    const url = new URL(raw, 'http://localhost:8000')
    const method = init?.method ?? 'GET'

    if (method === 'GET' && /^\/api\/shop\/orders\/[^/]+$/.test(url.pathname)) {
      return Promise.resolve(jsonResponse(200, order))
    }

    if (method === 'POST' && url.pathname.endsWith('/status')) {
      const body = JSON.parse(String(init?.body)) as { status: OrderStatus }
      const from = order.status
      order = {
        ...order,
        status: body.status,
        history: [
          ...order.history,
          { at: '2025-03-08T10:00:00Z', from_status: from, to_status: body.status },
        ],
      }
      return Promise.resolve(
        jsonResponse(200, { order_number: order.order_number, status: body.status }),
      )
    }

    if (method === 'POST' && url.pathname.endsWith('/items')) {
      const body = JSON.parse(String(init?.body)) as {
        kind: 'labor' | 'part'
        description: string
        quantity: number
        hours: number
        unit_price_cents: number
      }
      const amount =
        body.kind === 'labor'
          ? Math.round(body.hours * body.unit_price_cents)
          : Math.round(body.quantity * body.unit_price_cents)
      const item: OrderItem = { id: 99, ...body, amount_cents: amount }
      order = { ...order, items: [...order.items, item] }
      return Promise.resolve(jsonResponse(201, item))
    }

    return Promise.resolve(
      jsonResponse(404, { error: { code: 'not_found', message: 'Nicht gefunden.' } }),
    )
  })
  vi.stubGlobal('fetch', fn)
  return { fn, order }
}

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/shop/orders/${ORDER_NUMBER}`]}>
      <Routes>
        <Route path="/shop/orders/:orderNumber" element={<ShopOrderDetail />} />
      </Routes>
    </MemoryRouter>,
  )
}

function statusGroup(): HTMLElement {
  return screen.getByRole('group', { name: 'Status setzen' })
}

/** Testing Library normalises a no-break space to a plain space in text. */
function money(cents: number): string {
  return formatMoney(cents).replace(/\u00A0/g, ' ')
}

beforeEach(() => {
  setAuthToken(null)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ShopOrderDetail transitions', () => {
  it('offers exactly the allowed follow-up transition for a requested order', async () => {
    installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const group = statusGroup()

    expect(within(group).getByRole('button', { name: 'bestätigt' })).toBeEnabled()
    for (const label of ['angefragt', 'in Arbeit', 'fertig', 'abgeholt']) {
      const button = within(group).getByRole('button', { name: label })
      expect(button).toBeDisabled()
      expect(button).toHaveAttribute('aria-disabled', 'true')
    }
  })

  it('offers only "fertig" as the follow-up of an in-progress order', async () => {
    installFetch('in_progress')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const group = statusGroup()

    expect(within(group).getByRole('button', { name: 'fertig' })).toBeEnabled()
    for (const label of ['angefragt', 'bestätigt', 'in Arbeit', 'abgeholt']) {
      expect(within(group).getByRole('button', { name: label })).toBeDisabled()
    }
  })

  it('does not send a request when a disabled transition is clicked', async () => {
    const { fn } = installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()
    await user.click(within(statusGroup()).getByRole('button', { name: 'abgeholt' }))

    const statusCalls = fn.mock.calls.filter(
      ([input, init]) =>
        String(input).endsWith('/status') && (init as RequestInit | undefined)?.method === 'POST',
    )
    expect(statusCalls).toHaveLength(0)
  })

  it('confirms a requested order and shows it as bestätigt right away', async () => {
    const { fn } = installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()
    await user.click(within(statusGroup()).getByRole('button', { name: 'bestätigt' }))

    await screen.findByText('bestätigt', { selector: '.badge--lg' })

    const statusCall = fn.mock.calls.find(
      ([input, init]) =>
        String(input).endsWith('/status') && (init as RequestInit | undefined)?.method === 'POST',
    )
    expect(statusCall).toBeDefined()
    expect(JSON.parse(String((statusCall?.[1] as RequestInit).body))).toEqual({
      status: 'confirmed',
    })
  })

  it('asks for confirmation before an irreversible step and applies it afterwards', async () => {
    installFetch('in_progress')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()
    await user.click(within(statusGroup()).getByRole('button', { name: 'fertig' }))

    const dialog = await screen.findByRole('dialog', { name: 'Statuswechsel bestätigen' })
    expect(within(dialog).getByText(/auf „fertig“ setzen/)).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Bestätigen' }))

    await screen.findByText('fertig', { selector: '.badge--lg' })
  })
})

describe('ShopOrderDetail position capture', () => {
  it('saves a labour position and shows it in the order immediately', async () => {
    const { fn } = installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()

    await user.type(screen.getByLabelText(/Beschreibung/), 'Arbeitszeit Bremsenwechsel')
    await user.type(screen.getByLabelText('Menge/Std.'), '2,5')
    await user.type(screen.getByLabelText('Einzelpreis'), '89,00')
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }))

    expect(await screen.findByText('Arbeitszeit Bremsenwechsel')).toBeInTheDocument()
    const row = screen.getByText('Arbeitszeit Bremsenwechsel').closest('tr') as HTMLElement
    expect(within(row).getByText(money(22250))).toBeInTheDocument()

    const itemCall = fn.mock.calls.find(
      ([input, init]) =>
        String(input).endsWith('/items') && (init as RequestInit | undefined)?.method === 'POST',
    )
    expect(itemCall).toBeDefined()
    expect(JSON.parse(String((itemCall?.[1] as RequestInit).body))).toEqual({
      kind: 'labor',
      description: 'Arbeitszeit Bremsenwechsel',
      quantity: 0,
      hours: 2.5,
      unit_price_cents: 8900,
    })
  })

  it('saves a parts position with its quantity', async () => {
    const { fn } = installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()

    await user.selectOptions(screen.getByLabelText('Art'), 'part')
    await user.type(screen.getByLabelText(/Beschreibung/), 'Bremsbelagsatz vorne')
    await user.type(screen.getByLabelText('Menge/Std.'), '2')
    await user.type(screen.getByLabelText('Einzelpreis'), '54,90')
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }))

    expect(await screen.findByText('Bremsbelagsatz vorne')).toBeInTheDocument()
    const row = screen.getByText('Bremsbelagsatz vorne').closest('tr') as HTMLElement
    expect(within(row).getByText(money(10980))).toBeInTheDocument()

    const itemCall = fn.mock.calls.find(
      ([input, init]) =>
        String(input).endsWith('/items') && (init as RequestInit | undefined)?.method === 'POST',
    )
    expect(JSON.parse(String((itemCall?.[1] as RequestInit).body))).toEqual({
      kind: 'part',
      description: 'Bremsbelagsatz vorne',
      quantity: 2,
      hours: 0,
      unit_price_cents: 5490,
    })
  })

  it('renders the loaded positions and the status history', async () => {
    installFetch('in_progress')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    expect(screen.getByText('Arbeitszeit — Inspektion')).toBeInTheDocument()
    const row = screen.getByText('Arbeitszeit — Inspektion').closest('tr') as HTMLElement
    expect(within(row).getByText(money(22250))).toBeInTheDocument()
    expect(
      screen.getByText(formatTimestamp('2025-03-07T14:32:00Z')),
    ).toBeInTheDocument()
  })

  it('is neutral until the description is touched', async () => {
    installFetch('requested')
    renderDetail()

    await screen.findByRole('heading', { level: 1, name: ORDER_NUMBER })
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }))

    expect(await screen.findByText('Bitte eine Beschreibung angeben.')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByLabelText(/Beschreibung/)).toHaveAttribute('aria-invalid', 'true'),
    )
  })
})

describe('ShopOrderDetail formatting helpers', () => {
  it('formats integer cents the German way', () => {
    expect(formatMoney(0)).toBe('0,00\u00A0€')
    expect(formatMoney(22250)).toBe('222,50\u00A0€')
    expect(formatMoney(123456)).toBe('1.234,56\u00A0€')
  })

  it('parses German decimal inputs', () => {
    expect(parseDecimal('2,5')).toBe(2.5)
    expect(parseDecimal('')).toBe(0)
    expect(parseEurosToCents('89,00')).toBe(8900)
    expect(parseEurosToCents('54,90')).toBe(5490)
  })
})
