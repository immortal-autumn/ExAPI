<template>
  <section class="space-y-4" aria-labelledby="private-security-title">
    <div class="card p-5">
      <div class="flex items-start gap-3">
        <span class="rounded-lg bg-emerald-100 p-2 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
          <Icon name="shield" size="md" />
        </span>
        <div>
          <h2 id="private-security-title" class="font-semibold text-gray-900 dark:text-white">
            Private operator boundary
          </h2>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
            Operator identity is bound to an allowed WireGuard peer by the dedicated control listener.
            The browser does not use passwords, passkeys, one-time codes, session tokens, OAuth login,
            or CAPTCHA challenges.
          </p>
        </div>
      </div>
    </div>

    <div class="card p-5 text-sm text-gray-600 dark:text-gray-300">
      <h3 class="font-medium text-gray-900 dark:text-white">Request protections</h3>
      <ul class="mt-3 list-disc space-y-1.5 pl-5">
        <li>Every control request carries the explicit ExAPI control marker.</li>
        <li>Host, Origin, Fetch Metadata, and operator peer address are validated server-side.</li>
        <li>Control WebSockets negotiate the <code>exapi-control</code> subprotocol.</li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
</script>
