import { describe, expect, it } from 'vitest'
import { SETTINGS_TABS, getNextSettingsTab } from '../useSettingsTabs'

describe('useSettingsTabs', () => {
  it('keeps the expected settings tab order', () => {
    expect(SETTINGS_TABS.map((tab) => tab.key)).toEqual([
      'general',
      'security',
      'gateway',
      'email',
      'backup',
    ])
    expect(SETTINGS_TABS.map((tab) => tab.key)).not.toContain('users')
    expect(SETTINGS_TABS.map((tab) => tab.key)).not.toContain('payment')
  })

  it('wraps arrow-key navigation across tabs', () => {
    expect(getNextSettingsTab('general', 'ArrowLeft')).toBe('backup')
    expect(getNextSettingsTab('general', 'ArrowUp')).toBe('backup')
    expect(getNextSettingsTab('general', 'ArrowRight')).toBe('security')
    expect(getNextSettingsTab('backup', 'ArrowRight')).toBe('general')
  })

  it('supports Home and End keyboard navigation', () => {
    expect(getNextSettingsTab('gateway', 'Home')).toBe('general')
    expect(getNextSettingsTab('gateway', 'End')).toBe('backup')
  })

  it('ignores unsupported keyboard events', () => {
    expect(getNextSettingsTab('gateway', 'Tab')).toBeNull()
    expect(getNextSettingsTab('gateway', 'Enter')).toBeNull()
  })
})
