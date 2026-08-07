export type SettingsTab =
  | 'general'
  | 'security'
  | 'gateway'
  | 'email'
  | 'backup'

export type SettingsTabIcon =
  | 'home'
  | 'shield'
  | 'server'
  | 'mail'
  | 'database'

export interface SettingsTabMeta {
  key: SettingsTab
  icon: SettingsTabIcon
}
