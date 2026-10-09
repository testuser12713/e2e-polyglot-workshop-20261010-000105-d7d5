/**
 * Auftragsdetail des Werkstattbereichs (mockup `design/mockups/auftrag.html`).
 *
 * Loads one order from `GET /api/shop/orders/{order_number}` and shows its meta
 * data, its captured positions and the status timeline. The status control
 * offers exactly the allowed follow-up transitions of the current status; every
 * other transition stays visibly disabled (`aria-disabled`). Irreversible steps
 * ask for a ConfirmDialog first. A saved position and an accepted status change
 * are reflected immediately.
 */
import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, request } from '../lib/api'
import './ShopOrderDetail.css'

export type OrderStatus =
  | 'requested'
  | 'confirmed'
  | 'in_progress'
  | 'done'
  | 'picked_up'

export type ItemKind = 'labor' | 'part'

export interface OrderItem {
  id: number
  kind: ItemKind
  description: string
  quantity: number
  hours: number
  unit_price_cents: number
  amount_cents: number
}

export interface StatusEntry {
  at: string
  from_status: OrderStatus | null
  to_status: OrderStatus
}

export interface Customer {
  id: number
  name: string
  email: string
  phone: string
}

export interface Vehicle {
  id: number
  plate: string
  make: string
  model: string
  mileage: number
}

export interface OrderDetail {
  order_number: string
  status: OrderStatus
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

export interface ItemInput {
  kind: ItemKind
  description: string
  quantity: number
  hours: number
  unit_price_cents: number
}

/** The five statuses in workflow order. */
export const ORDER_STATUSES: OrderStatus[] = [
  'requested',
  'confirmed',
  'in_progress',
  'done',
  'picked_up',
]

export const STATUS_LABELS: Record<OrderStatus, string> = {
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

/** Exactly one allowed follow-up transition per status (linear state machine). */
export const NEXT_STATUS: Record<OrderStatus, OrderStatus | null> = {
  requested: 'confirmed',
  confirmed: 'in_progress',
  in_progress: 'done',
  done: 'picked_up',
  picked_up: null,
}

/**
 * Steps that must be confirmed in a ConfirmDialog because they cannot be
 * undone: finishing the order (triggers the invoice) and the customer pickup.
 */
const IRREVERSIBLE: OrderStatus[] = ['done', 'picked_up']

function isOrderStatus(value: string): value is OrderStatus {
  return (ORDER_STATUSES as string[]).includes(value)
}

/** '2025-03-07' -> '07.03.2025' (DESIGN.md value formatting). */
export function formatDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value)
  if (!match) {
    return value
  }
  return `${match[3]}.${match[2]}.${match[1]}`
}

/** RFC3339 UTC -> '07.03.2025, 14:32 (UTC)' — never a local-time conversion. */
export function formatTimestamp(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    `${pad(date.getUTCDate())}.${pad(date.getUTCMonth() + 1)}.${date.getUTCFullYear()}, ` +
    `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())} (UTC)`
  )
}

/** Integer cents -> '1.234,56 €' with a no-break space before the unit. */
export function formatMoney(cents: number): string {
  const negative = cents < 0
  const abs = Math.abs(Math.round(cents))
  const euros = String(Math.floor(abs / 100)).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  const rest = String(abs % 100).padStart(2, '0')
  return `${negative ? '-' : ''}${euros},${rest}\u00A0€`
}

/** Decimal with comma and at most two fraction digits ('2,5'). */
export function formatNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return '0'
  }
  const rounded = Math.round(value * 100) / 100
  const [integer, fraction] = String(rounded).split('.')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return fraction ? `${grouped},${fraction}` : grouped
}

/** Parse a German-or-dot user input ('2,5') into a finite number (0 on garbage). */
export function parseDecimal(value: string): number {
  const normalized = value.replace(/\./g, '').replace(',', '.').trim()
  if (!normalized) {
    return 0
  }
  const parsed = Number.parseFloat(normalized)
  return Number.isFinite(parsed) ? parsed : 0
}

/** Parse a euro amount ('68,40') into integer cents. */
export function parseEurosToCents(value: string): number {
  return Math.round(parseDecimal(value) * 100)
}

/** StatusBadge — pill with a dot and the fixed German label (DESIGN.md). */
export function StatusBadge({ status, large = false }: { status: OrderStatus; large?: boolean }) {
  const modifier = STATUS_CSS[status] ?? 'angefragt'
  return (
    <span className={`badge badge--${modifier}${large ? ' badge--lg' : ''}`}>
      {STATUS_LABELS[status] ?? status}
    </span>
  )
}

function ItemRow({ item }: { item: OrderItem }) {
  const quantity =
    item.kind === 'labor' ? `${formatNumber(item.hours)} h` : formatNumber(item.quantity)
  return (
    <tr>
      <td>
        <span className="pos-kind">
          {item.kind === 'labor' ? 'Arbeitszeit' : 'Teil'}
        </span>
        {item.description}
      </td>
      <td className="num">{quantity}</td>
      <td className="num">{formatMoney(item.unit_price_cents)}</td>
      <td className="num">{formatMoney(item.amount_cents)}</td>
    </tr>
  )
}

interface ConfirmDialogProps {
  orderNumber: string
  target: OrderStatus
  busy: boolean
  onCancel: () => void
  onConfirm: () => void
}

function ConfirmDialog({ orderNumber, target, busy, onCancel, onConfirm }: ConfirmDialogProps) {
  const confirmRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    confirmRef.current?.focus()
  }, [])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onCancel()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onCancel])

  return (
    <div
      className="modal is-open"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onCancel()
        }
      }}
    >
      <div className="modal__dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 className="modal__title" id="confirm-title">
          Statuswechsel bestätigen
        </h2>
        <p className="modal__body">
          Auftrag {orderNumber} auf „{STATUS_LABELS[target]}“ setzen?
        </p>
        <div className="modal__actions">
          <button type="button" className="btn btn--secondary" onClick={onCancel}>
            Abbrechen
          </button>
          <button
            ref={confirmRef}
            type="button"
            className="btn btn--primary"
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? 'Wird gesendet …' : 'Bestätigen'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function ShopOrderDetail() {
  const { orderNumber = '' } = useParams()

  const [order, setOrder] = useState<OrderDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<{ message: string; unauthorized: boolean } | null>(null)

  const [actionError, setActionError] = useState<string | null>(null)
  const [savingStatus, setSavingStatus] = useState(false)
  const [pendingTarget, setPendingTarget] = useState<OrderStatus | null>(null)
  const [toast, setToast] = useState<string | null>(null)

  const [kind, setKind] = useState<ItemKind>('labor')
  const [description, setDescription] = useState('')
  const [menge, setMenge] = useState('')
  const [unitPrice, setUnitPrice] = useState('')
  const [touched, setTouched] = useState(false)
  const [savingItem, setSavingItem] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)

    request<OrderDetail>('GET', `/api/shop/orders/${encodeURIComponent(orderNumber)}`)
      .then((data) => {
        if (!cancelled) {
          setOrder(data)
        }
      })
      .catch((err: unknown) => {
        if (cancelled) {
          return
        }
        const message =
          err instanceof ApiError ? err.message : 'Der Auftrag konnte nicht geladen werden.'
        setError({ message, unauthorized: err instanceof ApiError && err.status === 401 })
        setOrder(null)
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [orderNumber])

  useEffect(() => {
    if (!toast) {
      return
    }
    const timer = window.setTimeout(() => setToast(null), 5000)
    return () => window.clearTimeout(timer)
  }, [toast])

  const nextStatus = order ? NEXT_STATUS[order.status] : null

  const applyStatus = async (target: OrderStatus) => {
    if (!order) {
      return
    }
    setActionError(null)
    setSavingStatus(true)
    const from = order.status
    try {
      const result = await request<{ order_number: string; status: OrderStatus }>(
        'POST',
        `/api/shop/orders/${encodeURIComponent(orderNumber)}/status`,
        { status: target },
      )
      const status = isOrderStatus(result.status) ? result.status : target
      setOrder((previous) =>
        previous
          ? {
              ...previous,
              status,
              history: [
                ...previous.history,
                { at: new Date().toISOString(), from_status: from, to_status: status },
              ],
            }
          : previous,
      )
      setToast(`Status auf „${STATUS_LABELS[status]}“ gesetzt.`)
    } catch (err: unknown) {
      const message =
        err instanceof ApiError ? err.message : 'Der Statuswechsel ist fehlgeschlagen.'
      setActionError(message)
    } finally {
      setSavingStatus(false)
    }
  }

  const requestStatusChange = (target: OrderStatus) => {
    if (!order || target !== NEXT_STATUS[order.status]) {
      return
    }
    if (IRREVERSIBLE.includes(target)) {
      setPendingTarget(target)
      return
    }
    void applyStatus(target)
  }

  const lineTotalCents = Math.round(parseDecimal(menge) * parseEurosToCents(unitPrice))
  const descriptionMissing = touched && description.trim().length === 0

  const addItem = async () => {
    if (!order) {
      return
    }
    setTouched(true)
    const trimmed = description.trim()
    if (!trimmed) {
      return
    }
    setActionError(null)
    setSavingItem(true)
    const value = parseDecimal(menge)
    const unitPriceCents = parseEurosToCents(unitPrice)
    const payload: ItemInput =
      kind === 'labor'
        ? {
            kind: 'labor',
            description: trimmed,
            quantity: 0,
            hours: value,
            unit_price_cents: unitPriceCents,
          }
        : {
            kind: 'part',
            description: trimmed,
            quantity: value,
            hours: 0,
            unit_price_cents: unitPriceCents,
          }
    try {
      const item = await request<OrderItem>(
        'POST',
        `/api/shop/orders/${encodeURIComponent(orderNumber)}/items`,
        payload,
      )
      setOrder((previous) =>
        previous ? { ...previous, items: [...previous.items, item] } : previous,
      )
      setDescription('')
      setMenge('')
      setUnitPrice('')
      setTouched(false)
      setToast('Position gespeichert.')
    } catch (err: unknown) {
      const message =
        err instanceof ApiError ? err.message : 'Die Position konnte nicht gespeichert werden.'
      setActionError(message)
    } finally {
      setSavingItem(false)
    }
  }

  if (loading) {
    return (
      <section className="page">
        <p className="detail-status" aria-live="polite">
          Wird geladen …
        </p>
      </section>
    )
  }

  if (error || !order) {
    return (
      <section className="page">
        <header className="page-header">
          <h1>Auftragsdetails</h1>
        </header>
        <div className="alert alert--error" role="alert">
          <div>
            <p>{error?.message ?? 'Der Auftrag konnte nicht geladen werden.'}</p>
            {error?.unauthorized ? (
              <p>
                <Link to="/shop/login">Zur Anmeldung</Link>
              </p>
            ) : (
              <p>
                <Link to="/shop/orders">Zurück zur Auftragsliste</Link>
              </p>
            )}
          </div>
        </div>
      </section>
    )
  }

  const netCents = order.items.reduce((sum, item) => sum + item.amount_cents, 0)
  const taxCents = Math.round(netCents * 0.19)
  const grossCents = netCents + taxCents
  const timeline = [...order.history].reverse()

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <div className="eyebrow">Auftrag</div>
          <div className="order-head">
            <h1 className="mono">{order.order_number}</h1>
            <StatusBadge status={order.status} large />
          </div>
          <p className="page-header__subtitle">
            {order.make} {order.model} · <span className="mono">{order.vehicle_plate}</span> ·
            Kunde {order.customer_name}
          </p>
        </div>
        <Link className="btn btn--secondary" to="/shop/orders">
          Zurück zur Liste
        </Link>
      </header>

      <div className="detail-grid">
        <div className="stack-4">
          <section className="card" aria-labelledby="meta-title">
            <div className="card__header">
              <span className="card__label" id="meta-title">
                Auftragsdaten
              </span>
            </div>
            <dl className="kv">
              <dt>Auftragsnummer</dt>
              <dd className="mono">{order.order_number}</dd>
              <dt>Kennzeichen</dt>
              <dd className="mono">{order.vehicle_plate}</dd>
              <dt>Fahrzeug</dt>
              <dd>
                {order.make} {order.model}
              </dd>
              <dt>Kilometerstand</dt>
              <dd className="num">{formatNumber(order.vehicle.mileage)} km</dd>
              <dt>Wunschtermin</dt>
              <dd className="num">{formatDate(order.desired_date)}</dd>
              <dt>Kunde</dt>
              <dd>{order.customer.name}</dd>
            </dl>
            <hr className="hr-sunken" />
            <div className="card__label">Problembeschreibung</div>
            <p className="problem-text">{order.problem || 'Keine Problembeschreibung erfasst.'}</p>
          </section>

          <section className="card" aria-labelledby="pos-title">
            <div className="card__header">
              <span className="card__label" id="pos-title">
                Erfasste Positionen
              </span>
            </div>
            {order.items.length === 0 ? (
              <p className="note">Keine Positionen erfasst.</p>
            ) : (
              <>
                <table className="line-table">
                  <thead>
                    <tr>
                      <th scope="col">Beschreibung</th>
                      <th scope="col" className="num">
                        Menge/Std.
                      </th>
                      <th scope="col" className="num">
                        Einzelpreis
                      </th>
                      <th scope="col" className="num">
                        Betrag
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {order.items.map((item) => (
                      <ItemRow key={item.id} item={item} />
                    ))}
                  </tbody>
                </table>
                <hr className="hr-sunken" />
                <ul className="summary-list" aria-label="Summe der Positionen">
                  <li className="summary-list__row">
                    <span>Netto</span>
                    <span className="mono">{formatMoney(netCents)}</span>
                  </li>
                  <li className="summary-list__row">
                    <span>MwSt. 19 %</span>
                    <span className="mono">{formatMoney(taxCents)}</span>
                  </li>
                  <li className="summary-list__row summary-list__row--total">
                    <span>Brutto</span>
                    <span className="mono">{formatMoney(grossCents)}</span>
                  </li>
                </ul>
              </>
            )}
          </section>

          <section className="card" aria-labelledby="new-pos-title">
            <div className="card__header">
              <span className="card__label" id="new-pos-title">
                Position erfassen
              </span>
            </div>
            <p className="note">
              Arbeitszeit- und Teile-Positionen erscheinen später in der Rechnung. Beträge in
              Euro, gerundet auf Cent.
            </p>
            <div className="pos-editor">
              <div className="field">
                <label className="field__label" htmlFor="pos-kind">
                  Art
                </label>
                <select
                  className="input"
                  id="pos-kind"
                  value={kind}
                  onChange={(event) => setKind(event.target.value as ItemKind)}
                >
                  <option value="labor">Arbeitszeit</option>
                  <option value="part">Teil</option>
                </select>
              </div>
              <div className={`field${descriptionMissing ? ' field--error' : ''}`}>
                <label className="field__label" htmlFor="pos-description">
                  Beschreibung <span className="field__req">*</span>
                </label>
                <input
                  className="input"
                  id="pos-description"
                  type="text"
                  placeholder="z. B. Arbeitszeit Achsvermessung"
                  value={description}
                  aria-invalid={descriptionMissing}
                  onChange={(event) => {
                    setDescription(event.target.value)
                    setTouched(true)
                  }}
                />
                {descriptionMissing ? (
                  <span className="field__error" role="alert">
                    Bitte eine Beschreibung angeben.
                  </span>
                ) : null}
              </div>
              <div className="field">
                <label className="field__label" htmlFor="pos-menge">
                  Menge/Std.
                </label>
                <input
                  className="input num ta-right"
                  id="pos-menge"
                  type="text"
                  inputMode="decimal"
                  placeholder="0,0"
                  value={menge}
                  onChange={(event) => setMenge(event.target.value)}
                />
              </div>
              <div className="field">
                <label className="field__label" htmlFor="pos-preis">
                  Einzelpreis
                </label>
                <input
                  className="input num ta-right"
                  id="pos-preis"
                  type="text"
                  inputMode="decimal"
                  placeholder="0,00"
                  value={unitPrice}
                  onChange={(event) => setUnitPrice(event.target.value)}
                />
              </div>
              <div className="field">
                <span className="field__label">Betrag</span>
                <div className="pos-row__total" aria-live="polite">
                  {formatMoney(lineTotalCents)}
                </div>
              </div>
              <button
                className="btn btn--secondary"
                type="button"
                onClick={() => void addItem()}
                disabled={savingItem}
              >
                {savingItem ? 'Wird gesendet …' : 'Hinzufügen'}
              </button>
            </div>
            {actionError ? (
              <div className="alert alert--error action-error" role="alert">
                {actionError}
              </div>
            ) : null}
          </section>
        </div>

        <div className="stack-4">
          <section className="card" aria-labelledby="status-title">
            <div className="card__header">
              <span className="card__label" id="status-title">
                Statuswechsel
              </span>
            </div>
            <p className="note">Nur der erlaubte Folgeübergang des aktuellen Status ist anwählbar.</p>
            <div className="segmented" role="group" aria-label="Status setzen">
              {ORDER_STATUSES.map((status) => {
                const isCurrent = status === order.status
                const isAllowed = status === nextStatus
                return (
                  <button
                    key={status}
                    type="button"
                    className="segmented__btn"
                    aria-pressed={isCurrent}
                    aria-current={isCurrent ? 'true' : undefined}
                    aria-disabled={!isAllowed}
                    disabled={!isAllowed || savingStatus}
                    onClick={() => requestStatusChange(status)}
                  >
                    {STATUS_LABELS[status]}
                  </button>
                )
              })}
            </div>
            <p className="note status-hint" aria-live="polite">
              {nextStatus
                ? `Erlaubter nächster Schritt: ${STATUS_LABELS[nextStatus]}.`
                : 'Der Auftrag ist abgeschlossen; kein weiterer Statuswechsel möglich.'}
            </p>
            {actionError ? (
              <div className="alert alert--error action-error" role="alert">
                {actionError}
              </div>
            ) : null}
          </section>

          <section className="card" aria-labelledby="timeline-title">
            <div className="card__header">
              <span className="card__label" id="timeline-title">
                Statusverlauf
              </span>
            </div>
            {timeline.length === 0 ? (
              <p className="note">Noch kein Statuswechsel protokolliert.</p>
            ) : (
              <ul className="timeline" aria-live="polite">
                {timeline.map((entry, index) => (
                  <li
                    key={`${entry.at}-${entry.to_status}`}
                    className={`timeline__item timeline__item--${STATUS_CSS[entry.to_status]}${
                      index > 0 ? ' timeline__item--old' : ''
                    }`}
                  >
                    <div className="timeline__row">
                      <span className="timeline__time">{formatTimestamp(entry.at)}</span>
                      <StatusBadge status={entry.to_status} />
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      </div>

      {pendingTarget ? (
        <ConfirmDialog
          orderNumber={order.order_number}
          target={pendingTarget}
          busy={savingStatus}
          onCancel={() => setPendingTarget(null)}
          onConfirm={() => {
            const target = pendingTarget
            setPendingTarget(null)
            void applyStatus(target)
          }}
        />
      ) : null}

      {toast ? (
        <div className="toast" role="status" aria-live="polite">
          <span>{toast}</span>
          <button
            className="toast__dismiss"
            type="button"
            aria-label="Schließen"
            onClick={() => setToast(null)}
          >
            ×
          </button>
        </div>
      ) : null}
    </section>
  )
}
