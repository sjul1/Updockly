import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import History from "./History.vue";

// var to avoid TDZ with hoisted vi.mock
var apiMocks: {
  getUpdateHistory: ReturnType<typeof vi.fn>;
  deleteHistoryEntry: ReturnType<typeof vi.fn>;
};

vi.mock("../services/api", () => ({
  api: (apiMocks = {
    getUpdateHistory: vi.fn().mockResolvedValue([
      {
        id: "1",
        containerId: "c1",
        containerName: "alpha",
        image: "img:1",
        source: "agent",
        status: "success",
        message: "ok",
        createdAt: new Date().toISOString(),
      },
      {
        id: "2",
        containerId: "c2",
        containerName: "bravo",
        image: "img:2",
        source: "agent",
        status: "warning",
        message: "warn",
        createdAt: new Date().toISOString(),
      },
      {
        id: "3",
        containerId: "c3",
        containerName: "charlie",
        image: "img:3",
        source: "local",
        status: "error",
        message: "fail",
        createdAt: new Date().toISOString(),
      },
    ]),
    deleteHistoryEntry: vi.fn().mockResolvedValue(undefined),
  }),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

vi.mock("vue-toastification", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

const mountHistory = () => {
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () => h(History);
      },
    })
  );

  app.provide("formatAppTime", (value: string) => value.toString());
  app.provide("formatAppDateTime", (value: string) => value.toString());

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

describe("History", () => {
  it("loads entries and shows aggregate stats and counts", async () => {
    const { root, unmount } = mountHistory();
    await flush();

    expect(apiMocks.getUpdateHistory).toHaveBeenCalled();
    expect(root.textContent).toContain("Success: 1");
    expect(root.textContent).toContain("Warnings: 1");
    expect(root.textContent).toContain("Failed: 1");
    expect(root.textContent).toContain("Agents: 2");
    expect(root.textContent).toContain("Local: 1");
    expect(root.textContent).toContain("Showing 3 of 3 records");

    unmount();
  });

  it("filters list via search box and toggles live badge", async () => {
    const { root, unmount } = mountHistory();
    await flush();

    const searchInput = root.querySelector('input[placeholder*=\"Search container\"]') as HTMLInputElement;
    searchInput.value = "alpha";
    searchInput.dispatchEvent(new Event("input", { bubbles: true, cancelable: true }));
    await flush();

    expect(root.textContent).toContain("Showing 1 of 3 records");

    const liveBadge = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Live")
    ) as HTMLButtonElement | undefined;
    liveBadge?.click();
    await flush();
    expect(liveBadge?.textContent).toContain("Paused");

    unmount();
  });
});
