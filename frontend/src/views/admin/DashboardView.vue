<template>
  <AppLayout>
    <div class="space-y-6">
      <!--
        ExAPI is a peer-authenticated, single-user control plane. The former
        customer analytics dashboard is intentionally not part of this product
        surface; keeping it out of the route module prevents dormant chart.js
        and vue-chartjs chunks from being emitted for the private dashboard.
      -->
      <SingleUserCockpitPanel v-if="privateGatewayControlPlane" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import SingleUserCockpitPanel from './components/SingleUserCockpitPanel.vue'
import { isSingleUserPrivateControlPlaneBrowser } from '@/router/singleUserGatewayMode'

const privateGatewayControlPlane = isSingleUserPrivateControlPlaneBrowser()

// Keep the explicit mode check as a guard for future product variants. A
// private deployment must never initialize customer analytics providers.
onMounted(() => {
  if (privateGatewayControlPlane) return
})
</script>

<style scoped>
</style>
