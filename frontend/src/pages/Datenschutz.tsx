import './LegalPages.css'

/**
 * Privacy page (AC-31).
 *
 * Reachable from the footer link "Datenschutz" on every page. Content follows
 * design/mockups/datenschutz.html: the controller, the processed personal data
 * (customer, vehicle and invoice data), the purpose and legal basis, the fact
 * that no data is shared with third parties and no external resources are
 * loaded, the retention and the data subject rights.
 */
export default function Datenschutz() {
  return (
    <section className="page legal-page">
      <header className="page-header">
        <p className="legal-page__eyebrow">Rechtliches</p>
        <h1>Datenschutzerklärung</h1>
        <p className="page-header__subtitle">
          Informationen zur Verarbeitung personenbezogener Daten nach Art. 13
          DSGVO
        </p>
      </header>

      <div className="card legal-sections">
        <section className="legal-section" aria-labelledby="ds-verantwortlicher">
          <h2 id="ds-verantwortlicher" className="legal-section__title">
            1. Verantwortlicher
          </h2>
          <p>
            Kfz-Werkstatt Berger GmbH, Werkstattstraße 12, 80331 München,
            kontakt@werkstatt-berger.example, Telefon +49 89 1234567.
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="ds-daten">
          <h2 id="ds-daten" className="legal-section__title">
            2. Verarbeitete Daten
          </h2>
          <p>
            Wir verarbeiten Kundendaten (Name, E-Mail-Adresse, Telefonnummer),
            Fahrzeugdaten (Kennzeichen, Marke, Modell, Kilometerstand) sowie
            Auftrags- und Rechnungsdaten, die Sie im Werkstatt-Portal angeben
            oder die bei der Auftragsabwicklung entstehen.
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="ds-zweck">
          <h2 id="ds-zweck" className="legal-section__title">
            3. Zweck und Rechtsgrundlage
          </h2>
          <p>
            Die Verarbeitung erfolgt zur Bearbeitung Ihrer Terminanfrage, zur
            Abwicklung des Werkstattauftrags und zur Rechnungsstellung (Art. 6
            Abs. 1 lit. b DSGVO). Gesetzliche Aufbewahrungspflichten beruhen auf
            Art. 6 Abs. 1 lit. c DSGVO.
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="ds-fremdressourcen">
          <h2 id="ds-fremdressourcen" className="legal-section__title">
            4. Keine Weitergabe, keine Fremdressourcen
          </h2>
          <p>
            Ihre Daten werden nicht an Dritte verkauft. Das Portal lädt keine
            Ressourcen von fremden Hosts: Schriften und Skripte werden aus dem
            eigenen Build ausgeliefert; es werden keine Analyse- oder
            Marketing-Dienste eingebunden.
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="ds-speicherdauer">
          <h2 id="ds-speicherdauer" className="legal-section__title">
            5. Speicherdauer
          </h2>
          <p>
            Auftrags- und Rechnungsdaten bewahren wir im Rahmen der gesetzlichen
            Fristen auf. Anfragen ohne Auftragsergebnis löschen wir, sobald sie
            für die Bearbeitung nicht mehr erforderlich sind.
          </p>
        </section>

        <hr className="legal-divider" />

        <section className="legal-section" aria-labelledby="ds-rechte">
          <h2 id="ds-rechte" className="legal-section__title">
            6. Ihre Rechte
          </h2>
          <p>
            Sie haben das Recht auf Auskunft, Berichtigung, Löschung,
            Einschränkung der Verarbeitung, Datenübertragbarkeit sowie
            Widerspruch. Wenden Sie sich dazu an
            kontakt@werkstatt-berger.example. Ihnen steht ein Beschwerderecht
            bei einer Datenschutzaufsichtsbehörde zu.
          </p>
        </section>
      </div>
    </section>
  )
}
