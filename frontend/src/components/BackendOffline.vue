<script setup lang="ts">
import { WifiOff } from "lucide-vue-next";
import { computed, onMounted, ref } from "vue";

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

const cleanErrorMessage = computed(() => {
  if (!props.errorMessage) return "";
  if (props.errorMessage.includes("<html")) {
    const match = props.errorMessage.match(/<h1>(.*?)<\/h1>/i);
    if (match && match[1]) {
      return match[1];
    }
    const titleMatch = props.errorMessage.match(/<title>(.*?)<\/title>/i);
    if (titleMatch && titleMatch[1]) {
      return titleMatch[1];
    }
    return "502 Bad Gateway"; // Fallback for common nginx error
  }
  return props.errorMessage;
});

const handleRetry = () => emit("retry");

const videoSrc = ref("");

onMounted(() => {
  videoSrc.value = "/robot.mp4";
});
</script>

<template>
  <div
    class="rounded-3xl border border-base-300 bg-base-100 p-10 text-center shadow-xl space-y-6"
  >
    <div
      class="mx-auto flex size-20 items-center justify-center rounded-full bg-error/10 text-error"
    >
      <WifiOff class="size-10" />
    </div>
    <div class="space-y-2">
      <h2 class="text-2xl font-semibold">Unable to reach the Updockly API</h2>
      <p class="text-base-content/70">
        Updockly can't connect to the server right now. We'll keep retrying, but
        you can also trigger another health check below.
      </p>
    </div>
    <video
      :src="videoSrc"
      autoplay
      loop
      muted
      playsinline
      preload="none"
      class="w-48 mx-auto rounded-2xl mb-4"
    ></video>
    <p
      v-if="cleanErrorMessage"
      class="text-sm text-base-content/80 font-mono break-all"
    >
      {{ cleanErrorMessage }}
    </p>
    <div class="flex flex-col items-center gap-2">
      <button
        class="btn btn-primary"
        :disabled="props.checking"
        @click="handleRetry"
      >
        <span
          v-if="props.checking"
          class="loading loading-spinner loading-sm"
        />
        <span v-else>Retry to connect</span>
      </button>
      <span class="text-xs text-base-content/60">
        We'll refresh automatically once the server responds.
      </span>
    </div>
  </div>
</template>
