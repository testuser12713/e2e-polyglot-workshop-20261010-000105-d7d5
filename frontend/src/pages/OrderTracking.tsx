import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, request } from '../lib/api'

/**
 * Customer order tracking ("Status abfragen", AC-11).
 *
 * A customer enters the order number and the plate; both must belong to the
 * same order. A mismatch (unknown order OR wrong plate) is answered by the API
 * with 404 and shown as one single alert — never a partial result and never a
 * hint which of the two values was wrong.
 */

const MISMATCH_MESSAGE = 'Zu diesen Angaben wurde kein Auftrag gefunden.'

const EMPTY_FIELDS_MESSAGE =
  'Bitte geben Sie Auftragsnummer und Kennzeichen ein.'

interface Vehicle {
  id: number
  plate: string
  make: string
  model: string
  mileage: number
}

interface Customer {
  id: number
  name: string
  email: string
  phone: string
}

interface OrderItem {
  id: number
  kind: 'labor' | 'part'
  description: string
  quantity: number
  hours: number
  unit_price_cents: number
  amount_cents: number
}

interface StatusEntry {
  at: string
  from_status: string | null
  to_status: string
}

interface OrderDetail {
  order_number: string
  status: string
  desired_date: string
  vehicle_plate: string
  make: string
  model: string
  customer_name: string
  problem: string
  customer: Customer
  vehicle: Vehicle
  items: OrderItem[]
  history: StatusEntry[]
}

interface TrackingResponse {
  order: OrderDetail
  invoice: unknown | null
}

const STATUS_LABELS: Record<string, string> = {
  requested: 'angefragt',
  confirmed: 'bestätigt',
  in_progress: 'in Arbeit',
  done: 'fertig',
  picked_up: 'abgeholt',
}

const STATUS_COLORS: Record<string, { fg: string; bg: string }> = {
  requested: {
    fg: 'var(--color-status_angefragt)',
    bg: 'var(--color-status_angefragt_bg)',
  },
  confirmed: {
    fg: 'var(--color-status_bestaetigt)',
    bg: 'var(--color-status_bestaetigt_bg)',
  },
  in_progress: {
    fg: 'var(--color-status_in_arbeit)',
    bg: 'var(--color-status_in_arbeit_bg)',
  },
  done: {
    fg: 'var(--color-status_fertig)',
    bg: 'var(--color-status_fertig_bg)',
  },
  picked_up: {
    fg: 'var(--color-status_abgeholt)',
    bg: 'var(--color-status_abgeholt_bg)',
  },
}

function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
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

function formatTimestamp(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getUTCDate())}.${pad(date.getUTCMonth() + 1)}.${date.getUTCFullYear()}, ${pad(
    date.getUTCHours(),
  )}:${pad(date.getUTCMinutes())} (UTC)`
}

interface StatusBadgeProps {
  status: string
  large?: boolean
}

function StatusBadge({ status, large = false }: StatusBadgeProps) {
  const colors = STATUS_COLORS[status] ?? {
    fg: 'var(--color-fg)',
    bg: 'var(--color-surface_sunken)',
  }
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 'var(--space-0)',
        padding: large
          ? 'var(--space-1) var(--space-3)'
          : 'var(--space-0) var(--space-2)',
        borderRadius: 'var(--radius-pill)',
        fontSize: large ? 'var(--size-lg)' : 'var(--size-xs)',
        fontWeight: 600,
        textTransform: large ? 'none' : 'uppercase',
        letterSpacing: large ? 0 : '0.02em',
        border: `1px solid ${colors.fg}`,
        color: colors.fg,
        background: colors.bg,
        whiteSpace: 'nowrap',
      }}
    >
      <span
        aria-hidden="true"
        style={{
          width: large ? 8 : 6,
          height: large ? 8 : 6,
          borderRadius: '50%',
          background: 'currentColor',
        }}
      />
      {statusLabel(status)}
    </span>
  )
}

export default function OrderTracking() {
  const { orderNumber: routeOrderNumber } = useParams<{
    orderNumber?: string
  }>()
  const [orderNumber, setOrderNumber] = useState(routeOrderNumber ?? '')
  const [plate, setPlate] = useState('')
  const [result, setResult] = useState<TrackingResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const cleanOrderNumber = orderNumber.trim().toUpperCase()
    const cleanPlate = plate.trim().toUpperCase().replace(/\s+/g, ' ')

    if (!cleanOrderNumber || !cleanPlate) {
      setResult(null)
      setError(EMPTY_FIELDS_MESSAGE)
      return
    }

    setLoading(true)
    setError(null)
    setResult(null)
    try {
      const data = await request<TrackingResponse>(
        'GET',
        `/api/public/orders/${encodeURIComponent(cleanOrderNumber)}?plate=${encodeURIComponent(cleanPlate)}`,
      )
      setResult(data)
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setError(MISMATCH_MESSAGE)
      } else if (err instanceof ApiError) {
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

  const order = result?.order ?? null
  const history = order
    ? [...order.history].sort(
        (a, b) =>
          new Date(b.at).getTime() - new Date(a.at).getTime(),
      )
    : []

  return (
    <section className="page">
      <div style={{ maxWidth: 560, margin: '0 auto' }}>
        <header className="page-header">
          <h1>Status abfragen</h1>
          <p className="page-header__subtitle">
            Mit Auftragsnummer und Kennzeichen den Fortschritt Ihres Auftrags
            einsehen.
          </p>
        </header>

        <div className="card">
          <p
            style={{
              fontSize: 'var(--size-sm)',
              color: 'var(--color-muted)',
              marginBottom: 'var(--space-3)',
            }}
          >
            Geben Sie Auftragsnummer und Kennzeichen ein. Beide Angaben müssen
            zum selben Auftrag gehören.
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
                <label htmlFor="track-order-number">Auftragsnummer</label>
                <input
                  id="track-order-number"
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
                <label htmlFor="track-plate">Kennzeichen</label>
                <input
                  id="track-plate"
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
              <button
                type="submit"
                className="btn"
                disabled={loading}
                style={{ width: '100%' }}
              >
                {loading ? 'Wird gesendet …' : 'Status abfragen'}
              </button>
            </div>
          </form>
        </div>

        {order ? (
          <div style={{ marginTop: 'var(--space-5)' }}>
            <div className="card">
              <div
                style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 'var(--space-2)',
                  marginBottom: 'var(--space-3)',
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
                    Aktueller Status
                  </div>
                  <StatusBadge status={order.status} large />
                </div>
                <div style={{ textAlign: 'right' }}>
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
                    {order.order_number}
                  </div>
                </div>
              </div>

              <dl
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'auto 1fr',
                  gap: 'var(--space-1) var(--space-4)',
                  fontSize: 'var(--size-sm)',
                  margin: 0,
                  marginBottom: 'var(--space-4)',
                }}
              >
                <dt style={{ color: 'var(--color-muted)' }}>Fahrzeug</dt>
                <dd style={{ margin: 0, textAlign: 'right' }}>
                  {order.make} {order.model} ·{' '}
                  <span className="mono">{order.vehicle_plate}</span>
                </dd>
                <dt style={{ color: 'var(--color-muted)' }}>Wunschtermin</dt>
                <dd style={{ margin: 0, textAlign: 'right' }}>
                  {formatDate(order.desired_date)}
                </dd>
              </dl>

              <div
                style={{
                  height: 1,
                  background: 'var(--color-border)',
                  border: 0,
                  margin: '0 0 var(--space-3)',
                }}
              />

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
                Statusverlauf
              </div>
              {history.length === 0 ? (
                <p
                  style={{
                    fontSize: 'var(--size-sm)',
                    color: 'var(--color-muted)',
                  }}
                >
                  Für diesen Auftrag liegt noch kein Statusverlauf vor.
                </p>
              ) : (
                <ol
                  aria-live="polite"
                  style={{
                    listStyle: 'none',
                    margin: 0,
                    padding: 0,
                  }}
                >
                  {history.map((entry, index) => (
                    <li
                      key={`${entry.at}-${entry.to_status}-${index}`}
                      style={{
                        display: 'flex',
                        flexWrap: 'wrap',
                        alignItems: 'center',
                        gap: 'var(--space-2)',
                        paddingBottom:
                          index === history.length - 1
                            ? 0
                            : 'var(--space-3)',
                      }}
                    >
                      <span
                        className="mono"
                        style={{
                          fontSize: 'var(--size-sm)',
                          color:
                            index === 0
                              ? 'var(--color-fg)'
                              : 'var(--color-muted)',
                        }}
                      >
                        {formatTimestamp(entry.at)}
                      </span>
                      <StatusBadge status={entry.to_status} />
                    </li>
                  ))}
                </ol>
              )}
            </div>

            <div
              className="alert alert--info"
              style={{ marginTop: 'var(--space-3)' }}
            >
              <span aria-hidden="true">i</span>
              <span>
                Liegt zu diesem Auftrag eine Rechnung vor, finden Sie sie unter{' '}
                <Link to="/invoice">Rechnung einsehen</Link>.
              </span>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  )
}
