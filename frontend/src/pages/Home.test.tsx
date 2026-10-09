import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import Home from './Home'

/*
 * The landing page is tested inside a small routing harness: each target page
 * of the customer area is a stub with a recognisable heading, so the tests can
 * prove that every link is wired to the agreed path without depending on the
 * (parallel built) target components themselves.
 */
function renderHome() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/appointment" element={<h1>Terminanfrage</h1>} />
        <Route path="/track" element={<h1>Statusabfrage</h1>} />
        <Route path="/invoice" element={<h1>Rechnung</h1>} />
        <Route path="/shop/login" element={<h1>Werkstatt-Anmeldung</h1>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Home', () => {
  it('renders the landing page header and the two sections', () => {
    renderHome()

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: 'Willkommen im Werkstatt-Portal',
      }),
    ).toBeInTheDocument()

    const anliegen = screen
      .getByRole('heading', { level: 2, name: 'Ihre Anliegen' })
      .closest('section') as HTMLElement
    expect(anliegen).not.toBeNull()

    const werkstatt = screen
      .getByRole('heading', { level: 2, name: 'Werkstattbereich' })
      .closest('section') as HTMLElement
    expect(werkstatt).not.toBeNull()
  })

  it('links each customer function to its route', () => {
    renderHome()

    const anliegen = screen
      .getByRole('heading', { level: 2, name: 'Ihre Anliegen' })
      .closest('section') as HTMLElement

    expect(
      within(anliegen).getByRole('link', { name: /Termin anfragen/ }),
    ).toHaveAttribute('href', '/appointment')
    expect(
      within(anliegen).getByRole('link', { name: /Status abfragen/ }),
    ).toHaveAttribute('href', '/track')
    expect(
      within(anliegen).getByRole('link', { name: /Rechnung/ }),
    ).toHaveAttribute('href', '/invoice')

    const werkstatt = screen
      .getByRole('heading', { level: 2, name: 'Werkstattbereich' })
      .closest('section') as HTMLElement
    expect(
      within(werkstatt).getByRole('link', { name: /Anmeldung/ }),
    ).toHaveAttribute('href', '/shop/login')
  })

  it('navigates to the appointment request', async () => {
    const user = userEvent.setup()
    renderHome()

    await user.click(screen.getByRole('link', { name: /Termin anfragen/ }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Terminanfrage' }),
    ).toBeInTheDocument()
  })

  it('navigates to the status lookup', async () => {
    const user = userEvent.setup()
    renderHome()

    await user.click(screen.getByRole('link', { name: /Status abfragen/ }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Statusabfrage' }),
    ).toBeInTheDocument()
  })

  it('navigates to the invoice view', async () => {
    const user = userEvent.setup()
    renderHome()

    await user.click(screen.getByRole('link', { name: /Rechnung/ }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Rechnung' }),
    ).toBeInTheDocument()
  })

  it('navigates to the workshop login', async () => {
    const user = userEvent.setup()
    renderHome()

    await user.click(screen.getByRole('link', { name: /Anmeldung/ }))
    expect(
      screen.getByRole('heading', { level: 1, name: 'Werkstatt-Anmeldung' }),
    ).toBeInTheDocument()
  })
})
