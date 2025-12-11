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
  ShieldCheck,
  Play,
  Pause,
  Sparkles,
  Square,
  FileText,
  ServerCog,
} from "lucide-vue-next";
import { useToast } from "vue-toastification";
import SectionHeader from "./SectionHeader.vue";
import PortsModal from "./containers/PortsModal.vue";
import AgentContainersList from "./containers/AgentContainersList.vue";
import LocalContainersList from "./containers/LocalContainersList.vue";

interface Progress {
  status?: string;
  message?: string;
  progressDetail?: {
    current: number;
    total: number;
  };
  id?: string;
  error?: string;
  rolledBack?: boolean;
  rollbackMessage?: string;
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
const portsModalHost = ref<string | null>(null);
const portsFilter = ref("");

const allPorts = computed(() => {
  const list: { host: string; container: string; ports: string[]; isAgent: boolean }[] = [];

  // Local
  containers.value.forEach((c) => {
    if (c.Ports && c.Ports.length > 0) {
      list.push({
        host: hostInfo.hostname || "Localhost",
        container: c.Name,
        ports: c.Ports,
        isAgent: false,
      });
    }
  });

  // Agents
  agentContainers.value.forEach((a) => {
    if (a.containers) {
      a.containers.forEach((c) => {
        if (c.ports && c.ports.length > 0) {
          list.push({
            host: a.name,
            container: c.name || c.id,
            ports: c.ports,
            isAgent: true,
          });
        }
      });
    }
  });

  return list;
});

const currentPorts = computed(() => {
  if (!portsModalHost.value) return [];
  const host = portsModalHost.value;
  const filter = portsFilter.value.toLowerCase();
  return allPorts.value.filter((p) => {
    if (p.host !== host) return false;
    if (!filter) return true;
    return (
      p.container.toLowerCase().includes(filter) ||
      p.ports.some((port) => port.toLowerCase().includes(filter))
    );
  });
});

const openPortsModal = (host: string) => {
  portsModalHost.value = host;
  portsFilter.value = "";
};

const progress = reactive(new Map<string, Progress[]>());
const refreshTimer = ref<number | null>(null);
const REFRESH_INTERVAL = 30000;
const REFRESH_JITTER = 5000;
const agentRefreshTimer = ref<number | null>(null);
const AGENT_REFRESH_INTERVAL = 5000;
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

const handleRowCheckUpdate = (container: Container) => checkUpdate(container);
const handleRowInstall = (container: Container) => installUpdate(container);
const handleRowToggleAuto = (container: Container) =>
  toggleAutoUpdate(container);
const handleRowOpenQuick = (container: Container, evt: MouseEvent) =>
  openLocalQuickAction(container, evt);
const handleAgentToggleAuto = (agent: Agent, container: AgentContainer) =>
  toggleAgentAutoUpdate(agent, container);
const handleAgentInstall = (agent: Agent, container: AgentContainer) =>
  installAgentContainer(agent, container);
const handleAgentOpenQuick = (
  agent: Agent,
  container: AgentContainer,
  evt: MouseEvent
) => openAgentQuickAction(agent, container, evt);

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
    (hostInfo as any).cpu = info.cpu;
    (hostInfo as any).memory = info.memory;
  } catch (error) {
    console.error("Failed to load host info", error);
  }
};

const refreshAll = () => {
  void fetchContainers();
  void fetchAgentContainers();
  void fetchHostInfo();
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
      if (data.rolledBack) {
        toast.info(
          data.rollbackMessage ??
            `Update failed but ${container.Name} was rolled back to its previous state.`
        );
      }
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
  if (!refreshTimer.value) {
    const baseInterval =
      REFRESH_INTERVAL + Math.floor(Math.random() * REFRESH_JITTER);
    refreshTimer.value = window.setInterval(autoRefreshTick, baseInterval);
  }
  startAgentRefresh();
};

const stopAutoRefresh = () => {
  if (refreshTimer.value) {
    window.clearInterval(refreshTimer.value);
    refreshTimer.value = null;
  }
  stopAgentRefresh();
};

const autoRefreshTick = () => {
  if (!autoRefreshEnabled.value) return;
  if (hasInFlightActions.value) return;
  if (loading.value) return;
  void fetchContainers({ silent: true });
  void fetchHostInfo();
};

const startAgentRefresh = () => {
  if (agentRefreshTimer.value) return;
  const baseInterval =
    AGENT_REFRESH_INTERVAL + Math.floor(Math.random() * REFRESH_JITTER);
  agentRefreshTimer.value = window.setInterval(agentRefreshTick, baseInterval);
};

const stopAgentRefresh = () => {
  if (agentRefreshTimer.value) {
    window.clearInterval(agentRefreshTimer.value);
    agentRefreshTimer.value = null;
  }
};

const agentRefreshTick = () => {
  if (!autoRefreshEnabled.value) return;
  if (hasInFlightActions.value) return;
  if (loadingAgents.value) return;
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
    agentRefreshTick();
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
    <SectionHeader
      title="Containers"
      :icon="ServerCog"
    >
      <template #eyebrow>
        <Sparkles class="h-4 w-4 text-primary" />
        Fleet status
      </template>
      <template #subtitle>
        Observe running services, trigger updates, and keep auto-update in sync.
      </template>
      <template #badges>
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
      </template>
      <template #meta>
        <span class="text-xs text-base-content/60">
          {{
            lastUpdated
              ? `Updated ${formatTime(lastUpdated)}`
              : "Updated --:--:--"
          }}
        </span>
      </template>
      <template #actions>
        <button
          class="btn btn-ghost btn-square"
          @click="refreshAll"
          :disabled="loading || loadingAgents"
          aria-label="Refresh containers"
          title="Refresh containers and agents"
        >
          <RefreshCw
            class="w-4 h-4"
            :class="{ 'animate-spin': loading || loadingAgents }"
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
      </template>
    </SectionHeader>

    <div v-if="loading" class="text-center py-12">
      <p>Loading containers...</p>
    </div>

    <LocalContainersList
      v-else
      :containers="containers"
      :filtered-containers="filteredContainers"
      :host-info="hostInfo as any"
      :loading="loading"
      :last-updated="lastUpdated"
      :format-time="formatTime"
      :containers-collapsed="containersCollapsed"
      :filter-text="filterText"
      :status-filter="statusFilter"
      :auto-update-filter="autoUpdateFilter"
      :installing="installing"
      :checking-update="checkingUpdate"
      :update-available-overrides="updateAvailableOverrides"
      :auto-refresh-enabled="autoRefreshEnabled"
      @toggle-collapse="toggleContainersCollapse"
      @open-ports="openPortsModal"
      @set-filter-text="(val: string) => (filterText = val)"
      @set-status-filter="(val: 'all' | 'running' | 'stopped') => (statusFilter = val)"
      @set-auto-filter="(val: 'all' | 'enabled' | 'disabled') => (autoUpdateFilter = val)"
      @sort="sort"
      @refresh="fetchContainers({ silent: true })"
      @toggle-auto-refresh="toggleAutoRefresh"
      @check-update="handleRowCheckUpdate"
      @install="handleRowInstall"
      @toggle-auto="handleRowToggleAuto"
      @open-quick="handleRowOpenQuick"
    />

    <div v-if="loadingAgents" class="text-sm text-base-content/60">
      Loading agents...
    </div>
    <div
      v-else-if="agentsWithContainers.length === 0"
      class="text-sm text-base-content/60"
    >
      No agent container data yet. Ensure agents have reported in.
    </div>
    <AgentContainersList
      v-else
      :agents="agentsWithContainers"
      :format-time="formatTime"
      :agent-collapsed="agentCollapsed"
      :agent-filter-text="agentFilterText"
      :agent-status-filter="agentStatusFilter"
      :agent-auto-update-filter="agentAutoUpdateFilter"
      :agent-sort-by="agentSortBy"
      :agent-sort-order="agentSortOrder"
      :agent-filtered-containers="agentFilteredContainers"
      :sort-agent-containers="sortAgentContainers"
      :agent-online="agentOnline"
      :agent-container-state="agentContainerState"
      :agent-container-status-text="agentContainerStatusText"
      :agent-installing="agentInstalling"
      :agent-checking-update="agentCheckingUpdate"
      :agent-auto-updating="agentAutoUpdating"
      :agent-action-key="agentActionKey"
      :open-ports-modal="openPortsModal"
      :toggle-agent-collapse="toggleAgentCollapse"
      :status-label="statusLabel"
      :auto-update-label="autoUpdateLabel"
      :install-agent-container="handleAgentInstall"
      :toggle-agent-auto-update="handleAgentToggleAuto"
      :open-agent-quick-action="handleAgentOpenQuick"
    />
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

  <!-- Port Mapping Modal (Updated) -->
  <PortsModal
    :host="portsModalHost"
    :items="currentPorts"
    v-model:filter="portsFilter"
    @close="portsModalHost = null"
  />

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
