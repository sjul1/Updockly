<script setup lang="ts">
import { ArrowDownUp, Check, ChevronDown, ChevronUp, CircleX, Cpu, MemoryStick, Network, X } from "lucide-vue-next";
import ContainerTableRow from "./ContainerTableRow.vue";
import type { Container } from "../../services/api";

defineProps<{
  containers: Container[];
  filteredContainers: Container[];
  hostInfo: {
    dockerVersion: string;
    platform: string;
    hostname: string;
    lastSeen: string;
    cpu?: number;
    memory?: number;
  };
  loading: boolean;
  lastUpdated: Date | null;
  formatTime: (value: string | number | Date | null | undefined) => string;
  containersCollapsed: boolean;
  filterText: string;
  statusFilter: "all" | "running" | "stopped";
  autoUpdateFilter: "all" | "enabled" | "disabled";
  installing: Record<string, boolean>;
  checkingUpdate: Record<string, boolean>;
  updateAvailableOverrides: Record<string, boolean>;
  autoRefreshEnabled: boolean;
}>();

const emit = defineEmits<{
  (e: "toggle-collapse"): void;
  (e: "open-ports", host: string): void;
  (e: "set-filter-text", value: string): void;
  (e: "set-status-filter", value: "all" | "running" | "stopped"): void;
  (e: "set-auto-filter", value: "all" | "enabled" | "disabled"): void;
  (e: "sort", key: keyof Container): void;
  (e: "refresh"): void;
  (e: "toggle-auto-refresh"): void;
  (e: "check-update", container: Container): void;
  (e: "install", container: Container): void;
  (e: "toggle-auto", container: Container): void;
  (e: "open-quick", container: Container, evt: MouseEvent): void;
}>();

const statusLabel = (val: "all" | "running" | "stopped") => {
  if (val === "running") return "Running";
  if (val === "stopped") return "Stopped";
  return "All";
};

const autoUpdateLabel = (val: "all" | "enabled" | "disabled") => {
  if (val === "enabled") return "Enabled";
  if (val === "disabled") return "Disabled";
  return "All";
};
</script>

<template>
  <div
    class="rounded-2xl border border-base-200 shadow-lg bg-base-100 relative overflow-visible"
  >
    <div
      class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between px-4 py-3 border-b border-base-200 cursor-pointer"
      @click="emit('toggle-collapse')"
    >
      <div class="flex flex-col">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-lg font-semibold">Localhost</span>
          <span class="text-sm text-base-content/60 sm:hidden">
            {{ hostInfo.hostname || "unknown" }}
          </span>
          <span class="inline-flex h-2 w-2 rounded-full bg-success"></span>
        </div>
        <div class="text-xs text-base-content/60 mt-1 hidden sm:block">
          {{ hostInfo.hostname || "unknown" }} ·
          {{ hostInfo.dockerVersion || "unknown" }} ·
          {{ hostInfo.platform || "unknown" }} · Last seen:
          {{ hostInfo.lastSeen ? formatTime(hostInfo.lastSeen) : "never" }}
        </div>
        <div class="text-xs text-base-content/60 mt-1 sm:hidden">
          <div class="flex flex-wrap gap-x-2">
            <div>
              {{ hostInfo.dockerVersion || "unknown" }} ·
              {{ hostInfo.platform || "unknown" }}
            </div>
            <div>
              Last seen:
              {{ hostInfo.lastSeen ? formatTime(hostInfo.lastSeen) : "never" }}
            </div>
          </div>
        </div>
      </div>
      <div class="flex items-center gap-2 mt-2 sm:mt-0 self-start sm:self-center">
        <div
          v-if="hostInfo.cpu !== undefined"
          class="badge badge-ghost gap-1 font-mono text-xs hidden sm:inline-flex"
        >
          <Cpu class="w-3 h-3" /> {{ (hostInfo.cpu as number).toFixed(1) }}%
        </div>
        <div
          v-if="hostInfo.memory !== undefined"
          class="badge badge-ghost gap-1 font-mono text-xs hidden sm:inline-flex"
        >
          <MemoryStick class="w-3 h-3" /> {{ (hostInfo.memory as number).toFixed(1) }}%
        </div>
        <button
          class="btn btn-ghost btn-xs"
          @click.stop="emit('open-ports', hostInfo.hostname || 'Localhost')"
          title="View Port Mapping"
        >
          <Network class="w-4 h-4" />
        </button>
        <span class="badge badge-ghost gap-1">
          {{ containers.length || 0 }} containers
        </span>
        <button class="btn btn-ghost btn-xs" tabindex="-1">
          <ChevronDown v-if="containersCollapsed" class="w-4 h-4" />
          <ChevronUp v-else class="w-4 h-4" />
        </button>
      </div>
    </div>
    <div
      class="transition-all duration-300"
      :style="
        containersCollapsed
          ? 'max-height:0; opacity:0; overflow:hidden'
          : 'max-height:9999px; opacity:1; overflow:visible'
      "
    >
      <div class="px-4 py-3 border-b border-base-200">
        <div class="flex flex-wrap gap-4 items-end">
          <div class="form-control w-full sm:w-auto relative">
            <input
              type="text"
              placeholder="Search by name..."
              class="input input-bordered input-sm rounded-xl w-full sm:min-w-[240px] pr-10"
              :value="filterText"
              @input="emit('set-filter-text', ($event.target as HTMLInputElement).value)"
            />
            <button
              v-if="filterText"
              class="btn btn-ghost btn-xs absolute right-1 top-1/2 -translate-y-1/2"
              aria-label="Clear search"
              @click="emit('set-filter-text', '')"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
          <div class="grid grid-cols-2 gap-2 w-full sm:contents">
            <div class="form-control w-full sm:w-auto">
              <div class="flex items-center gap-2">
                <div class="dropdown dropdown-bottom w-full sm:w-auto">
                  <label
                    tabindex="0"
                    class="btn btn-ghost btn-sm rounded-xl border border-base-300 px-3 w-full sm:w-auto justify-between"
                  >
                    <span class="truncate">Status: {{ statusLabel(statusFilter) }}</span>
                    <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                  </label>
                  <ul
                    tabindex="0"
                    class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 border border-base-200 z-[1]"
                  >
                    <li>
                      <button type="button" @click="emit('set-status-filter', 'all')">
                        <div class="flex items-center gap-2">
                          <Check v-if="statusFilter === 'all'" class="w-4 h-4 text-success" />
                          <span>All Statuses</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="emit('set-status-filter', 'running')">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'running'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Running</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="emit('set-status-filter', 'stopped')">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'stopped'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Stopped</span>
                        </div>
                      </button>
                    </li>
                  </ul>
                </div>
                <button
                  v-if="statusFilter !== 'all'"
                  class="btn btn-ghost btn-xs hidden sm:inline-flex"
                  aria-label="Clear status filter"
                  @click="emit('set-status-filter', 'all')"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>
            <div class="form-control w-full sm:w-auto">
              <div class="flex items-center gap-2">
                <div class="dropdown dropdown-bottom w-full sm:w-auto">
                  <label
                    tabindex="0"
                    class="btn btn-ghost btn-sm rounded-xl border border-base-300 px-3 w-full sm:w-auto justify-between"
                  >
                    <span class="truncate">
                      Auto-update: {{ autoUpdateLabel(autoUpdateFilter) }}
                    </span>
                    <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                  </label>
                  <ul
                    tabindex="0"
                    class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-56 border border-base-200 z-[1]"
                  >
                    <li>
                      <button type="button" @click="emit('set-auto-filter', 'all')">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="autoUpdateFilter === 'all'"
                            class="w-4 h-4 text-success"
                          />
                          <span>All Auto-updates</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="emit('set-auto-filter', 'enabled')">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="autoUpdateFilter === 'enabled'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Enabled</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="emit('set-auto-filter', 'disabled')">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="autoUpdateFilter === 'disabled'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Disabled</span>
                        </div>
                      </button>
                    </li>
                  </ul>
                </div>
                <button
                  v-if="autoUpdateFilter !== 'all'"
                  class="btn btn-ghost btn-xs hidden sm:inline-flex"
                  aria-label="Clear auto-update filter"
                  @click="emit('set-auto-filter', 'all')"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
          <div class="ml-auto text-xs text-base-content/60 w-full sm:w-auto text-right">
            Showing {{ filteredContainers.length }} of {{ containers.length }} records
          </div>
        </div>
      </div>
      <div
        v-if="filteredContainers.length === 0"
        class="px-4 py-8 text-center text-sm text-base-content/70"
      >
        <div class="flex flex-col items-center gap-3">
          <CircleX class="w-12 h-12 text-error" />
          <div class="space-y-1">
            <div class="text-base font-semibold">No Containers Found</div>
            <div>No containers match the current filters.</div>
          </div>
        </div>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table table-fixed w-full">
          <thead>
            <tr>
              <th
                @click="emit('sort', 'State')"
                class="cursor-pointer w-12 sm:w-28 lg:w-54 p-2 sm:p-4"
              >
                <div class="flex items-center justify-center sm:justify-start gap-2">
                  <span class="hidden sm:inline">Status</span>
                  <ArrowDownUp class="w-4 h-4 shrink-0" />
                </div>
              </th>

              <th @click="emit('sort', 'Name')" class="cursor-pointer">
                <div class="flex items-center gap-2">
                  Name <ArrowDownUp class="w-4 h-4 shrink-0" />
                </div>
              </th>

              <th
                @click="emit('sort', 'AutoUpdate')"
                class="cursor-pointer text-center w-14 sm:w-32 p-1"
              >
                <div class="flex items-center justify-center gap-1">
                  <span class="hidden sm:inline">Auto-Update</span>
                  <ArrowDownUp class="w-4 h-4 shrink-0" />
                </div>
              </th>

              <th class="text-right w-[5.5rem] sm:w-36">Actions</th>
            </tr>
          </thead>
          <tbody>
            <ContainerTableRow
              v-for="container in filteredContainers"
              :key="container.ID"
              :container="container"
              :installing="installing"
              :checking-update="checkingUpdate"
              :update-available-overrides="updateAvailableOverrides"
              @check-update="(c) => emit('check-update', c)"
              @install="(c) => emit('install', c)"
              @toggle-auto="(c) => emit('toggle-auto', c)"
              @open-quick="(c, evt) => emit('open-quick', c, evt)"
            />
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
