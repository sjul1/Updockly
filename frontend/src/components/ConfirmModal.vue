<script setup lang="ts">
import { onMounted, onBeforeUnmount } from "vue";
import { AlertTriangle, Check, X } from "lucide-vue-next";

const props = defineProps<{
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  hideCancel?: boolean;
}>();

const emit = defineEmits<{
  (e: "confirm"): void;
  (e: "cancel"): void;
}>();

const handleBackdrop = (event: MouseEvent) => {
  if (event.target === event.currentTarget) {
    emit("cancel");
  }
};

const confirm = () => emit("confirm");
const cancel = () => emit("cancel");

const handleKeydown = (event: KeyboardEvent) => {
  if (!props.open) return;

  if (event.key === "Enter") {
    event.preventDefault();
    confirm();
  }
  if (event.key === "Escape") {
    event.preventDefault();
    cancel();
  }
};

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", handleKeydown);
});
</script>

<template>
  <teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[200] flex items-center justify-center bg-base-300/60 backdrop-blur-sm px-4"
      @click="handleBackdrop"
      role="presentation"
    >
      <div
        class="relative w-full max-w-lg rounded-2xl border border-base-200/80 bg-gradient-to-br from-base-100 via-base-100 to-base-200/60 shadow-2xl"
        role="dialog"
        :aria-label="title"
      >
        <div class="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary via-secondary to-accent"></div>
        <div class="p-6 space-y-4">
          <div class="flex items-start gap-3">
            <div class="h-11 w-11 rounded-full border border-warning/30 bg-warning/10 text-warning flex items-center justify-center">
              <AlertTriangle class="w-6 h-6" />
            </div>
            <div class="flex-1 space-y-1">
              <h3 class="text-lg font-semibold leading-6 text-base-content">{{ title }}</h3>
              <p class="text-sm text-base-content/70 break-words" style="word-break: break-all;">{{ message }}</p>
            </div>
            <button
              class="btn btn-ghost btn-sm btn-square"
              type="button"
              @click="cancel"
              aria-label="Close dialog"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
          <div class="flex flex-wrap items-center justify-end gap-2 pt-3 border-t border-base-200/70">
            <button
              v-if="!props.hideCancel"
              class="btn btn-ghost gap-2"
              type="button"
              @click="cancel"
            >
              <X class="w-4 h-4" />
              {{ props.cancelLabel || "Cancel" }}
            </button>
            <button class="btn btn-primary gap-2" type="button" @click="confirm">
              <Check class="w-4 h-4" />
              {{ props.confirmLabel || "Confirm" }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </teleport>
</template>
