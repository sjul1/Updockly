<script setup lang="ts">
import type { Component } from "vue";

defineProps<{
  title: string;
  icon?: Component;
}>();
</script>

<template>
  <div
    class="relative overflow-hidden rounded-3xl border border-base-200 bg-gradient-to-r from-primary/10 via-secondary/10 to-accent/10 p-6 shadow-xl"
  >
    <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div class="space-y-2">
        <div v-if="$slots.eyebrow" class="inline-flex items-center gap-2 rounded-full bg-base-100/80 px-3 py-1 text-xs font-semibold shadow">
          <slot name="eyebrow" />
        </div>
        <div class="flex items-center gap-2">
          <component v-if="icon" :is="icon" class="h-6 w-6 text-primary" />
          <h1 class="text-3xl font-bold pr-36 md:pr-0">{{ title }}</h1>
        </div>
        <p v-if="$slots.subtitle" class="text-sm text-base-content/70 max-w-2xl">
          <slot name="subtitle" />
        </p>
        <div v-if="$slots.badges" class="flex flex-wrap gap-3">
          <slot name="badges" />
        </div>
      </div>
      <div
        class="absolute top-6 right-6 flex flex-col items-end gap-2 md:static md:self-start"
      >
        <slot name="meta" />
        <div v-if="$slots.actions" class="flex items-center gap-2">
          <slot name="actions" />
        </div>
      </div>
    </div>
  </div>
</template>
