<script setup lang="ts">
import {
  ref,
  onMounted,
  onBeforeUnmount,
  reactive,
  computed,
  inject,
} from "vue";
import {
  api,
  type Agent,
  type AgentContainer,
  type Container,
} from "../services/api";
import {
  RefreshCw,
  AlertCircle,
  Download,
  CircleX,
  ArrowDownUp,
  ShieldCheck,
  Play,
  Pause,
  Sparkles,
  ChevronDown,
  ChevronUp,
  X,
  Check,
  MoreVertical,
  Square,
  FileText,
} from "lucide-vue-next";
import { useToast } from "vue-toastification";

interface Progress {
  status?: string;
  message?: string;
  progressDetail?: {
    current: number;
    total: number;
  };
  id?: string;
  error?: string;
}

const containers = ref<Container[]>([]);
const agentContainers = ref<Agent[]>([]);
const hostInfo = reactive({
  dockerVersion: "",
  platform: "",
  hostname: "",
  lastSeen: "",
});
const loading = ref(true);
const loadingAgents = ref(false);
const lastUpdated = ref<Date | null>(null);
const formatTime = inject<
  (value: string | number | Date | null | undefined) => string
>("formatAppTime", (value) => {
  if (!value) return "--:--:--";
  const date = value instanceof Date ? value : new Date(value);
  return date.toLocaleTimeString();
});
const checkingUpdate = reactive<Record<string, boolean>>({});
const installing = reactive<Record<string, boolean>>({});
const containerAction = reactive<Record<string, string | null>>({});
const agentCheckingUpdate = reactive<Record<string, boolean>>({});
const agentInstalling = reactive<Record<string, boolean>>({});
const agentAutoUpdating = reactive<Record<string, boolean>>({});
const agentCheckStartedAt = reactive<Record<string, number>>({});
const agentInstallStartedAt = reactive<Record<string, number>>({});
const agentUpdateSnapshot = reactive<Record<string, boolean | undefined>>({});
const agentPendingCheckName = reactive<Record<string, string>>({});
const agentPendingRefresh = ref<number | null>(null);
const toast = useToast();
const logsModalOpen = ref(false);
const logsModalContent = ref("");
const logsModalTitle = ref("");
const progress = reactive(new Map<string, Progress[]>());
const refreshTimer = ref<number | null>(null);
const REFRESH_INTERVAL = 30000;
const updateAvailableOverrides = reactive<Record<string, boolean>>({});
const autoRefreshEnabled = ref(true);
const quickActionState = reactive<{
  type: "local" | "agent" | null;
  container: Container | AgentContainer | null;
  agent: Agent | null;
  top: number;
  left: number;
}>({
  type: null,
  container: null,
  agent: null,
  top: 0,
  left: 0,
});
const hasInFlightActions = computed(
  () =>
    Object.values(checkingUpdate).some(Boolean) ||
    Object.values(installing).some(Boolean)
);

const agentOnline = (agent: Agent) => {
  if (!agent.lastSeen) return false;
  const ts = new Date(agent.lastSeen).getTime();
  return Date.now() - ts < 5 * 60 * 1000;
};

const agentsWithContainers = computed(() =>
  [...agentContainers.value]
    .filter((a) => (a.containers?.length ?? 0) > 0)
    .sort((a, b) => Number(agentOnline(b)) - Number(agentOnline(a)))
);

const containersCollapsed = ref(true);
const agentCollapsed = reactive<Record<string, boolean>>({});

const toggleContainersCollapse = () => {
  containersCollapsed.value = !containersCollapsed.value;
};

const toggleAgentCollapse = (agentId: string) => {
  agentCollapsed[agentId] = !agentCollapsed[agentId];
};

const statusLabel = (val: "all" | "running" | "stopped" | undefined) => {
  if (val === "running") return "Running";
  if (val === "stopped") return "Stopped";
  return "All";
};

const autoUpdateLabel = (val: "all" | "enabled" | "disabled" | undefined) => {
  if (val === "enabled") return "Enabled";
  if (val === "disabled") return "Disabled";
  return "All";
};

const filterText = ref("");
const statusFilter = ref<"all" | "running" | "stopped">("all");
const autoUpdateFilter = ref<"all" | "enabled" | "disabled">("all");
const agentFilterText = reactive<Record<string, string>>({});
const agentStatusFilter = reactive<
  Record<string, "all" | "running" | "stopped">
>({});
const agentAutoUpdateFilter = reactive<
  Record<string, "all" | "enabled" | "disabled">
>({});
const agentSortBy = reactive<Record<string, keyof AgentContainer | null>>({});
const agentSortOrder = reactive<Record<string, "asc" | "desc">>({});
const sortBy = ref<keyof Container | null>(null);
const sortOrder = ref<"asc" | "desc">("asc");

const fleetStats = computed(() => {
  const onlineAgents = agentContainers.value.filter((a) => agentOnline(a));
  const agentList = onlineAgents.flatMap((a) => a.containers ?? []);
  const running =
    containers.value.filter((c) => c.State === "running").length +
    agentList.filter((c) => c?.state === "running").length;
  const stopped =
    containers.value.filter((c) => c.State !== "running").length +
    agentList.filter((c) => c?.state !== "running").length;
  const autoUpdate =
    containers.value.filter((c) => c.AutoUpdate).length +
    agentList.filter((c) => c?.autoUpdate).length;
  return { running, stopped, autoUpdate };
});

const filteredContainers = computed(() => {
  let filtered = containers.value;

  if (filterText.value) {
    filtered = filtered.filter((c) =>
      c.Name.toLowerCase().includes(filterText.value.toLowerCase())
    );
  }

  if (statusFilter.value !== "all") {
    if (statusFilter.value === "running") {
      filtered = filtered.filter((c) => c.State === "running");
    } else {
      filtered = filtered.filter((c) => c.State !== "running");
    }
  }

  if (autoUpdateFilter.value !== "all") {
    if (autoUpdateFilter.value === "enabled") {
      filtered = filtered.filter((c) => c.AutoUpdate);
    } else {
      filtered = filtered.filter((c) => !c.AutoUpdate);
    }
  }

  if (sortBy.value) {
    filtered.sort((a, b) => {
      const aVal = (a[sortBy.value!] ?? "") as any;
      const bVal = (b[sortBy.value!] ?? "") as any;
      if (aVal < bVal) return sortOrder.value === "asc" ? -1 : 1;
      if (aVal > bVal) return sortOrder.value === "asc" ? 1 : -1;
      return 0;
    });
  }

  return filtered;
});

const sort = (key: keyof Container) => {
  if (sortBy.value === key) {
    sortOrder.value = sortOrder.value === "asc" ? "desc" : "asc";
  } else {
    sortBy.value = key;
    sortOrder.value = "asc";
  }
};

const fetchContainers = async (options: { silent?: boolean } = {}) => {
  const { silent = false } = options;
  if (!silent) {
    loading.value = true;
  }
  try {
    containers.value = await api.getContainers();
    containers.value.forEach((c) => {
      if (installing[c.ID]) {
        updateAvailableOverrides[c.ID] = true;
        c.UpdateAvailable = true;
      } else if (updateAvailableOverrides[c.ID] !== undefined) {
        c.UpdateAvailable = updateAvailableOverrides[c.ID];
      } else if (c.UpdateAvailable) {
        updateAvailableOverrides[c.ID] = true;
      }
    });
    lastUpdated.value = new Date();
  } catch (error) {
    console.error("Failed to fetch containers:", error);
    toast.error("Failed to load containers.");
  } finally {
    if (!silent) {
      loading.value = false;
    }
  }
};

const fetchAgentContainers = async (options: { silent?: boolean } = {}) => {
  const { silent = false } = options;
  if (!silent) {
    loadingAgents.value = true;
  }
  try {
    const agents = await api.getAgents();
    agentContainers.value = agents;
    let needsFollowUp = false;
    agents.forEach((agent) => {
      if (agentCollapsed[agent.id] === undefined) {
        agentCollapsed[agent.id] = true;
      }
      if (agentSortBy[agent.id] === undefined) {
        agentSortBy[agent.id] = null;
      }
      if (agentSortOrder[agent.id] === undefined) {
        agentSortOrder[agent.id] = "asc";
      }
      if (agentFilterText[agent.id] === undefined) {
        agentFilterText[agent.id] = "";
      }
      if (agentStatusFilter[agent.id] === undefined) {
        agentStatusFilter[agent.id] = "all";
      }
      if (agentAutoUpdateFilter[agent.id] === undefined) {
        agentAutoUpdateFilter[agent.id] = "all";
      }
      agent.containers?.forEach((c) => {
        const key = agentActionKey(agent.id, c?.id);
        const prevUpdate = agentUpdateSnapshot[key];
        agentUpdateSnapshot[key] = c?.updateAvailable;
        if (agentCheckingUpdate[key]) {
          const started = agentCheckStartedAt[key] ?? 0;
          const hasError =
            c?.state === "error" &&
            c?.checkedAt &&
            new Date(c.checkedAt).getTime() > started;
          const hasResult =
            !hasError &&
            typeof c?.updateAvailable === "boolean" &&
            c?.checkedAt &&
            new Date(c.checkedAt).getTime() > started;
          if (hasError) {
            const pendingName = agentPendingCheckName[key];
            const message =
              c?.status ||
              `Check failed for ${
                pendingName || c?.name || c?.id || "container"
              }.`;
            toast.error(message);
            agentCheckingUpdate[key] = false;
            delete agentCheckStartedAt[key];
            delete agentPendingCheckName[key];
          } else if (hasResult) {
            const pendingName = agentPendingCheckName[key];
            if (pendingName) {
              if (c?.updateAvailable) {
                toast.info(`Update available for ${pendingName}.`);
              } else {
                toast.success(`No update available for ${pendingName}.`);
              }
            }
            agentCheckingUpdate[key] = false;
            delete agentCheckStartedAt[key];
            delete agentPendingCheckName[key];
          }
        }
        const updateFlagChanged =
          prevUpdate !== undefined && c?.updateAvailable !== prevUpdate;
        if (agentInstalling[key] && updateFlagChanged) {
          agentInstalling[key] = false;
          delete agentInstallStartedAt[key];
          needsFollowUp = true;
        }
      });
    });
    if (needsFollowUp) {
      refreshAgentsSoon();
    }
  } catch (error) {
    console.error("Failed to fetch agent containers:", error);
    toast.error("Failed to load agent containers.");
  } finally {
    if (!silent) {
      loadingAgents.value = false;
    }
    if (hasPendingAgentActions()) {
      startPendingAgentRefresh();
    } else {
      stopPendingAgentRefresh();
    }
  }
};

const fetchHostInfo = async () => {
  try {
    const info = await api.getHostInfo();
    hostInfo.dockerVersion = info.dockerVersion;
    hostInfo.platform = info.platform;
    hostInfo.hostname = info.hostname;
    hostInfo.lastSeen = info.lastSeen;
  } catch (error) {
    console.error("Failed to load host info", error);
  }
};

const checkUpdate = async (container: Container | null | undefined) => {
  if (!container) return;
  checkingUpdate[container.ID] = true;
  updateAvailableOverrides[container.ID] = false;
  try {
    const response = await api.checkContainerUpdate(container.ID);
    container.UpdateAvailable = response.updateAvailable;
    updateAvailableOverrides[container.ID] = response.updateAvailable;
    if (response.updateAvailable) {
      toast.info(`Update available for ${container.Name}.`);
    } else {
      toast.success(`No update available for ${container.Name}.`);
    }
  } catch (error) {
    console.error("Failed to check for updates:", error);
    toast.error(`Failed to check for updates for ${container.Name}.`);
  } finally {
    checkingUpdate[container.ID] = false;
  }
};

const installUpdate = (container: Container) => {
  progress.set(container.ID, []);
  installing[container.ID] = true;
  updateAvailableOverrides[container.ID] = true;
  api.updateContainer(container.ID, (data: Progress) => {
    if (data.error) {
      installing[container.ID] = false;
      toast.error(data.error);
      return;
    }

    if (data.message) {
      installing[container.ID] = false;
      toast.success(data.message);
      // clear cached update flag after a successful install
      updateAvailableOverrides[container.ID] = false;
      container.UpdateAvailable = false;
      void fetchContainers({ silent: true });
      return;
    }

    const p = progress.get(container.ID);
    if (p) {
      p.push(data);
      if (p.length > 8) {
        p.shift();
      }
    }
  });
};

const toggleAutoUpdate = async (container: Container) => {
  const newValue = !container.AutoUpdate;
  try {
    await api.toggleAutoUpdate(container.ID, newValue);
    container.AutoUpdate = newValue;
    toast.success(
      `'Auto-update' for ${container.Name} is now ${
        newValue ? "enabled" : "disabled"
      }.`
    );
  } catch (error) {
    console.error("Failed to toggle auto-update:", error);
    toast.error(`Failed to change auto-update for ${container.Name}.`);
  }
};

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
  if (hasInFlightActions.value) return;
  if (loading.value) return;
  void fetchContainers({ silent: true });
  void fetchAgentContainers({ silent: true });
};

const hasPendingAgentActions = () =>
  Object.values(agentCheckingUpdate).some(Boolean) ||
  Object.values(agentInstalling).some(Boolean);

const startPendingAgentRefresh = () => {
  if (agentPendingRefresh.value) return;
  agentPendingRefresh.value = window.setInterval(
    () => void fetchAgentContainers({ silent: true }),
    3000
  );
};

const stopPendingAgentRefresh = () => {
  if (agentPendingRefresh.value) {
    window.clearInterval(agentPendingRefresh.value);
    agentPendingRefresh.value = null;
  }
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

const closeQuickAction = () => {
  quickActionState.type = null;
  quickActionState.container = null;
  quickActionState.agent = null;
};

const openLocalQuickAction = (container: Container, event: MouseEvent) => {
  if (
    quickActionState.type === "local" &&
    (quickActionState.container as Container | null)?.ID === container.ID
  ) {
    closeQuickAction();
    return;
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const width = 176;
  const offset = 4;
  let left = rect.right - width;
  left = Math.max(8, Math.min(window.innerWidth - width - 8, left));
  let top = rect.bottom + offset;
  const menuHeight = 200;
  if (top + menuHeight > window.innerHeight - 8) {
    top = rect.top - menuHeight - offset;
    if (top < 8) {
      top = window.innerHeight - menuHeight - 8;
    }
  }
  quickActionState.type = "local";
  quickActionState.container = container;
  quickActionState.agent = null;
  quickActionState.top = top;
  quickActionState.left = left;
};

const openAgentQuickAction = (
  agent: Agent,
  container: AgentContainer,
  event: MouseEvent
) => {
  if (
    quickActionState.type === "agent" &&
    (quickActionState.container as AgentContainer | null)?.id ===
      container.id &&
    (quickActionState.agent as Agent | null)?.id === agent.id
  ) {
    closeQuickAction();
    return;
  }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const width = 176;
  const offset = 4;
  let left = rect.right - width;
  left = Math.max(8, Math.min(window.innerWidth - width - 8, left));
  let top = rect.bottom + offset;
  const menuHeight = 200;
  if (top + menuHeight > window.innerHeight - 8) {
    top = rect.top - menuHeight - offset;
    if (top < 8) {
      top = window.innerHeight - menuHeight - 8;
    }
  }
  quickActionState.type = "agent";
  quickActionState.container = container;
  quickActionState.agent = agent;
  quickActionState.top = top;
  quickActionState.left = left;
};

const triggerQuickActionCheckLocal = async () => {
  const target = quickActionState.container as Container | null;
  if (!target) return;
  await checkUpdate(target);
  closeQuickAction();
};

const triggerQuickActionCheckAgent = async () => {
  const target = quickActionState.container as AgentContainer | null;
  const agent = quickActionState.agent as Agent | null;
  if (!target || !agent) return;
  await checkAgentContainer(agent, target);
  closeQuickAction();
};

const performContainerAction = async (
  container: Container,
  action: "start" | "stop" | "restart" | "logs"
) => {
  const id = container.ID;
  containerAction[id] = action;
  try {
    if (action === "start") {
      await api.startContainer(id);
      toast.success(`Started ${container.Name}`);
    } else if (action === "stop") {
      await api.stopContainer(id);
      toast.success(`Stopped ${container.Name}`);
    } else if (action === "restart") {
      await api.restartContainer(id);
      toast.success(`Restarted ${container.Name}`);
    } else if (action === "logs") {
      logsModalTitle.value = `${
        container.Name || container.ID
      } · Last 200 logs`;
      logsModalContent.value = "Loading logs...";
      logsModalOpen.value = true;
      const { logs } = await api.getContainerLogs(id, 200);
      logsModalContent.value = logs || "No logs available.";
      logsModalOpen.value = true;
    }
    await fetchContainers({ silent: true });
  } catch (error) {
    console.error(`Failed to ${action} container`, error);
    toast.error(`Unable to ${action} ${container.Name}`);
  } finally {
    containerAction[id] = null;
    closeQuickAction();
  }
};

const copyLogs = async () => {
  try {
    await navigator.clipboard.writeText(logsModalContent.value || "");
    toast.success("Logs copied to clipboard");
  } catch (error) {
    try {
      const el = document.createElement("textarea");
      el.value = logsModalContent.value || "";
      el.style.position = "fixed";
      el.style.left = "-9999px";
      document.body.appendChild(el);
      el.select();
      document.execCommand("copy");
      document.body.removeChild(el);
      toast.success("Logs copied to clipboard");
    } catch (fallbackErr) {
      console.error("Failed to copy logs", error, fallbackErr);
      toast.error("Unable to copy logs");
    }
  }
};

onMounted(() => {
  void fetchContainers();
  void fetchAgentContainers();
  void fetchHostInfo();
  startAutoRefresh();
});

onBeforeUnmount(() => {
  stopAutoRefresh();
});

const agentActionKey = (agentId: string, containerId?: string) =>
  `${agentId}:${containerId ?? ""}`;

const performAgentContainerAction = async (
  agent: Agent,
  container: AgentContainer,
  action: "start" | "stop" | "restart" | "logs"
) => {
  if (!container?.id) return;
  const key = agentActionKey(agent.id, container.id);
  agentInstalling[key] = action === "restart";
  try {
    if (action === "start") {
      await api.startAgentContainer(agent.id, container.id);
      toast.success(`Start requested for ${container.name || container.id}`);
    } else if (action === "stop") {
      await api.stopAgentContainer(agent.id, container.id);
      toast.success(`Stop requested for ${container.name || container.id}`);
    } else if (action === "restart") {
      await api.restartAgentContainer(agent.id, container.id);
      toast.success(`Restart requested for ${container.name || container.id}`);
    } else if (action === "logs") {
      logsModalTitle.value = `${
        container.name || container.id
      } · Last 200 logs`;
      logsModalContent.value = "Loading logs...";
      logsModalOpen.value = true;
      const { logs } = await api.getAgentContainerLogs(
        agent.id,
        container.id,
        200
      );
      logsModalContent.value = logs || "No logs available.";
      logsModalOpen.value = true;
    }
    refreshAgentsSoon();
  } catch (error) {
    console.error(`Failed to ${action} agent container`, error);
    toast.error(`Unable to ${action} ${container.name || container.id}`);
  } finally {
    agentInstalling[key] = false;
  }
};

const refreshAgentsSoon = () => {
  window.setTimeout(() => void fetchAgentContainers({ silent: true }), 1500);
};

const checkAgentContainer = async (agent: Agent, container: AgentContainer) => {
  const key = agentActionKey(agent.id, container?.id);
  agentCheckingUpdate[key] = true;
  agentCheckStartedAt[key] = Date.now();
  agentPendingCheckName[key] = container?.name || container?.id || "container";
  try {
    await api.createAgentCommand(agent.id, "check-update", container?.id ?? "");
    toast.success(
      `Check requested for ${container?.name ?? "container"} on ${agent.name}.`
    );
    refreshAgentsSoon();
    startPendingAgentRefresh();
  } catch (error) {
    console.error("Failed to enqueue agent check update:", error);
    toast.error(
      `Failed to check updates for ${container?.name ?? "container"}.`
    );
    agentCheckingUpdate[key] = false;
    delete agentPendingCheckName[key];
  } finally {
  }
};

const installAgentContainer = async (
  agent: Agent,
  container: AgentContainer
) => {
  const key = agentActionKey(agent.id, container?.id);
  agentInstalling[key] = true;
  agentInstallStartedAt[key] = Date.now();
  try {
    await api.createAgentCommand(
      agent.id,
      "update-container",
      container?.id ?? ""
    );
    toast.success(
      `Update queued for ${container?.name ?? "container"} on ${agent.name}.`
    );
    refreshAgentsSoon();
    startPendingAgentRefresh();
  } catch (error) {
    console.error("Failed to enqueue agent update:", error);
    toast.error(
      `Failed to install update for ${container?.name ?? "container"}.`
    );
  }
};

const toggleAgentAutoUpdate = async (
  agent: Agent,
  container: AgentContainer
) => {
  const key = agentActionKey(agent.id, container?.id);
  const newValue = !container?.autoUpdate;
  agentAutoUpdating[key] = true;
  try {
    await api.toggleAgentContainerAutoUpdate(
      agent.id,
      container?.id ?? "",
      newValue
    );
    container.autoUpdate = newValue;
    toast.success(
      `'Auto-update' for ${container?.name ?? "container"} is now ${
        newValue ? "enabled" : "disabled"
      } on ${agent.name}.`
    );
    refreshAgentsSoon();
  } catch (error) {
    console.error("Failed to toggle agent auto-update:", error);
    toast.error(
      `Failed to change auto-update for ${container?.name ?? "container"}.`
    );
  } finally {
    agentAutoUpdating[key] = false;
  }
};

const agentContainerState = (
  agent: Agent,
  container?: AgentContainer
): string => {
  if (!agentOnline(agent)) return "unknown";
  return container?.state ?? "unknown";
};

const agentContainerStatusText = (
  agent: Agent,
  container?: AgentContainer
): string => {
  if (!agentOnline(agent)) return "unknown";
  if (container?.state === "error") return "error";
  return container?.status || container?.state || "unknown";
};

const agentFilteredContainers = (agent: Agent) => {
  let filtered = [...(agent.containers ?? [])];
  const text = (agentFilterText[agent.id] ?? "").toLowerCase();
  const status = agentStatusFilter[agent.id] ?? "all";
  const auto = agentAutoUpdateFilter[agent.id] ?? "all";

  if (text) {
    filtered = filtered.filter((c) =>
      (c?.name || c?.id || "").toLowerCase().includes(text)
    );
  }

  if (status !== "all") {
    filtered =
      status === "running"
        ? filtered.filter((c) => agentContainerState(agent, c) === "running")
        : filtered.filter((c) => agentContainerState(agent, c) !== "running");
  }

  if (auto !== "all") {
    filtered =
      auto === "enabled"
        ? filtered.filter((c) => c?.autoUpdate)
        : filtered.filter((c) => !c?.autoUpdate);
  }

  const sortKey = agentSortBy[agent.id];
  if (sortKey) {
    const order = agentSortOrder[agent.id] ?? "asc";
    filtered.sort((a, b) => {
      const aVal =
        sortKey === "state"
          ? agentContainerState(agent, a) ?? "" // ensure offline uses unknown
          : ((a?.[sortKey] ?? "") as any);
      const bVal =
        sortKey === "state"
          ? agentContainerState(agent, b) ?? ""
          : ((b?.[sortKey] ?? "") as any);
      if (aVal < bVal) return order === "asc" ? -1 : 1;
      if (aVal > bVal) return order === "asc" ? 1 : -1;
      return 0;
    });
  }

  return filtered;
};

const sortAgentContainers = (agentId: string, key: keyof AgentContainer) => {
  if (agentSortBy[agentId] === key) {
    agentSortOrder[agentId] =
      agentSortOrder[agentId] === "asc" ? "desc" : "asc";
  } else {
    agentSortBy[agentId] = key;
    agentSortOrder[agentId] = "asc";
  }
};
</script>

<template>
  <div class="space-y-6 w-full">
    <div
      class="relative overflow-hidden rounded-3xl border border-base-200 bg-gradient-to-r from-primary/10 via-secondary/10 to-accent/10 p-6 shadow-xl"
    >
      <div
        class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between"
      >
        <div class="space-y-2">
          <div
            class="inline-flex items-center gap-2 rounded-full bg-base-100/80 px-3 py-1 text-xs font-semibold shadow"
          >
            <Sparkles class="h-4 w-4 text-primary" />
            Fleet status
          </div>
          <h1 class="text-3xl font-bold pr-36 md:pr-0">Containers</h1>
          <p class="text-sm text-base-content/70 max-w-2xl">
            Observe running services, trigger updates, and keep auto-update in
            sync.
          </p>
          <div class="flex flex-wrap gap-3">
            <div class="badge badge-success gap-2">
              <Play class="h-3.5 w-3.5" /> Running:
              {{ fleetStats.running }}
            </div>
            <div class="badge badge-neutral gap-2">
              <Pause class="h-3.5 w-3.5" /> Stopped:
              {{ fleetStats.stopped }}
            </div>
            <div class="badge badge-info gap-2">
              <ShieldCheck class="h-3.5 w-3.5" /> Auto-update:
              {{ fleetStats.autoUpdate }}
            </div>
          </div>
        </div>
        <div
          class="absolute top-6 right-6 flex flex-col items-end gap-2 md:static md:self-start"
        >
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
              @click="fetchContainers({ silent: true })"
              :disabled="loading"
              aria-label="Refresh containers"
              title="Refresh containers"
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

    <div v-if="loading" class="text-center py-12">
      <p>Loading containers...</p>
    </div>

    <div
      class="rounded-2xl border border-base-200 shadow-lg bg-base-100 relative overflow-visible"
      v-else
    >
      <div
        class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between px-4 py-3 border-b border-base-200 cursor-pointer"
        @click="toggleContainersCollapse"
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
                {{
                  hostInfo.lastSeen ? formatTime(hostInfo.lastSeen) : "never"
                }}
              </div>
            </div>
          </div>
        </div>
        <div
          class="flex items-center gap-2 mt-2 sm:mt-0 self-start sm:self-center"
        >
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
                v-model="filterText"
              />
              <button
                v-if="filterText"
                class="btn btn-ghost btn-xs absolute right-1 top-1/2 -translate-y-1/2"
                aria-label="Clear search"
                @click="filterText = ''"
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
                      <span class="truncate"
                        >Status: {{ statusLabel(statusFilter) }}</span
                      >
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
                            <span>All Statuses</span>
                          </div>
                        </button>
                      </li>
                      <li>
                        <button type="button" @click="statusFilter = 'running'">
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
                        <button type="button" @click="statusFilter = 'stopped'">
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
                      <span class="truncate"
                        >Auto-update:
                        {{ autoUpdateLabel(autoUpdateFilter) }}</span
                      >
                      <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                    </label>
                    <ul
                      tabindex="0"
                      class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-56 border border-base-200 z-[1]"
                    >
                      <li>
                        <button type="button" @click="autoUpdateFilter = 'all'">
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
                        <button
                          type="button"
                          @click="autoUpdateFilter = 'enabled'"
                        >
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
                        <button
                          type="button"
                          @click="autoUpdateFilter = 'disabled'"
                        >
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
                    @click="autoUpdateFilter = 'all'"
                  >
                    <X class="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
            <div
              class="ml-auto text-xs text-base-content/60 w-full sm:w-auto text-right"
            >
              Showing {{ filteredContainers.length }} of
              {{ containers.length }} records
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
                  @click="sort('State')"
                  class="cursor-pointer w-12 sm:w-28 lg:w-54 p-2 sm:p-4"
                >
                  <div
                    class="flex items-center justify-center sm:justify-start gap-2"
                  >
                    <span class="hidden sm:inline">Status</span>
                    <ArrowDownUp class="w-4 h-4 shrink-0" />
                  </div>
                </th>

                <th @click="sort('Name')" class="cursor-pointer">
                  <div class="flex items-center gap-2">
                    Name <ArrowDownUp class="w-4 h-4 shrink-0" />
                  </div>
                </th>

                <th
                  @click="sort('AutoUpdate')"
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
              <template
                v-for="container in filteredContainers"
                :key="container.ID"
              >
                <tr class="hover group">
                  <td class="p-2 sm:p-4">
                    <div
                      class="flex items-center justify-center sm:justify-start gap-2"
                    >
                      <RefreshCw
                        v-if="
                          installing[container.ID] ||
                          checkingUpdate[container.ID]
                        "
                        class="w-5 h-5 text-primary animate-spin shrink-0"
                        title="Updating"
                      />
                      <AlertCircle
                        v-else-if="container.UpdateAvailable"
                        class="w-5 h-5 text-warning shrink-0"
                        title="Update Available"
                      />
                      <span
                        v-else
                        class="h-3 w-3 rounded-full shrink-0"
                        :class="{
                          'bg-success': container.State === 'running',
                          'bg-error': container.State !== 'running',
                        }"
                      ></span>

                      <span class="font-medium hidden sm:block truncate">
                        {{ container.Status }}
                      </span>
                    </div>
                  </td>

                  <td class="max-w-0 align-middle">
                    <div class="flex flex-col justify-center">
                      <div
                        class="font-mono font-bold truncate text-sm sm:text-base"
                        :title="container.Name"
                      >
                        {{ container.Name }}
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
                        @click="toggleAutoUpdate(container)"
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
                        >
                          <path d="M20 6 9 17l-5-5"></path>
                        </g>
                      </svg>
                    </label>
                  </td>

                  <td class="text-right p-2 sm:p-4">
                    <div class="flex justify-end items-center gap-1 relative">
                      <button
                        v-if="
                          container.UpdateAvailable || installing[container.ID]
                        "
                        class="btn btn-sm btn-info px-2 sm:px-3"
                        @click="installUpdate(container)"
                        :disabled="installing[container.ID]"
                        title="Install Update"
                      >
                        <Download
                          v-if="!installing[container.ID]"
                          class="w-4 h-4"
                        />
                        <RefreshCw v-else class="w-4 h-4 animate-spin" />
                        <span class="hidden sm:inline ml-1">Install</span>
                      </button>
                      <div class="relative" style="z-index: 60">
                        <button
                          class="btn btn-ghost btn-sm btn-square"
                          aria-label="Quick actions"
                          @click="(e) => openLocalQuickAction(container, e)"
                        >
                          <MoreVertical class="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="loadingAgents" class="text-sm text-base-content/60">
      Loading agents...
    </div>
    <div
      v-else-if="agentsWithContainers.length === 0"
      class="text-sm text-base-content/60"
    >
      No agent container data yet. Ensure agents have reported in.
    </div>
    <div v-else class="space-y-6">
      <div
        v-for="agent in agentsWithContainers"
        :key="agent.id"
        class="rounded-2xl border border-base-200 shadow-lg bg-base-100 relative overflow-visible"
      >
        <div
          class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between px-4 py-3 border-b border-base-200 cursor-pointer"
          @click="toggleAgentCollapse(agent.id)"
        >
          <div class="flex flex-col">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-lg font-semibold">{{ agent.name }}</span>
              <span class="text-sm text-base-content/60 sm:hidden">
                {{ agent.hostname || "unknown host" }}
              </span>
              <span
                class="inline-flex h-2 w-2 rounded-full"
                :class="agentOnline(agent) ? 'bg-success' : 'bg-base-300'"
              ></span>
            </div>
            <div class="text-xs text-base-content/60 mt-1 hidden sm:block">
              {{ agent.hostname || "unknown host" }} ·
              {{ agent.dockerVersion || "Awaiting heartbeat" }} ·
              {{ agent.platform || "platform unknown" }} · Last seen:
              {{ agent.lastSeen ? formatTime(agent.lastSeen) : "never" }}
            </div>
            <div class="text-xs text-base-content/60 mt-1 sm:hidden">
              <div class="flex flex-wrap gap-x-2">
                <div>
                  {{ agent.dockerVersion || "Awaiting heartbeat" }} ·
                  {{ agent.platform || "platform unknown" }}
                </div>
                <div>
                  Last seen:
                  {{ agent.lastSeen ? formatTime(agent.lastSeen) : "never" }}
                </div>
              </div>
            </div>
          </div>
          <div
            class="flex items-center gap-2 mt-2 sm:mt-0 self-start sm:self-center"
          >
            <span class="badge badge-ghost gap-1">
              {{ agent.containers?.length || 0 }} containers
            </span>
            <button class="btn btn-ghost btn-xs" tabindex="-1">
              <ChevronDown v-if="agentCollapsed[agent.id]" class="w-4 h-4" />
              <ChevronUp v-else class="w-4 h-4" />
            </button>
          </div>
        </div>
        <div
          class="transition-all duration-300"
          :style="
            agentCollapsed[agent.id]
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
                  class="input input-bordered input-sm rounded-xl w-full sm:min-w-[220px] pr-10"
                  v-model="agentFilterText[agent.id]"
                />
                <button
                  v-if="agentFilterText[agent.id]"
                  class="btn btn-ghost btn-xs absolute right-1 top-1/2 -translate-y-1/2"
                  aria-label="Clear search"
                  @click="agentFilterText[agent.id] = ''"
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
                        <span class="truncate">
                          Status: {{ statusLabel(agentStatusFilter[agent.id]) }}
                        </span>
                        <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                      </label>
                      <ul
                        tabindex="0"
                        class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 border border-base-200 z-[1]"
                      >
                        <li>
                          <button
                            type="button"
                            @click="agentStatusFilter[agent.id] = 'all'"
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="agentStatusFilter[agent.id] === 'all'"
                                class="w-4 h-4 text-success"
                              />
                              <span>All Statuses</span>
                            </div>
                          </button>
                        </li>
                        <li>
                          <button
                            type="button"
                            @click="agentStatusFilter[agent.id] = 'running'"
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="agentStatusFilter[agent.id] === 'running'"
                                class="w-4 h-4 text-success"
                              />
                              <span>Running</span>
                            </div>
                          </button>
                        </li>
                        <li>
                          <button
                            type="button"
                            @click="agentStatusFilter[agent.id] = 'stopped'"
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="agentStatusFilter[agent.id] === 'stopped'"
                                class="w-4 h-4 text-success"
                              />
                              <span>Stopped</span>
                            </div>
                          </button>
                        </li>
                      </ul>
                    </div>
                    <button
                      v-if="agentStatusFilter[agent.id] !== 'all'"
                      class="btn btn-ghost btn-xs hidden sm:inline-flex"
                      aria-label="Clear status filter"
                      @click="agentStatusFilter[agent.id] = 'all'"
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
                          Auto-update:
                          {{ autoUpdateLabel(agentAutoUpdateFilter[agent.id]) }}
                        </span>
                        <ChevronDown class="w-4 h-4 opacity-70 flex-shrink-0" />
                      </label>
                      <ul
                        tabindex="0"
                        class="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-56 border border-base-200 z-[1]"
                      >
                        <li>
                          <button
                            type="button"
                            @click="agentAutoUpdateFilter[agent.id] = 'all'"
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="agentAutoUpdateFilter[agent.id] === 'all'"
                                class="w-4 h-4 text-success"
                              />
                              <span>All Auto-updates</span>
                            </div>
                          </button>
                        </li>
                        <li>
                          <button
                            type="button"
                            @click="agentAutoUpdateFilter[agent.id] = 'enabled'"
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="
                                  agentAutoUpdateFilter[agent.id] === 'enabled'
                                "
                                class="w-4 h-4 text-success"
                              />
                              <span>Enabled</span>
                            </div>
                          </button>
                        </li>
                        <li>
                          <button
                            type="button"
                            @click="
                              agentAutoUpdateFilter[agent.id] = 'disabled'
                            "
                          >
                            <div class="flex items-center gap-2">
                              <Check
                                v-if="
                                  agentAutoUpdateFilter[agent.id] === 'disabled'
                                "
                                class="w-4 h-4 text-success"
                              />
                              <span>Disabled</span>
                            </div>
                          </button>
                        </li>
                      </ul>
                    </div>
                    <button
                      v-if="agentAutoUpdateFilter[agent.id] !== 'all'"
                      class="btn btn-ghost btn-xs hidden sm:inline-flex"
                      aria-label="Clear auto-update filter"
                      @click="agentAutoUpdateFilter[agent.id] = 'all'"
                    >
                      <X class="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
              <div
                class="ml-auto text-xs text-base-content/60 w-full sm:w-auto text-right"
              >
                Showing
                {{ agentFilteredContainers(agent).length }} of
                {{ agent.containers?.length || 0 }} records
              </div>
            </div>
          </div>
          <div
            v-if="agentFilteredContainers(agent).length === 0"
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
                    @click="sortAgentContainers(agent.id, 'state')"
                    class="cursor-pointer w-12 sm:w-28 lg:w-54 p-2 sm:p-4"
                  >
                    <div
                      class="flex items-center justify-center sm:justify-start gap-2"
                    >
                      <span class="hidden sm:inline">Status</span>
                      <ArrowDownUp class="w-4 h-4 shrink-0" />
                    </div>
                  </th>

                  <th
                    @click="sortAgentContainers(agent.id, 'name')"
                    class="cursor-pointer"
                  >
                    <div class="flex items-center gap-2">
                      Name <ArrowDownUp class="w-4 h-4 shrink-0" />
                    </div>
                  </th>

                  <th
                    @click="sortAgentContainers(agent.id, 'autoUpdate')"
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
                <tr
                  v-for="container in agentFilteredContainers(agent)"
                  :key="container?.id"
                  class="hover group"
                >
                  <td class="p-2 sm:p-4">
                    <div
                      class="flex items-center justify-center sm:justify-start gap-2"
                    >
                      <RefreshCw
                        v-if="
                          agentInstalling[
                            agentActionKey(agent.id, container?.id)
                          ] ||
                          agentCheckingUpdate[
                            agentActionKey(agent.id, container?.id)
                          ]
                        "
                        class="w-5 h-5 text-primary animate-spin shrink-0"
                        title="Updating"
                      />
                      <AlertCircle
                        v-else-if="
                          agentOnline(agent) && container?.updateAvailable
                        "
                        class="w-5 h-5 text-warning shrink-0"
                        title="Update Available"
                      />
                      <span
                        v-else
                        class="h-3 w-3 rounded-full shrink-0"
                        :class="{
                          'bg-success':
                            agentContainerState(agent, container) === 'running',
                          'bg-error':
                            agentContainerState(agent, container) !== 'running',
                        }"
                      ></span>

                      <span class="font-medium hidden sm:block truncate">
                        {{ agentContainerStatusText(agent, container) }}
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
                        'opacity-50 pointer-events-none': !agentOnline(agent),
                      }"
                    >
                      <input
                        type="checkbox"
                        :checked="container?.autoUpdate"
                        :disabled="
                          agentAutoUpdating[
                            agentActionKey(agent.id, container?.id)
                          ] || !agentOnline(agent)
                        "
                        @click="
                          toggleAgentAutoUpdate(
                            agent,
                            container as AgentContainer
                          )
                        "
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
                        >
                          <path d="M20 6 9 17l-5-5"></path>
                        </g>
                      </svg>
                    </label>
                  </td>

                  <td class="text-right p-2 sm:p-4">
                    <div class="flex justify-end items-center gap-1 relative">
                      <button
                        v-if="
                          agentOnline(agent) &&
                          (container?.updateAvailable ||
                            agentInstalling[
                              agentActionKey(agent.id, container?.id)
                            ])
                        "
                        class="btn btn-sm btn-info px-2 sm:px-3"
                        @click="
                          installAgentContainer(
                            agent,
                            container as AgentContainer
                          )
                        "
                        :disabled="
                          agentInstalling[
                            agentActionKey(agent.id, container?.id)
                          ] || !agentOnline(agent)
                        "
                        title="Install Update"
                      >
                        <Download
                          v-if="
                            !agentInstalling[
                              agentActionKey(agent.id, container?.id)
                            ]
                          "
                          class="w-4 h-4"
                        />
                        <RefreshCw v-else class="w-4 h-4 animate-spin" />
                        <span class="hidden sm:inline ml-1">Install</span>
                      </button>
                      <div class="relative" style="z-index: 60">
                        <button
                          class="btn btn-ghost btn-sm btn-square"
                          aria-label="Quick actions"
                          @click="(e) => openAgentQuickAction(agent, container as AgentContainer, e)"
                        >
                          <MoreVertical class="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div v-if="quickActionState.type" class="fixed inset-0 z-[100]">
      <div class="absolute inset-0" @click="closeQuickAction"></div>
      <div
        class="absolute w-44 rounded-xl border border-base-200 bg-base-100 shadow-xl pointer-events-auto p-2 space-y-1"
        :style="{
          top: `${quickActionState.top}px`,
          left: `${quickActionState.left}px`,
        }"
      >
        <template
          v-if="quickActionState.type === 'local' && quickActionState.container"
        >
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="containerAction[(quickActionState.container as Container).ID] === 'start'"
            @click="
              performContainerAction(
                quickActionState.container as Container,
                'start'
              )
            "
          >
            <Play class="w-4 h-4 text-success" />
            Start
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="containerAction[(quickActionState.container as Container).ID] === 'stop'"
            @click="
              performContainerAction(
                quickActionState.container as Container,
                'stop'
              )
            "
          >
            <Square class="w-4 h-4 text-error" />
            Stop
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="containerAction[(quickActionState.container as Container).ID] === 'restart'"
            @click="
              performContainerAction(
                quickActionState.container as Container,
                'restart'
              )
            "
          >
            <RefreshCw
              class="w-4 h-4"
              :class="{
              'animate-spin':
                containerAction[(quickActionState.container as Container).ID] === 'restart',
            }"
            />
            Restart
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="containerAction[(quickActionState.container as Container).ID] === 'logs'"
            @click="
              performContainerAction(
                quickActionState.container as Container,
                'logs'
              )
            "
          >
            <FileText class="w-4 h-4" />
            Logs
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            @click="triggerQuickActionCheckLocal"
            :disabled="
            checkingUpdate[(quickActionState.container as Container).ID] ||
            installing[(quickActionState.container as Container).ID]
          "
          >
            <RefreshCw
              class="w-4 h-4"
              :class="{
              'animate-spin':
                checkingUpdate[(quickActionState.container as Container).ID],
            }"
            />
            Check updates
          </button>
        </template>
        <template
          v-else-if="
            quickActionState.type === 'agent' &&
            quickActionState.container &&
            quickActionState.agent
          "
        >
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="agentInstalling[agentActionKey(quickActionState.agent.id, (quickActionState.container as AgentContainer).id)]"
            @click="
              performAgentContainerAction(
                quickActionState.agent as Agent,
                quickActionState.container as AgentContainer,
                'start'
              )
            "
          >
            <Play class="w-4 h-4 text-success" />
            Start
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="agentInstalling[agentActionKey(quickActionState.agent.id, (quickActionState.container as AgentContainer).id)]"
            @click="
              performAgentContainerAction(
                quickActionState.agent as Agent,
                quickActionState.container as AgentContainer,
                'stop'
              )
            "
          >
            <Square class="w-4 h-4 text-error" />
            Stop
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="agentInstalling[agentActionKey(quickActionState.agent.id, (quickActionState.container as AgentContainer).id)]"
            @click="
              performAgentContainerAction(
                quickActionState.agent as Agent,
                quickActionState.container as AgentContainer,
                'restart'
              )
            "
          >
            <RefreshCw
              class="w-4 h-4"
              :class="{
              'animate-spin':
                agentInstalling[
                  agentActionKey(
                    quickActionState.agent.id,
                    (quickActionState.container as AgentContainer).id
                  )
                ],
            }"
            />
            Restart
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            @click="
              performAgentContainerAction(
                quickActionState.agent as Agent,
                quickActionState.container as AgentContainer,
                'logs'
              )
            "
          >
            <FileText class="w-4 h-4" />
            Logs
          </button>
          <button
            class="btn btn-ghost btn-sm w-full justify-start gap-2"
            :disabled="
            agentCheckingUpdate[
              agentActionKey(
                quickActionState.agent.id,
                (quickActionState.container as AgentContainer).id
              )
            ] ||
            agentInstalling[
              agentActionKey(
                quickActionState.agent.id,
                (quickActionState.container as AgentContainer).id
              )
            ]
          "
            @click="triggerQuickActionCheckAgent"
          >
            <RefreshCw
              class="w-4 h-4"
              :class="{
              'animate-spin':
                agentCheckingUpdate[
                  agentActionKey(
                    quickActionState.agent.id,
                    (quickActionState.container as AgentContainer).id
                  )
                ],
            }"
            />
            Check updates
          </button>
        </template>
      </div>
    </div>
  </Teleport>

  <dialog v-if="logsModalOpen" class="modal modal-open">
    <div class="modal-box max-w-3xl">
      <h3 class="font-bold text-lg mb-2">{{ logsModalTitle }}</h3>
      <pre
        class="bg-base-200 rounded-xl p-3 max-h-96 overflow-auto text-xs whitespace-pre-wrap"
        >{{ logsModalContent }}
      </pre>
      <div class="modal-action">
        <button class="btn btn-ghost" @click="logsModalOpen = false">
          Close
        </button>
        <button class="btn btn-primary" @click="copyLogs">Copy</button>
      </div>
    </div>
    <form method="dialog" class="modal-backdrop" @click="logsModalOpen = false">
      <button aria-label="close"></button>
    </form>
  </dialog>
</template>
