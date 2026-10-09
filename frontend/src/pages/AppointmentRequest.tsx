import { useState, type CSSProperties, type FormEvent } from 'react'
import { request } from '../lib/api'
import {
  REQUIRED_FIELDS,
  type AppointmentField,
  type AppointmentFormValues,
  validateForm,
} from '../lib/validation'

interface OrderCreated {
  order_number: string
  status: string
}

interface PositionDraft {
  id: number
  kind: 'labor' | 'part'
  description: string
  amount: string
  unitPrice: string
}

const EMPTY_VALUES: AppointmentFormValues = {
  name: '',
  email: '',
  phone: '',
  plate: '',
  make: '',
  model: '',
  mileage: '',
  desiredDate: '',
  problem: '',
}

const LABELS: Record<AppointmentField, string> = {
  name: 'Name',
  email: 'E-Mail',
  phone: 'Telefon',
  plate: 'Kennzeichen',
  make: 'Marke',
  model: 'Modell',
  mileage: 'Kilometerstand',
  desiredDate: 'Wunschtermin',
  problem: 'Problembeschreibung',
}

let positionSeq = 0

function newPosition(): PositionDraft {
  positionSeq += 1
  return { id: positionSeq, kind: 'labor', description: '', amount: '', unitPrice: '' }
}

/** Parse a German decimal input ("2,5", "1.234,56", "89") into a number. */
function parseDecimal(value: string): number {
  const raw = value.trim().replace(/\s/g, '')
  if (raw.length === 0) {
    return 0
  }
  const normalized = raw.includes(',')
    ? raw.replace(/\./g, '').replace(',', '.')
    : raw
  const parsed = Number.parseFloat(normalized)
  return Number.isFinite(parsed) ? parsed : 0
}

/** The line total of one position, in integer cents. */
function lineTotalCents(position: PositionDraft): number {
  const quantity = parseDecimal(position.amount)
  const unitPriceCents = Math.round(parseDecimal(position.unitPrice) * 100)
  return Math.round(quantity * unitPriceCents)
}

/** Format integer cents as the product-wide German money string. */
function formatCents(cents: number): string {
  const sign = cents < 0 ? '-' : ''
  const absolute = Math.abs(cents)
  const euros = Math.floor(absolute / 100)
  const rest = String(absolute % 100).padStart(2, '0')
  const grouped = String(euros).replace(/\B(?=(\d{3})+(?!\d))/g, '.')
  return `${sign}${grouped},${rest}\u00A0€`
}

const gridTwo: CSSProperties = {
  display: 'grid',
  gap: 'var(--space-3)',
  gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
}

const sectionTitle: CSSProperties = {
  fontSize: 'var(--size-sm)',
  fontWeight: 600,
  textTransform: 'uppercase',
  letterSpacing: '0.02em',
  color: 'var(--color-muted)',
  marginBottom: 'var(--space-3)',
}

const summaryRow: CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  gap: 'var(--space-3)',
  paddingBlock: 'var(--space-1)',
  fontSize: 'var(--size-sm)',
}

const helperStyle: CSSProperties = {
  fontSize: 'var(--size-xs)',
  color: 'var(--color-muted)',
}

interface FormFieldProps {
  id: AppointmentField
  value: string
  error: string | null
  onValueChange: (value: string) => void
  onTouched: () => void
  type?: string
  autoComplete?: string
  placeholder?: string
  helper?: string
  textarea?: boolean
  mono?: boolean
  numeric?: boolean
  min?: number
}

function FormField(props: FormFieldProps) {
  const label = LABELS[props.id]
  const helperId = `${props.id}-helper`
  const errorId = `${props.id}-error`
  const describedBy = props.error ? errorId : props.helper ? helperId : undefined
  const className = [props.mono ? 'mono' : '', props.numeric ? 'numeric' : '']
    .filter(Boolean)
    .join(' ')

  return (
    <div className={`field${props.error ? ' field--error' : ''}`}>
      <label htmlFor={props.id}>
        {label}{' '}
        <span style={{ color: 'var(--color-danger)' }} aria-hidden="true">
          *
        </span>
      </label>
      {props.textarea ? (
        <textarea
          id={props.id}
          name={props.id}
          value={props.value}
          placeholder={props.placeholder}
          required
          aria-invalid={props.error ? true : undefined}
          aria-describedby={describedBy}
          onChange={(event) => props.onValueChange(event.target.value)}
          onBlur={props.onTouched}
        />
      ) : (
        <input
          id={props.id}
          name={props.id}
          type={props.type ?? 'text'}
          value={props.value}
          placeholder={props.placeholder}
          autoComplete={props.autoComplete}
          min={props.min}
          required
          className={className || undefined}
          aria-invalid={props.error ? true : undefined}
          aria-describedby={describedBy}
          onChange={(event) => props.onValueChange(event.target.value)}
          onBlur={props.onTouched}
        />
      )}
      {props.helper && !props.error ? (
        <p id={helperId} style={helperStyle}>
          {props.helper}
        </p>
      ) : null}
      {props.error ? (
        <p id={errorId} className="field__error" role="alert">
          {props.error}
        </p>
      ) : null}
    </div>
  )
}

export default function AppointmentRequest() {
  const [values, setValues] = useState<AppointmentFormValues>(EMPTY_VALUES)
  const [touched, setTouched] = useState<Partial<Record<AppointmentField, boolean>>>({})
  const [submitAttempted, setSubmitAttempted] = useState(false)
  const [positions, setPositions] = useState<PositionDraft[]>(() => [newPosition()])
  const [submitting, setSubmitting] = useState(false)
  const [orderNumber, setOrderNumber] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const errors = validateForm(values)

  const errorFor = (field: AppointmentField): string | null =>
    touched[field] || submitAttempted ? errors[field] ?? null : null

  const setValue = (field: AppointmentField, value: string): void => {
    setValues((previous) => ({ ...previous, [field]: value }))
  }

  const markTouched = (field: AppointmentField): void => {
    setTouched((previous) => ({ ...previous, [field]: true }))
  }

  const updatePosition = (id: number, patch: Partial<PositionDraft>): void => {
    setPositions((previous) =>
      previous.map((position) => (position.id === id ? { ...position, ...patch } : position)),
    )
  }

  const removePosition = (id: number): void => {
    setPositions((previous) =>
      previous.length > 1 ? previous.filter((position) => position.id !== id) : previous,
    )
  }

  const netto = positions.reduce((sum, position) => sum + lineTotalCents(position), 0)
  const tax = Math.round(netto * 0.19)
  const brutto = netto + tax

  const buildItems = () =>
    positions
      .filter(
        (position) =>
          position.description.trim() !== '' ||
          position.amount.trim() !== '' ||
          position.unitPrice.trim() !== '',
      )
      .map((position) => {
        const quantity = parseDecimal(position.amount)
        return {
          kind: position.kind,
          description: position.description.trim(),
          quantity: position.kind === 'part' ? quantity : 0,
          hours: position.kind === 'labor' ? quantity : 0,
          unit_price_cents: Math.round(parseDecimal(position.unitPrice) * 100),
        }
      })

  const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault()
    setSubmitAttempted(true)
    setFormError(null)

    const nextErrors = validateForm(values)
    const firstInvalid = REQUIRED_FIELDS.find((field) => nextErrors[field])
    if (firstInvalid) {
      document.getElementById(firstInvalid)?.focus()
      return
    }

    setSubmitting(true)
    try {
      const created = await request<OrderCreated>('POST', '/api/public/orders', {
        customer: {
          name: values.name.trim(),
          email: values.email.trim(),
          phone: values.phone.trim(),
        },
        vehicle: {
          plate: values.plate.trim().toUpperCase(),
          make: values.make.trim(),
          model: values.model.trim(),
          mileage: Number(values.mileage),
        },
        desired_date: values.desiredDate,
        problem: values.problem.trim(),
        items: buildItems(),
      })
      setOrderNumber(created.order_number)
    } catch (error) {
      setFormError(
        error instanceof Error
          ? error.message
          : 'Es ist ein unerwarteter Fehler aufgetreten. Bitte versuchen Sie es erneut.',
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="page" style={{ maxWidth: '720px', marginInline: 'auto' }}>
      <header className="page-header">
        <div
          style={{
            fontSize: 'var(--size-xs)',
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            color: 'var(--color-accent)',
            marginBottom: 'var(--space-0)',
          }}
        >
          Kundenbereich
        </div>
        <h1>Terminanfrage</h1>
        <p className="page-header__subtitle">
          Erfassen Sie Ihr Fahrzeug und Ihr Anliegen. Der Auftrag startet im Status
          „angefragt“ und erhält eine Auftragsnummer.
        </p>
      </header>

      {orderNumber ? (
        <div className="alert alert--success" role="status" style={{ marginBottom: 'var(--space-4)' }}>
          <span aria-hidden="true">✓</span>
          <span>
            <span style={{ display: 'block', fontWeight: 600 }}>
              Anfrage gesendet — Auftrag {orderNumber} wurde angelegt.
            </span>
            <span>
              Wir melden uns zur Terminbestätigung. Den Status können Sie jederzeit
              über die Statusabfrage verfolgen.
            </span>
          </span>
        </div>
      ) : null}

      {formError ? (
        <div className="alert alert--error" role="alert" style={{ marginBottom: 'var(--space-4)' }}>
          <span aria-hidden="true">!</span>
          <span>{formError}</span>
        </div>
      ) : null}

      <form id="termin-form" noValidate onSubmit={handleSubmit}>
        <p style={{ ...helperStyle, marginBottom: 'var(--space-3)' }}>
          Mit <span style={{ color: 'var(--color-danger)' }}>*</span> gekennzeichnete
          Felder sind Pflichtfelder.
        </p>

        <div className="card">
          <div style={{ marginBottom: 'var(--space-5)' }}>
            <div style={sectionTitle}>Kontakt</div>
            <div style={gridTwo}>
              <FormField
                id="name"
                value={values.name}
                error={errorFor('name')}
                autoComplete="name"
                onValueChange={(value) => setValue('name', value)}
                onTouched={() => markTouched('name')}
              />
              <FormField
                id="email"
                value={values.email}
                error={errorFor('email')}
                type="email"
                autoComplete="email"
                helper="Für Rückfragen zur Terminbestätigung."
                onValueChange={(value) => setValue('email', value)}
                onTouched={() => markTouched('email')}
              />
              <FormField
                id="phone"
                value={values.phone}
                error={errorFor('phone')}
                type="tel"
                autoComplete="tel"
                onValueChange={(value) => setValue('phone', value)}
                onTouched={() => markTouched('phone')}
              />
            </div>
          </div>

          <hr style={{ height: 1, background: 'var(--color-border)', border: 0, margin: 0 }} />

          <div style={{ marginTop: 'var(--space-5)' }}>
            <div style={sectionTitle}>Fahrzeug</div>
            <div style={gridTwo}>
              <FormField
                id="plate"
                value={values.plate}
                error={errorFor('plate')}
                placeholder="M-AB 1234"
                helper="Format: M-AB 1234"
                mono
                onValueChange={(value) => setValue('plate', value)}
                onTouched={() => markTouched('plate')}
              />
              <FormField
                id="make"
                value={values.make}
                error={errorFor('make')}
                placeholder="BMW"
                onValueChange={(value) => setValue('make', value)}
                onTouched={() => markTouched('make')}
              />
              <FormField
                id="model"
                value={values.model}
                error={errorFor('model')}
                placeholder="320d"
                onValueChange={(value) => setValue('model', value)}
                onTouched={() => markTouched('model')}
              />
              <FormField
                id="mileage"
                value={values.mileage}
                error={errorFor('mileage')}
                type="number"
                min={0}
                placeholder="87450"
                helper="Ganzzahlige Kilometer, z. B. 87450"
                numeric
                onValueChange={(value) => setValue('mileage', value)}
                onTouched={() => markTouched('mileage')}
              />
            </div>
          </div>

          <hr
            style={{
              height: 1,
              background: 'var(--color-border)',
              border: 0,
              margin: 'var(--space-5) 0 0',
            }}
          />

          <div style={{ marginTop: 'var(--space-5)' }}>
            <div style={sectionTitle}>Termin</div>
            <div style={gridTwo}>
              <FormField
                id="desiredDate"
                value={values.desiredDate}
                error={errorFor('desiredDate')}
                type="date"
                helper="Bevorzugter Termin für die Annahme."
                onValueChange={(value) => setValue('desiredDate', value)}
                onTouched={() => markTouched('desiredDate')}
              />
            </div>
          </div>
        </div>

        <div className="card" style={{ marginTop: 'var(--space-5)' }}>
          <div style={sectionTitle}>Problem</div>
          <FormField
            id="problem"
            value={values.problem}
            error={errorFor('problem')}
            textarea
            placeholder="Bitte beschreiben Sie das Problem möglichst genau …"
            onValueChange={(value) => setValue('problem', value)}
            onTouched={() => markTouched('problem')}
          />
        </div>

        <div className="card" style={{ marginTop: 'var(--space-5)' }}>
          <div style={sectionTitle}>Erste Positionen (optional)</div>
          <p style={{ ...helperStyle, marginBottom: 'var(--space-3)' }}>
            Sie kennen bereits Arbeiten oder Teile? Erfassen Sie sie hier. Alle Beträge
            sind in Euro, gerundet auf Cent.
          </p>

          <div>
            {positions.map((position, index) => (
              <div
                key={position.id}
                style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  alignItems: 'flex-end',
                  gap: 'var(--space-2)',
                  paddingBlock: 'var(--space-3)',
                  borderBottom: '1px solid var(--color-border)',
                }}
              >
                <div className="field" style={{ flex: '0 0 120px', marginBottom: 0 }}>
                  <label htmlFor={`pos-${position.id}-kind`}>Art</label>
                  <select
                    id={`pos-${position.id}-kind`}
                    value={position.kind}
                    onChange={(event) =>
                      updatePosition(position.id, {
                        kind: event.target.value === 'part' ? 'part' : 'labor',
                      })
                    }
                  >
                    <option value="labor">Arbeitszeit</option>
                    <option value="part">Teil</option>
                  </select>
                </div>
                <div className="field" style={{ flex: '1 1 200px', marginBottom: 0 }}>
                  <label htmlFor={`pos-${position.id}-beschreibung`}>Beschreibung</label>
                  <input
                    id={`pos-${position.id}-beschreibung`}
                    type="text"
                    value={position.description}
                    placeholder="z. B. Bremsbeläge vorne"
                    onChange={(event) =>
                      updatePosition(position.id, { description: event.target.value })
                    }
                  />
                </div>
                <div className="field" style={{ flex: '0 0 110px', marginBottom: 0 }}>
                  <label htmlFor={`pos-${position.id}-menge`}>Menge/Stunden</label>
                  <input
                    id={`pos-${position.id}-menge`}
                    type="text"
                    inputMode="decimal"
                    className="numeric"
                    style={{ textAlign: 'right' }}
                    value={position.amount}
                    placeholder="0,0"
                    onChange={(event) =>
                      updatePosition(position.id, { amount: event.target.value })
                    }
                  />
                </div>
                <div className="field" style={{ flex: '0 0 120px', marginBottom: 0 }}>
                  <label htmlFor={`pos-${position.id}-preis`}>Einzelpreis</label>
                  <input
                    id={`pos-${position.id}-preis`}
                    type="text"
                    inputMode="decimal"
                    className="numeric"
                    style={{ textAlign: 'right' }}
                    value={position.unitPrice}
                    placeholder="0,00"
                    onChange={(event) =>
                      updatePosition(position.id, { unitPrice: event.target.value })
                    }
                  />
                </div>
                <div className="field" style={{ flex: '0 0 120px', marginBottom: 0 }}>
                  <label htmlFor={`pos-${position.id}-summe`}>Summe</label>
                  <input
                    id={`pos-${position.id}-summe`}
                    type="text"
                    readOnly
                    tabIndex={-1}
                    className="mono numeric"
                    style={{ textAlign: 'right' }}
                    value={formatCents(lineTotalCents(position))}
                  />
                </div>
                <button
                  type="button"
                  className="btn btn--ghost"
                  aria-label={`Position ${index + 1} entfernen`}
                  onClick={() => removePosition(position.id)}
                >
                  Entfernen
                </button>
              </div>
            ))}
          </div>

          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)', marginTop: 'var(--space-3)' }}>
            <button
              type="button"
              className="btn btn--secondary"
              onClick={() => setPositions((previous) => [...previous, newPosition()])}
            >
              Position hinzufügen
            </button>
          </div>

          <hr
            style={{
              height: 1,
              background: 'var(--color-border)',
              border: 0,
              margin: 'var(--space-3) 0',
            }}
          />

          <ul
            style={{
              listStyle: 'none',
              margin: 0,
              padding: 0,
              marginInlineStart: 'auto',
              minWidth: '260px',
              maxWidth: '320px',
            }}
          >
            <li style={summaryRow}>
              <span>Netto</span>
              <span className="mono numeric">{formatCents(netto)}</span>
            </li>
            <li style={summaryRow}>
              <span>19 % MwSt.</span>
              <span className="mono numeric">{formatCents(tax)}</span>
            </li>
            <li
              style={{
                ...summaryRow,
                marginTop: 'var(--space-2)',
                paddingTop: 'var(--space-3)',
                borderTop: '1px solid var(--color-accent)',
                fontSize: 'var(--size-lg)',
                fontWeight: 600,
              }}
            >
              <span>Brutto</span>
              <span className="mono numeric">{formatCents(brutto)}</span>
            </li>
          </ul>
        </div>

        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 'var(--space-3)',
            marginTop: 'var(--space-4)',
          }}
        >
          <button type="submit" className="btn" disabled={submitting}>
            {submitting ? 'Wird gesendet …' : 'Termin anfragen'}
          </button>
          <span style={helperStyle}>
            Nach dem Absenden erhalten Sie eine Auftragsnummer. Es wird keine E-Mail
            versendet.
          </span>
        </div>
      </form>
    </section>
  )
}
