<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  RefreshCw,
  Sparkles,
  Clock3,
  History as HistoryIcon,
  AlertCircle,
  CheckCircle2,
  Server,
  User,
  Filter,
  RotateCcw,
  Trash2,
  ChevronDown,
  Check,
  X,
  HelpCircle,
  AlertTriangle,
} from "lucide-vue-next";
import { ApiError, api, type UpdateHistory } from "../services/api";
import { useToast } from "vue-toastification";
import ConfirmModal from "./ConfirmModal.vue";

type StatusFilter = "all" | "success" | "warning" | "error";
type SourceFilter = "all" | "local" | "agent";

const toast = useToast();
const formatTime = inject< (value: string | number | Date | null | undefined) => string >(
  "formatAppTime",
  (value) => {
    if (!value) return "--:--:--";
    const date = value instanceof Date ? value : new Date(value);
    return date.toLocaleTimeString();
  }
);
const formatDateTime = inject<
  (value: string | number | Date | null | undefined) => string
>("formatAppDateTime", (value) => {
  if (!value) return "";
  const date = value instanceof Date ? value : new Date(value);
  return date.toLocaleString();
});
const entries = ref<UpdateHistory[]>([]);
const loading = ref(true);
const lastUpdated = ref<Date | null>(null);
const limit = ref(200);
const filterText = ref("");
const statusFilter = ref<StatusFilter>("all");
const sourceFilter = ref<SourceFilter>("all");
const rollingBack = ref<Record<string, boolean>>({});
const refreshTimer = ref<number | null>(null);
const autoRefreshEnabled = ref(true);
const expanded = ref<Record<string, boolean>>({});
const REFRESH_INTERVAL = 30000;
const rollbackModalOpen = ref(false);
const pendingRollback = ref<UpdateHistory | null>(null);
const deleteModalOpen = ref(false);
const pendingDelete = ref<UpdateHistory | null>(null);
const deleting = ref<Record<string, boolean>>({});

const isAutoUpdateEntry = (entry: UpdateHistory) =>
  entry.message === "Auto-update done";
const isInfoEntry = (entry: UpdateHistory) => entry.status === "info";

const stats = computed(() => {
  const total = entries.value.length;
  const success = entries.value.filter((e) => e.status === "success").length;
  const warnings = entries.value.filter((e) => e.status === "warning").length;
  const failed = entries.value.filter((e) => e.status === "error").length;
  const agent = entries.value.filter((e) => e.source === "agent").length;
  const local = entries.value.filter((e) => e.source === "local").length;
  return { total, success, warnings, failed, agent, local };
});

const rollbackImageMap = computed<Record<string, string>>(() => {
  const byEntry: Record<string, string> = {};
  const previousImageByContainer: Record<string, string> = {};
  const chronological = [...entries.value].sort(
    (a, b) =>
      new Date(a.createdAt || 0).getTime() -
      new Date(b.createdAt || 0).getTime(),
  );

  for (const entry of chronological) {
    const containerKey = entry.containerName || entry.containerId;
    if (!containerKey) {
      continue;
    }

    const previous = previousImageByContainer[containerKey];
    if (previous) {
      byEntry[entry.id] = previous;
    }

    const candidate =
      entry.imageDigest?.trim() || entry.image?.trim();
    if (candidate) {
      previousImageByContainer[containerKey] = candidate;
    }
  }

  return byEntry;
});

const rollbackImageForEntry = (entry: UpdateHistory) => {
  if (isAutoUpdateEntry(entry)) {
    return "Auto-update done";
  }
  if (isInfoEntry(entry)) {
    return "Auto-update done";
  }
  return (
    rollbackImageMap.value[entry.id]?.trim() ||
    entry.imageDigest?.trim() ||
    entry.image?.trim() ||
    "unknown"
  );
};

const fetchHistory = async (options?: { silent?: boolean } | Event) => {
  const silent =
    !!options &&
    typeof options === "object" &&
    "silent" in options &&
    (options as { silent?: boolean }).silent === true;
  if (!silent) {
    loading.value = true;
  }
  try {
    entries.value = await api.getUpdateHistory(limit.value);
    lastUpdated.value = new Date();
  } catch (error) {
    console.error("Failed to load history:", error);
    toast.error("Unable to load history.");
  } finally {
    if (!silent) {
      loading.value = false;
    }
  }
};

const filteredEntries = computed(() => {
  const text = filterText.value.toLowerCase();
  let list = [...entries.value];

  if (text) {
    list = list.filter((entry) => {
      const rollbackImage = rollbackImageForEntry(entry).toLowerCase();
      return (
        entry.containerName.toLowerCase().includes(text) ||
        entry.image.toLowerCase().includes(text) ||
        rollbackImage.includes(text) ||
        (entry.agentName || "").toLowerCase().includes(text) ||
        entry.message.toLowerCase().includes(text)
      );
    });
  }

  if (statusFilter.value !== "all") {
    list = list.filter((entry) => entry.status === statusFilter.value);
  }

  if (sourceFilter.value !== "all") {
    list = list.filter((entry) => entry.source === sourceFilter.value);
  }

  return list;
});

const statusBadge = (status: string) => {
  if (status === "success") return "badge-success";
  if (status === "warning") return "badge-warning";
  if (status === "error") return "badge-error";
  if (status === "info") return "badge-info";
  return "badge-ghost";
};

const sourceLabel = (entry: UpdateHistory) => {
  if (isAutoUpdateEntry(entry) || isInfoEntry(entry)) {
    return "";
  }
  if (entry.source === "agent") {
    return entry.agentName || "Agent";
  }
  return "Local";
};

const formatDate = (value: string) => formatDateTime(value);

const rollbackMessage = computed(() => {
  const entry = pendingRollback.value;
  if (!entry) return "";
  const image = rollbackImageForEntry(entry) || "unknown image";
  const containerRef =
    entry.containerName || entry.containerId || "this container";
  const agentSuffix =
    entry.source === "agent" ? ` via ${entry.agentName || "agent"}` : "";
  return `Rollback ${containerRef}${agentSuffix} to ${image}? This will recreate the container with that image.`;
});

const openRollbackModal = (entry: UpdateHistory) => {
  pendingRollback.value = entry;
  rollbackModalOpen.value = true;
};

const closeRollbackModal = () => {
  rollbackModalOpen.value = false;
  pendingRollback.value = null;
};

const deleteMessage = computed(() => {
  const entry = pendingDelete.value;
  if (!entry) return "";
  const containerRef =
    entry.containerName || entry.containerId || "this container";
  return `Delete history entry for ${containerRef}? This will remove it from the list.`;
});

const openDeleteModal = (entry: UpdateHistory) => {
  pendingDelete.value = entry;
  deleteModalOpen.value = true;
};

const closeDeleteModal = () => {
  deleteModalOpen.value = false;
  pendingDelete.value = null;
};

const rollbackToEntry = async () => {
  const entry = pendingRollback.value;
  if (!entry) return;
  const image = rollbackImageForEntry(entry);
  const containerId = entry.containerId;
  const containerName = entry.containerName;
  const containerRef = containerName || containerId;

  const stopWithError = (message: string) => {
    toast.error(message);
    closeRollbackModal();
  };

  if (!image || image === "unknown") {
    stopWithError("No image recorded for this point in history.");
    return;
  }

  if (entry.source === "agent") {
    if (!entry.agentId) {
      stopWithError("Agent reference missing for this record.");
      return;
    }
    if (!containerId) {
      stopWithError("Container ID missing for this record.");
      return;
    }
  } else if (!containerRef) {
    stopWithError("Container reference missing for this record.");
    return;
  }

  rollingBack.value = { ...rollingBack.value, [entry.id]: true };
  try {
    if (entry.source === "agent" && entry.agentId && containerId) {
      await api.rollbackAgentContainer(entry.agentId, containerId, image, entry.id);
      toast.success(
        `Requested rollback of ${containerName || containerId} via ${
          entry.agentName || "agent"
        }`
      );
    } else if (containerRef) {
      await api.rollbackContainer(containerRef, image, entry.id);
      toast.success(`Rolling back ${containerRef} to ${image}`);
    }
    await fetchHistory();
    closeRollbackModal();
  } catch (error) {
    let message = "Rollback failed.";
    if (error instanceof ApiError) {
      message = error.message;
    } else if (error instanceof Error) {
      message = error.message;
    }
    toast.error(message);
  } finally {
    rollingBack.value = { ...rollingBack.value, [entry.id]: false };
  }
};

const deleteHistoryEntry = async () => {
  const entry = pendingDelete.value;
  if (!entry) return;
  deleting.value = { ...deleting.value, [entry.id]: true };
  try {
    await api.deleteHistoryEntry(entry.id);
    toast.success("History entry deleted.");
    await fetchHistory({ silent: true });
    closeDeleteModal();
  } catch (error) {
    let message = "Failed to delete history entry.";
    if (error instanceof ApiError) {
      message = error.message;
    } else if (error instanceof Error) {
      message = error.message;
    }
    toast.error(message);
  } finally {
    deleting.value = { ...deleting.value, [entry.id]: false };
  }
};

const toggleAutoRefresh = () => {
  autoRefreshEnabled.value = !autoRefreshEnabled.value;
};

const startAutoRefresh = () => {
  if (!autoRefreshEnabled.value) return;
  stopAutoRefresh();
  refreshTimer.value = window.setInterval(() => {
    void fetchHistory({ silent: true });
  }, REFRESH_INTERVAL);
};

const stopAutoRefresh = () => {
  if (refreshTimer.value) {
    clearInterval(refreshTimer.value);
    refreshTimer.value = null;
  }
};

const toggleExpanded = (entry: UpdateHistory, event?: MouseEvent) => {
  if (event) {
    const target = event.target as HTMLElement;
    if (
      target?.closest("button, a, input, select, textarea, [role='button']")
    ) {
      return;
    }
  }
  expanded.value = {
    ...expanded.value,
    [entry.id]: !expanded.value[entry.id],
  };
};

watch(limit, () => {
  void fetchHistory();
});

onMounted(() => {
  void fetchHistory();
  startAutoRefresh();
});

watch(autoRefreshEnabled, (enabled) => {
  if (enabled) {
    startAutoRefresh();
  } else {
    stopAutoRefresh();
  }
});

onBeforeUnmount(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="space-y-6 w-full">
    <div
      class="relative overflow-hidden rounded-3xl border border-base-200 bg-gradient-to-r from-secondary/10 via-primary/10 to-accent/10 p-6 shadow-xl"
    >
      <div
        class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between"
      >
        <div class="space-y-2">
          <div
            class="inline-flex items-center gap-2 rounded-full bg-base-100/80 px-3 py-1 text-xs font-semibold shadow"
          >
            <Sparkles class="h-4 w-4 text-secondary" />
            Update history
          </div>
          <h1 class="text-3xl font-bold flex items-center gap-2 pr-36 md:pr-0">
            <HistoryIcon class="w-7 h-7 text-primary" />
            History
          </h1>
          <p class="text-sm text-base-content/70 max-w-2xl">
            Review every container update across your fleet. Filter by source,
            status, or search by name.
          </p>
          <div class="flex flex-wrap gap-3">
            <div class="badge badge-success gap-2">
              <CheckCircle2 class="h-3.5 w-3.5" /> Success: {{ stats.success }}
            </div>
            <div class="badge badge-warning gap-2">
              <AlertCircle class="h-3.5 w-3.5" /> Warnings: {{ stats.warnings }}
            </div>
            <div class="badge badge-error gap-2">
              <AlertCircle class="h-3.5 w-3.5" /> Failed: {{ stats.failed }}
            </div>
            <div class="badge badge-primary gap-2">
              <Server class="h-3.5 w-3.5" /> Agents: {{ stats.agent }}
            </div>
            <div class="badge badge-ghost gap-2">
              <User class="h-3.5 w-3.5" /> Local: {{ stats.local }}
            </div>
          </div>
        </div>
        <div class="absolute top-6 right-6 flex flex-col items-end gap-2 md:static md:self-start">
          <span class="text-xs text-base-content/60">
            {{
              lastUpdated
                ? `Updated ${formatTime(lastUpdated)}`
                : "Updated --:--:--"
            }}
          </span>
          <div class="flex items-center gap-2">
            <button
              class="btn btn-ghost btn-square"
              @click="fetchHistory"
              :disabled="loading"
              aria-label="Refresh history"
              title="Refresh history"
            >
              <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
            </button>
            <button
              type="button"
              class="badge gap-2 border border-primary/40 cursor-pointer"
              :class="
                autoRefreshEnabled
                  ? 'badge-primary text-primary-content'
                  : 'badge-ghost text-base-content'
              "
              @click="toggleAutoRefresh"
              :aria-pressed="autoRefreshEnabled"
              :title="
                autoRefreshEnabled
                  ? 'Click to pause auto-refresh'
                  : 'Click to resume auto-refresh'
              "
            >
              <Sparkles class="h-3.5 w-3.5" />
              {{ autoRefreshEnabled ? "Live" : "Paused" }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="rounded-2xl border border-base-200 shadow-lg bg-base-100">
      <div
        class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between px-4 py-3 border-b border-base-200"
      >
        <div class="flex flex-wrap items-end gap-3 w-full md:w-auto">
          <div class="form-control w-full sm:w-auto relative">
            <input
              type="text"
              placeholder="Search container, agent, or message..."
              class="input input-bordered input-sm rounded-xl w-full sm:min-w-[260px] pr-10"
              v-model="filterText"
            />
            <Filter
              class="w-4 h-4 absolute right-3 top-1/2 -translate-y-1/2 text-base-content/60"
            />
          </div>
          <div class="grid grid-cols-3 gap-2 w-full sm:contents">
            <div class="form-control w-full sm:w-auto">
              <div class="flex items-center gap-2">
                <div class="dropdown dropdown-bottom w-full sm:w-auto">
                  <label
                    tabindex="0"
                    class="btn btn-ghost btn-sm rounded-xl border border-base-300 px-3 w-full sm:w-auto justify-between"
                  >
                    <span class="truncate">
                      Status:
                      {{
                        statusFilter === "all"
                          ? "All"
                          : statusFilter === "success"
                          ? "Success"
                          : statusFilter === "warning"
                          ? "Warnings"
                          : "Failed"
                      }}
                    </span>
                    <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                  </label>
                  <ul
                    tabindex="0"
                    class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 border border-base-200 z-[1]"
                  >
                    <li>
                      <button type="button" @click="statusFilter = 'all'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'all'"
                            class="w-4 h-4 text-success"
                          />
                          <span>All</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="statusFilter = 'success'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'success'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Success</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="statusFilter = 'error'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'error'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Failed</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="statusFilter = 'warning'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="statusFilter === 'warning'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Warnings</span>
                        </div>
                      </button>
                    </li>
                  </ul>
                </div>
                <button
                  v-if="statusFilter !== 'all'"
                  class="btn btn-ghost btn-xs hidden sm:inline-flex"
                  aria-label="Clear status filter"
                  @click="statusFilter = 'all'"
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
                      Source:
                      {{
                        sourceFilter === "all"
                          ? "All"
                          : sourceFilter === "local"
                          ? "Local"
                          : "Agents"
                      }}
                    </span>
                    <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                  </label>
                  <ul
                    tabindex="0"
                    class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 border border-base-200 z-[1]"
                  >
                    <li>
                      <button type="button" @click="sourceFilter = 'all'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="sourceFilter === 'all'"
                            class="w-4 h-4 text-success"
                          />
                          <span>All</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="sourceFilter = 'local'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="sourceFilter === 'local'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Local</span>
                        </div>
                      </button>
                    </li>
                    <li>
                      <button type="button" @click="sourceFilter = 'agent'">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="sourceFilter === 'agent'"
                            class="w-4 h-4 text-success"
                          />
                          <span>Agents</span>
                        </div>
                      </button>
                    </li>
                  </ul>
                </div>
                <button
                  v-if="sourceFilter !== 'all'"
                  class="btn btn-ghost btn-xs hidden sm:inline-flex"
                  aria-label="Clear source filter"
                  @click="sourceFilter = 'all'"
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
                    <span class="truncate">Limit: {{ limit }}</span>
                    <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                  </label>
                  <ul
                    tabindex="0"
                    class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 border border-base-200 z-[1]"
                  >
                    <li v-for="opt in [50, 100, 200, 400]" :key="opt">
                      <button type="button" @click="limit = opt">
                        <div class="flex items-center gap-2">
                          <Check
                            v-if="limit === opt"
                            class="w-4 h-4 text-success"
                          />
                          <span>{{ opt }}</span>
                        </div>
                      </button>
                    </li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="text-xs text-base-content/60">
          Showing {{ filteredEntries.length }} of {{ stats.total }} records
        </div>
      </div>

      <div
        v-if="loading"
        class="px-4 py-10 text-center text-sm text-base-content/70"
      >
        <div class="flex flex-col items-center gap-3">
          <RefreshCw class="w-5 h-5 animate-spin text-primary" />
          <div>Loading history...</div>
        </div>
      </div>

      <div
        v-else-if="filteredEntries.length === 0"
        class="px-4 py-12 text-center text-sm text-base-content/70"
      >
        <div class="flex flex-col items-center gap-3">
          <AlertCircle class="w-10 h-10 text-warning" />
          <div class="space-y-1">
            <div class="text-base font-semibold">No history yet</div>
            <div>Trigger an update to see it appear here.</div>
          </div>
        </div>
      </div>

      <div v-else class="overflow-x-auto">
        <table class="table w-full">
          <thead>
            <tr>
              <th class="w-40">
                <div class="flex items-center gap-2">
                  <Clock3 class="w-4 h-4" /> Time
                </div>
              </th>
              <th>Container</th>
              <th>Rollback image</th>
              <th>Source</th>
              <th class="w-32 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="entry in filteredEntries" :key="entry.id">
              <tr
                class="hover cursor-pointer"
                @click="toggleExpanded(entry, $event)"
              >
                <td class="text-sm text-base-content/70 whitespace-nowrap">
                  <div class="flex items-center gap-2">
                    <CheckCircle2
                      v-if="entry.status === 'success'"
                      class="w-4 h-4 text-success"
                      aria-label="Success"
                    />
                    <AlertTriangle
                      v-else-if="entry.status === 'warning'"
                      class="w-4 h-4 text-warning"
                      aria-label="Warning"
                    />
                    <AlertCircle v-else-if="entry.status === 'error'" class="w-4 h-4 text-error" aria-label="Failed" />
                    <HelpCircle
                      v-else-if="isInfoEntry(entry) || isAutoUpdateEntry(entry)"
                      class="w-4 h-4 text-info"
                      aria-label="Info"
                    />
                    <span>{{ formatDate(entry.createdAt) }}</span>
                  </div>
                </td>
                <td class="font-mono max-w-xs overflow-hidden truncate">
                  <template v-if="!isAutoUpdateEntry(entry)">
                    {{ entry.containerName || entry.containerId }}
                  </template>
                </td>
                <td class="font-mono max-w-xs overflow-hidden truncate">
                  {{ rollbackImageForEntry(entry) || (isInfoEntry(entry) ? "" : "unknown") }}
                </td>
                <td>
                  <div v-if="sourceLabel(entry)" class="badge badge-outline gap-2">
                    <Server v-if="entry.source === 'agent'" class="w-4 h-4" />
                    <User v-else class="w-4 h-4" />
                    <span>{{ sourceLabel(entry) }}</span>
                  </div>
                </td>
                <td class="text-right">
                  <div class="flex items-center justify-end gap-2">
                    <button
                      class="btn btn-ghost btn-xs btn-square"
                      :disabled="
                        !rollbackImageForEntry(entry) ||
                        rollbackImageForEntry(entry) === 'unknown' ||
                        (entry.source === 'agent' &&
                          (!entry.agentId || !entry.containerId)) ||
                        loading ||
                        rollingBack[entry.id]
                      "
                      @click="openRollbackModal(entry)"
                      title="Rollback container to this image"
                    >
                      <RotateCcw
                        class="w-4 h-4"
                        :class="{ 'animate-spin': rollingBack[entry.id] }"
                      />
                    </button>
                    <button
                      class="btn btn-ghost btn-xs btn-square text-error hover:bg-error/10"
                      :disabled="loading || deleting[entry.id]"
                      @click.stop="openDeleteModal(entry)"
                      title="Delete history entry"
                    >
                      <Trash2 class="w-4 h-4" />
                    </button>
                    <button
                      class="btn btn-ghost btn-xs btn-square"
                      :aria-expanded="expanded[entry.id] === true"
                      @click.stop="toggleExpanded(entry)"
                      :title="
                        expanded[entry.id]
                          ? 'Collapse details'
                          : 'Expand details'
                      "
                    >
                      <svg
                        v-if="expanded[entry.id]"
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M5 15l7-7 7 7"
                        />
                      </svg>
                      <svg
                        v-else
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        stroke-width="2"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M19 9l-7 7-7-7"
                        />
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="expanded[entry.id]" class="bg-base-200/40">
                <td colspan="5" class="text-sm text-base-content/80">
                  <div class="p-4 space-y-2">
                    <div class="flex flex-wrap gap-3 items-center">
                      <span class="badge" :class="statusBadge(entry.status)">
                        {{ entry.status }}
                      </span>
                      <span class="badge badge-ghost gap-2">
                        <Clock3 class="w-4 h-4" />
                        {{ formatDate(entry.createdAt) }}
                      </span>
                      <span class="badge badge-outline gap-2">
                        <Server
                          v-if="entry.source === 'agent'"
                          class="w-4 h-4"
                        />
                        <User v-else class="w-4 h-4" />
                        {{ sourceLabel(entry) }}
                      </span>
                    </div>
                    <div v-if="!isInfoEntry(entry)" class="grid gap-3 sm:grid-cols-2">
                      <div>
                        <div
                          class="text-xs text-base-content/60 uppercase tracking-wide"
                        >
                          Rollback image
                        </div>
                        <div class="font-mono break-all">
                          {{ rollbackImageForEntry(entry) }}
                        </div>
                      </div>
                      <div
                        v-if="
                          rollbackImageMap[entry.id] &&
                          rollbackImageMap[entry.id] !== entry.image
                        "
                      >
                        <div
                          class="text-xs text-base-content/60 uppercase tracking-wide"
                        >
                          Recorded image
                        </div>
                        <div class="font-mono break-all">
                          {{ entry.image || "unknown" }}
                        </div>
                      </div>
                      <div>
                        <div
                          class="text-xs text-base-content/60 uppercase tracking-wide"
                        >
                          Container ref
                        </div>
                        <div class="font-mono break-all">
                          {{
                            entry.containerName ||
                            entry.containerId ||
                            "unknown"
                          }}
                        </div>
                      </div>
                    </div>
                    <div>
                      <div
                        class="text-xs text-base-content/60 uppercase tracking-wide"
                      >
                        Message
                      </div>
                      <div class="whitespace-pre-wrap">{{ entry.message }}</div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <ConfirmModal
      :open="rollbackModalOpen"
      title="Confirm rollback"
      :message="rollbackMessage"
      confirm-label="Rollback"
      @confirm="rollbackToEntry"
      @cancel="closeRollbackModal"
    />
    <ConfirmModal
      :open="deleteModalOpen"
      title="Delete history entry"
      :message="deleteMessage"
      confirm-label="Delete"
      @confirm="deleteHistoryEntry"
      @cancel="closeDeleteModal"
    />
  </div>
</template>
