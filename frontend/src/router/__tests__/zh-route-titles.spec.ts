import { describe, expect, it } from 'vitest'

import router from '../index'

const expectedTitles: Record<string, string> = {
  Setup: '安装向导',
  KeyUsage: '密钥用量查询',
}

describe('Chinese-only route fallback titles', () => {
  it('does not expose English titles when a titleKey is unavailable', () => {
    for (const [name, expected] of Object.entries(expectedTitles)) {
      const route = router.getRoutes().find((item) => item.name === name)
      expect(route?.meta.title, name).toBe(expected)
    }
  })
})
