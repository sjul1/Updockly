import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import Containers from "./Containers.vue";

// Child stubs
vi.mock("./containers/LocalContainersList.vue", () => ({
  default: defineComponent({
    name: "LocalContainersList",
    props: {
      containers: { type: Array, default: () => [] },
      filteredContainers: { type: Array, default: () => [] },
      hostInfo: { type: Object, default: () => ({}) },
      loading: Boolean,
      lastUpdated: [Date, String, Number],
      formatTime: Function,
      containersCollapsed: Boolean,
      filterText: String,
      statusFilter: String,
      autoUpdateFilter: String,
      installing: Object,
      checkingUpdate: Object,
      updateAvailableOverrides: Object,
      autoRefreshEnabled: Boolean,
    },
    emits: [
      "toggle-collapse",
      "open-ports",
      "set-filter-text",
      "set-status-filter",
      "set-auto-filter",
      "sort",
      "refresh",
      "toggle-auto-refresh",
      "check-update",
      "install",
      "toggle-auto",
      "open-quick",
    ],
    setup(props) {
      return () => h("div", { class: "local-list" }, `Local ${props.filteredContainers?.length ?? 0}`);
    },
  }),
}));

vi.mock("./containers/AgentContainersList.vue", () => ({
  default: defineComponent({
    name: "AgentContainersList",
    props: {
      agents: { type: Array, default: () => [] },
      formatTime: Function,
      agentCollapsed: Object,
      agentFilterText: Object,
      agentStatusFilter: Object,
      agentAutoUpdateFilter: Object,
      agentSortBy: Object,
      agentSortOrder: Object,
      agentFilteredContainers: Function,
      sortAgentContainers: Function,
      agentOnline: Function,
      agentContainerState: Function,
      agentContainerStatusText: Function,
      agentInstalling: Object,
      agentCheckingUpdate: Object,
      agentAutoUpdating: Object,
      agentActionKey: Function,
      openPortsModal: Function,
      toggleAgentCollapse: Function,
      statusLabel: Function,
      autoUpdateLabel: Function,
      installAgentContainer: Function,
      toggleAgentAutoUpdate: Function,
      openAgentQuickAction: Function,
    },
    setup(props) {
      return () => h("div", { class: "agent-list" }, `Agents ${props.agents?.length ?? 0}`);
    },
  }),
}));

vi.mock("./containers/PortsModal.vue", () => ({
  default: defineComponent({
    name: "PortsModal",
    props: ["open", "ports", "host", "filter"],
    emits: ["close", "update:filter"],
    setup(props) {
      return () => (props.open ? h("div", { class: "ports-modal" }, `Ports ${props.host}`) : null);
    },
  }),
}));

// Toast & API mocks
vi.mock("vue-toastification", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

var apiMocks: {
  getContainers: ReturnType<typeof vi.fn>;
  getHostInfo: ReturnType<typeof vi.fn>;
  getAgents: ReturnType<typeof vi.fn>;
};

vi.mock("../services/api", () => ({
  api: (apiMocks = {
    getContainers: vi.fn().mockResolvedValue([
      {
        ID: "1",
        Name: "web",
        Image: "web:1",
        State: "running",
        Status: "Up",
        AutoUpdate: true,
        Ports: ["80:80"],
      },
      {
        ID: "2",
        Name: "db",
        Image: "db:1",
        State: "exited",
        Status: "Exited",
        AutoUpdate: false,
      },
    ]),
    getHostInfo: vi.fn().mockResolvedValue({
      dockerVersion: "25.0",
      platform: "linux",
      hostname: "local",
      lastSeen: new Date().toISOString(),
    }),
    getAgents: vi.fn().mockResolvedValue([
      {
        id: "a1",
        name: "agent-1",
        hostname: "remote",
        notes: "",
        lastSeen: new Date().toISOString(),
        containers: [
          {
            id: "c3",
            name: "cache",
            image: "cache:1",
            state: "running",
            status: "Up",
            autoUpdate: false,
            ports: ["6379"],
          },
        ],
      },
    ]),
  }),
}));

const mountContainers = () => {
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () => h(Containers);
      },
    })
  );

  app.provide("formatAppTime", (value: string | number | Date) => value.toString());

  app.mount(root);

  return {
    root,
    unmount: () => {
      app.unmount();
      root.remove();
    },
  };
};

const flush = async () => {
  await Promise.resolve();
  await nextTick();
  await Promise.resolve();
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers();
});

afterEach(() => {
  vi.runOnlyPendingTimers();
  vi.useRealTimers();
});

describe("Containers", () => {
  it("renders fleet badges with computed counts and toggles live mode", async () => {
    const { root, unmount } = mountContainers();
    await flush();

    expect(apiMocks.getContainers).toHaveBeenCalled();
    expect(apiMocks.getAgents).toHaveBeenCalled();
    expect(root.textContent).toContain("Running: 2");
    expect(root.textContent).toContain("Stopped: 1");
    expect(root.textContent).toContain("Auto-update: 1");

    const liveBadge = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Live")
    ) as HTMLButtonElement | undefined;
    expect(liveBadge).toBeTruthy();
    liveBadge?.click();
    await flush();
    expect(liveBadge?.textContent).toContain("Paused");

    unmount();
  });

  it("refreshes data on manual refresh", async () => {
    const { root, unmount } = mountContainers();
    await flush();

    const refreshBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.getAttribute("aria-label") === "Refresh containers"
    ) as HTMLButtonElement | undefined;
    refreshBtn?.click();
    await flush();

    expect(apiMocks.getContainers).toHaveBeenCalledTimes(2);
    expect(apiMocks.getAgents).toHaveBeenCalledTimes(2);

    unmount();
  });
});
