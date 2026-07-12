import type { SettingsTab, SettingsTabMeta } from './types'

export const SETTINGS_TABS: SettingsTabMeta[] = [
  { key: 'general', icon: 'home' },
  { key: 'agreement', icon: 'document' },
  { key: 'features', icon: 'bolt' },
  { key: 'security', icon: 'shield' },
  { key: 'gateway', icon: 'server' },
  { key: 'email', icon: 'mail' },
  { key: 'backup', icon: 'database' },
]

export function getNextSettingsTab(
  current: SettingsTab,
  key: string,
): SettingsTab | null {
  const currentIndex = SETTINGS_TABS.findIndex((tab) => tab.key === current)
  if (currentIndex < 0) {
    return null
  }

  if (key === 'Home') {
    return SETTINGS_TABS[0].key
  }
  if (key === 'End') {
    return SETTINGS_TABS[SETTINGS_TABS.length - 1].key
  }

  const direction = key === 'ArrowLeft' || key === 'ArrowUp'
    ? -1
    : key === 'ArrowRight' || key === 'ArrowDown'
      ? 1
      : 0

  if (direction === 0) {
    return null
  }

  const nextIndex = (currentIndex + direction + SETTINGS_TABS.length) % SETTINGS_TABS.length
  return SETTINGS_TABS[nextIndex].key
}
