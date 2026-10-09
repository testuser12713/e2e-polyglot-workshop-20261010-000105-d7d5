import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import Datenschutz from './Datenschutz'

describe('Datenschutz', () => {
  it('renders the privacy heading and subtitle', () => {
    render(<Datenschutz />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'Datenschutzerklärung' }),
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        /Informationen zur Verarbeitung personenbezogener Daten nach Art\. 13 DSGVO/,
      ),
    ).toBeInTheDocument()
  })

  it('renders every required privacy section', () => {
    render(<Datenschutz />)

    for (const section of [
      '1. Verantwortlicher',
      '2. Verarbeitete Daten',
      '3. Zweck und Rechtsgrundlage',
      '4. Keine Weitergabe, keine Fremdressourcen',
      '5. Speicherdauer',
      '6. Ihre Rechte',
    ]) {
      expect(
        screen.getByRole('heading', { level: 2, name: section }),
      ).toBeInTheDocument()
    }
  })

  it('names the stored personal data with purpose and retention', () => {
    render(<Datenschutz />)

    expect(screen.getByText(/Name, E-Mail-Adresse, Telefonnummer/)).toBeInTheDocument()
    expect(
      screen.getByText(/Kennzeichen, Marke, Modell, Kilometerstand/),
    ).toBeInTheDocument()
    expect(
      screen.getAllByText(/Auftrags- und Rechnungsdaten/).length,
    ).toBeGreaterThan(0)
    expect(screen.getByText(/Rechnungsstellung/)).toBeInTheDocument()
    expect(screen.getByText(/gesetzlichen Fristen/)).toBeInTheDocument()
  })
})
