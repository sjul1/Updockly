<script setup lang="ts">
import {
  ArrowDownUp,
  Check,
  ChevronDown,
  ChevronUp,
  CircleX,
  Cpu,
  MemoryStick,
  Network,
  X,
} from "lucide-vue-next";
import AgentContainerRow from "./AgentContainerRow.vue";
import type { Agent, AgentContainer } from "../../services/api";

defineProps<{
  agents: Agent[];
  formatTime: (value: string | number | Date | null | undefined) => string;
  agentCollapsed: Record<string, boolean>;
  agentFilterText: Record<string, string>;
  agentStatusFilter: Record<string, "all" | "running" | "stopped">;
  agentAutoUpdateFilter: Record<string, "all" | "enabled" | "disabled">;
  agentSortBy: Record<string, keyof AgentContainer | null>;
  agentSortOrder: Record<string, "asc" | "desc">;
  agentFilteredContainers: (agent: Agent) => AgentContainer[];
  sortAgentContainers: (agentId: string, key: keyof AgentContainer) => void;
  agentOnline: (agent: Agent) => boolean;
  agentContainerState: (agent: Agent, container?: AgentContainer) => string;
  agentContainerStatusText: (
    agent: Agent,
    container?: AgentContainer
  ) => string;
  agentInstalling: Record<string, boolean>;
  agentCheckingUpdate: Record<string, boolean>;
  agentAutoUpdating: Record<string, boolean>;
  agentActionKey: (agentId: string, containerId?: string) => string;
  openPortsModal: (host: string) => void;
  toggleAgentCollapse: (agentId: string) => void;
  statusLabel: (val: "all" | "running" | "stopped" | undefined) => string;
  autoUpdateLabel: (val: "all" | "enabled" | "disabled" | undefined) => string;
  installAgentContainer: (agent: Agent, container: AgentContainer) => void;
  toggleAgentAutoUpdate: (agent: Agent, container: AgentContainer) => void;
  openAgentQuickAction: (
    agent: Agent,
    container: AgentContainer,
    evt: MouseEvent
  ) => void;
}>();
</script>

<template>
  <div class="space-y-6">
    <div
      v-for="agent in agents"
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
        <div class="flex items-center gap-2 mt-2 sm:mt-0 self-start sm:self-center">
          <div
            v-if="(agent as any).cpu !== undefined"
            class="badge badge-ghost gap-1 font-mono text-xs hidden sm:inline-flex"
          >
            <Cpu class="w-3 h-3" /> {{ ((agent as any).cpu).toFixed(1) }}%
          </div>
          <div
            v-if="(agent as any).memory !== undefined"
            class="badge badge-ghost gap-1 font-mono text-xs hidden sm:inline-flex"
          >
            <MemoryStick class="w-3 h-3" /> {{ ((agent as any).memory).toFixed(1) }}%
          </div>
          <button
            class="btn btn-ghost btn-xs"
            @click.stop="openPortsModal(agent.name)"
            title="View Port Mapping"
          >
            <Network class="w-4 h-4" />
          </button>
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
                              v-if="agentAutoUpdateFilter[agent.id] === 'enabled'"
                              class="w-4 h-4 text-success"
                            />
                            <span>Enabled</span>
                          </div>
                        </button>
                      </li>
                      <li>
                        <button
                          type="button"
                          @click="agentAutoUpdateFilter[agent.id] = 'disabled'"
                        >
                          <div class="flex items-center gap-2">
                            <Check
                              v-if="agentAutoUpdateFilter[agent.id] === 'disabled'"
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
              <AgentContainerRow
                v-for="container in agentFilteredContainers(agent)"
                :key="container?.id"
                :agent="agent"
                :container="container as AgentContainer"
                :online="agentOnline(agent)"
                :state="agentContainerState(agent, container as AgentContainer)"
                :status-text="agentContainerStatusText(agent, container as AgentContainer)"
                :installing="
                  !!agentInstalling[agentActionKey(agent.id, container?.id)]
                "
                :checking-update="
                  !!agentCheckingUpdate[agentActionKey(agent.id, container?.id)]
                "
                :auto-updating="
                  !!agentAutoUpdating[agentActionKey(agent.id, container?.id)]
                "
                @toggle-auto="toggleAgentAutoUpdate"
                @install="installAgentContainer"
                @open-quick="openAgentQuickAction"
              />
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
