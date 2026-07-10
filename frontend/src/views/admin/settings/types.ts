export type SettingsTab =
  | 'general'
  | 'agreement'
  | 'features'
  | 'security'
  | 'users'
  | 'gateway'
  | 'payment'
  | 'email'
  | 'backup'

export type SettingsTabIcon =
  | 'home'
  | 'document'
  | 'bolt'
  | 'shield'
  | 'user'
  | 'server'
  | 'creditCard'
  | 'mail'
  | 'database'

export interface SettingsTabMeta {
  key: SettingsTab
  icon: SettingsTabIcon
}
