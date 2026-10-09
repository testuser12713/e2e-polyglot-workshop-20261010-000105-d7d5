import { useState } from 'react'
import { NavLink, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import AppointmentRequest from './pages/AppointmentRequest'
import OrderTracking from './pages/OrderTracking'
import Invoice from './pages/Invoice'
import ShopLogin from './pages/ShopLogin'
import ShopOrders from './pages/ShopOrders'
import ShopOrderDetail from './pages/ShopOrderDetail'
import ShopDashboard from './pages/ShopDashboard'
import Impressum from './pages/Impressum'
import Datenschutz from './pages/Datenschutz'
import RequireSession from './components/RequireSession'
import { logout, useSession } from './lib/session'

function NotFound() {
  return (
    <section className="page">
      <header className="page-header">
        <h1>Seite nicht gefunden</h1>
        <p className="page-header__subtitle">
          Die angeforderte Seite existiert nicht.
        </p>
      </header>
      <div className="card">
        <p className="stub-note">
          Bitte nutzen Sie die Navigation, um zu einer gültigen Seite zu
          gelangen.
        </p>
      </div>
    </section>
  )
}

export default function App() {
  const [menuOpen, setMenuOpen] = useState(false)
  const session = useSession()
  const navigate = useNavigate()
  const closeMenu = () => setMenuOpen(false)

  const handleLogout = () => {
    logout()
    closeMenu()
    navigate('/shop/login', { replace: true })
  }

  return (
    <div className="app-shell">
      <header className="topnav">
        <div className="container topnav__inner">
          <NavLink
            className="topnav__wordmark"
            to="/appointment"
            onClick={closeMenu}
          >
            Werkstatt-Portal
          </NavLink>
          <button
            type="button"
            className="topnav__menu-button"
            aria-expanded={menuOpen}
            aria-controls="primary-nav"
            onClick={() => setMenuOpen((open) => !open)}
          >
            Menü
          </button>
          <nav
            id="primary-nav"
            className={`topnav__nav${menuOpen ? ' is-open' : ''}`}
            aria-label="Hauptnavigation"
            onClick={closeMenu}
          >
            <NavLink className="topnav__area" to="/appointment">
              Kundenbereich
            </NavLink>
            <NavLink className="topnav__area" to="/shop/orders">
              Werkstattbereich
            </NavLink>
            <NavLink className="topnav__link" to="/track">
              Status abfragen
            </NavLink>
            <NavLink className="topnav__link" to="/invoice">
              Rechnung
            </NavLink>
            <NavLink className="topnav__link" to="/shop/dashboard">
              Dashboard
            </NavLink>
            {session ? (
              <>
                <span
                  className="topnav__email"
                  title={session.employee.email}
                  style={{
                    color: 'var(--color-muted)',
                    fontSize: 'var(--size-sm)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    maxWidth: '180px',
                  }}
                >
                  {session.employee.email}
                </span>
                <button
                  type="button"
                  className="btn btn--ghost"
                  onClick={handleLogout}
                >
                  Abmelden
                </button>
              </>
            ) : (
              <NavLink className="topnav__link" to="/shop/login">
                Anmeldung
              </NavLink>
            )}
          </nav>
        </div>
      </header>

      <main className="app-main">
        <div className="container">
          <Routes>
            <Route path="/" element={<Navigate to="/appointment" replace />} />
            <Route path="/appointment" element={<AppointmentRequest />} />
            <Route path="/track" element={<OrderTracking />} />
            <Route path="/track/:orderNumber" element={<OrderTracking />} />
            <Route path="/invoice" element={<Invoice />} />
            <Route path="/invoice/:orderNumber" element={<Invoice />} />
            <Route path="/impressum" element={<Impressum />} />
            <Route path="/datenschutz" element={<Datenschutz />} />
            <Route path="/shop" element={<Navigate to="/shop/orders" replace />} />
            <Route path="/shop/login" element={<ShopLogin />} />
            <Route
              path="/shop/orders"
              element={
                <RequireSession>
                  <ShopOrders />
                </RequireSession>
              }
            />
            <Route
              path="/shop/orders/:orderNumber"
              element={
                <RequireSession>
                  <ShopOrderDetail />
                </RequireSession>
              }
            />
            <Route
              path="/shop/dashboard"
              element={
                <RequireSession>
                  <ShopDashboard />
                </RequireSession>
              }
            />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </div>
      </main>

      <footer className="legal-footer">
        <div className="container legal-footer__inner">
          <span>© {new Date().getFullYear()} Werkstatt-Portal</span>
          <nav className="legal-footer__links" aria-label="Rechtliches">
            <NavLink to="/impressum">Impressum</NavLink>
            <NavLink to="/datenschutz">Datenschutz</NavLink>
          </nav>
        </div>
      </footer>
    </div>
  )
}
