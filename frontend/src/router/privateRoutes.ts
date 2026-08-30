import type { RouteRecordRaw } from 'vue-router'
import { SINGLE_USER_RETIRED_ROUTES } from '@/config/singleUserProduct'

const retiredCustomerRoutes: RouteRecordRaw[] = SINGLE_USER_RETIRED_ROUTES.map((path, index) => ({
  path,
  name: `RetiredCustomerFeature${index}`,
  component: () => import('@/views/RetiredFeatureView.vue'),
  meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'retiredFeature.title' },
}))

/** The complete route table for the private-only ExAPI operator UI. */
export const privateRoutes: RouteRecordRaw[] = [
  { path: '/', redirect: '/admin/dashboard' },
  { path: '/home', redirect: '/admin/dashboard' },
  { path: '/admin', redirect: '/admin/dashboard' },
  { path: '/keys', redirect: '/admin/api-keys' },
  { path: '/admin/channels', redirect: '/admin/channels/pricing' },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.dashboard.title' },
  },
  {
    path: '/admin/api-keys',
    name: 'AdminAPIKeys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'keys.title' },
  },
  {
    path: '/batch-image',
    name: 'OperatorBatchImage',
    component: () => import('@/views/user/BatchImageGuideView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'batchImage.title' },
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.accounts.title' },
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.groups.title' },
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.proxies.title' },
  },
  {
    path: '/admin/channels/pricing',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.channels.title' },
  },
  {
    path: '/admin/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.channelMonitor.title' },
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.ops.title' },
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.usage.title' },
  },
  {
    path: '/admin/audit-logs',
    name: 'AdminAuditLogs',
    component: () => import('@/views/admin/AuditLogView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.audit.title' },
  },
  {
    path: '/admin/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.riskControl.title', requiresRiskControl: true },
  },
  {
    path: '/admin/prompt-audit',
    name: 'AdminPromptAudit',
    component: () => import('@/features/prompt-audit/PromptAuditView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.promptAudit.title', requiresRiskControl: true },
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/PrivateSettingsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, titleKey: 'admin.settings.title' },
  },
  ...retiredCustomerRoutes,
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
  },
]
