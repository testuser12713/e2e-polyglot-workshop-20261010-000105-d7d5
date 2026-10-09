import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { ApiError, request } from '../lib/api'
import './ShopDashboard.css'

/**
 * The workshop dashboard (AC-21).
 *
 * Shows three figures from `GET /api/shop/dashboard` — open orders, orders
 * finished today and the current month's revenue from invoices — plus the
 * newest orders and a count per status, both derived from `GET /api/shop/orders`.
 * Every control navigates to the order list; failures render a readable German
 * alert instead of a technical error page (AC-22).
 */

type OrderStatus =
  | 'requested'
  | 'confirmed'
  | 'in_progress'
  | 'done'
  | 'picked_up'

interface DashboardMetrics {
  open_orders: number
  finished_today: number
  revenue_month_cents: number
}

interface OrderSummary {
  order_number: string
  status: OrderStatus
  desired_date: string
  vehicle_plate: string
  make: string
  model: string
  customer_name: string
}

const STATUS_ORDER: OrderStatus[] = [
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

const STATUS_BADGE_CLASS: Record<OrderStatus, string> = {
  requested: 'dash-badge--angefragt',
  confirmed: 'dash-badge--bestaetigt',
  in_progress: 'dash-badge--in-arbeit',
  done: 'dash-badge--fertig',
  picked_up: 'dash-badge--abgeholt',
}

const GERMAN_MONTHS = [
  'Januar',
  'Februar',
  'März',
  'April',
  'Mai',
  'Juni',
  'Juli',
  'August',
  'September',
  'Oktober',
  'November',
  'Dezember',
]

const FALLBACK_ERROR =
  'Die Kennzahlen konnten nicht geladen werden. Bitte versuchen Sie es erneut.'

/** Format integer cents as the product's money form '1.234,56 €'. */
export function formatCents(cents: number): string {
  const rounded = Math.round(cents)
  const sign = rounded < 0 ? '-' : ''
  const abs = Math.abs(rounded)
  const euros = Math.floor(abs / 100)
  const rest = abs % 100
  const grouped = String(euros).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${sign}${grouped},${String(rest).padStart(2, '0')}\u00A0€`
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.message.trim().length > 0) {
    return error.message
  }
  return fallback
}

function StatusBadge({ status }: { status: OrderStatus }) {
  return (
    <span className={`dash-badge ${STATUS_BADGE_CLASS[status]}`}>
      {STATUS_LABELS[status]}
    </span>
  )
}

function StatCard({
  testId,
  label,
  value,
  context,
  icon,
}: {
  testId: string
  label: string
  value: string
  context: string
  icon: JSX.Element
}) {
  return (
    <div className="dash-stat">
      <span className="dash-stat__icon" aria-hidden="true">
        {icon}
      </span>
      <div>
        <div className="dash-stat__label">{label}</div>
        <div className="dash-stat__value" data-testid={testId}>
          {value}
        </div>
        <div className="dash-stat__context">{context}</div>
      </div>
    </div>
  )
}

function NewestOrders({ orders }: { orders: OrderSummary[] }) {
  if (orders.length === 0) {
    return <p className="dash-empty">Noch keine Aufträge vorhanden.</p>
  }

  return (
    <>
      <ul className="dash-orders-cards">
        {orders.map((order) => (
          <li className="dash-order-card" key={order.order_number}>
            <div className="dash-order-card__top">
              <span className="dash-order-card__no">{order.order_number}</span>
              <StatusBadge status={order.status} />
            </div>
            <div className="dash-order-card__row">
              <span className="dash-order-card__label">Kennzeichen</span>
              <span className="dash-order-card__value mono">
                {order.vehicle_plate}
              </span>
            </div>
            <div className="dash-order-card__row">
              <span className="dash-order-card__label">Fahrzeug</span>
              <span className="dash-order-card__value">
                {order.make} {order.model}
              </span>
            </div>
          </li>
        ))}
      </ul>

      <table className="dash-table">
        <thead>
          <tr>
            <th scope="col">Auftragsnummer</th>
            <th scope="col">Kennzeichen</th>
            <th scope="col">Fahrzeug</th>
            <th scope="col">Status</th>
          </tr>
        </thead>
        <tbody>
          {orders.map((order) => (
            <tr key={order.order_number}>
              <td className="dash-cell-no">{order.order_number}</td>
              <td className="dash-cell-plate">{order.vehicle_plate}</td>
              <td>
                {order.make} {order.model}
              </td>
              <td>
                <StatusBadge status={order.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  )
}

export default function ShopDashboard() {
  const now = new Date()
  const monthLabel = `${GERMAN_MONTHS[now.getMonth()]} ${now.getFullYear()}`
  const todayLabel = `${String(now.getDate()).padStart(2, '0')}.${String(
    now.getMonth() + 1,
  ).padStart(2, '0')}.${now.getFullYear()}`

  const [metrics, setMetrics] = useState<DashboardMetrics | null>(null)
  const [metricsLoading, setMetricsLoading] = useState(true)
  const [metricsError, setMetricsError] = useState<string | null>(null)

  const [orders, setOrders] = useState<OrderSummary[] | null>(null)
  const [ordersLoading, setOrdersLoading] = useState(true)
  const [ordersError, setOrdersError] = useState<string | null>(null)

  useEffect(() => {
    let active = true

    async function loadMetrics() {
      try {
        const data = await request<DashboardMetrics>('GET', '/api/shop/dashboard')
        if (active) {
          setMetrics(data)
          setMetricsError(null)
        }
      } catch (error) {
        if (active) {
          setMetricsError(errorMessage(error, FALLBACK_ERROR))
        }
      } finally {
        if (active) {
          setMetricsLoading(false)
        }
      }
    }

    async function loadOrders() {
      try {
        const data = await request<{ orders: OrderSummary[] }>(
          'GET',
          '/api/shop/orders',
        )
        if (active) {
          setOrders(Array.isArray(data.orders) ? data.orders : [])
          setOrdersError(null)
        }
      } catch (error) {
        if (active) {
          setOrdersError(errorMessage(error, FALLBACK_ERROR))
        }
      } finally {
        if (active) {
          setOrdersLoading(false)
        }
      }
    }

    void loadMetrics()
    void loadOrders()

    return () => {
      active = false
    }
  }, [])

  const newestOrders = (orders ?? [])
    .slice()
    .sort((a, b) => b.order_number.localeCompare(a.order_number))
    .slice(0, 3)

  const countByStatus = STATUS_ORDER.map((status) => ({
    status,
    count: (orders ?? []).filter((order) => order.status === status).length,
  }))

  return (
    <section className="page shop-dashboard">
      <div className="shop-dashboard__header">
        <header className="page-header">
          <h1>Dashboard</h1>
          <p className="page-header__subtitle">
            Überblick über offene Aufträge, Tagesabschlüsse und Monatsumsatz.
          </p>
        </header>
        <Link className="btn" to="/shop/orders">
          Zur Auftragsliste
        </Link>
      </div>

      {metricsError ? (
        <div className="alert alert--error" role="alert">
          <span>{metricsError}</span>
        </div>
      ) : null}

      <section className="dash-stat-grid" aria-label="Kennzahlen">
        <StatCard
          testId="stat-open-orders"
          label="Offene Aufträge"
          value={metrics ? String(metrics.open_orders) : metricsLoading ? '…' : '–'}
          context="angefragt, bestätigt, in Arbeit"
          icon={
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2" />
              <rect x="9" y="3" width="6" height="4" rx="1" />
            </svg>
          }
        />
        <StatCard
          testId="stat-finished-today"
          label="Heute fertig"
          value={
            metrics ? String(metrics.finished_today) : metricsLoading ? '…' : '–'
          }
          context={`Statuswechsel nach „fertig“ am ${todayLabel}`}
          icon={
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <path d="M20 6 9 17l-5-5" />
            </svg>
          }
        />
        <StatCard
          testId="stat-revenue-month"
          label={`Umsatz ${monthLabel}`}
          value={
            metrics
              ? formatCents(metrics.revenue_month_cents)
              : metricsLoading
                ? '…'
                : '–'
          }
          context="aus Rechnungen des laufenden Monats"
          icon={
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
            </svg>
          }
        />
      </section>

      <div className="dash-grid dash-section-gap">
        <section className="card" aria-labelledby="recent-orders-title">
          <div className="dash-card__header">
            <span className="dash-card__label" id="recent-orders-title">
              Neueste Aufträge
            </span>
            <Link to="/shop/orders">Alle anzeigen</Link>
          </div>
          {ordersLoading ? (
            <p className="dash-loading">Wird geladen …</p>
          ) : ordersError ? (
            <div className="alert alert--error" role="alert">
              <span>{ordersError}</span>
            </div>
          ) : (
            <NewestOrders orders={newestOrders} />
          )}
        </section>

        <section className="card" aria-labelledby="by-status-title">
          <div className="dash-card__header">
            <span className="dash-card__label" id="by-status-title">
              Aufträge nach Status
            </span>
          </div>
          <ul className="dash-summary-list">
            {countByStatus.map(({ status, count }) => (
              <li className="dash-summary-list__row" key={status}>
                <StatusBadge status={status} />
                <span
                  className="dash-summary-list__count"
                  data-testid={`status-count-${status}`}
                >
                  {count}
                </span>
              </li>
            ))}
          </ul>
          <div className="dash-card__footer">
            <Link className="btn btn--secondary btn--block" to="/shop/orders">
              Aufträge filtern
            </Link>
          </div>
        </section>
      </div>
    </section>
  )
}
