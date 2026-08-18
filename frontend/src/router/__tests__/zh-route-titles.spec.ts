import { describe, expect, it } from 'vitest'

import router from '../index'

describe('private route titles', () => {
  it('uses localized title keys instead of legacy English fallback titles', () => {
    const routes = router.getRoutes().filter((route) => route.name && route.name !== 'NotFound')

    expect(routes.map((route) => route.name)).not.toContain('Setup')
    expect(routes.map((route) => route.name)).not.toContain('KeyUsage')
    for (const route of routes) {
      expect(route.meta.titleKey, String(route.name)).toEqual(expect.any(String))
      expect(route.meta.title, String(route.name)).toBeUndefined()
    }
  })
})
