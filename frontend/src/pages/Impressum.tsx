import './LegalPages.css'

/**
 * Imprint page (AC-31).
 *
 * Reachable from the footer link "Impressum" on every page. Content follows
 * design/mockups/impressum.html: the mandatory provider information in the
 * sections Anbieter, Vertreten durch, Kontakt, Registereintrag and
 * "Verantwortlich für den Inhalt nach § 18 Abs. 2 MStV".
 */
export default function Impressum() {
  return (
    <section className="page legal-page">
      <header className="page-header">
        <p className="legal-page__eyebrow">Rechtliches</p>
        <h1>Impressum</h1>
        <p className="page-header__subtitle">Angaben gemäß § 5 TMG</p>
      </header>

      <div className="card legal-sections">
        <section className="legal-section" aria-labelledby="im-anbieter">
          <h2 id="im-anbieter" className="legal-section__title">
            Anbieter
          </h2>
          <p>
            Kfz-Werkstatt Berger GmbH
            <br />
            Werkstattstraße 12
            <br />
            80331 München
            <br />
            Deutschland
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="im-vertreten">
          <h2 id="im-vertreten" className="legal-section__title">
            Vertreten durch
          </h2>
          <p>Geschäftsführer: Thomas Berger</p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="im-kontakt">
          <h2 id="im-kontakt" className="legal-section__title">
            Kontakt
          </h2>
          <p>
            Telefon: +49 89 1234567
            <br />
            E-Mail: kontakt@werkstatt-berger.example
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="im-register">
          <h2 id="im-register" className="legal-section__title">
            Registereintrag
          </h2>
          <p>
            Handelsregister: HRB 123456
            <br />
            Registergericht: Amtsgericht München
            <br />
            Umsatzsteuer-Identifikationsnummer gemäß § 27a UStG:
            DE123456789
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="im-verantwortlich">
          <h2 id="im-verantwortlich" className="legal-section__title">
            Verantwortlich für den Inhalt nach § 18 Abs. 2 MStV
          </h2>
          <p>Thomas Berger, Anschrift wie oben</p>
        </section>
      </div>
    </section>
  )
}
