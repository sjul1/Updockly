<script setup lang="ts">
import {
  ref,
  onMounted,
  onBeforeUnmount,
  computed,
  inject,
  type ComputedRef,
} from "vue";
import { api, type Schedule } from "../services/api";
import ConfirmModal from "./ConfirmModal.vue";
import {
  CalendarClock,
  PlusCircle,
  RefreshCw,
  Clock3,
  Sparkles,
  Shield,
  Pencil,
  Trash2,
} from "lucide-vue-next";

const DEFAULT_CRON_EXPRESSION = "0 4 * * *";

const scheduleName = ref("");
const cronExpression = ref(DEFAULT_CRON_EXPRESSION);
const schedules = ref<Schedule[]>([]);
const autoUpdateCount = ref(0);
const lastUpdated = ref<Date | null>(null);
const formatTime = inject<
  (value: string | number | Date | null | undefined) => string
>("formatAppTime", (value) => {
  if (!value) return "--:--:--";
  const date = value instanceof Date ? value : new Date(value);
  return date.toLocaleTimeString();
});
const formatDateTime = inject<
  (value: string | number | Date | null | undefined) => string
>("formatAppDateTime", (value) => {
  if (!value) return "";
  const date = value instanceof Date ? value : new Date(value);
  return date.toLocaleString();
});
const appTimezone = inject<ComputedRef<string>>(
  "appTimezone",
  computed(() => Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC")
);
const refreshTimer = ref<number | null>(null);
const loading = ref(false);
const REFRESH_INTERVAL = 30000;
const autoRefreshEnabled = ref(true);
const cronPresets = [
  { label: "Daily at midnight", value: "0 0 * * *" },
  { label: "Weekdays 2am", value: "0 2 * * 1-5" },
  { label: "Every 15 minutes", value: "*/15 * * * *" },
];

const editingScheduleId = ref<string | null>(null);
const editedName = ref("");
const editedCronExpression = ref("");

const isConfirmOpen = ref(false);
const scheduleToDeleteId = ref<string | null>(null);

// Regex for a normalized (single-space, 5-field) cron expression
const cronRegex =
  /^([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)$/;

const isCronTokenValid = (token: string, min: number, max: number) => {
  if (!token.trim()) return false;

  let base = token.trim();
  if (base.includes("/")) {
    const [rawBase = "", rawStep = ""] = base.split("/");
    const step = Number(rawStep);
    if (!Number.isInteger(step) || step <= 0) return false;
    base = rawBase;
  }

  let start = min;
  let end = max;

  if (!base) return false;
  if (base === "*") {
    return true;
  }

  if (base.includes("-")) {
    const [rawStart, rawEnd] = base.split("-");
    const parsedStart = Number(rawStart);
    const parsedEnd = Number(rawEnd);
    if (!Number.isInteger(parsedStart) || !Number.isInteger(parsedEnd)) {
      return false;
    }
    start = Math.max(min, parsedStart);
    end = Math.min(max, parsedEnd);
    return start <= end;
  }

  const value = Number(base);
  if (!Number.isInteger(value)) return false;
  if (value === 7 && max === 6) {
    return true; // allow 0 or 7 for Sunday
  }
  return value >= min && value <= max;
};

const isCronFieldValid = (field: string, min: number, max: number) => {
  const tokens = field.split(",").map((part) => part.trim()).filter(Boolean);
  if (tokens.length === 0) return false;
  return tokens.every((token) => isCronTokenValid(token, min, max));
};

const cronTokenMatches = (token: string, value: number, min: number, max: number) => {
  if (token === "*") return true;

  let step = 1;
  let baseToken = token;

  if (token.includes("/")) {
    const [rawBase = "", rawStep = ""] = token.split("/");
    const parsedStep = Number(rawStep);
    if (!Number.isInteger(parsedStep) || parsedStep <= 0) return false;
    step = parsedStep;
    baseToken = rawBase;
  }

  let start = min;
  let end = max;

  if (baseToken.includes("-")) {
    const [rawStart, rawEnd] = baseToken.split("-");
    const parsedStart = Number(rawStart);
    const parsedEnd = Number(rawEnd);
    if (!Number.isInteger(parsedStart) || !Number.isInteger(parsedEnd)) {
      return false;
    }
    start = Math.max(min, parsedStart);
    end = Math.min(max, parsedEnd);
    if (start > end) return false;
  } else if (baseToken === "*") {
    // already handled above
  } else {
    const parsed = Number(baseToken);
    if (!Number.isInteger(parsed)) return false;
    if (parsed === 7 && max === 6) {
      return value === 0;
    }
    return parsed === value;
  }

  if (value < start || value > end) return false;
  return (value - start) % step === 0;
};

const cronFieldMatches = (field: string, value: number, min: number, max: number) => {
  const tokens = field.split(",").map((part) => part.trim()).filter(Boolean);
  if (tokens.length === 0) return false;
  return tokens.some((token) => cronTokenMatches(token, value, min, max));
};

const cronMatches = (expr: string, date: Date) => {
  const normalized = normalizeCronValue(expr);
  const parts = normalized.split(" ");
  if (parts.length !== 5) return false;

  const [minute = "", hour = "", day = "", month = "", weekday = ""] = parts;
  const matchers = [
    { field: minute, value: date.getMinutes(), min: 0, max: 59 },
    { field: hour, value: date.getHours(), min: 0, max: 23 },
    { field: day, value: date.getDate(), min: 1, max: 31 },
    { field: month, value: date.getMonth() + 1, min: 1, max: 12 },
    { field: weekday, value: date.getDay(), min: 0, max: 6 },
  ];

  return matchers.every((m) =>
    cronFieldMatches(m.field, m.value, m.min, m.max)
  );
};

const getNextRunForCron = (expr: string, fromDate: Date) => {
  if (!isCronExpressionValid(expr)) return null;

  // start at current minute boundary
  const cursor = new Date(fromDate);
  cursor.setSeconds(0, 0);

  const MAX_LOOKAHEAD_MINUTES = 60 * 24 * 60; // ~60 days
  for (let i = 0; i <= MAX_LOOKAHEAD_MINUTES; i++) {
    if (cronMatches(expr, cursor)) {
      return new Date(cursor);
    }
    cursor.setMinutes(cursor.getMinutes() + 1);
  }
  return null;
};

// Normalize ONLY for validation/description/API calls
const normalizeCronValue = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) return "";
  const parts = trimmed.split(/\s+/).filter(Boolean).slice(0, 5);
  return parts.join(" ");
};

const isCronExpressionValid = (value: string) => {
  const normalized = normalizeCronValue(value);
  if (!cronRegex.test(normalized)) return false;

  const [minute = "", hour = "", day = "", month = "", weekday = ""] =
    normalized.split(" ");

  return (
    isCronFieldValid(minute, 0, 59) &&
    isCronFieldValid(hour, 0, 23) &&
    isCronFieldValid(day, 1, 31) &&
    isCronFieldValid(month, 1, 12) &&
    isCronFieldValid(weekday, 0, 6)
  );
};

const normalizedCronExpression = computed(() =>
  normalizeCronValue(cronExpression.value)
);
const normalizedEditedCronExpression = computed(() =>
  normalizeCronValue(editedCronExpression.value)
);

const isCronValid = computed(() =>
  isCronExpressionValid(normalizedCronExpression.value)
);
const isEditedCronValid = computed(() =>
  isCronExpressionValid(normalizedEditedCronExpression.value)
);

const formatDurationUntil = (target: Date, from: Date) => {
  const diffMs = target.getTime() - from.getTime();
  if (diffMs <= 0) return "now";

  const totalMinutes = Math.round(diffMs / 60000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  const parts = [];
  if (hours > 0) {
    parts.push(`${hours} hour${hours === 1 ? "" : "s"}`);
  }
  parts.push(`${minutes} minute${minutes === 1 ? "" : "s"}`);

  return parts.join(" ");
};

const nextScheduledRuns = computed(() => {
  const now = new Date();
  return schedules.value
    .map((schedule) => {
      const nextRun = getNextRunForCron(schedule.CronExpression, now);
      if (!nextRun) return null;
      return {
        schedule,
        nextRun,
        until: formatDurationUntil(nextRun, now),
      };
    })
    .filter(
      (
        item
      ): item is { schedule: Schedule; nextRun: Date; until: string } =>
        item !== null
    )
    .sort((a, b) => a.nextRun.getTime() - b.nextRun.getTime())
    .slice(0, 3);
});

const fetchSchedules = async () => {
  if (loading.value) return;
  loading.value = true;
  try {
    schedules.value = await api.getSchedules();
    lastUpdated.value = new Date();
  } catch (error) {
    console.error("Failed to fetch schedules:", error);
  } finally {
    loading.value = false;
  }
};

const fetchAutoUpdateCount = async () => {
  try {
    const response = await api.getAutoUpdateCount();
    autoUpdateCount.value = response.count;
  } catch (error) {
    console.error("Failed to fetch auto-update count:", error);
  }
};

const createSchedule = async () => {
  if (!scheduleName.value || !isCronValid.value) {
    return;
  }
  try {
    await api.createSchedule(
      scheduleName.value,
      normalizedCronExpression.value
    );
    scheduleName.value = "";
    cronExpression.value = DEFAULT_CRON_EXPRESSION;
    await fetchSchedules();
  } catch (error) {
    console.error("Failed to create schedule:", error);
  }
};

const editSchedule = (schedule: Schedule) => {
  editingScheduleId.value = schedule.ID;
  editedName.value = schedule.Name;
  editedCronExpression.value = schedule.CronExpression;
};

const saveSchedule = async (id: string) => {
  if (!isEditedCronValid.value) {
    return;
  }
  try {
    await api.updateSchedule(
      id,
      editedName.value,
      normalizedEditedCronExpression.value
    );
    editingScheduleId.value = null;
    await fetchSchedules();
  } catch (error) {
    console.error("Failed to save schedule:", error);
  }
};

const cancelEdit = () => {
  editingScheduleId.value = null;
};

const openConfirmModal = (id: string) => {
  scheduleToDeleteId.value = id;
  isConfirmOpen.value = true;
};

const handleDeleteConfirm = async () => {
  if (scheduleToDeleteId.value) {
    try {
      await api.deleteSchedule(scheduleToDeleteId.value);
      await fetchSchedules();
    } catch (error) {
      console.error("Failed to delete schedule:", error);
    }
  }
  isConfirmOpen.value = false;
};

const startAutoRefresh = () => {
  if (!autoRefreshEnabled.value) return;
  if (refreshTimer.value) return;
  refreshTimer.value = window.setInterval(() => {
    fetchSchedules();
    fetchAutoUpdateCount();
  }, REFRESH_INTERVAL);
};

const stopAutoRefresh = () => {
  if (refreshTimer.value) {
    window.clearInterval(refreshTimer.value);
    refreshTimer.value = null;
  }
};

const toggleAutoRefresh = () => {
  autoRefreshEnabled.value = !autoRefreshEnabled.value;
  if (autoRefreshEnabled.value) {
    startAutoRefresh();
  } else {
    stopAutoRefresh();
  }
};

const applyCronPreset = (value: string) => {
  // Keep user-facing spacing exactly as preset
  cronExpression.value = value;
};

const cronFieldLabels = ["minute", "hour", "day of month", "month", "weekday"];

const activeCronField = ref<number | null>(null);
const activeEditedCronField = ref<number | null>(null);

// Map cursor position -> field index (0-4), handling multiple spaces
const getCronFieldIndexFromCursor = (value: string, cursorPos: number) => {
  const pos = Math.max(0, Math.min(cursorPos, value.length));
  if (pos === 0) return 0;

  const left = value.slice(0, pos);
  const leftParts = left.split(/\s+/).filter(Boolean);
  const tokensToLeft = leftParts.length;

  const prevChar = value[pos - 1] ?? "";
  const isPrevSpace = /\s/.test(prevChar);

  let index: number;

  if (!isPrevSpace) {
    // Cursor is inside or just after a token
    index = tokensToLeft > 0 ? tokensToLeft - 1 : 0;
  } else {
    // Cursor is in spaces between tokens
    // Treat that as "the next field" (crontab.guru style)
    index = tokensToLeft;
  }

  if (index < 0) index = 0;
  if (index > 4) index = 4;
  return index;
};

const getCronDescription = (expr: string): string => {
  if (!expr.trim()) return "";
  if (!isCronExpressionValid(expr)) {
    return "Invalid cron expression";
  }

  const tokens = expr.trim().split(/\s+/);
  if (tokens.length !== 5) {
    return "Invalid cron expression";
  }
  const [min, hour, day, month, dow] = tokens as [
    string,
    string,
    string,
    string,
    string
  ];

  // Popular simple patterns
  if (expr.trim() === "* * * * *") {
    return "Every minute";
  }

  if (
    min === "0" &&
    hour === "0" &&
    day === "*" &&
    month === "*" &&
    dow === "*"
  ) {
    return "Every day at 00:00";
  }

  if (
    min.startsWith("*/") &&
    hour === "*" &&
    day === "*" &&
    month === "*" &&
    dow === "*"
  ) {
    const n = Number(min.slice(2));
    if (Number.isInteger(n) && n > 0) {
      return `Every ${n} minute${n > 1 ? "s" : ""}`;
    }
  }

  const minNum = Number(min);
  const hourNum = Number(hour);

  // "Every day at HH:MM"
  if (
    Number.isInteger(minNum) &&
    Number.isInteger(hourNum) &&
    day === "*" &&
    month === "*" &&
    dow === "*"
  ) {
    const hh = hourNum.toString().padStart(2, "0");
    const mm = minNum.toString().padStart(2, "0");
    return `Every day at ${hh}:${mm}`;
  }

  // Fallback: structured but basic description
  return `Minute: ${min}, hour: ${hour}, day of month: ${day}, month: ${month}, weekday: ${dow}`;
};

const cronDescription = computed(() =>
  getCronDescription(normalizedCronExpression.value)
);
const editedCronDescription = computed(() =>
  getCronDescription(normalizedEditedCronExpression.value)
);

const handleCronFocusOrClick = (mode: "create" | "edit", event: Event) => {
  const target = event.target as HTMLInputElement;
  const value = target.value;
  const pos = target.selectionStart ?? value.length;
  const fieldIndex = getCronFieldIndexFromCursor(value, pos);

  if (mode === "create") {
    activeCronField.value = fieldIndex;
  } else {
    activeEditedCronField.value = fieldIndex;
  }
};

const handleCronBlur = (mode: "create" | "edit") => {
  if (mode === "create") {
    cronExpression.value = normalizeCronValue(cronExpression.value);
    activeCronField.value = null;
  } else {
    editedCronExpression.value = normalizeCronValue(
      editedCronExpression.value
    );
    activeEditedCronField.value = null;
  }
};

const handleCronInput = (mode: "create" | "edit", event: Event) => {
  const target = event.target as HTMLInputElement;
  const rawValue = target.value; // keep user spacing as-is

  if (mode === "create") {
    cronExpression.value = rawValue;
    const pos = target.selectionStart ?? rawValue.length;
    activeCronField.value = getCronFieldIndexFromCursor(rawValue, pos);
  } else {
    editedCronExpression.value = rawValue;
    const pos = target.selectionStart ?? rawValue.length;
    activeEditedCronField.value = getCronFieldIndexFromCursor(rawValue, pos);
  }
};

onMounted(() => {
  fetchSchedules();
  fetchAutoUpdateCount();
  startAutoRefresh();
});

onBeforeUnmount(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="space-y-6">
    <div
      class="relative overflow-hidden rounded-3xl border border-base-200 bg-gradient-to-r from-secondary/15 via-primary/10 to-accent/10 p-6 shadow-xl"
    >
      <div
        class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between"
      >
        <div class="space-y-2">
          <div
            class="inline-flex items-center gap-2 rounded-full bg-base-100/80 px-3 py-1 text-xs font-semibold shadow"
          >
            <CalendarClock class="h-4 w-4 text-secondary" />
            Schedules
          </div>
          <h1 class="text-3xl font-bold pr-36 md:pr-0">Scheduled Updates</h1>
          <p class="text-sm text-base-content/70 max-w-2xl">
            Coordinate update windows and keep your auto-update fleet
            predictable.
          </p>
          <div class="flex flex-wrap gap-3">
            <div class="badge badge-info gap-2">
              <Shield class="h-3.5 w-3.5" /> Auto-update targets:
              {{ autoUpdateCount }}
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
              @click="fetchSchedules"
              :disabled="loading"
              aria-label="Refresh schedules"
              title="Refresh schedules"
            >
              <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': loading }" />
            </button>
            <div
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
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
      <div class="lg:col-span-5">
        <div class="card bg-base-100 shadow-xl border border-base-200">
          <div class="card-body space-y-4">
            <div class="flex items-center justify-between">
              <h2 class="card-title">New Schedule</h2>
            </div>
            <p class="text-sm text-base-content/70">
              This will trigger an update for {{ autoUpdateCount }} container(s)
              with auto-update enabled.
            </p>
            <form @submit.prevent="createSchedule" class="space-y-4">
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Schedule Name</span>
                </label>
                <input
                  v-model="scheduleName"
                  type="text"
                  placeholder="e.g., Daily midnight updates"
                  class="input input-bordered input-sm w-full rounded-xl focus:ring focus:ring-primary/20"
                  required
                />
              </div>
              <div class="form-control">
                <label class="label">
                  <span class="label-text">Cron Expression</span>
                </label>
                <div
                  class="flex flex-wrap items-center gap-2 text-[0.75rem] text-base-content/70 mb-1"
                >
                  <span
                    v-for="(label, index) in cronFieldLabels"
                    :key="label"
                    class="badge badge-outline transition-colors"
                    :class="{
                      'badge-primary text-primary-content':
                        activeCronField === index && cronExpression,
                    }"
                  >
                    {{ label }}
                  </span>
                  <span class="text-base-content/50">
                    Format: 5 fields, space separated
                  </span>
                </div>

                <input
                  v-model="cronExpression"
                  type="text"
                  placeholder="e.g., 0 0 * * *"
                  class="input input-bordered input-sm w-full rounded-xl focus:ring focus:ring-primary/20 font-mono"
                  @input="handleCronInput('create', $event)"
                  @focus="handleCronFocusOrClick('create', $event)"
                  @click="handleCronFocusOrClick('create', $event)"
                  @keyup="handleCronFocusOrClick('create', $event)"
                  @blur="handleCronBlur('create')"
                  :class="{ 'input-error': cronExpression && !isCronValid }"
                  required
                />

                <p
                  v-if="cronExpression"
                  class="mt-1 text-xs text-base-content/70 flex items-center gap-1"
                >
                  <Clock3 class="h-3 w-3" />
                  {{ cronDescription }}
                </p>

                <div class="label flex-col items-start gap-1">
                  <span class="label-text-alt">
                    Learn more about
                    <a href="https://crontab.guru/" target="_blank" class="link"
                      >cron expressions</a
                    >
                  </span>
                  <span class="label-text-alt text-base-content/60">
                    Seconds not supported; use standard 5-field cron.
                  </span>
                </div>
                <div class="flex flex-wrap gap-2 mt-2">
                  <button
                    v-for="preset in cronPresets"
                    :key="preset.value"
                    type="button"
                    class="btn btn-outline btn-xs rounded-full"
                    @click="applyCronPreset(preset.value)"
                  >
                    {{ preset.label }}
                  </button>
                </div>
                <span
                  v-if="cronExpression && !isCronValid"
                  class="text-error text-xs mt-1"
                  >Invalid cron expression format.</span
                >
              </div>
              <div class="card-actions justify-end">
                <button
                  type="submit"
                  class="btn btn-primary w-full"
                  :disabled="!isCronValid || !scheduleName"
                >
                  <PlusCircle class="w-4 h-4" />
                  Create schedule
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>

      <div class="lg:col-span-7 space-y-4">
        <div class="card bg-base-100 shadow-xl border border-base-200">
          <div class="card-body space-y-4">
            <div class="flex items-center justify-between">
              <div>
                <h2 class="card-title">Existing Schedules</h2>
                <p class="text-sm text-base-content/60">
                  Edit, pause, or delete scheduled update windows.
                </p>
              </div>
            </div>
            <div class="overflow-x-auto rounded-xl border border-base-200">
              <table class="table w-full table-fixed">
                <thead>
                  <tr>
                    <th class="w-24 sm:w-auto">Name</th>
                    <th class="w-24 sm:w-auto"><span class="hidden sm:inline">Cron Expression</span><span class="sm:hidden">Cron</span></th>
                    <th class="w-24 sm:w-32 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="schedule in schedules" :key="schedule.ID">
                    <template v-if="editingScheduleId === schedule.ID">
                      <td class="w-24 sm:w-auto">
                        <input
                          v-model="editedName"
                          type="text"
                          class="input input-bordered input-xs w-full rounded-xl focus:ring focus:ring-primary/20"
                          required
                        />
                      </td>
                      <td class="w-24 sm:w-auto">
                        <input
                          v-model="editedCronExpression"
                          type="text"
                          class="input input-bordered input-xs w-full rounded-xl focus:ring focus:ring-primary/20 font-mono"
                          @input="handleCronInput('edit', $event)"
                          @focus="handleCronFocusOrClick('edit', $event)"
                          @click="handleCronFocusOrClick('edit', $event)"
                          @keyup="handleCronFocusOrClick('edit', $event)"
                          @blur="handleCronBlur('edit')"
                          :class="{
                            'input-error':
                              editedCronExpression && !isEditedCronValid,
                          }"
                        />
                        <p
                          v-if="editedCronExpression"
                          class="mt-1 text-[0.7rem] text-base-content/70 flex items-center gap-1"
                        >
                          <Clock3 class="h-3 w-3" />
                          {{ editedCronDescription }}
                        </p>
                        <span
                          v-if="editedCronExpression && !isEditedCronValid"
                          class="text-error text-xs mt-1"
                          >Invalid cron expression format.</span
                        >
                      </td>
                      <td class="space-x-2 w-24 sm:w-32 text-right">
                        <button
                          @click="saveSchedule(schedule.ID)"
                          class="btn btn-ghost btn-sm text-success"
                          :disabled="!isEditedCronValid || !editedName"
                        >
                          Save
                        </button>
                        <button
                          @click="cancelEdit"
                          class="btn btn-ghost btn-sm"
                        >
                          Cancel
                        </button>
                      </td>
                    </template>
                    <template v-else>
                      <td class="font-semibold w-24 sm:w-auto truncate">{{ schedule.Name }}</td>
                      <td class="font-mono text-sm w-24 sm:w-auto truncate">
                        {{ schedule.CronExpression }}
                      </td>
                      <td class="space-x-2 w-24 sm:w-32 text-right">
                        <button
                          @click="editSchedule(schedule)"
                          class="btn btn-ghost btn-sm btn-circle"
                          title="Edit schedule"
                        >
                          <Pencil class="w-4 h-4" />
                        </button>
                        <button
                          @click="openConfirmModal(schedule.ID)"
                          class="btn btn-ghost btn-sm btn-circle text-error"
                          title="Delete schedule"
                        >
                          <Trash2 class="w-4 h-4" />
                        </button>
                      </td>
                    </template>
                  </tr>
                  <tr v-if="schedules.length === 0">
                    <td
                      colspan="3"
                      class="text-center py-6 text-base-content/60"
                    >
                      No schedules created yet.
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div class="card bg-base-100 shadow-lg border border-base-200">
          <div class="card-body space-y-3">
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm text-base-content/60">Next window</p>
                <h3 class="text-lg font-semibold">Predictable cadence</h3>
              </div>
              <div class="badge badge-ghost gap-1 text-xs">
                <Clock3 class="h-3.5 w-3.5" />
                Preview
              </div>
            </div>
            <div v-if="nextScheduledRuns.length" class="space-y-3">
              <div
                v-for="item in nextScheduledRuns"
                :key="item.schedule.ID"
                class="flex flex-col gap-1 border border-base-200 rounded-xl p-3"
              >
                <div class="flex items-center justify-between gap-2">
                  <span class="font-semibold truncate">{{
                    item.schedule.Name
                  }}</span>
                  <span class="badge badge-outline font-mono text-[0.75rem]">{{
                    item.schedule.CronExpression
                  }}</span>
                </div>
                <div class="text-xs text-base-content/70 flex items-center gap-2 flex-wrap">
                  <Clock3 class="h-3 w-3" />
                  <span>{{ formatDateTime(item.nextRun) }}</span>
                  <span class="badge badge-ghost text-[0.7rem]">
                    In {{ item.until }}
                  </span>
                </div>
              </div>
              <p class="text-[0.75rem] text-base-content/60 flex items-center gap-2 flex-wrap">
                Showing next run(s) in timezone:
                <span class="badge badge-outline text-[0.7rem]">
                  {{ appTimezone }}
                </span>
              </p>
            </div>
            <div v-else class="text-sm text-base-content/60">
              No upcoming runs yet. Create a schedule to preview its next window.
            </div>
          </div>
        </div>
      </div>
    </div>
    <ConfirmModal
      :open="isConfirmOpen"
      title="Delete Schedule"
      message="Are you sure you want to delete this schedule? This action cannot be undone."
      @confirm="handleDeleteConfirm"
      @cancel="isConfirmOpen = false"
    />
  </div>
</template>
