<template>
  <span
    v-if="currentVersion"
    class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[10px] text-gray-500 dark:bg-dark-800 dark:text-dark-400"
    :title="`ExAPI ${currentVersion}`"
  >
    v{{ currentVersion }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  version?: string
  disableUpdates?: boolean
}>()

// Private ExAPI is upgraded through the reviewed deployment workflow. The
// browser intentionally exposes no download, restart, or rollback controls.
const currentVersion = computed(() => props.version?.trim() || '')
</script>
