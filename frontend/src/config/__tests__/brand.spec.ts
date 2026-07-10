import { describe, expect, it } from 'vitest'
import { BRAND, getDefaultPaymentProductPrefix, getDefaultSiteName } from '../brand'

describe('brand defaults', () => {
  it('uses ExAPI as the product name', () => {
    expect(BRAND.productName).toBe('ExAPI')
    expect(getDefaultSiteName()).toBe('ExAPI')
  })

  it('uses ExAPI for payment product prefixes', () => {
    expect(getDefaultPaymentProductPrefix()).toBe('ExAPI')
  })
})
