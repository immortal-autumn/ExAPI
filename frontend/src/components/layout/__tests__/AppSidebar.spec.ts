// @vitest-environment node

import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar single-user gateway navigation', () => {
  it('filters multi-user and payment management routes through the shared restriction helper', () => {
    expect(componentSource).toContain("isSingleUserPrivateControlPlaneBrowser")
    expect(componentSource).toContain('!isSingleUserGatewayRestrictedPath(item.path)')
    expect(componentSource).toContain('!isSingleUserGatewayRestrictedPath(child.path)')
  })

  it('uses the centralized private-control-plane host helper', () => {
    expect(componentSource).toContain('authStore.isSimpleMode || isSingleUserPrivateControlPlaneBrowser()')
    expect(componentSource).not.toContain("window.location.hostname\n  return host === '100.97.17.1'")
  })

  it('does not add custom-page links to the private route matrix', () => {
    const privateBranch = componentSource.slice(
      componentSource.indexOf('if (singleUserPrivateControlPlane.value)'),
      componentSource.indexOf("visible.push({ path: '/admin/settings'"),
    )
    expect(privateBranch).not.toContain('customMenuItemsForAdmin')
    expect(privateBranch).not.toContain('/custom/')
  })

  it('fetches admin settings through one lifecycle path', () => {
    expect(componentSource.match(/adminSettingsStore\.fetch\(\)/g)).toHaveLength(1)
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})
