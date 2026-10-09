import { useState, type FormEvent } from 'react'
import { useParams } from 'react-router-dom'
import { ApiError, request } from '../lib/api'

/**
 * Customer invoice view ("Rechnung", AC-12).
 *
 * The customer enters the order number and the plate; the invoice is fetched
 * from `/api/public/orders/{order_number}/invoice?plate=`. A wrong plate or a
 * missing invoice is answered with 404 and shown as the German message from the
 * standard error body — no invoice data is rendered in that case.
 */

const EMPTY_FIELDS_MESSAGE =
  'Bitte geben Sie Auftragsnummer und Kennzeichen ein.'

interface InvoiceItem {
  description: string
  amount_cents: number
}

interface Invoice {
  invoice_number: string
  order_number: string
  items: InvoiceItem[]
  net_cents: number
  tax_cents: number
  gross_cents: number
  created_at: string
}

function formatMoney(cents: number): string {
  const sign = cents < 0 ? '-' : ''
  const absolute = Math.abs(Math.round(cents))
  const euros = Math.floor(absolute / 100)
  const rest = absolute % 100
  const grouped = String(euros).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${sign}${grouped},${String(rest).padStart(2, '0')}\u00A0€`
}

function formatDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  if (match) {
    return `${match[3]}.${match[2]}.${match[1]}`
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getUTCDate())}.${pad(date.getUTCMonth() + 1)}.${date.getUTCFullYear()}`
}

const CELL_STYLE = {
  padding: 'var(--space-2)',
  borderBottom: '1px solid var(--color-border)',
  fontSize: 'var(--size-sm)',
} as const

const NUM_CELL_STYLE = {
  ...CELL_STYLE,
  textAlign: 'right',
  fontFamily: 'var(--font-mono)',
  fontVariantNumeric: 'tabular-nums',
  whiteSpace: 'nowrap',
} as const

export default function Invoice() {
  const { orderNumber: routeOrderNumber } = useParams<{
    orderNumber?: string
  }>()
  const [orderNumber, setOrderNumber] = useState(routeOrderNumber ?? '')
  const [plate, setPlate] = useState('')
  const [invoice, setInvoice] = useState<Invoice | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const cleanOrderNumber = orderNumber.trim().toUpperCase()
    const cleanPlate = plate.trim().toUpperCase().replace(/\s+/g, ' ')

    if (!cleanOrderNumber || !cleanPlate) {
      setInvoice(null)
      setError(EMPTY_FIELDS_MESSAGE)
      return
    }

    setLoading(true)
    setError(null)
    setInvoice(null)
    try {
      const data = await request<Invoice>(
        'GET',
        `/api/public/orders/${encodeURIComponent(cleanOrderNumber)}/invoice?plate=${encodeURIComponent(cleanPlate)}`,
      )
      setInvoice(data)
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError(
          'Es ist ein unerwarteter Fehler aufgetreten. Bitte versuchen Sie es erneut.',
        )
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="page">
      <header className="page-header">
        <h1>Rechnung</h1>
        <p className="page-header__subtitle">
          Alle Beträge in Euro, gerundet auf Cent.
        </p>
      </header>

      <div className="card" style={{ marginBottom: 'var(--space-4)' }}>
        <p
          style={{
            fontSize: 'var(--size-sm)',
            color: 'var(--color-muted)',
            marginBottom: 'var(--space-3)',
          }}
        >
          Geben Sie Auftragsnummer und Kennzeichen ein, um die Rechnung zu
          diesem Auftrag einzusehen.
        </p>

        {error ? (
          <div
            className="alert alert--error"
            role="alert"
            style={{ marginBottom: 'var(--space-3)' }}
          >
            <span aria-hidden="true">⚠</span>
            <span>{error}</span>
          </div>
        ) : null}

        <form onSubmit={handleSubmit} noValidate>
          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: 'var(--space-3)',
            }}
          >
            <div className="field" style={{ flex: '1 1 200px' }}>
              <label htmlFor="invoice-order-number">Auftragsnummer</label>
              <input
                id="invoice-order-number"
                className="mono"
                type="text"
                value={orderNumber}
                placeholder="AW-2025-000123"
                autoComplete="off"
                style={{ textTransform: 'uppercase' }}
                onChange={(event) => setOrderNumber(event.target.value)}
              />
            </div>
            <div className="field" style={{ flex: '1 1 200px' }}>
              <label htmlFor="invoice-plate">Kennzeichen</label>
              <input
                id="invoice-plate"
                className="mono"
                type="text"
                value={plate}
                placeholder="M-AB 1234"
                autoComplete="off"
                style={{ textTransform: 'uppercase' }}
                onChange={(event) => setPlate(event.target.value)}
              />
            </div>
          </div>
          <div style={{ marginTop: 'var(--space-3)' }}>
            <button type="submit" className="btn" disabled={loading}>
              {loading ? 'Wird gesendet …' : 'Rechnung abrufen'}
            </button>
          </div>
        </form>
      </div>

      {invoice ? (
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: 'var(--space-4)',
            alignItems: 'flex-start',
          }}
        >
          <div
            style={{
              flex: '3 1 360px',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-4)',
            }}
          >
            <div className="card">
              <div
                style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: 'var(--space-4)',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div
                    style={{
                      fontSize: 'var(--size-xs)',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.02em',
                      color: 'var(--color-muted)',
                      marginBottom: 'var(--space-1)',
                    }}
                  >
                    Rechnungsnummer
                  </div>
                  <div className="mono" style={{ fontWeight: 600 }}>
                    {invoice.invoice_number}
                  </div>
                </div>
                <div>
                  <div
                    style={{
                      fontSize: 'var(--size-xs)',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.02em',
                      color: 'var(--color-muted)',
                      marginBottom: 'var(--space-1)',
                    }}
                  >
                    Auftragsnummer
                  </div>
                  <div className="mono" style={{ fontWeight: 600 }}>
                    {invoice.order_number}
                  </div>
                </div>
                <div>
                  <div
                    style={{
                      fontSize: 'var(--size-xs)',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.02em',
                      color: 'var(--color-muted)',
                      marginBottom: 'var(--space-1)',
                    }}
                  >
                    Rechnungsdatum
                  </div>
                  <div style={{ fontWeight: 600 }}>
                    {formatDate(invoice.created_at)}
                  </div>
                </div>
              </div>
            </div>

            <div className="card">
              <div
                style={{
                  fontSize: 'var(--size-sm)',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.02em',
                  color: 'var(--color-muted)',
                  marginBottom: 'var(--space-3)',
                }}
              >
                Positionen
              </div>
              <table
                style={{
                  width: '100%',
                  borderCollapse: 'collapse',
                }}
              >
                <thead>
                  <tr>
                    <th
                      scope="col"
                      style={{
                        textAlign: 'left',
                        fontSize: 'var(--size-xs)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.02em',
                        color: 'var(--color-muted)',
                        padding: 'var(--space-2)',
                        borderBottom:
                          '1px solid var(--color-border_strong)',
                      }}
                    >
                      Beschreibung
                    </th>
                    <th
                      scope="col"
                      style={{
                        ...NUM_CELL_STYLE,
                        borderBottom:
                          '1px solid var(--color-border_strong)',
                        fontSize: 'var(--size-xs)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.02em',
                        color: 'var(--color-muted)',
                      }}
                    >
                      Menge/Stunden
                    </th>
                    <th
                      scope="col"
                      style={{
                        ...NUM_CELL_STYLE,
                        borderBottom:
                          '1px solid var(--color-border_strong)',
                        fontSize: 'var(--size-xs)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.02em',
                        color: 'var(--color-muted)',
                      }}
                    >
                      Einzelpreis
                    </th>
                    <th
                      scope="col"
                      style={{
                        ...NUM_CELL_STYLE,
                        borderBottom:
                          '1px solid var(--color-border_strong)',
                        fontSize: 'var(--size-xs)',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        letterSpacing: '0.02em',
                        color: 'var(--color-muted)',
                      }}
                    >
                      Betrag
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {invoice.items.length === 0 ? (
                    <tr>
                      <td
                        colSpan={4}
                        style={{
                          ...CELL_STYLE,
                          textAlign: 'center',
                          color: 'var(--color-muted)',
                        }}
                      >
                        Keine Positionen erfasst.
                      </td>
                    </tr>
                  ) : (
                    invoice.items.map((item, index) => (
                      <tr key={`${item.description}-${index}`}>
                        <td style={CELL_STYLE}>{item.description}</td>
                        <td style={NUM_CELL_STYLE}>–</td>
                        <td style={NUM_CELL_STYLE}>–</td>
                        <td style={NUM_CELL_STYLE}>
                          {formatMoney(item.amount_cents)}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <aside
            className="card"
            style={{ flex: '2 1 260px' }}
          >
            <div
              style={{
                fontSize: 'var(--size-sm)',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.02em',
                color: 'var(--color-muted)',
                marginBottom: 'var(--space-3)',
              }}
            >
              Zusammenfassung
            </div>
            <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
              <li
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 'var(--space-3)',
                  paddingBlock: 'var(--space-1)',
                  fontSize: 'var(--size-sm)',
                }}
              >
                <span>Netto</span>
                <span className="mono numeric">
                  {formatMoney(invoice.net_cents)}
                </span>
              </li>
              <li
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 'var(--space-3)',
                  paddingBlock: 'var(--space-1)',
                  fontSize: 'var(--size-sm)',
                }}
              >
                <span>MwSt. 19 %</span>
                <span className="mono numeric">
                  {formatMoney(invoice.tax_cents)}
                </span>
              </li>
              <li
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 'var(--space-3)',
                  marginTop: 'var(--space-2)',
                  paddingTop: 'var(--space-3)',
                  borderTop: '1px solid var(--color-accent)',
                  fontSize: 'var(--size-lg)',
                  fontWeight: 600,
                }}
              >
                <span>Brutto</span>
                <span
                  className="mono numeric"
                  style={{ fontSize: 'var(--size-lg)' }}
                >
                  {formatMoney(invoice.gross_cents)}
                </span>
              </li>
            </ul>

            <div
              className="alert alert--info"
              style={{ marginTop: 'var(--space-4)' }}
            >
              <span aria-hidden="true">i</span>
              <span>
                Es wird keine E-Mail versendet; die Rechnung liegt im
                Postausgang.
              </span>
            </div>
          </aside>
        </div>
      ) : null}
    </section>
  )
}
