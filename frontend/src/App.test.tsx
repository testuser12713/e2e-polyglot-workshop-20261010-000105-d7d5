import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import App from './App'

function renderApp(initialPath = '/appointment') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <App />
    </MemoryRouter>,
  )
}

describe('App shell', () => {
  it('renders the wordmark, the area navigation and the legal footer', () => {
    renderApp()

    expect(
      screen.getByRole('link', { name: 'Werkstatt-Portal' }),
    ).toHaveAttribute('href', '/appointment')

    const mainNav = screen.getByRole('navigation', { name: 'Hauptnavigation' })
    expect(within(mainNav).getByRole('link', { name: 'Kundenbereich' })).toBeInTheDocument()
    expect(
      within(mainNav).getByRole('link', { name: 'Werkstattbereich' }),
    ).toBeInTheDocument()

    const legalNav = screen.getByRole('navigation', { name: 'Rechtliches' })
    expect(within(legalNav).getByRole('link', { name: 'Impressum' })).toHaveAttribute(
      'href',
      '/impressum',
    )
    expect(
      within(legalNav).getByRole('link', { name: 'Datenschutz' }),
    ).toHaveAttribute('href', '/datenschutz')
  })

  it('starts on the appointment request page', () => {
    renderApp()
    expect(
      screen.getByRole('heading', { level: 1, name: 'Terminanfrage' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Werkstatt-Portal' }),
    ).toHaveAttribute('aria-current', 'page')
  })

  it('uses the header brand to reach the start page from another page', async () => {
    const user = userEvent.setup()
    renderApp('/track')

    expect(
      screen.getByRole('heading', { level: 1, name: 'Status abfragen' }),
    ).toBeInTheDocument()

    const brand = screen.getByRole('link', { name: 'Werkstatt-Portal' })
    await user.click(brand)

    expect(
      screen.getByRole('heading', { level: 1, name: 'Terminanfrage' }),
    ).toBeInTheDocument()
  })

  it('switches the public stub pages and sends unauthenticated workshop links to the login', async () => {
    const user = userEvent.setup()
    renderApp('/appointment')

    await user.click(screen.getByRole('link', { name: 'Status abfragen' }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Status abfragen' }),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('link', { name: 'Werkstattbereich' }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung' }),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('link', { name: 'Dashboard' }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung' }),
    ).toBeInTheDocument()

    await user.click(screen.getByRole('link', { name: 'Anmeldung' }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Anmeldung' }),
    ).toBeInTheDocument()
  })

  it('reaches both legal pages from every page', async () => {
    const user = userEvent.setup()

    for (const path of ['/appointment', '/shop/orders']) {
      const { unmount } = renderApp(path)

      const legalNav = screen.getByRole('navigation', { name: 'Rechtliches' })
      await user.click(within(legalNav).getByRole('link', { name: 'Impressum' }))
      expect(
        screen.getByRole('heading', { level: 1, name: 'Impressum' }),
      ).toBeInTheDocument()

      await user.click(within(legalNav).getByRole('link', { name: 'Datenschutz' }))
      expect(
        screen.getByRole('heading', { level: 1, name: 'Datenschutz' }),
      ).toBeInTheDocument()

      unmount()
    }
  })

  it('toggles the mobile menu button', async () => {
    const user = userEvent.setup()
    renderApp()

    const toggle = screen.getByRole('button', { name: 'Menü' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')

    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
  })

  it('redirects an unknown path to a readable not-found page', () => {
    renderApp('/does-not-exist')
    expect(
      screen.getByRole('heading', { level: 1, name: 'Seite nicht gefunden' }),
    ).toBeInTheDocument()
  })
})
