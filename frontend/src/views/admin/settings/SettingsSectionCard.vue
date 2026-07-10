<template>
  <section class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        <slot name="title" />
      </h2>
      <p
        v-if="$slots.description"
        data-test="section-description"
        class="mt-1 text-sm text-gray-500 dark:text-gray-400"
      >
        <slot name="description" />
      </p>
    </div>
    <div class="space-y-4 p-6">
      <slot />
    </div>
  </section>
</template>
