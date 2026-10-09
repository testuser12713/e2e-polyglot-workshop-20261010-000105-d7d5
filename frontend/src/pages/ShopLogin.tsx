import { type FormEvent, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ApiError } from '../lib/api'
import { login } from '../lib/session'

/** One generic message for every rejected credential — never reveals which part was wrong. */
const GENERIC_LOGIN_ERROR = 'E-Mail oder Passwort ist falsch.'

const UNEXPECTED_ERROR =
  'Es ist ein unerwarteter Fehler aufgetreten. Bitte versuchen Sie es erneut.'

interface LoginLocationState {
  from?: string
}

function Spinner() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      aria-hidden="true"
      focusable="false"
    >
      <circle
        cx="8"
        cy="8"
        r="6"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeDasharray="26"
        strokeDashoffset="10"
        strokeLinecap="round"
      >
        <animateTransform
          attributeName="transform"
          type="rotate"
          from="0 8 8"
          to="360 8 8"
          dur="0.8s"
          repeatCount="indefinite"
        />
      </circle>
    </svg>
  )
}

export default function ShopLogin() {
  const navigate = useNavigate()
  const location = useLocation()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const requestedPath =
    (location.state as LoginLocationState | null)?.from ?? '/shop/orders'

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) {
      return
    }
    setError(null)
    setSubmitting(true)
    try {
      await login(email, password)
      navigate(requestedPath, { replace: true })
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 401) {
        setError(GENERIC_LOGIN_ERROR)
      } else if (caught instanceof ApiError) {
        setError(caught.message)
      } else {
        setError(UNEXPECTED_ERROR)
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="page">
      <div
        className="container container--decision"
        style={{ maxWidth: '420px' }}
      >
        <header
          className="page-header"
          style={{ textAlign: 'center' }}
        >
          <h1>Anmeldung</h1>
          <p className="page-header__subtitle">Werkstattbereich</p>
        </header>

        <div className="card">
          {error ? (
            <div className="alert alert--error" role="alert" style={{ marginBottom: 'var(--space-3)' }}>
              <span aria-hidden="true">⚠</span>
              <span>{error}</span>
            </div>
          ) : null}

          <form onSubmit={handleSubmit} noValidate>
            <div className="field">
              <label htmlFor="login-email">E-Mail</label>
              <input
                id="login-email"
                name="email"
                type="email"
                autoComplete="username"
                placeholder="name@werkstatt-berger.de"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
              />
            </div>

            <div className="field">
              <label htmlFor="login-password">Passwort</label>
              <input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </div>

            <div className="btn-row" style={{ marginTop: 'var(--space-4)' }}>
              <button
                type="submit"
                className="btn btn--primary btn--block"
                disabled={submitting}
                aria-disabled={submitting}
              >
                {submitting ? (
                  <>
                    <Spinner />
                    Wird gesendet …
                  </>
                ) : (
                  'Anmelden'
                )}
              </button>
            </div>
          </form>

          <p
            className="stub-note"
            style={{ textAlign: 'center', marginTop: 'var(--space-3)' }}
          >
            Zugang für Werkstattmitarbeiter
          </p>
        </div>
      </div>
    </section>
  )
}
