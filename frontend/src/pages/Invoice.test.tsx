import { afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import Invoice from './Invoice'

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

function renderPage(initialEntries: string[] = ['/invoice']) {
  return render(
    <MemoryRouter
      initialEntries={initialEntries}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <Invoice />
    </MemoryRouter>,
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

const invoiceResponse = {
  invoice_number: 'RE-2025-000123',
  order_number: 'AW-2025-000123',
  items: [
    {
      description: 'Arbeitszeit — Inspektion und Bremsenwechsel',
      amount_cents: 22250,
    },
    { description: 'Bremsbelagsatz vorne', amount_cents: 6840 },
    { description: 'Bremsscheibe vorne', amount_cents: 10980 },
  ],
  net_cents: 40070,
  tax_cents: 7613,
  gross_cents: 47683,
  created_at: '2025-03-07T10:00:00Z',
}

describe('Invoice', () => {
  it('renders the invoice header, positions and summary amounts', async () => {
    stubFetch(200, invoiceResponse)
    const user = userEvent.setup()
    renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'aw-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'm-ab 1234')
    await user.click(screen.getByRole('button', { name: 'Rechnung abrufen' }))

    expect(await screen.findByText('RE-2025-000123')).toBeInTheDocument()
    expect(screen.getByText('AW-2025-000123')).toBeInTheDocument()
    expect(screen.getByText('07.03.2025')).toBeInTheDocument()

    expect(
      screen.getByText('Arbeitszeit — Inspektion und Bremsenwechsel'),
    ).toBeInTheDocument()
    expect(screen.getByText('Bremsbelagsatz vorne')).toBeInTheDocument()

    expect(screen.getByText('Netto')).toBeInTheDocument()
    expect(screen.getByText('MwSt. 19 %')).toBeInTheDocument()
    expect(screen.getByText('Brutto')).toBeInTheDocument()
    expect(screen.getByText(/400,70/)).toBeInTheDocument()
    expect(screen.getByText(/76,13/)).toBeInTheDocument()
    expect(screen.getByText(/476,83/)).toBeInTheDocument()

    expect(
      screen.getByText(/Es wird keine E-Mail versendet/),
    ).toBeInTheDocument()
    expect(
      screen.getByText(/Alle Beträge in Euro, gerundet auf Cent/),
    ).toBeInTheDocument()
  })

  it('calls the invoice endpoint with the entered order number and plate', async () => {
    const fetchMock = stubFetch(200, invoiceResponse)
    const user = userEvent.setup()
    renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'aw-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'm-ab 1234')
    await user.click(screen.getByRole('button', { name: 'Rechnung abrufen' }))

    await screen.findByText('RE-2025-000123')
    const [url] = fetchMock.mock.calls[0] as unknown as [string]
    expect(url).toContain('/api/public/orders/AW-2025-000123/invoice')
    expect(url).toContain('plate=M-AB%201234')
  })

  it('shows the German error message and no invoice data for a wrong plate', async () => {
    stubFetch(404, {
      error: {
        code: 'not_found',
        message: 'Zu diesen Angaben wurde keine Rechnung gefunden.',
      },
    })
    const user = userEvent.setup()
    renderPage()

    await user.type(
      screen.getByLabelText('Auftragsnummer'),
      'AW-2025-000123',
    )
    await user.type(screen.getByLabelText('Kennzeichen'), 'B-XY 9999')
    await user.click(screen.getByRole('button', { name: 'Rechnung abrufen' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(
      'Zu diesen Angaben wurde keine Rechnung gefunden.',
    )
    expect(screen.queryByText('RE-2025-000123')).not.toBeInTheDocument()
    expect(screen.queryByText('Netto')).not.toBeInTheDocument()
  })
})
