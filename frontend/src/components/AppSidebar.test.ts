import { describe, it, expect, vi } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import AppSidebar, { type NavItem } from "./AppSidebar.vue";

const FakeIcon = defineComponent({
  name: "FakeIcon",
  setup: () => () => h("span", { class: "fake-icon" }, "icon"),
});

const defaultNav: NavItem[] = [
  { id: "login", label: "Dashboard", icon: FakeIcon },
  { id: "containers", label: "Containers", icon: FakeIcon },
];

const mountSidebar = (propsOverride?: Partial<InstanceType<typeof AppSidebar>["$props"]>) => {
  const updatePanel = vi.fn();
  const toggleTheme = vi.fn();
  const logout = vi.fn();
  const close = vi.fn();

  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(AppSidebar, {
            navItems: defaultNav,
            active: "login",
            theme: "light",
            backendVersion: "1.2.3",
            healthStatus: "OK",
            userName: "Ada",
            isAuthenticated: true,
            hideSupportButton: false,
            ...propsOverride,
            "onUpdate:panel": updatePanel,
            onToggleTheme: toggleTheme,
            onLogout: logout,
            onClose: close,
          });
      },
    })
  );

  app.mount(root);

  return {
    root,
    updatePanel,
    toggleTheme,
    logout,
    close,
    unmount: () => {
      app.unmount();
      root.remove();
    },
  };
};

describe("AppSidebar", () => {
  it("renders nav items, status, user info, and emits actions", async () => {
    const { root, updatePanel, toggleTheme, logout, close, unmount } = mountSidebar();
    await nextTick();

    const navBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Containers")
    );
    navBtn?.click();
    expect(updatePanel).toHaveBeenCalledWith("containers");

    const themeBtn = root.querySelector('button[title*="Switch to dark mode"]') as HTMLButtonElement;
    themeBtn?.click();
    expect(toggleTheme).toHaveBeenCalledTimes(1);

    const logoutBtn = root.querySelector('button[title="Sign Out"]') as HTMLButtonElement;
    logoutBtn?.click();
    expect(logout).toHaveBeenCalledTimes(1);

    const closeBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.className.includes("lg:hidden")
    );
    closeBtn?.click();
    expect(close).toHaveBeenCalledTimes(1);

    const versionText = root.querySelector(".font-mono")?.textContent ?? "";
    expect(versionText).toContain("1.2.3");
    expect(root.textContent).toContain("Ada");

    const supportLink = root.querySelector('a[href="https://buymeacoffee.com/joul"]');
    expect(supportLink).toBeTruthy();

    unmount();
  });

  it("hides support block when requested", async () => {
    const { root, unmount } = mountSidebar({ hideSupportButton: true });
    await nextTick();

    const supportLink = root.querySelector('a[href="https://buymeacoffee.com/joul"]');
    expect(supportLink).toBeFalsy();

    unmount();
  });
});
