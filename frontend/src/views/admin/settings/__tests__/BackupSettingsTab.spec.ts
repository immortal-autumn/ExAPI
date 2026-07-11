import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/views/admin/BackupView.vue', () => ({
  default: { template: '<div>backup-settings-view</div>' },
}))

import BackupSettingsTab from '../tabs/BackupSettingsTab.vue'

describe('BackupSettingsTab', () => {
  it('renders the existing backup settings view', () => {
    const wrapper = mount(BackupSettingsTab)

    expect(wrapper.text()).toContain('backup-settings-view')
  })
})
