<template>
  <section class="space-y-4" aria-labelledby="private-security-title">
    <div class="card p-5">
      <div class="flex items-start gap-3">
        <span class="rounded-lg bg-emerald-100 p-2 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
          <Icon name="shield" size="md" />
        </span>
        <div>
          <h2 id="private-security-title" class="font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.security.privateBoundaryTitle') }}
          </h2>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.settings.security.privateBoundaryDescription') }}
          </p>
        </div>
      </div>
    </div>

    <div class="card p-5 text-sm text-gray-600 dark:text-gray-300">
      <h3 class="font-medium text-gray-900 dark:text-white">{{ t('admin.settings.security.requestProtectionsTitle') }}</h3>
      <ul class="mt-3 list-disc space-y-1.5 pl-5">
        <li>{{ t('admin.settings.security.controlMarker') }}</li>
        <li>{{ t('admin.settings.security.requestValidation') }}</li>
        <li>{{ t('admin.settings.security.controlWebSockets') }}</li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
</script>
