/**
 * Validation for the customer appointment request form (AC-13).
 *
 * The rules are pure functions so the screen can decide *when* to show a
 * message: a field is only flagged once it was touched or the form was
 * submitted, never on a fresh form. All messages are the German, human
 * readable texts from the design mockup.
 */

export interface AppointmentFormValues {
  name: string
  email: string
  phone: string
  plate: string
  make: string
  model: string
  mileage: string
  desiredDate: string
  problem: string
}

export type AppointmentField = keyof AppointmentFormValues

/** The fields the customer must fill before the request can be sent. */
export const REQUIRED_FIELDS: readonly AppointmentField[] = [
  'name',
  'email',
  'phone',
  'plate',
  'make',
  'model',
  'mileage',
  'desiredDate',
  'problem',
]

export const FIELD_ERRORS: Readonly<Record<AppointmentField, string>> = {
  name: 'Bitte geben Sie Ihren Namen ein.',
  email: 'Bitte geben Sie eine gültige E-Mail-Adresse ein.',
  phone: 'Bitte geben Sie eine Telefonnummer ein.',
  plate: 'Bitte geben Sie das Kennzeichen ein.',
  make: 'Bitte geben Sie die Marke ein.',
  model: 'Bitte geben Sie das Modell ein.',
  mileage: 'Bitte geben Sie den Kilometerstand an.',
  desiredDate: 'Bitte wählen Sie einen Wunschtermin.',
  problem: 'Bitte beschreiben Sie das Problem.',
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const MILEAGE_PATTERN = /^\d+$/

/** True when a value is empty or only whitespace. */
export function isBlank(value: string): boolean {
  return value.trim().length === 0
}

/** A pragmatic e-mail check: something, an @, a domain and a dot. */
export function isValidEmail(value: string): boolean {
  return EMAIL_PATTERN.test(value.trim())
}

/** Mileage is a non-negative whole number of kilometres. */
export function isValidMileage(value: string): boolean {
  const trimmed = value.trim()
  if (trimmed.length === 0) {
    return false
  }
  return MILEAGE_PATTERN.test(trimmed)
}

/** The error message for a single field, or null when it is valid. */
export function validateField(
  field: AppointmentField,
  value: string,
): string | null {
  if (field === 'email') {
    return isValidEmail(value) ? null : FIELD_ERRORS.email
  }
  if (field === 'mileage') {
    return isValidMileage(value) ? null : FIELD_ERRORS.mileage
  }
  return isBlank(value) ? FIELD_ERRORS[field] : null
}

/** Every error of the form keyed by field; an empty object means valid. */
export function validateForm(
  values: AppointmentFormValues,
): Partial<Record<AppointmentField, string>> {
  const errors: Partial<Record<AppointmentField, string>> = {}
  for (const field of REQUIRED_FIELDS) {
    const error = validateField(field, values[field])
    if (error) {
      errors[field] = error
    }
  }
  return errors
}
