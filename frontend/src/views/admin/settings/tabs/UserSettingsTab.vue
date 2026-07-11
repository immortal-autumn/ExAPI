<template>
  <div class="space-y-6">
    <UserDefaultSettingsPanel
      :form="form"
      :subscription-groups="subscriptionGroups"
      :default-subscription-group-options="defaultSubscriptionGroupOptions"
      :add-default-subscription="addDefaultSubscription"
      :remove-default-subscription="removeDefaultSubscription"
    />

    <UserAuthSourceDefaultsPanel
      :form="form"
      :auth-source-defaults="authSourceDefaults"
      :auth-source-defaults-meta="authSourceDefaultsMeta"
      :subscription-groups="subscriptionGroups"
      :default-subscription-group-options="defaultSubscriptionGroupOptions"
      :add-auth-source-default-subscription="addAuthSourceDefaultSubscription"
      :remove-auth-source-default-subscription="removeAuthSourceDefaultSubscription"
    />
  </div>
</template>

<script setup lang="ts">
import UserDefaultSettingsPanel from '@/views/admin/settings/users/UserDefaultSettingsPanel.vue'
import UserAuthSourceDefaultsPanel from '@/views/admin/settings/users/UserAuthSourceDefaultsPanel.vue'

type AnyHandler = (...args: any[]) => void

defineProps<{
  form: any
  subscriptionGroups: any[]
  defaultSubscriptionGroupOptions: any[]
  addDefaultSubscription: AnyHandler
  removeDefaultSubscription: AnyHandler
  authSourceDefaults: Record<string, any>
  authSourceDefaultsMeta: Array<{ source: string; title: string; description: string }>
  addAuthSourceDefaultSubscription: AnyHandler
  removeAuthSourceDefaultSubscription: AnyHandler
}>()
</script>
