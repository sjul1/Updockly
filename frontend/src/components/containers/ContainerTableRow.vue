<script setup lang="ts">
import { AlertCircle, MoreVertical, RefreshCw } from "lucide-vue-next";
import type { Container } from "../../services/api";

const props = defineProps<{
  container: Container;
  installing: Record<string, boolean>;
  checkingUpdate: Record<string, boolean>;
  updateAvailableOverrides: Record<string, boolean>;
}>();

const emit = defineEmits<{
  (e: "check-update", container: Container): void;
  (e: "install", container: Container): void;
  (e: "toggle-auto", container: Container): void;
  (e: "open-quick", container: Container, evt: MouseEvent): void;
}>();

const isInstalling = (id: string) => props.installing[id];
const isChecking = (id: string) => props.checkingUpdate[id];
const updateAvailable = (c: Container) =>
  props.updateAvailableOverrides[c.ID] || c.UpdateAvailable;
const emitToggleAuto = () => emit("toggle-auto", props.container);
const emitInstall = () => emit("install", props.container);
const emitOpenQuick = (evt: MouseEvent) =>
  emit("open-quick", props.container, evt);
const statusText = (c: Container) =>
  c.Status || c.State || (c.State === "running" ? "running" : "unknown");
</script>

<template>
  <tr class="hover group">
    <td class="p-2 sm:p-4">
      <div class="flex items-center justify-center sm:justify-start gap-2">
        <RefreshCw
          v-if="isInstalling(container.ID) || isChecking(container.ID)"
          class="w-5 h-5 text-primary animate-spin shrink-0"
          title="Updating"
        />
        <AlertCircle
          v-else-if="updateAvailable(container)"
          class="w-5 h-5 text-warning shrink-0"
          title="Update available"
        />
        <span
          v-else
          class="h-3 w-3 rounded-full shrink-0"
          :class="container.State === 'running' ? 'bg-success' : 'bg-error'"
        ></span>

        <span class="font-medium hidden sm:block truncate">
          {{ statusText(container) }}
        </span>
      </div>
    </td>

    <td class="p-2 sm:p-4">
      <div class="flex flex-col justify-center">
        <div
          class="font-mono font-bold truncate text-sm sm:text-base"
          :title="container.Name || container.ID"
        >
          {{ container.Name || container.ID }}
        </div>
        <div
          class="font-mono text-xs text-base-content/50 truncate"
          :title="container.Image"
        >
          {{ container.Image }}
        </div>
      </div>
    </td>

    <td class="text-center p-1">
      <label
        class="toggle text-base-content scale-75 sm:scale-100 origin-center"
      >
        <input
          type="checkbox"
          :checked="container.AutoUpdate"
          @change="emitToggleAuto"
        />
        <svg
          aria-label="disabled"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="4"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M18 6 6 18" />
          <path d="m6 6 12 12" />
        </svg>
        <svg
          aria-label="enabled"
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
        >
          <g
            stroke-linejoin="round"
            stroke-linecap="round"
            stroke-width="4"
            fill="none"
            stroke="currentColor"
            class="text-success"
          >
            <path d="M20 6 9 17l-5-5"></path>
          </g>
        </svg>
      </label>
    </td>

    <td class="text-right p-2 sm:p-4">
      <div class="flex justify-end items-center gap-1 relative">
        <button
          v-if="updateAvailable(container) || isInstalling(container.ID)"
          class="btn btn-sm btn-info px-2 sm:px-3"
          @click="emitInstall"
          :disabled="isInstalling(container.ID)"
          title="Install Update"
        >
          <RefreshCw
            v-if="isInstalling(container.ID)"
            class="w-4 h-4 animate-spin"
          />
          <RefreshCw v-else class="w-4 h-4" />
          <span class="hidden sm:inline ml-1">Install</span>
        </button>
        <div class="relative" style="z-index: 60">
          <button
            class="btn btn-ghost btn-sm btn-square"
            aria-label="Quick actions"
            @click="emitOpenQuick"
          >
            <MoreVertical class="w-4 h-4" />
          </button>
        </div>
      </div>
    </td>
  </tr>
</template>
