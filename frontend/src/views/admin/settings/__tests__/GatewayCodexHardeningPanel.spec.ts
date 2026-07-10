import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

import GatewayCodexHardeningPanel from '../gateway/GatewayCodexHardeningPanel.vue'

describe('GatewayCodexHardeningPanel', () => {
  it('renders Codex hardening settings and delegates row actions', async () => {
    const addFingerprint = vi.fn()
    const removeFingerprint = vi.fn()
    const addBlacklist = vi.fn()
    const removeBlacklist = vi.fn()
    const addWhitelist = vi.fn()
    const removeWhitelist = vi.fn()

    const wrapper = mount(GatewayCodexHardeningPanel, {
      props: {
        form: {
          min_codex_version: '0.1.0',
          max_codex_version: '1.0.0',
          codex_cli_only_allow_app_server_clients: true,
        },
        codexFingerprintRows: [{ type: 'header_exact', match: 'x-codex', required: false }],
        codexFingerprintNoRequired: true,
        codexBlacklistRows: [{ originator: 'bad', uaContains: 'bot' }],
        codexWhitelistRows: [{ originator: 'good', uaContains: 'cli', skipEngineFingerprint: false }],
        addCodexFingerprintRow: addFingerprint,
        removeCodexFingerprintRow: removeFingerprint,
        addCodexBlacklistRow: addBlacklist,
        removeCodexBlacklistRow: removeBlacklist,
        addCodexWhitelistRow: addWhitelist,
        removeCodexWhitelistRow: removeWhitelist,
      },
      global: {
        stubs: {
          Toggle: { template: '<button type="button">toggle</button>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.codexHardeningTitle')
    expect(wrapper.text()).toContain('admin.settings.gatewayForwarding.codexFingerprintNoRequiredWarn')
    expect(wrapper.find('input[placeholder="admin.settings.gatewayForwarding.minCodexVersionPlaceholder"]').exists()).toBe(true)
    expect(wrapper.find('input[placeholder="admin.settings.gatewayForwarding.maxCodexVersionPlaceholder"]').exists()).toBe(true)

    const addButtons = wrapper.findAll('button').filter((button) => button.text() === 'admin.settings.gatewayForwarding.codexAddRow')
    await addButtons[0].trigger('click')
    await addButtons[1].trigger('click')
    await addButtons[2].trigger('click')

    expect(addFingerprint).toHaveBeenCalledTimes(1)
    expect(addBlacklist).toHaveBeenCalledTimes(1)
    expect(addWhitelist).toHaveBeenCalledTimes(1)
  })
})
