<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, inject } from "vue";
import {
  Activity,
  RefreshCw,
  Zap,
  ShieldCheck,
  CalendarClock,
  ServerCog,
  Play,
  Sparkles,
  CheckCircle2,
  AlertCircle,
  AlertTriangle,
} from "lucide-vue-next";
import {
  api,
  type DashboardStats,
  type UpdateHistory,
  type Schedule,
  type RunningHistoryEntry,
} from "../services/api";

const props = defineProps<{
  dashboard: DashboardStats | null;
  loadingBootstrap: boolean;
}>();

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

const emit = defineEmits<{
  (event: "refresh-dashboard"): void;
}>();

const lastSync = computed(() => {
  if (!props.dashboard?.time) return "--";
  return formatTime(props.dashboard.time);
});

const autoRefreshEnabled = ref(true);
const refreshTimer = ref<number | null>(null);
const REFRESH_INTERVAL = 30000;

const startAutoRefresh = () => {
  if (refreshTimer.value) return;
  refreshTimer.value = window.setInterval(autoRefreshTick, REFRESH_INTERVAL);
};

const stopAutoRefresh = () => {
  if (refreshTimer.value) {
    window.clearInterval(refreshTimer.value);
    refreshTimer.value = null;
  }
};

const autoRefreshTick = () => {
  if (!autoRefreshEnabled.value) return;
  if (props.loadingBootstrap) return;
  emit("refresh-dashboard");
  void fetchRecentHistory();
  void fetchNextSchedule();
  void fetchRunningHistory();
};

const toggleAutoRefresh = () => {
  autoRefreshEnabled.value = !autoRefreshEnabled.value;
  if (autoRefreshEnabled.value) {
    startAutoRefresh();
    autoRefreshTick();
  } else {
    stopAutoRefresh();
  }
};

const runningHistoryBars = computed(() => {
  const now = new Date();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const byKey = new Map<string, RunningHistoryEntry>();
  runningHistory.value.forEach((entry) => {
    const d = new Date(entry.date);
    const key = `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
    byKey.set(key, entry);
  });

  const days = Array.from({ length: 7 }, (_, idx) => {
    const day = new Date(start);
    day.setDate(day.getDate() - (6 - idx));
    const key = `${day.getFullYear()}-${day.getMonth()}-${day.getDate()}`;
    const entry = byKey.get(key);
    let running = entry?.running ?? 0;
    let total = entry?.total ?? 0;

    const sameDay =
      day.getFullYear() === start.getFullYear() &&
      day.getMonth() === start.getMonth() &&
      day.getDate() === start.getDate();
    if (sameDay) {
      if (typeof props.dashboard?.runningContainers === "number") {
        running = props.dashboard.runningContainers;
      }
      if (typeof props.dashboard?.totalContainers === "number") {
        total = props.dashboard.totalContainers;
      }
    }

    return {
      date: day.toISOString(),
      running,
      total,
    };
  });

  const maxRunning = Math.max(1, ...days.map((d) => d.running));

  return days.map((d) => {
    const label = new Date(d.date).toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    });
    const height = Math.max(8, Math.round((d.running / maxRunning) * 100));
    return { ...d, label, height };
  });
});

const deploymentCadence = computed(() => {
  const now = new Date();
  const dayMs = 24 * 60 * 60 * 1000;
  const startDay = new Date(now.getFullYear(), now.getMonth(), now.getDate());

  const buckets = Array.from({ length: 7 }, (_, idx) => {
    const dayStart = new Date(startDay.getTime() - (6 - idx) * dayMs);
    return {
      label: dayStart.toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
      }),
      successLocal: 0,
      successAgent: 0,
      warningLocal: 0,
      warningAgent: 0,
      errorLocal: 0,
      errorAgent: 0,
      total: 0,
    };
  });

  recentHistory.value.forEach((entry) => {
    const ts = new Date(entry.createdAt || 0).getTime();
    if (Number.isNaN(ts)) return;
    const dayIdx = buckets.findIndex((_, idx) => {
      const start = new Date(startDay.getTime() - (6 - idx) * dayMs).getTime();
      const end = start + dayMs;
      return ts >= start && ts < end;
    });
    if (dayIdx === -1) return;

    const target = buckets[dayIdx];
    if (!target) return;
    const isAgent = entry.source === "agent";
    const isError = entry.status === "error";
    const isWarning = entry.status === "warning";

    if (isAgent && isError) target.errorAgent += 1;
    else if (isAgent && isWarning) target.warningAgent += 1;
    else if (isAgent) target.successAgent += 1;
    else if (isError) target.errorLocal += 1;
    else if (isWarning) target.warningLocal += 1;
    else target.successLocal += 1;
    target.total += 1;
  });

  const maxTotal = Math.max(1, ...buckets.map((b) => b.total));
  return buckets.map((bucket) => {
    const scale = (count: number) =>
      Math.max(4, Math.round((count / maxTotal) * 100));
    return {
      ...bucket,
      heights: {
        successLocal: scale(bucket.successLocal),
        successAgent: scale(bucket.successAgent),
        warningLocal: scale(bucket.warningLocal),
        warningAgent: scale(bucket.warningAgent),
        errorLocal: scale(bucket.errorLocal),
        errorAgent: scale(bucket.errorAgent),
      },
      heightTotal: scale(bucket.total),
    };
  });
});

const activityFeed = computed(() => {
  return recentHistory.value.map((entry) => ({
    label: entry.containerName || entry.containerId || "Container update",
    time: formatDateTime(entry.createdAt),
    detail: entry.message || entry.image || "Updated",
    tone:
      entry.status === "error"
        ? "error"
        : entry.status === "warning"
        ? "warning"
        : "success",
  }));
});

const recentStats = computed(() => {
  const total = recentHistory.value.length;
  const success = recentHistory.value.filter(
    (h) => h.status === "success"
  ).length;
  const warning = recentHistory.value.filter(
    (h) => h.status === "warning"
  ).length;
  const error = recentHistory.value.filter((h) => h.status === "error").length;
  return { total, success, warning, error };
});

const recentHistory = ref<UpdateHistory[]>([]);
const upcomingSchedule = ref<{ schedule: Schedule; nextRun: Date } | null>(
  null
);
const runningHistory = ref<RunningHistoryEntry[]>([]);

const fetchRecentHistory = async () => {
  try {
    recentHistory.value = await api.getUpdateHistory(400);
  } catch (error) {
    console.error("Failed to load recent history:", error);
  }
};

const fetchNextSchedule = async () => {
  try {
    const schedules = await api.getSchedules();
    const now = new Date();
    let best: { schedule: Schedule; nextRun: Date } | null = null;
    schedules.forEach((s) => {
      const next = getNextRunForCron(s.CronExpression, now);
      if (!next) return;
      if (!best || next < best.nextRun) {
        best = { schedule: s, nextRun: next };
      }
    });
    upcomingSchedule.value = best;
  } catch (error) {
    console.error("Failed to load schedules:", error);
  }
};

const fetchRunningHistory = async () => {
  try {
    runningHistory.value = await api.getRunningHistory();
  } catch (error) {
    console.error("Failed to load running history:", error);
  }
};

const getNextRunForCron = (expr: string, fromDate: Date) => {
  const cronRegex =
    /^([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)\s+([\d*/,-]+)$/;
  const normalizeCronValue = (value: string) => {
    const trimmed = value.trim();
    if (!trimmed) return "";
    const parts = trimmed.split(/\s+/).filter(Boolean).slice(0, 5);
    return parts.join(" ");
  };
  const isCronTokenValid = (token: string, min: number, max: number) => {
    if (!token.trim()) return false;
    let base = token.trim();
    if (base.includes("/")) {
      const [rawBase = "", rawStep = ""] = base.split("/");
      base = rawBase;
      const step = Number(rawStep);
      if (!Number.isInteger(step) || step <= 0) return false;
    }
    if (base === "" || base === "*") return true;
    if (base.includes("-")) {
      const [rawStart, rawEnd] = base.split("-");
      const start = Number(rawStart);
      const end = Number(rawEnd);
      return (
        Number.isInteger(start) &&
        Number.isInteger(end) &&
        start >= min &&
        end <= max &&
        start <= end
      );
    }
    const value = Number(base);
    if (!Number.isInteger(value)) return false;
    if (value === 7 && max === 6) return true; // allow 7 for Sunday
    return value >= min && value <= max;
  };

  const isCronFieldValid = (field: string, min: number, max: number) => {
    const tokens = field
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean);
    if (tokens.length === 0) return false;
    return tokens.every((token) => isCronTokenValid(token, min, max));
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

  const cronTokenMatches = (
    token: string,
    value: number,
    min: number,
    max: number
  ) => {
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

  const cronFieldMatches = (
    field: string,
    value: number,
    min: number,
    max: number
  ) => {
    const tokens = field
      .split(",")
      .map((part) => part.trim())
      .filter(Boolean);
    if (tokens.length === 0) return false;
    return tokens.some((token) => cronTokenMatches(token, value, min, max));
  };

  const cronMatches = (value: string, date: Date) => {
    const normalized = normalizeCronValue(value);
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

  if (!isCronExpressionValid(expr)) return null;

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

onMounted(() => {
  startAutoRefresh();
  void fetchRecentHistory();
  void fetchNextSchedule();
  void fetchRunningHistory();
});

onBeforeUnmount(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="space-y-8">
    <div
      class="relative overflow-hidden rounded-3xl border border-base-200 bg-gradient-to-br from-primary/10 via-primary/5 to-accent/5 shadow-xl"
    >
      <div class="absolute inset-0 bg-grid-primary/5 opacity-60"></div>
      <div
        class="relative flex flex-col p-8 md:flex-row md:items-start md:justify-between"
      >
        <div class="space-y-3">
          <div class="flex items-start justify-between gap-3">
            <div
              class="inline-flex items-center gap-2 rounded-full bg-base-100/70 px-3 py-1 text-xs font-semibold shadow-sm"
            >
              <Activity class="h-4 w-4 text-primary" />
              Live overview
            </div>
          </div>
          <div class="space-y-2">
            <h1 class="text-3xl md:text-4xl font-bold leading-tight">
              {{ props.dashboard?.message || "Container fleet overview" }}
            </h1>
            <p class="text-base text-base-content/70 max-w-2xl">
              Track container uptime, scheduled updates, and automation health
              in one place.
            </p>
          </div>
          <div class="flex items-center gap-3 text-sm text-base-content/60">
            <div class="flex items-center gap-2">
              <span
                class="inline-flex h-2 w-2 rounded-full bg-success animate-pulse"
              ></span>
              Synced {{ lastSync }}
            </div>
            <div class="hidden sm:block text-base-content/40">•</div>
            <div class="hidden sm:flex items-center gap-2 text-sm">
              <ShieldCheck class="h-4 w-4" />
              {{ props.dashboard?.scheduleCount ?? 0 }} schedules guard your
              updates
            </div>
          </div>
        </div>

        <div
          class="flex items-center gap-2 absolute top-6 right-6 md:static md:self-start md:ml-auto"
        >
          <button
            class="btn btn-ghost btn-square"
            @click="emit('refresh-dashboard')"
            :disabled="props.loadingBootstrap"
            aria-label="Refresh dashboard"
            title="Refresh dashboard"
          >
            <RefreshCw
              class="w-4 h-4"
              :class="{ 'animate-spin': props.loadingBootstrap }"
            />
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

    <div class="grid grid-cols-2 gap-4 md:grid-cols-2 xl:grid-cols-5">
      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Total containers</span>
            <ServerCog class="h-5 w-5 text-primary" />
          </div>
          <div class="text-3xl font-bold">
            {{ props.dashboard?.totalContainers ?? "—" }}
          </div>
          <p class="text-xs text-base-content/60">
            Includes running and paused services discovered via Docker.
          </p>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all overflow-visible z-10"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Running</span>
            <Zap class="h-5 w-5 text-accent" />
          </div>
          <div class="flex items-end gap-2">
            <span class="text-3xl font-bold">{{
              props.dashboard?.runningContainers ?? "—"
            }}</span>
            <span class="badge badge-success badge-sm">active</span>
          </div>
          <div class="h-16 flex items-end gap-2">
            <div
              v-for="bar in runningHistoryBars"
              :key="bar.date"
              class="group relative w-6 rounded-lg bg-accent/60 transition-all duration-500 flex items-end justify-center"
              :style="{ height: `${bar.height}%` }"
            >
              <span class="text-[0.65rem] text-white/80 pb-1">
                {{ bar.running }}
              </span>
              <div
                class="pointer-events-none absolute bottom-full mb-2 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-md border border-base-300 bg-base-100 px-2 py-1 text-[0.7rem] shadow-md opacity-0 group-hover:opacity-100 transition z-50"
              >
                <div class="font-semibold text-base-content">
                  {{ bar.label }}
                </div>
                <div class="text-base-content/70">
                  {{ bar.running }} running / {{ bar.total }} total
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Auto-update ready</span>
            <ShieldCheck class="h-5 w-5 text-success" />
          </div>
          <div class="text-3xl font-bold">
            {{ props.dashboard?.autoUpdateEnabled ?? "—" }}
          </div>
          <p class="text-xs text-base-content/60">
            Containers that will be patched automatically when updates ship.
          </p>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Schedules</span>
            <CalendarClock class="h-5 w-5 text-secondary" />
          </div>
          <div class="flex items-end gap-2">
            <span class="text-3xl font-bold">{{
              props.dashboard?.scheduleCount ?? "—"
            }}</span>
            <span class="text-xs text-base-content/60">automation slots</span>
          </div>
          <div
            class="progress progress-secondary h-2"
            :value="props.dashboard?.scheduleCount || 0"
            max="10"
          ></div>
          <div
            v-if="upcomingSchedule"
            class="text-xs text-base-content/70 flex items-center gap-2"
          >
            <span class="badge badge-outline badge-secondary text-[0.7rem]">
              Next
            </span>
            <span class="font-semibold text-base-content">
              In
              {{ formatDurationUntil(upcomingSchedule.nextRun, new Date()) }}
            </span>
          </div>
          <div v-else class="text-xs text-base-content/60">
            No upcoming schedules found.
          </div>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Agents</span>
            <ServerCog class="h-5 w-5 text-primary" />
          </div>
          <div class="flex items-end gap-2">
            <span class="text-3xl font-bold">{{
              props.dashboard?.agentCount ?? "—"
            }}</span>
            <span class="badge badge-outline badge-success gap-1 text-xs">
              {{ props.dashboard?.agentOnline ?? 0 }} online
            </span>
          </div>
          <p class="text-xs text-base-content/60">
            Remote hosts reporting Docker state back to Updockly.
          </p>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hover:-translate-y-1 transition-all md:hidden"
      >
        <div class="card-body space-y-3">
          <div
            class="flex items-center justify-between text-sm text-base-content/60"
          >
            <span>Recent activity</span>
            <Activity class="h-5 w-5 text-info" />
          </div>
          <div class="space-y-2">
            <div class="flex flex-col gap-2">
              <div
                class="flex items-center justify-between rounded-xl border border-base-200 p-2"
                title="Successful updates"
              >
                <div class="flex items-center gap-2 text-sm">
                  <CheckCircle2 class="h-4 w-4 text-success" />
                  <span>Success</span>
                </div>
                <span class="font-bold text-lg">{{ recentStats.success }}</span>
              </div>
              <div
                class="flex items-center justify-between rounded-xl border border-base-200 p-2"
                title="Warnings / rollbacks"
              >
                <div class="flex items-center gap-2 text-sm">
                  <AlertTriangle class="h-4 w-4 text-warning" />
                  <span>Warnings</span>
                </div>
                <span class="font-bold text-lg">{{ recentStats.warning }}</span>
              </div>
              <div
                class="flex items-center justify-between rounded-xl border border-base-200 p-2"
                title="Failed updates"
              >
                <div class="flex items-center gap-2 text-sm">
                  <AlertCircle class="h-4 w-4 text-error" />
                  <span>Failed</span>
                </div>
                <span class="font-bold text-lg">{{ recentStats.error }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-3">
      <div
        class="card bg-base-100 shadow-lg border border-base-200 lg:col-span-2"
      >
        <div class="card-body space-y-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-base-content/60">Automation runway</p>
              <h3 class="text-xl font-semibold">Deployment cadence</h3>
            </div>
            <div class="badge badge-outline badge-primary gap-2 text-[0.7rem]">
              <Play class="h-3.5 w-3.5" />
              Last 7 days
            </div>
          </div>
          <div class="grid grid-cols-7 gap-3 items-end h-full">
            <div
              v-for="(bucket, idx) in deploymentCadence"
              :key="`bar-${idx}`"
              class="relative flex flex-col justify-end rounded-xl bg-base-200/80 border border-base-300 transition-all duration-300 hover:border-primary/40 group h-full min-h-50"
            >
              <div class="flex flex-col-reverse h-40 px-2 pt-2 gap-1 h-full">
                <div
                  v-if="bucket.heights.errorAgent > 4"
                  class="relative w-full rounded-lg bg-error/80 transition"
                  :style="{ height: `${bucket.heights.errorAgent}%` }"
                  :title="`Agent errors: ${bucket.errorAgent}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.errorAgent }}
                  </span>
                </div>
                <div
                  v-if="bucket.heights.errorLocal > 4"
                  class="relative w-full rounded-lg bg-error/60 transition"
                  :style="{ height: `${bucket.heights.errorLocal}%` }"
                  :title="`Local errors: ${bucket.errorLocal}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.errorLocal }}
                  </span>
                </div>
                <div
                  v-if="bucket.heights.warningAgent > 4"
                  class="relative w-full rounded-lg bg-warning/70 transition"
                  :style="{ height: `${bucket.heights.warningAgent}%` }"
                  :title="`Agent warnings: ${bucket.warningAgent}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.warningAgent }}
                  </span>
                </div>
                <div
                  v-if="bucket.heights.warningLocal > 4"
                  class="relative w-full rounded-lg bg-warning/60 transition"
                  :style="{ height: `${bucket.heights.warningLocal}%` }"
                  :title="`Local warnings: ${bucket.warningLocal}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.warningLocal }}
                  </span>
                </div>
                <div
                  v-if="bucket.heights.successAgent > 4"
                  class="relative w-full rounded-lg bg-info/60 transition"
                  :style="{ height: `${bucket.heights.successAgent}%` }"
                  :title="`Agent success: ${bucket.successAgent}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.successAgent }}
                  </span>
                </div>
                <div
                  v-if="bucket.heights.successLocal > 4"
                  class="relative w-full rounded-lg bg-success/70 transition"
                  :style="{ height: `${bucket.heights.successLocal}%` }"
                  :title="`Local success: ${bucket.successLocal}`"
                >
                  <span
                    class="absolute inset-0 flex items-center justify-center text-[0.65rem] font-semibold text-white opacity-0 group-hover:opacity-100 transition pointer-events-none"
                  >
                    {{ bucket.successLocal }}
                  </span>
                </div>
                <div
                  v-if="bucket.total === 0"
                  class="w-full h-1 bg-base-300 rounded-full"
                ></div>
              </div>
              <div class="p-2 text-center space-y-1">
                <div class="text-[0.75rem] font-semibold text-base-content/70">
                  {{ bucket.total }}
                </div>
                <div class="text-[0.7rem] text-base-content/50">
                  {{ bucket.label }}
                </div>
              </div>
            </div>
          </div>
          <div
            class="flex flex-wrap items-center gap-3 text-[0.7rem] text-base-content/60"
          >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-success/70"></span>
              Local success</span
            >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-info/60"></span>
              Agent success</span
            >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-warning/60"></span>
              Local warnings</span
            >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-warning/70"></span>
              Agent warnings</span
            >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-error/60"></span>
              Local errors</span
            >
            <span class="flex items-center gap-1"
              ><span class="inline-block h-3 w-3 rounded bg-error/80"></span>
              Agent errors</span
            >
            <span class="text-[0.7rem] text-base-content/50"
              >Counts per 24h day (last 7 days).</span
            >
          </div>
        </div>
      </div>

      <div
        class="card bg-base-100 shadow-lg border border-base-200 hidden md:block"
      >
        <div class="card-body space-y-5">
          <div class="flex items-center justify-between">
            <h3 class="text-lg font-semibold">Recent activity</h3>
            <span class="text-xs text-base-content/60"></span>
          </div>
          <div class="grid grid-cols-3 gap-3">
            <div
              class="flex items-center justify-between rounded-xl border border-base-200 p-3"
              title="Successful updates"
            >
              <div class="flex items-center gap-2">
                <CheckCircle2 class="h-5 w-5 text-success" />
                <span class="sr-only">Success</span>
              </div>
              <span class="text-xl font-bold">{{ recentStats.success }}</span>
            </div>
            <div
              class="flex items-center justify-between rounded-xl border border-base-200 p-3"
              title="Warnings / rollbacks"
            >
              <div class="flex items-center gap-2">
                <AlertTriangle class="h-5 w-5 text-warning" />
                <span class="sr-only">Warnings</span>
              </div>
              <span class="text-xl font-bold">{{ recentStats.warning }}</span>
            </div>
            <div
              class="flex items-center justify-between rounded-xl border border-base-200 p-3"
              title="Failed updates"
            >
              <div class="flex items-center gap-2">
                <AlertCircle class="h-5 w-5 text-error" />
                <span class="sr-only">Failed</span>
              </div>
              <span class="text-xl font-bold">{{ recentStats.error }}</span>
            </div>
          </div>
          <div class="space-y-3">
            <p class="text-xs text-base-content/60">
              Latest updates (showing {{ Math.min(activityFeed.length, 2) }}):
            </p>
            <ul class="space-y-3">
              <li
                v-for="item in activityFeed.slice(0, 2)"
                :key="item.label + item.time"
                class="flex items-start gap-3 rounded-xl border border-base-200/70 p-3"
              >
                <span
                  class="mt-1 inline-flex h-2.5 w-2.5 rounded-full"
                  :class="{
                    'bg-success': item.tone === 'success',
                    'bg-warning': item.tone === 'warning',
                    'bg-error': item.tone === 'error',
                    'bg-info': item.tone === 'info',
                    'bg-base-content/30': item.tone === 'neutral',
                  }"
                ></span>
                <div class="flex-1">
                  <div
                    class="flex items-center justify-between text-sm font-semibold"
                  >
                    <span>{{ item.label }}</span>
                    <span class="text-xs text-base-content/50">{{
                      item.time
                    }}</span>
                  </div>
                  <p class="text-xs text-base-content/60">{{ item.detail }}</p>
                </div>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bg-grid-primary {
  background-image: linear-gradient(
      to right,
      rgba(0, 0, 0, 0.05) 1px,
      transparent 1px
    ),
    linear-gradient(to bottom, rgba(0, 0, 0, 0.05) 1px, transparent 1px);
  background-size: 28px 28px;
}
</style>
