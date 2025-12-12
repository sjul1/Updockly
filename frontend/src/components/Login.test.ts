import { describe, it, expect, vi, beforeEach } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import Login from "./Login.vue";

const toastSuccess = vi.fn();
const toastError = vi.fn();

// var avoids TDZ when vi.mock is hoisted
var apiMocks: {
  forgotPassword: ReturnType<typeof vi.fn>;
  resetPassword: ReturnType<typeof vi.fn>;
  resetPasswordWithToken: ReturnType<typeof vi.fn>;
  initiateReset2FA: ReturnType<typeof vi.fn>;
  finalizeReset2FA: ReturnType<typeof vi.fn>;
};

vi.mock("vue-toastification", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
}));

vi.mock("../services/api", () => ({
  api:
    (apiMocks = {
      forgotPassword: vi.fn().mockResolvedValue(undefined),
      resetPassword: vi.fn().mockResolvedValue(undefined),
      resetPasswordWithToken: vi.fn().mockResolvedValue(undefined),
      initiateReset2FA: vi.fn().mockResolvedValue({
        secret: "ABC",
        qrCode: "data:image/png;base64,123",
        tempToken: "temp",
      }),
      finalizeReset2FA: vi.fn().mockResolvedValue({ recoveryCodes: ["1", "2"] }),
    }),
}));

const mountLogin = (propsOverride: Record<string, unknown> = {}) => {
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(Login, {
            loading: false,
            authenticated: false,
            userName: "",
            twoFactorRequired: false,
            ssoEnabled: false,
            ...propsOverride,
          });
      },
    })
  );

  app.mount(root);

  return {
    root,
    unmount: () => {
      app.unmount();
      root.remove();
    },
  };
};

beforeEach(() => {
  vi.clearAllMocks();
});

const flush = async () => {
  await Promise.resolve();
  await nextTick();
  await Promise.resolve();
};

describe("Login", () => {
  it("submits credentials and shows inline error on failure", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("bad creds"));
    const { root, unmount } = mountLogin({ onSubmit });
    await nextTick();

    const username = root.querySelector('input[name="username"]') as HTMLInputElement;
    const password = root.querySelector('input[name="password"]') as HTMLInputElement;
    const form = root.querySelector("form") as HTMLFormElement;

    username.value = "admin";
    username.dispatchEvent(new Event("input"));
    password.value = "secret";
    password.dispatchEvent(new Event("input"));
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));

    await flush();
    expect(onSubmit).toHaveBeenCalledWith({ username: "admin", password: "secret" });
    expect(root.textContent).toContain("bad creds");

    const forgotLink = root.querySelector('button.link.link-primary') as HTMLButtonElement;
    expect(forgotLink).toBeTruthy();

    unmount();
  });

  it("switches to email reset mode and calls forgotPassword API", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("bad creds"));
    const { root, unmount } = mountLogin({ onSubmit });
    await nextTick();

    // Trigger error so the forgot password link shows up
    const loginForm = root.querySelector("form") as HTMLFormElement;
    loginForm.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    const forgotLink = Array.from(
      root.querySelectorAll("button.link.link-primary")
    ).find((btn) => btn.textContent?.includes("Forgot")) as HTMLButtonElement | undefined;
    expect(forgotLink).toBeTruthy();
    forgotLink?.click();
    await flush();

    const emailInput = root.querySelector('input[type="email"]') as HTMLInputElement;
    emailInput.value = "admin@example.com";
    emailInput.dispatchEvent(new Event("input"));

    const resetForm = root.querySelector("form") as HTMLFormElement;
    resetForm.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(apiMocks.forgotPassword).toHaveBeenCalledWith("admin@example.com");
    expect(toastSuccess).toHaveBeenCalled();

    unmount();
  });

  it("handles two-factor verification attempts and shows reset link on failure", async () => {
    const onVerify = vi.fn().mockRejectedValue(new Error("invalid code"));
    const { root, unmount } = mountLogin({
      twoFactorRequired: true,
      onVerify2fa: onVerify,
    });
    await nextTick();

    const codeInput = root.querySelector('input[name="2fa-code"]') as HTMLInputElement;
    const form = root.querySelector("form") as HTMLFormElement;

    codeInput.value = "123456";
    codeInput.dispatchEvent(new Event("input"));
    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(onVerify).toHaveBeenCalledWith("123456");
    expect(root.textContent).toContain("invalid code");

    const reset2faLink = root.querySelector('button.link.link-warning') as HTMLButtonElement;
    expect(reset2faLink?.textContent).toContain("Reset 2FA");

    unmount();
  });
});
