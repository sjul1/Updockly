<script setup lang="ts">
import { computed } from "vue";
import { ArrowDownUp, History as HistoryIcon, Play, ShieldCheck, RefreshCw, PlayCircle, Square, AlertTriangle, TerminalSquare, Server } from "lucide-vue-next";
import type { Container } from "../../services/api";

const props = defineProps<{
  container: Container;
  updateAvailableOverrides: Record<string, boolean>;
  installing: Record<string, boolean>;
  checkingUpdate: Record<string, boolean>;
  logs: Record<string, string>;
  autoRefreshEnabled: boolean;
}>();

const emit = defineEmits<{
  (e: "check-update", id: string): void;
  (e: "update", id: string): void;
  (e: "toggle-auto", id: string, enabled: boolean): void;
  (e: "start", id: string): void;
  (e: "stop", id: string): void;
  (e: "restart", id: string): void;
  (e: "logs", id: string): void;
}>();

const isInstalling = computed(() => props.installing[props.container.ID]);
const isChecking = computed(() => props.checkingUpdate[props.container.ID]);
const updateAvailable = computed(
  () =>
    props.updateAvailableOverrides[props.container.ID] ||
    props.container.UpdateAvailable
);
</script>

<template>
  <div class="rounded-xl border border-base-200 bg-base-100/70 p-4 shadow-sm">
    <div class="flex items-start justify-between gap-2">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
          <span class="inline-flex h-2 w-2 rounded-full" :class="container.State === 'running' ? 'bg-success' : 'bg-error'"></span>
          <h3 class="font-semibold text-base-content">{{ container.Name || container.ID }}</h3>
        </div>
        <p class="text-xs text-base-content/60 font-mono break-all">{{ container.Image }}</p>
        <div class="flex flex-wrap gap-2 text-xs text-base-content/70">
          <span class="badge badge-ghost gap-1">
            <Server class="w-3 h-3" /> {{ container.State }}
          </span>
          <span v-if="container.Status" class="badge badge-ghost gap-1">
            <TerminalSquare class="w-3 h-3" /> {{ container.Status }}
          </span>
          <span v-if="container.Ports?.length" class="badge badge-ghost gap-1">
            <HistoryIcon class="w-3 h-3" /> {{ container.Ports.join(", ") }}
          </span>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span v-if="container.AutoUpdate" class="badge badge-success gap-1 text-xs">
          <ShieldCheck class="w-3 h-3" /> Auto
        </span>
        <span
          v-if="updateAvailable"
          class="badge badge-warning gap-1 text-xs"
        >
          <AlertTriangle class="w-3 h-3" /> Update
        </span>
      </div>
    </div>

    <div class="mt-3 grid grid-cols-2 md:grid-cols-4 gap-2">
      <button
        class="btn btn-sm btn-primary gap-2"
        :disabled="isInstalling || isChecking || !updateAvailable || container.State !== 'running'"
        @click="emit('check-update', container.ID)"
      >
        <RefreshCw class="w-4 h-4" />
        Check
      </button>
      <button
        class="btn btn-sm btn-secondary gap-2"
        :disabled="isInstalling || !updateAvailable"
        @click="emit('update', container.ID)"
      >
        <ArrowDownUp class="w-4 h-4" />
        Update
      </button>
      <button
        class="btn btn-sm btn-ghost gap-2"
        :class="container.AutoUpdate ? 'btn-outline' : ''"
        @click="emit('toggle-auto', container.ID, !container.AutoUpdate)"
      >
        <ShieldCheck class="w-4 h-4" />
        {{ container.AutoUpdate ? "Disable Auto" : "Enable Auto" }}
      </button>
      <button
        class="btn btn-sm btn-ghost gap-2"
        @click="emit('logs', container.ID)"
      >
        <TerminalSquare class="w-4 h-4" />
        Logs
      </button>
      <button
        class="btn btn-sm btn-outline gap-2"
        :disabled="container.State === 'running' || isInstalling"
        @click="emit('start', container.ID)"
      >
        <Play class="w-4 h-4" />
        Start
      </button>
      <button
        class="btn btn-sm btn-outline gap-2"
        :disabled="container.State !== 'running' || isInstalling"
        @click="emit('stop', container.ID)"
      >
        <Square class="w-4 h-4" />
        Stop
      </button>
      <button
        class="btn btn-sm btn-outline gap-2"
        :disabled="container.State !== 'running' || isInstalling"
        @click="emit('restart', container.ID)"
      >
        <RefreshCw class="w-4 h-4" />
        Restart
      </button>
      <button
        class="btn btn-sm btn-outline gap-2"
        :disabled="!logs[container.ID]"
        @click="emit('logs', container.ID)"
      >
        <PlayCircle class="w-4 h-4" />
        View Logs
      </button>
    </div>
  </div>
</template>
