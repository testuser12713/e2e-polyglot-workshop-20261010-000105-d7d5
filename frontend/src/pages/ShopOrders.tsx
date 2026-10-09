/**
 * Auftragsliste des Werkstattbereichs (mockup `design/mockups/auftraege.html`).
 *
 * Shows the workshop orders from `GET /api/shop/orders` with a status filter
 * (the five statuses plus 'Alle') and a licence-plate search. Both filter
 * values travel as query parameters to the API (`?status=…&plate=…`) and are
 * mirrored in the browser URL so a reload keeps the current view. An empty
 * result renders an EmptyState instead of a blank table.
 */
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ApiError, request } from '../lib/api'
import './ShopOrders.css'

export type OrderStatus =
  | 'requested'
  | 'confirmed'
  | 'in_progress'
  | 'done'
  | 'picked_up'

export interface OrderSummary {
  order_number: string
  status: OrderStatus
  desired_date: string
  vehicle_plate: string
  make: string
  model: string
  customer_name: string
}

/** The five statuses in workflow order, used for the segmented filter. */
export const ORDER_STATUSES: OrderStatus[] = [
  'requested',
  'confirmed',
  'in_progress',
  'done',
  'picked_up',
]

const STATUS_LABELS: Record<OrderStatus, string> = {
  requested: 'angefragt',
  confirmed: 'bestätigt',
  in_progress: 'in Arbeit',
  done: 'fertig',
  picked_up: 'abgeholt',
}

/** Maps the API enum to the StatusBadge colour modifier from DESIGN.md. */
const STATUS_CSS: Record<OrderStatus, string> = {
  requested: 'angefragt',
  confirmed: 'bestaetigt',
  in_progress: 'in-arbeit',
  done: 'fertig',
  picked_up: 'abgeholt',
}

export function formatDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  if (!match) {
    return value
  }
  return `${match[3]}.${match[2]}.${match[1]}`
}

function isOrderStatus(value: string): value is OrderStatus {
  return (ORDER_STATUSES as string[]).includes(value)
}

/** StatusBadge — pill with a dot and the fixed German label (DESIGN.md). */
export function StatusBadge({ status }: { status: OrderStatus }) {
  const modifier = STATUS_CSS[status] ?? 'angefragt'
  const label = STATUS_LABELS[status] ?? status
  return <span className={`badge badge--${modifier}`}>{label}</span>
}

function vehicleLabel(order: OrderSummary): string {
  return `${order.make} ${order.model}`.trim()
}

export default function ShopOrders() {
  const [searchParams, setSearchParams] = useSearchParams()

  const statusParam = searchParams.get('status') ?? ''
  const status: OrderStatus | '' = isOrderStatus(statusParam) ? statusParam : ''
  const plate = searchParams.get('plate') ?? ''

  const [plateInput, setPlateInput] = useState(plate)
  const [orders, setOrders] = useState<OrderSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<{ message: string; unauthorized: boolean } | null>(
    null,
  )

  // Licence search: trim + uppercase after a 300ms debounce, then reflect the
  // value in the URL query so a reload keeps the search.
  useEffect(() => {
    const timer = window.setTimeout(() => {
      const normalized = plateInput.trim().toUpperCase()
      if (normalized === plate) {
        return
      }
      const params = new URLSearchParams(searchParams)
      if (normalized) {
        params.set('plate', normalized)
      } else {
        params.delete('plate')
      }
      setSearchParams(params, { replace: true })
    }, 300)
    return () => window.clearTimeout(timer)
  }, [plateInput, plate, searchParams, setSearchParams])

  // Load the (server-side filtered) order list whenever a filter changes.
  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    const params = new URLSearchParams()
    if (status) {
      params.set('status', status)
    }
    if (plate) {
      params.set('plate', plate)
    }
    const query = params.toString()

    request<{ orders: OrderSummary[] }>(
      'GET',
      `/api/shop/orders${query ? `?${query}` : ''}`,
    )
      .then((data) => {
        if (cancelled) {
          return
        }
        setOrders(Array.isArray(data.orders) ? data.orders : [])
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof ApiError
            ? err.message
            : 'Die Auftragsliste konnte nicht geladen werden.'
        setError({ message, unauthorized: err instanceof ApiError && err.status === 401 })
        setOrders([])
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [status, plate])

  const selectStatus = (next: string) => {
    const params = new URLSearchParams(searchParams)
    if (next) {
      params.set('status', next)
    } else {
      params.delete('status')
    }
    setSearchParams(params, { replace: true })
  }

  const resetFilters = () => {
    setPlateInput('')
    const params = new URLSearchParams(searchParams)
    params.delete('status')
    params.delete('plate')
    setSearchParams(params, { replace: true })
  }

  return (
    <section className="page">
      <header className="page-header">
        <h1>Aufträge</h1>
        <p className="page-header__subtitle">
          Alle Aufträge mit Status und Fahrzeugkennzeichen. Filtern Sie nach
          Status oder suchen Sie nach Kennzeichen.
        </p>
      </header>

      <div className="filterbar" role="group" aria-label="Aufträge filtern">
        <div className="segmented" role="group" aria-label="Nach Status filtern">
          <button
            type="button"
            className="segmented__btn"
            aria-pressed={status === ''}
            onClick={() => selectStatus('')}
          >
            Alle
          </button>
          {ORDER_STATUSES.map((value) => (
            <button
              key={value}
              type="button"
              className="segmented__btn"
              aria-pressed={status === value}
              onClick={() => selectStatus(value)}
            >
              {STATUS_LABELS[value]}
            </button>
          ))}
        </div>

        <div className={`search${plateInput ? ' is-filled' : ''}`}>
          <svg
            className="search__icon"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            aria-hidden="true"
          >
            <circle cx="11" cy="11" r="7" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <label className="visually-hidden" htmlFor="plate-search">
            Nach Kennzeichen suchen
          </label>
          <input
            className="input"
            id="plate-search"
            type="search"
            placeholder="Kennzeichen suchen"
            autoComplete="off"
            value={plateInput}
            onChange={(event) => setPlateInput(event.target.value)}
          />
          {plateInput ? (
            <button
              type="button"
              className="search__clear"
              aria-label="Suche zurücksetzen"
              onClick={() => setPlateInput('')}
            >
              ×
            </button>
          ) : null}
        </div>
      </div>

      {loading ? (
        <p className="orders-status" aria-live="polite">
          Wird geladen …
        </p>
      ) : error ? (
        <div className="alert alert--error" role="alert">
          <div>
            <p>{error.message}</p>
            {error.unauthorized ? (
              <p>
                <Link to="/shop/login">Zur Anmeldung</Link>
              </p>
            ) : null}
          </div>
        </div>
      ) : orders.length === 0 ? (
        <div className="table-wrap">
          <div className="empty">
            <svg
              className="empty__glyph"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
              aria-hidden="true"
            >
              <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2" />
              <rect x="9" y="3" width="6" height="4" rx="1" />
            </svg>
            <div className="empty__title">Noch keine Aufträge in diesem Status.</div>
            <p className="empty__hint">
              Passen Sie den Statusfilter an oder setzen Sie die Kennzeichensuche
              zurück.
            </p>
            <div className="empty__action">
              <button type="button" className="btn btn--secondary" onClick={resetFilters}>
                Filter zurücksetzen
              </button>
            </div>
          </div>
        </div>
      ) : (
        <div className="table-wrap">
          <div className="orders-cards">
            {orders.map((order) => (
              <div className="order-card" key={order.order_number}>
                <div className="order-card__top">
                  <span className="order-card__no">{order.order_number}</span>
                  <StatusBadge status={order.status} />
                </div>
                <div className="order-card__row">
                  <span className="order-card__label">Kennzeichen</span>
                  <span className="order-card__value mono">{order.vehicle_plate}</span>
                </div>
                <div className="order-card__row">
                  <span className="order-card__label">Fahrzeug</span>
                  <span className="order-card__value">{vehicleLabel(order)}</span>
                </div>
                <div className="order-card__row">
                  <span className="order-card__label">Wunschtermin</span>
                  <span className="order-card__value numeric">
                    {formatDate(order.desired_date)}
                  </span>
                </div>
                <div className="order-card__action">
                  <Link
                    className="btn btn--primary"
                    to={`/shop/orders/${order.order_number}`}
                  >
                    Öffnen
                  </Link>
                </div>
              </div>
            ))}
          </div>

          <table className="data-table">
            <thead>
              <tr>
                <th scope="col">Auftragsnummer</th>
                <th scope="col">Kennzeichen</th>
                <th scope="col">Fahrzeug</th>
                <th scope="col">Wunschtermin</th>
                <th scope="col">Status</th>
                <th scope="col" className="col-action">
                  Aktion
                </th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.order_number}>
                  <td className="cell-no">{order.order_number}</td>
                  <td className="cell-plate">{order.vehicle_plate}</td>
                  <td>{vehicleLabel(order)}</td>
                  <td className="numeric">{formatDate(order.desired_date)}</td>
                  <td>
                    <StatusBadge status={order.status} />
                  </td>
                  <td className="col-action">
                    <Link
                      className="btn btn--secondary btn--sm"
                      to={`/shop/orders/${order.order_number}`}
                    >
                      Öffnen
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
