<script setup lang="ts">
import { AlertCircle, Download, MoreVertical, RefreshCw } from "lucide-vue-next";
import type { Agent, AgentContainer } from "../../services/api";

const props = defineProps<{
  agent: Agent;
  container: AgentContainer;
  online: boolean;
  state: string;
  statusText: string;
  installing: boolean;
  checkingUpdate: boolean;
  autoUpdating: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle-auto", agent: Agent, container: AgentContainer): void;
  (e: "install", agent: Agent, container: AgentContainer): void;
  (e: "open-quick", agent: Agent, container: AgentContainer, evt: MouseEvent): void;
}>();

const emitToggleAuto = () => emit("toggle-auto", props.agent, props.container);
const emitInstall = () => emit("install", props.agent, props.container);
const emitOpenQuick = (evt: MouseEvent) =>
  emit("open-quick", props.agent, props.container, evt);
</script>

<template>
  <tr class="hover group">
    <td class="p-2 sm:p-4">
      <div class="flex items-center justify-center sm:justify-start gap-2">
        <RefreshCw
          v-if="installing || checkingUpdate"
          class="w-5 h-5 text-primary animate-spin shrink-0"
          title="Updating"
        />
        <AlertCircle
          v-else-if="online && container?.updateAvailable"
          class="w-5 h-5 text-warning shrink-0"
          title="Update Available"
        />
        <span
          v-else
          class="h-3 w-3 rounded-full shrink-0"
          :class="{
            'bg-success': state === 'running',
            'bg-error': state !== 'running',
          }"
        ></span>

        <span class="font-medium hidden sm:block truncate">
          {{ statusText }}
        </span>
      </div>
    </td>

    <td class="max-w-0 align-middle">
      <div class="flex flex-col justify-center">
        <div
          class="font-mono font-bold truncate text-sm sm:text-base"
          :title="container?.name || container?.id"
        >
          {{ container?.name || container?.id }}
        </div>
        <div
          class="font-mono text-xs text-base-content/50 truncate"
          :title="container?.image"
        >
          {{ container?.image }}
        </div>
      </div>
    </td>

    <td class="text-center p-1">
      <label
        class="toggle text-base-content scale-75 sm:scale-100 origin-center"
        :class="{
          'opacity-50 pointer-events-none': !online,
        }"
      >
        <input
          type="checkbox"
          :checked="container?.autoUpdate"
          :disabled="autoUpdating || !online"
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
          v-if="online && (container?.updateAvailable || installing)"
          class="btn btn-sm btn-info px-2 sm:px-3"
          @click="emitInstall"
          :disabled="installing || !online"
          title="Install Update"
        >
          <Download v-if="!installing" class="w-4 h-4" />
          <RefreshCw v-else class="w-4 h-4 animate-spin" />
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
