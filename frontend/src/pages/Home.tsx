/*
 * Customer area landing page ("Kundenbereich").
 *
 * Mirrors the design mockup design/mockups/index.html: the page header, the
 * "Ihre Anliegen" links into the customer functions (appointment request,
 * status lookup, invoice) and the "Werkstattbereich" block with the workshop
 * login. The LegalFooter is rendered by the app shell in App.tsx, so it stays
 * visible on this page as on every other one.
 */
import { Link } from 'react-router-dom'
import './Home.css'

export default function Home() {
  return (
    <section className="page">
      <header className="page-header">
        <p className="home-eyebrow">Kundenbereich</p>
        <h1>Willkommen im Werkstatt-Portal</h1>
        <p className="page-header__subtitle">
          Termin anfragen, Auftragsstatus verfolgen und Rechnungen einsehen —
          alles an einem Ort.
        </p>
      </header>

      <section className="home-section" aria-labelledby="anliegen-title">
        <h2 id="anliegen-title" className="home-section__label">
          Ihre Anliegen
        </h2>
        <div className="home-link-list">
          <Link className="home-link-card" to="/appointment">
            <span className="home-link-card__text">
              <span className="home-link-card__title">Termin anfragen</span>
              <span className="home-link-card__desc">
                Fahrzeugdaten, Wunschtermin und Problembeschreibung erfassen.
              </span>
            </span>
            <span className="home-link-card__arrow" aria-hidden="true">
              →
            </span>
          </Link>

          <Link className="home-link-card" to="/track">
            <span className="home-link-card__text">
              <span className="home-link-card__title">Status abfragen</span>
              <span className="home-link-card__desc">
                Auftragsnummer und Kennzeichen eingeben, aktuellen Status samt
                Verlauf sehen.
              </span>
            </span>
            <span className="home-link-card__arrow" aria-hidden="true">
              →
            </span>
          </Link>

          <Link className="home-link-card" to="/invoice">
            <span className="home-link-card__text">
              <span className="home-link-card__title">Rechnung</span>
              <span className="home-link-card__desc">
                Positionen, Netto, 19 % Mehrwertsteuer und Brutto einsehen.
              </span>
            </span>
            <span className="home-link-card__arrow" aria-hidden="true">
              →
            </span>
          </Link>
        </div>
      </section>

      <section className="home-section" aria-labelledby="werkstatt-title">
        <h2 id="werkstatt-title" className="home-section__label">
          Werkstattbereich
        </h2>
        <div className="home-link-list">
          <Link className="home-link-card" to="/shop/login">
            <span className="home-link-card__text">
              <span className="home-link-card__title">Anmeldung</span>
              <span className="home-link-card__desc">
                Zugang zur Auftragsliste, zum Dashboard und zur
                Auftragsbearbeitung.
              </span>
            </span>
            <span className="home-link-card__arrow" aria-hidden="true">
              →
            </span>
          </Link>
        </div>
      </section>
    </section>
  )
}
