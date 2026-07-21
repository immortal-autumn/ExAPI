<template>
  <div class="flex min-h-[40vh] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
    正在跳转到私有网关控制台…
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { singleUserGatewayRedirectPath } from '@/router/singleUserGatewayMode'

const router = useRouter()
const authStore = useAuthStore()

onMounted(() => {
  void router.replace(singleUserGatewayRedirectPath(authStore.isAdmin))
})
</script>
