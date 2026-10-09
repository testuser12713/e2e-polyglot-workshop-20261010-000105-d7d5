import { describe, expect, it } from 'vitest'
import {
  isBlank,
  isValidEmail,
  isValidMileage,
  validateField,
  validateForm,
  type AppointmentFormValues,
} from './validation'

const validValues: AppointmentFormValues = {
  name: 'Anna Beispiel',
  email: 'anna@example.com',
  phone: '030 123456',
  plate: 'M-AB 1234',
  make: 'BMW',
  model: '320d',
  mileage: '87450',
  desiredDate: '2025-06-01',
  problem: 'Bremsen quietschen beim Anhalten.',
}

describe('isBlank', () => {
  it('treats whitespace-only input as blank', () => {
    expect(isBlank('')).toBe(true)
    expect(isBlank('   ')).toBe(true)
    expect(isBlank('x')).toBe(false)
  })
})

describe('isValidEmail', () => {
  it('accepts a normal address and rejects a malformed one', () => {
    expect(isValidEmail('anna@example.com')).toBe(true)
    expect(isValidEmail('  anna@example.com ')).toBe(true)
    expect(isValidEmail('keine-adresse')).toBe(false)
    expect(isValidEmail('anna@example')).toBe(false)
    expect(isValidEmail('@example.com')).toBe(false)
  })
})

describe('isValidMileage', () => {
  it('accepts whole non-negative kilometres only', () => {
    expect(isValidMileage('87450')).toBe(true)
    expect(isValidMileage('0')).toBe(true)
    expect(isValidMileage('')).toBe(false)
    expect(isValidMileage('abc')).toBe(false)
    expect(isValidMileage('-5')).toBe(false)
  })
})

describe('validateField', () => {
  it('returns null for a valid value', () => {
    expect(validateField('name', 'Anna')).toBeNull()
    expect(validateField('email', 'anna@example.com')).toBeNull()
  })

  it('returns the German message for an invalid value', () => {
    expect(validateField('name', '   ')).toBe('Bitte geben Sie Ihren Namen ein.')
    expect(validateField('email', 'oops')).toBe(
      'Bitte geben Sie eine gültige E-Mail-Adresse ein.',
    )
  })
})

describe('validateForm', () => {
  it('returns no errors for a fully valid form', () => {
    expect(validateForm(validValues)).toEqual({})
  })

  it('flags every missing required field on an empty form', () => {
    const empty: AppointmentFormValues = {
      name: '',
      email: '',
      phone: '',
      plate: '',
      make: '',
      model: '',
      mileage: '',
      desiredDate: '',
      problem: '',
    }
    const errors = validateForm(empty)
    expect(Object.keys(errors)).toHaveLength(9)
    expect(errors.email).toBe('Bitte geben Sie eine gültige E-Mail-Adresse ein.')
  })

  it('flags a bad e-mail address even when everything else is filled', () => {
    const errors = validateForm({ ...validValues, email: 'keine-adresse' })
    expect(errors).toEqual({
      email: 'Bitte geben Sie eine gültige E-Mail-Adresse ein.',
    })
  })
})
