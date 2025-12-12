import { describe, it, expect, vi, beforeEach } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import Setup from "./Setup.vue";

const toastSuccess = vi.fn();
const toastError = vi.fn();

// var to avoid TDZ with hoisted vi.mock
var apiMocks: {
  setupTestDb: ReturnType<typeof vi.fn>;
  setupGenerate: ReturnType<typeof vi.fn>;
  setupCreate: ReturnType<typeof vi.fn>;
};

vi.mock("vue-toastification", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
}));

vi.mock("../services/api", () => ({
  api: (apiMocks = {
    setupTestDb: vi.fn().mockResolvedValue(undefined),
    setupGenerate: vi.fn().mockResolvedValue({
      secret: "SECRET123",
      qrCode: "data:image/png;base64,qr",
    }),
    setupCreate: vi.fn().mockResolvedValue({
      recoveryCodes: ["1", "2", "3"],
      jwtSecret: "jwt",
      vaultKey: "vault",
    }),
  }),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

const mountSetup = (propsOverride?: Record<string, unknown>) => {
  const onComplete = vi.fn();
  const toggleTheme = vi.fn();
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(Setup, {
            settings: {},
            theme: "light",
            onSetupComplete: onComplete,
            onToggleTheme: toggleTheme,
            ...propsOverride,
          });
      },
    })
  );

  app.mount(root);

  return {
    root,
    onComplete,
    toggleTheme,
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
});

describe("Setup", () => {
  it("walks through config to admin flow and reaches recovery codes", async () => {
    const { root, onComplete, unmount } = mountSetup();
    await nextTick();

    // Config step
    const dbInput = root.querySelector('input[type="text"]') as HTMLInputElement;
    dbInput.value = "postgres://user:pass@localhost:5432/db";
    dbInput.dispatchEvent(new Event("input"));
    const continueBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Test")
    ) as HTMLButtonElement | undefined;
    continueBtn?.click();
    await flush();

    expect(apiMocks.setupTestDb).toHaveBeenCalledWith("postgres://user:pass@localhost:5432/db");

    // Admin step
    const username = root.querySelector('input[placeholder="Username"]') as HTMLInputElement | null;
    const email = root.querySelector('input[type="email"]') as HTMLInputElement | null;
    const password = root.querySelector('input[placeholder="Password"]') as HTMLInputElement | null;
    expect(username).toBeTruthy();
    expect(email).toBeTruthy();
    expect(password).toBeTruthy();
    if (username && email && password) {
      username.value = "admin";
      username.dispatchEvent(new Event("input"));
      email.value = "admin@example.com";
      email.dispatchEvent(new Event("input"));
      password.value = "Password1!";
      password.dispatchEvent(new Event("input"));
    }

    const generateBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Generate QR Code")
    ) as HTMLButtonElement | undefined;
    generateBtn?.click();
    await flush();

    const codeInput = root.querySelector('input[placeholder="123456"]') as HTMLInputElement | null;
    expect(codeInput).toBeTruthy();
    if (codeInput) {
      codeInput.value = "123456";
      codeInput.dispatchEvent(new Event("input"));
    }

    const createBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Create Account")
    ) as HTMLButtonElement | undefined;
    createBtn?.click();
    await flush();

    expect(apiMocks.setupCreate).toHaveBeenCalled();
    expect(root.textContent).toContain("Save Recovery Codes");
    expect(onComplete).not.toHaveBeenCalled();

    const confirmBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("I have saved them")
    ) as HTMLButtonElement | undefined;
    confirmBtn?.click();
    await flush();

    const finishBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Finish Setup")
    ) as HTMLButtonElement | undefined;
    finishBtn?.click();
    await flush();

    const forceBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("I understand the risk")
    ) as HTMLButtonElement | undefined;
    forceBtn?.click();
    await flush();

    expect(onComplete).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("shows secrets warning before finishing when secrets not copied", async () => {
    const { root, onComplete, unmount } = mountSetup({
      settings: { jwtSecret: "abc", vaultKey: "def", recoveryCodes: ["1"] },
    });
    await nextTick();

    const confirmCodes = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("I have saved them")
    ) as HTMLButtonElement | undefined;
    confirmCodes?.click();
    await flush();

    const finishBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Finish Setup")
    ) as HTMLButtonElement | undefined;
    finishBtn?.click();
    await flush();

    const warningText = root.textContent ?? "";
    expect(warningText).toContain("Save these keys before continuing");
    expect(onComplete).not.toHaveBeenCalled();

    const forceBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("I understand the risk")
    ) as HTMLButtonElement | undefined;
    forceBtn?.click();
    await flush();

    expect(onComplete).toHaveBeenCalledTimes(1);

    unmount();
  });
});
