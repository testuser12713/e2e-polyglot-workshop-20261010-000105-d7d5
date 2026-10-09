import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useSession } from '../lib/session'

interface RequireSessionProps {
  children: ReactNode
}

/**
 * Route guard for the workshop area (AC-14).
 *
 * Without a valid session every guarded workshop page ends at the login. The
 * page the user wanted is remembered in the navigation state so the login can
 * send them back after a successful sign-in.
 */
export default function RequireSession({ children }: RequireSessionProps) {
  const session = useSession()
  const location = useLocation()

  if (!session) {
    return (
      <Navigate
        to="/shop/login"
        replace
        state={{ from: `${location.pathname}${location.search}` }}
      />
    )
  }

  return <>{children}</>
}
