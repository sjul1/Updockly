<script setup lang="ts">
import {
  onMounted,
  onBeforeUnmount,
  reactive,
  ref,
  computed,
  inject,
} from "vue";
import {
  RefreshCw,
  Plus,
  Trash2,
  Copy,
  KeyRound,
  ServerCog,
  ShieldCheck,
  Laptop,
  Sparkles,
  Pencil,
  Check,
  X,
  Download,
  AlertTriangle,
  FileDown,
  CheckCircle2,
  Shield,
} from "lucide-vue-next";
import { useToast } from "vue-toastification";
import { api, type Agent, type AgentWithToken } from "../services/api";
import ConfirmModal from "./ConfirmModal.vue";
import SectionHeader from "./SectionHeader.vue";

const agents = ref<Agent[]>([]);
const loading = ref(false);
const formatTime = inject<
  (value: string | number | Date | null | undefined) => string
>("formatAppTime", (value) => {
  if (!value) return "--:--:--";
  const date = value instanceof Date ? value : new Date(value);
  return date.toLocaleTimeString();
});

// -- Creation & Setup State --
const creating = ref(false);
const form = reactive({
  name: "",
  hostname: "",
  notes: "",
  tlsEnabled: false,
});
const lastToken = ref<AgentWithToken | null>(null);
const setupConfig = reactive({
  enableTLS: false,
  caDownloaded: false,
});

// -- List & Action State --
const rotating = reactive<Record<string, boolean>>({});
const removing = reactive<Record<string, boolean>>({});
const tlsUpdating = reactive<Record<string, boolean>>({});
const deletingAgent = ref<Agent | null>(null);
const deleteModalOpen = ref(false);
const rotateModalAgent = ref<Agent | null>(null);
const rotateModalOpen = ref(false);
const editingAgentId = ref<string | null>(null);
const editingNames = reactive<Record<string, string>>({});
const lastUpdated = ref<Date | null>(null);
const tokenTextEl = ref<HTMLElement | null>(null);
const autoRefreshEnabled = ref(true);
const refreshTimer = ref<number | null>(null);
const REFRESH_INTERVAL = 30000;

const toast = useToast();

const resetForm = () => {
  form.name = "";
  form.hostname = "";
  form.notes = "";
  form.tlsEnabled = false;
};

const finishSetup = () => {
  lastToken.value = null;
  resetForm();
};

// -- Data Loading --
const loadAgents = async (options: { silent?: boolean } = {}) => {
  const { silent = false } = options;
  if (!silent) {
    loading.value = true;
  }
  try {
    agents.value = await api.getAgents();
    lastUpdated.value = new Date();
  } catch (error) {
    console.error("Failed to load agents", error);
    toast.error("Unable to load agents");
  } finally {
    if (!silent) {
      loading.value = false;
    }
  }
};

// -- Actions --
const createAgent = async () => {
  if (!form.name.trim()) {
    toast.error("Agent name is required");
    return;
  }
  creating.value = true;
  // Reset setup config for new agent
  setupConfig.enableTLS = false;
  setupConfig.caDownloaded = false;
  lastToken.value = null;

  try {
    const created = await api.createAgent({
      name: form.name.trim(),
      hostname: form.hostname.trim(),
      notes: form.notes.trim(),
      tlsEnabled: form.tlsEnabled,
    });
    agents.value.unshift(created);
    lastToken.value = created;
    setupConfig.enableTLS = !!created.tlsEnabled;
    toast.success(`Agent "${created.name}" created`);
    // Note: We do NOT reset form here immediately so user can see what they just made in the success card if needed,
    // but usually we rely on the Success Card data.
  } catch (error) {
    console.error("Failed to create agent", error);
    toast.error("Unable to create agent");
  } finally {
    creating.value = false;
  }
};

const rotateToken = async (agent: Agent) => {
  confirmRotate(agent);
};

const deleteAgent = async (agent: Agent) => {
  confirmDelete(agent);
};

const copy = async (value: string | undefined) => {
  if (!value) return;
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value);
    } else {
      const textArea = document.createElement("textarea");
      textArea.value = value;
      textArea.style.position = "fixed";
      textArea.style.left = "-9999px";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
    }
    toast.success("Copied to clipboard");
  } catch {
    toast.error("Clipboard unavailable");
  }
};

const startRename = (agent: Agent) => {
  editingAgentId.value = agent.id;
  editingNames[agent.id] = agent.name || "";
};

const cancelRename = () => {
  editingAgentId.value = null;
};

const submitRename = async (agent: Agent) => {
  const name = editingNames[agent.id]?.trim();
  if (!name) {
    toast.error("Name is required");
    return;
  }
  try {
    const updated = await api.updateAgent(agent.id, {
      name,
      hostname: agent.hostname,
      notes: agent.notes,
      tlsEnabled: agent.tlsEnabled,
    });
    Object.assign(agent, updated);
    editingAgentId.value = null;
    toast.success("Agent updated");
  } catch (error) {
    console.error("Failed to update agent", error);
    toast.error("Unable to update agent");
  }
};

const confirmDelete = (agent: Agent) => {
  deletingAgent.value = agent;
  deleteModalOpen.value = true;
};

const performDelete = async () => {
  if (!deletingAgent.value) return;
  removing[deletingAgent.value.id] = true;
  try {
    await api.deleteAgent(deletingAgent.value.id);
    agents.value = agents.value.filter((a) => a.id !== deletingAgent.value?.id);
    // If we deleted the agent currently shown in setup, clear the setup
    if (lastToken.value?.id === deletingAgent.value.id) {
      lastToken.value = null;
    }
    toast.success(`Removed ${deletingAgent.value.name}`);
  } catch (error) {
    console.error("Failed to delete agent", error);
    toast.error("Unable to delete agent");
  } finally {
    if (deletingAgent.value) {
      removing[deletingAgent.value.id] = false;
    }
    deletingAgent.value = null;
    deleteModalOpen.value = false;
  }
};

const confirmRotate = (agent: Agent) => {
  rotateModalAgent.value = agent;
  rotateModalOpen.value = true;
};

const performRotate = async () => {
  if (!rotateModalAgent.value) return;
  rotating[rotateModalAgent.value.id] = true;
  try {
    const updated = await api.rotateAgentToken(rotateModalAgent.value.id);
    Object.assign(rotateModalAgent.value, updated);

    // Switch the setup view to this newly rotated agent so user can get the new token
    lastToken.value = updated;
    setupConfig.enableTLS = !!updated.tlsEnabled;

    toast.success(`Token rotated for ${rotateModalAgent.value.name}`);
  } catch (error) {
    console.error("Failed to rotate token", error);
    toast.error("Unable to rotate token");
  } finally {
    rotating[rotateModalAgent.value.id] = false;
    rotateModalAgent.value = null;
    rotateModalOpen.value = false;
  }
};

const downloadCACert = async () => {
  try {
    const blob = await api.downloadCACert();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "ca.crt";
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);
    setupConfig.caDownloaded = true;
    toast.success("CA Certificate downloaded");
    setTimeout(() => {
      setupConfig.caDownloaded = false;
    }, 2000);
  } catch (error) {
    console.error("Failed to download CA cert", error);
    toast.error("Unable to download CA certificate");
  }
};

const selectTokenText = () => {
  const el = tokenTextEl.value;
  if (!el) return;
  const selection = window.getSelection();
  if (!selection) return;
  const range = document.createRange();
  range.selectNodeContents(el);
  selection.removeAllRanges();
  selection.addRange(range);
};

const syncLastTokenTLS = (agentId: string, tlsEnabled: boolean) => {
  if (lastToken.value?.id === agentId) {
    lastToken.value = { ...lastToken.value, tlsEnabled };
    setupConfig.enableTLS = tlsEnabled;
  }
};

const updateAgentTLS = async (agent: Agent, enabled: boolean) => {
  const previous = !!agent.tlsEnabled;
  if (tlsUpdating[agent.id]) return;
  tlsUpdating[agent.id] = true;
  try {
    const updated = await api.updateAgent(agent.id, {
      name: agent.name,
      hostname: agent.hostname,
      notes: agent.notes,
      tlsEnabled: enabled,
    });
    Object.assign(agent, updated);
    syncLastTokenTLS(agent.id, !!updated.tlsEnabled);
    toast.success(`TLS ${enabled ? "enabled" : "disabled"} for ${agent.name}`);
  } catch (error) {
    agent.tlsEnabled = previous;
    syncLastTokenTLS(agent.id, previous);
    console.error("Failed to update agent TLS", error);
    toast.error("Unable to update TLS setting");
  } finally {
    tlsUpdating[agent.id] = false;
  }
};

const onAgentTLSToggle = async (agent: Agent, enabled: boolean) => {
  if (!!agent.tlsEnabled === enabled) return;
  await updateAgentTLS(agent, enabled);
};

// -- Helpers & Computed --
const formatLastSeen = (agent: Agent) => {
  if (!agent.lastSeen) return "Never";
  const ts = new Date(agent.lastSeen).getTime();
  const diff = Date.now() - ts;
  if (diff < 60_000) return "Just now";
  const minutes = Math.round(diff / 60_000);
  if (minutes < 120) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  return `${days}d ago`;
};

const agentOnline = (agent: Agent) => {
  if (!agent.lastSeen) return false;
  const ts = new Date(agent.lastSeen).getTime();
  return Date.now() - ts < 5 * 60 * 1000;
};

const totalAgents = computed(() => agents.value.length);
const onlineAgents = computed(
  () => agents.value.filter((agent) => agentOnline(agent)).length
);

const setupSnippet = computed(() => {
  const base = window.location.origin.replace(/\/+$/, "");
  const snippetLines = [
    "docker run -d --name updockly-agent \\",
    `  -e UPDOCKLY_SERVER="${base}" \\`,
    `  -e UPDOCKLY_AGENT_TOKEN="${lastToken.value?.token || "<token>"}" \\`,
    `  -e UPDOCKLY_AGENT_NAME="${lastToken.value?.name || "<agent-name>"}" \\`,
    "  -v /var/run/docker.sock:/var/run/docker.sock \\",
  ];

  if (setupConfig.enableTLS) {
    snippetLines.push(
      '  -e UPDOCKLY_CA_CERT="/app/ca.crt" \\',
      "  -v $(pwd)/ca.crt:/app/ca.crt:ro \\"
    );
  }

  snippetLines.push("  updockly/agent:latest");
  return snippetLines.join("\n");
});

// -- Lifecycle --
const startAutoRefresh = () => {
  if (refreshTimer.value) return;
  refreshTimer.value = window.setInterval(
    () => void loadAgents({ silent: true }),
    REFRESH_INTERVAL
  );
};

const stopAutoRefresh = () => {
  if (!refreshTimer.value) return;
  window.clearInterval(refreshTimer.value);
  refreshTimer.value = null;
};

const toggleAutoRefresh = () => {
  autoRefreshEnabled.value = !autoRefreshEnabled.value;
  if (autoRefreshEnabled.value) {
    startAutoRefresh();
    void loadAgents({ silent: true });
  } else {
    stopAutoRefresh();
  }
};

onMounted(() => {
  void loadAgents();
  startAutoRefresh();
});

onBeforeUnmount(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="space-y-6">
    <SectionHeader
      title="Remote agent fleet"
      :icon="ServerCog"
    >
      <template #eyebrow>
        <ServerCog class="h-4 w-4 text-primary" />
        Agents
      </template>
      <template #subtitle>
        Register lightweight agents to report Docker state from remote hosts.
      </template>
      <template #badges>
        <span class="badge badge-primary badge-outline gap-2">
          <ShieldCheck class="h-3.5 w-3.5" />
          {{ onlineAgents }} online
        </span>
        <span class="badge badge-outline gap-2">
          <ServerCog class="h-3.5 w-3.5" />
          {{ totalAgents }} total
        </span>
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
          @click="loadAgents({ silent: false })"
          :disabled="loading"
          title="Refresh agents"
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
        >
          <Sparkles class="h-3.5 w-3.5" />
          {{ autoRefreshEnabled ? "Live" : "Paused" }}
        </button>
      </template>
    </SectionHeader>

    <div class="space-y-6">
      <div class="rounded-3xl border border-base-200 bg-base-100 p-6 shadow-xl relative overflow-hidden">
        <div class="absolute inset-0 bg-gradient-to-br from-primary/5 via-transparent to-secondary/10 pointer-events-none"></div>
        <div class="relative">
        <div v-if="!lastToken" class="space-y-4">
          <div class="flex items-center gap-3">
            <div
              class="w-10 h-10 rounded-2xl bg-primary/10 flex items-center justify-center shadow-sm"
            >
              <Plus class="w-5 h-5 text-primary" />
            </div>
            <div>
              <h4 class="font-bold text-lg">Create a new agent</h4>
              <p class="text-xs text-base-content/60">
                Provision a remote worker and bind it securely.
              </p>
            </div>
          </div>
          <div
            class="grid grid-cols-[minmax(0,_1fr)_minmax(0,_1fr)_auto] gap-4 items-center"
          >
            <div class="form-control w-full">
              <input
                v-model="form.name"
                class="input input-bordered input-sm w-full rounded-lg"
                placeholder="Agent Name (e.g. prod-db-01)"
                @keyup.enter="createAgent"
              />
            </div>
            <div class="form-control w-full">
              <input
                v-model="form.hostname"
                class="input input-bordered input-sm w-full rounded-lg"
                placeholder="Hostname (Optional)"
                @keyup.enter="createAgent"
              />
            </div>
            <label
              class="flex items-center gap-2 text-sm px-3 py-1 rounded-full bg-base-200/60 border border-base-300 justify-center justify-self-end"
            >
              <input
                type="checkbox"
                class="toggle toggle-sm toggle-success"
                v-model="form.tlsEnabled"
              />
              <span class="text-xs text-base-content/70 whitespace-nowrap">Require TLS</span>
            </label>
          </div>
          <div class="form-control">
            <textarea
              v-model="form.notes"
              class="textarea textarea-bordered textarea-sm w-full rounded-lg"
              placeholder="Notes (Optional)"
              rows="2"
            ></textarea>
          </div>
          <div class="mt-2 flex items-center justify-end flex-wrap gap-3">
            <button
              class="btn btn-primary btn-sm rounded-full shadow-md"
              :class="{ loading: creating }"
              @click="createAgent"
            >
              Create Agent
            </button>
          </div>
        </div>

        <div
          v-else
          class="space-y-4 rounded-2xl border border-success/30 bg-success/5 p-4 shadow-sm"
        >
          <div class="flex items-center justify-between gap-2 text-success">
            <div class="flex items-center gap-2">
              <CheckCircle2 class="w-5 h-5" />
              <h4 class="font-bold">Agent Ready</h4>
            </div>
            <span
              class="text-[10px] uppercase font-bold tracking-wider opacity-60"
              >Setup Mode</span
            >
          </div>

          <div class="space-y-1">
            <div class="flex justify-between items-end">
              <span
                class="text-xs font-semibold text-base-content/60 uppercase tracking-wider"
                >Access Token</span
              >
            </div>
            <div class="relative group">
              <div
                class="p-3 bg-base-100 border border-base-300 rounded-lg font-mono text-xs break-all pr-10 shadow-sm"
                ref="tokenTextEl"
                @click="selectTokenText"
              >
                {{ lastToken.token }}
              </div>
              <button
                class="absolute right-1 top-1 btn btn-xs btn-square btn-ghost"
                @click="copy(lastToken.token)"
                title="Copy Token"
              >
                <Copy class="w-4 h-4" />
              </button>
            </div>
            <p class="text-[10px] text-error flex items-center gap-1 mt-1">
              <AlertTriangle class="w-3 h-3" />
              Save this token now. It cannot be viewed again.
            </p>
          </div>

          <div class="divider my-2 opacity-50"></div>

          <div class="space-y-3">
            <div class="flex items-center gap-2">
              <Laptop class="w-4 h-4 text-base-content/70" />
              <span class="text-sm font-semibold">Installation Setup</span>
            </div>

            <div
              v-if="setupConfig.enableTLS"
              class="flex items-center justify-between bg-base-100 p-2 rounded-lg border border-base-200"
            >
              <div class="flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-primary" />
                <div class="flex flex-col">
                  <span class="text-xs font-semibold">TLS enabled</span>
                  <span class="text-[10px] text-base-content/60"
                    >Download and mount ca.crt with the agent</span
                  >
                </div>
              </div>
              <button
                class="btn btn-xs btn-outline gap-2 border-dashed"
                @click="downloadCACert"
              >
                <Check
                  v-if="setupConfig.caDownloaded"
                  class="w-3 h-3 text-success"
                />
                <Download v-else class="w-3 h-3" />
                {{
                  setupConfig.caDownloaded
                    ? "Certificate Downloaded"
                    : "Download CA Certificate"
                }}
              </button>
            </div>

            <div class="space-y-1 pt-1">
              <div class="flex justify-between items-center">
                <span
                  class="text-[10px] text-base-content/60 uppercase tracking-wider"
                  >Docker Run Command</span
                >
                <button
                  class="btn btn-ghost btn-xs h-6 min-h-0 px-1 text-base-content/50"
                  @click="copy(setupSnippet)"
                >
                  <Copy class="w-3 h-3" />
                </button>
              </div>
              <div
                class="mockup-code bg-[#1e1e1e] text-gray-300 text-[10px] p-0 min-w-0"
              >
                <pre
                  class="p-3 whitespace-pre-wrap font-mono"
                ><code>{{ setupSnippet }}</code></pre>
              </div>
            </div>

            <p class="text-[10px] text-base-content/50 text-center">
              Run this on the remote Docker host.
            </p>

            <button
              class="btn btn-success btn-sm w-full rounded-lg text-white mt-2"
              @click="finishSetup"
            >
              I've deployed the agent / Done
            </button>
          </div>
        </div>
        </div>
      </div>

      <div class="card bg-base-100 border border-base-200 shadow-lg">
        <div class="card-body p-0 md:p-6 space-y-4">
          <div
            class="rounded-2xl border border-base-200 bg-base-100/60 overflow-hidden"
          >
            <div class="hidden md:block overflow-x-auto">
              <table class="table w-full">
                <thead class="bg-base-200/50">
                  <tr class="text-xs uppercase text-base-content/60">
                    <th>Name</th>
                    <th>Host</th>
                    <th>Status</th>
                    <th>Token</th>
                    <th>Docker</th>
                    <th class="text-center">TLS</th>
                    <th class="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="loading">
                    <td
                      colspan="7"
                      class="text-center py-12 text-sm text-base-content/60"
                    >
                      <div class="flex items-center justify-center gap-2">
                        <RefreshCw class="w-4 h-4 animate-spin" />
                        Loading fleet...
                      </div>
                    </td>
                  </tr>
                  <tr v-else-if="agents.length === 0">
                    <td
                      colspan="7"
                      class="text-center py-12 text-sm text-base-content/60"
                    >
                      <div class="flex flex-col items-center gap-2">
                        <ServerCog class="w-8 h-8 opacity-20" />
                        <p>No agents configured yet.</p>
                      </div>
                    </td>
                  </tr>
                  <tr
                    v-for="agent in agents"
                    :key="agent.id"
                    class="hover:bg-base-200/30 transition-colors"
                  >
                    <td class="font-semibold">
                      <div
                        v-if="editingAgentId === agent.id"
                        class="flex items-center gap-2"
                      >
                        <input
                          v-model="editingNames[agent.id]"
                          class="input input-bordered input-xs w-40"
                          @keyup.enter="submitRename(agent)"
                          @keyup.esc="cancelRename"
                          autoFocus
                        />
                      </div>
                      <div v-else class="flex items-center gap-2">
                        {{ agent.name }}
                      </div>
                    </td>
                    <td class="text-sm text-base-content/70">
                      {{ agent.hostname || "—" }}
                    </td>
                    <td class="text-sm">
                      <div class="flex items-center gap-2">
                        <span
                          class="inline-flex h-2.5 w-2.5 rounded-full ring-2 ring-base-100"
                          :class="
                            agentOnline(agent) ? 'bg-success' : 'bg-base-300'
                          "
                        />
                        <span class="text-xs text-base-content/70">
                          {{ formatLastSeen(agent) }}
                        </span>
                      </div>
                    </td>
                    <td class="text-sm">
                      <div class="flex items-center gap-2">
                        <template v-if="agent.tokenBound">
                          <CheckCircle2 class="w-4 h-4 text-success" />
                          <span class="text-xs text-base-content/70">
                            IP-bound
                          </span>
                        </template>
                        <template v-else>
                          <AlertTriangle class="w-4 h-4 text-warning" />
                          <span class="text-xs text-base-content/70">
                            Not bound
                          </span>
                        </template>
                      </div>
                    </td>
                    <td class="text-sm text-base-content/70">
                      <div class="flex flex-col">
                        <span class="font-medium">
                          {{ agent.dockerVersion || "—" }}
                        </span>
                        <span
                          v-if="agent.agentVersion"
                          class="text-[0.7rem] text-base-content/50"
                        >
                          v{{ agent.agentVersion }}
                        </span>
                      </div>
                    </td>
                    <td class="text-center text-sm">
                      <div class="flex items-center justify-center gap-2">
                        <span
                          class="inline-flex items-center justify-center w-7 h-7 rounded-full"
                          :class="
                            agent.tlsEnabled
                              ? 'bg-primary/10 text-primary'
                              : 'bg-base-200 text-base-content/50'
                          "
                          :title="
                            agent.tlsEnabled
                              ? 'TLS required for this agent'
                              : 'TLS not required'
                          "
                        >
                          <ShieldCheck
                            v-if="agent.tlsEnabled"
                            class="w-4 h-4"
                          />
                          <Shield v-else class="w-4 h-4" />
                        </span>
                        <input
                          type="checkbox"
                          class="toggle toggle-xs toggle-success"
                          :checked="!!agent.tlsEnabled"
                          :disabled="tlsUpdating[agent.id]"
                          @change="
                            (e) =>
                              onAgentTLSToggle(
                                agent,
                                (e.target as HTMLInputElement).checked
                              )
                          "
                        />
                      </div>
                    </td>
                    <td class="text-right">
                      <div class="flex justify-end gap-1">
                        <button
                          v-if="agent.tlsEnabled"
                          class="btn btn-ghost btn-xs btn-square"
                          title="Download CA Cert for Agent"
                          @click="downloadCACert"
                        >
                          <FileDown class="w-4 h-4 opacity-70" />
                        </button>
                        <button
                          class="btn btn-ghost btn-xs btn-square"
                          title="Rename agent"
                          @click="startRename(agent)"
                          v-if="editingAgentId !== agent.id"
                        >
                          <Pencil class="w-4 h-4 opacity-70" />
                        </button>
                        <div v-else class="flex items-center gap-1">
                          <button
                            class="btn btn-ghost btn-xs text-success"
                            title="Save"
                            @click="submitRename(agent)"
                          >
                            <Check class="w-4 h-4" />
                          </button>
                          <button
                            class="btn btn-ghost btn-xs text-error"
                            title="Cancel"
                            @click="cancelRename"
                          >
                            <X class="w-4 h-4" />
                          </button>
                        </div>
                        <button
                          class="btn btn-ghost btn-xs btn-square"
                          :class="{ loading: rotating[agent.id] }"
                          title="Rotate token"
                          @click="rotateToken(agent)"
                        >
                          <KeyRound class="w-4 h-4 opacity-70" />
                        </button>
                        <button
                          class="btn btn-ghost btn-xs btn-square text-error"
                          :class="{ loading: removing[agent.id] }"
                          title="Delete agent"
                          @click="deleteAgent(agent)"
                        >
                          <Trash2 class="w-4 h-4 opacity-70" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="md:hidden">
              <div v-if="loading" class="text-center py-6 text-sm">
                Loading agents...
              </div>
              <div
                v-else-if="agents.length === 0"
                class="text-center py-6 text-sm px-4"
              >
                No agents yet.
              </div>
              <div v-else class="divide-y divide-base-200">
                <div
                  v-for="agent in agents"
                  :key="agent.id"
                  class="p-4 space-y-3"
                >
                  <div class="flex items-start justify-between">
                    <div class="space-y-1">
                      <div class="font-semibold flex items-center gap-2">
                        <div
                          v-if="editingAgentId === agent.id"
                          class="flex items-center gap-2"
                        >
                          <input
                            v-model="editingNames[agent.id]"
                            class="input input-bordered input-xs w-full max-w-[140px]"
                            @keyup.enter="submitRename(agent)"
                            @keyup.esc="cancelRename"
                          />
                        </div>
                        <span v-else>{{ agent.name }}</span>
                      </div>
                      <div
                        class="text-xs text-base-content/70 flex items-center gap-2"
                      >
                        <span>{{ agent.hostname || "—" }}</span>
                        <span>•</span>
                        <div class="flex items-center gap-1">
                          <span
                            class="inline-flex h-1.5 w-1.5 rounded-full"
                            :class="
                              agentOnline(agent)
                                ? 'bg-success'
                                : 'bg-base-300'
                            "
                          />
                          <span>{{ formatLastSeen(agent) }}</span>
                        </div>
                      </div>
                      <div
                        class="flex items-center gap-3 text-xs text-base-content/70"
                      >
                        <div class="flex items-center gap-1">
                          <span
                            class="inline-flex items-center justify-center w-6 h-6 rounded-full"
                            :class="
                              agent.tlsEnabled
                                ? 'bg-primary/10 text-primary'
                                : 'bg-base-200 text-base-content/50'
                            "
                          >
                            <ShieldCheck
                              v-if="agent.tlsEnabled"
                              class="w-3.5 h-3.5"
                            />
                            <Shield v-else class="w-3.5 h-3.5" />
                          </span>
                          <span>{{
                            agent.tlsEnabled ? "TLS enabled" : "TLS off"
                          }}</span>
                        </div>
                        <div class="flex items-center gap-1 text-[10px]">
                          <template v-if="agent.tokenBound">
                            <CheckCircle2 class="w-3 h-3 text-success" />
                            <span>IP-bound</span>
                          </template>
                          <template v-else>
                            <AlertTriangle class="w-3 h-3 text-warning" />
                            <span>Not bound</span>
                          </template>
                        </div>
                        <label class="flex items-center gap-2 ml-auto">
                          <span
                            class="text-[10px] uppercase tracking-wide text-base-content/60"
                            >TLS</span
                          >
                          <input
                            type="checkbox"
                            class="toggle toggle-xs toggle-success"
                            :checked="!!agent.tlsEnabled"
                            :disabled="tlsUpdating[agent.id]"
                            @change="
                              (e) =>
                                onAgentTLSToggle(
                                  agent,
                                  (e.target as HTMLInputElement).checked
                                )
                            "
                          />
                        </label>
                      </div>
                    </div>
                    <div class="flex items-center gap-1">
                      <div
                        v-if="editingAgentId === agent.id"
                        class="flex items-center gap-1"
                      >
                        <button
                          class="btn btn-ghost btn-sm btn-square text-success"
                          @click="submitRename(agent)"
                        >
                          <Check class="w-4 h-4" />
                        </button>
                        <button
                          class="btn btn-ghost btn-sm btn-square text-error"
                          @click="cancelRename"
                        >
                          <X class="w-4 h-4" />
                        </button>
                      </div>
                      <button
                        v-else
                        class="btn btn-ghost btn-sm btn-square"
                        @click="startRename(agent)"
                      >
                        <Pencil class="w-4 h-4" />
                      </button>
                      <button
                        v-if="agent.tlsEnabled"
                        class="btn btn-ghost btn-sm btn-square"
                        @click="downloadCACert"
                        title="Download CA"
                      >
                        <FileDown class="w-4 h-4" />
                      </button>

                      <button
                        class="btn btn-ghost btn-sm btn-square"
                        :class="{ loading: rotating[agent.id] }"
                        @click="rotateToken(agent)"
                      >
                        <KeyRound class="w-4 h-4" />
                      </button>
                      <button
                        class="btn btn-ghost btn-sm btn-square text-error"
                        :class="{ loading: removing[agent.id] }"
                        @click="deleteAgent(agent)"
                      >
                        <Trash2 class="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <ConfirmModal
    :open="deleteModalOpen"
    title="Delete agent"
    message="This will revoke access for the agent and remove it from the list. This cannot be undone."
    confirm-label="Delete"
    @confirm="performDelete"
    @cancel="deleteModalOpen = false"
  />

  <ConfirmModal
    :open="rotateModalOpen"
    title="Rotate token"
    message="Rotating the token will revoke the old token. You will be shown a new token immediately."
    confirm-label="Rotate"
    @confirm="performRotate"
    @cancel="rotateModalOpen = false"
  />
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
