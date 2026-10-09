import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import Impressum from './Impressum'

describe('Impressum', () => {
  it('renders the imprint heading and subtitle', () => {
    render(<Impressum />)

    expect(
      screen.getByRole('heading', { level: 1, name: 'Impressum' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Angaben gemäß § 5 TMG')).toBeInTheDocument()
  })

  it('renders every required imprint section', () => {
    render(<Impressum />)

    for (const section of [
      'Anbieter',
      'Vertreten durch',
      'Kontakt',
      'Registereintrag',
      'Verantwortlich für den Inhalt nach § 18 Abs. 2 MStV',
    ]) {
      expect(
        screen.getByRole('heading', { level: 2, name: section }),
      ).toBeInTheDocument()
    }
  })

  it('names the provider and its contact details', () => {
    render(<Impressum />)

    expect(screen.getByText(/Kfz-Werkstatt Berger GmbH/)).toBeInTheDocument()
    expect(screen.getByText(/Geschäftsführer: Thomas Berger/)).toBeInTheDocument()
    expect(
      screen.getByText(/kontakt@werkstatt-berger\.example/),
    ).toBeInTheDocument()
    expect(screen.getByText(/Handelsregister: HRB 123456/)).toBeInTheDocument()
  })
})
