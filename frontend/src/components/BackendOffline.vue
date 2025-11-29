<script setup lang="ts">
import { WifiOff } from "lucide-vue-next";

const props = withDefaults(
  defineProps<{
    checking?: boolean;
    errorMessage?: string;
  }>(),
  {
    checking: false,
    errorMessage: "",
  }
);

const emit = defineEmits<{
  (e: "retry"): void;
}>();

const handleRetry = () => emit("retry");
</script>

<template>
  <div class="rounded-3xl border border-base-300 bg-base-100 p-10 text-center shadow-xl space-y-6">
    <div class="mx-auto flex size-20 items-center justify-center rounded-full bg-error/10 text-error">
      <WifiOff class="size-10" />
    </div>
    <div class="space-y-2">
      <h2 class="text-2xl font-semibold">Unable to reach the Updockly API</h2>
      <p class="text-base-content/70">
        Updockly can't connect to the server right now. We'll keep retrying,
        but you can also trigger another health check below.
      </p>
    </div>
    <p v-if="props.errorMessage" class="text-sm text-base-content/80 font-mono break-all">
      {{ props.errorMessage }}
    </p>
    <div class="flex flex-col items-center gap-2">
      <button
        class="btn btn-primary"
        :disabled="props.checking"
        @click="handleRetry"
      >
        <span v-if="props.checking" class="loading loading-spinner loading-sm" />
        <span v-else>Retry to connect</span>
      </button>
      <span class="text-xs text-base-content/60">
        We'll refresh automatically once the server responds.
      </span>
    </div>
  </div>
</template>