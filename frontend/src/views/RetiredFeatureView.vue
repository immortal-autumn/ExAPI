<template>
  <main class="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-dark-950">
    <section class="card w-full max-w-xl p-8 text-center" role="status" aria-live="polite">
      <p class="mb-3 text-6xl font-bold text-gray-200 dark:text-dark-700" aria-hidden="true">410</p>
      <h1 class="mb-3 text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('retiredFeature.title') }}
      </h1>
      <p class="mb-6 text-gray-600 dark:text-dark-300">
        {{ t('retiredFeature.description') }}
      </p>
      <router-link to="/admin/dashboard" class="btn btn-primary">
        {{ t('retiredFeature.returnToControlPlane') }}
      </router-link>
    </section>
  </main>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>
