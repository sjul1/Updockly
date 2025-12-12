import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import Dashboard from "./Dashboard.vue";

// var to avoid TDZ with hoisted vi.mock
var apiMocks: {
  getUpdateHistory: ReturnType<typeof vi.fn>;
  getSchedules: ReturnType<typeof vi.fn>;
  getRunningHistory: ReturnType<typeof vi.fn>;
};

vi.mock("../services/api", () => ({
  api: (apiMocks = {
    getUpdateHistory: vi.fn().mockResolvedValue([
      {
        id: "1",
        containerId: "c1",
        containerName: "app",
        image: "img",
        source: "agent",
        status: "success",
        message: "ok",
        createdAt: new Date().toISOString(),
      },
      {
        id: "2",
        containerId: "c2",
        containerName: "db",
        image: "img",
        source: "local",
        status: "warning",
        message: "warn",
        createdAt: new Date().toISOString(),
      },
      {
        id: "3",
        containerId: "c3",
        containerName: "cache",
        image: "img",
        source: "local",
        status: "error",
        message: "fail",
        createdAt: new Date().toISOString(),
      },
    ]),
    getSchedules: vi.fn().mockResolvedValue([
      { ID: "s1", Name: "Nightly", CronExpression: "0 0 * * *" },
    ]),
    getRunningHistory: vi.fn().mockResolvedValue([
      { id: "r1", date: new Date().toISOString(), running: 2, total: 3 },
    ]),
  }),
}));

const mountDashboard = (propsOverride?: Record<string, unknown>) => {
  const onRefresh = vi.fn();
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(
            Dashboard,
            {
              dashboard: {
                message: "hi",
                time: new Date().toISOString(),
                totalContainers: 5,
                runningContainers: 3,
                autoUpdateEnabled: 2,
                scheduleCount: 1,
                agentCount: 4,
                agentOnline: 3,
              },
              loadingBootstrap: false,
              onRefreshDashboard: onRefresh,
              ...propsOverride,
            },
            {}
          );
      },
    })
  );

  app.provide("formatAppTime", (value: string) => value.toString());
  app.provide("formatAppDateTime", (value: string) => value.toString());

  app.mount(root);

  return {
    root,
    onRefresh,
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

describe("Dashboard", () => {
  it("renders metrics and recent history stats", async () => {
    const { root, unmount } = mountDashboard();
    await flush();

    expect(apiMocks.getUpdateHistory).toHaveBeenCalled();
    expect(root.textContent).toContain("Total containers");
    expect(root.textContent).toContain("5");
    expect(root.textContent).toContain("Running");
    expect(root.textContent).toContain("Success");
    expect(root.textContent).toContain("Warnings");
    expect(root.textContent).toContain("Failed");

    unmount();
  });

  it("emits refresh and toggles live badge", async () => {
    const { root, onRefresh, unmount } = mountDashboard();
    await flush();

    const refreshBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.getAttribute("aria-label") === "Refresh dashboard"
    ) as HTMLButtonElement | undefined;
    refreshBtn?.click();
    expect(onRefresh).toHaveBeenCalledTimes(1);

    const liveBadge = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Live")
    ) as HTMLButtonElement | undefined;
    expect(liveBadge).toBeTruthy();
    liveBadge?.click();
    await flush();
    expect(liveBadge?.textContent).toContain("Paused");

    unmount();
  });
});
