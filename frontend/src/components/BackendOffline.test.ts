import { describe, it, expect, vi } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import BackendOffline from "./BackendOffline.vue";

const mountComponent = (props?: Record<string, unknown>) => {
  const retrySpy = vi.fn();
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(BackendOffline, {
            checking: false,
            errorMessage: "",
            ...props,
            onRetry: retrySpy,
          });
      },
    })
  );

  app.mount(root);

  return {
    root,
    retrySpy,
    unmount: () => {
      app.unmount();
      root.remove();
    },
  };
};

describe("BackendOffline", () => {
  it("cleans html error responses, loads video, and emits retry", async () => {
    const { root, retrySpy, unmount } = mountComponent({
      errorMessage: "<html><h1>502 Bad Gateway</h1></html>",
    });
    await nextTick();

    const messageEl = root.querySelector(".font-mono");
    expect(messageEl?.textContent?.trim()).toBe("502 Bad Gateway");

    const video = root.querySelector("video") as HTMLVideoElement | null;
    expect(video?.src).toContain("/robot.mp4");

    const retryBtn = root.querySelector("button.btn-primary") as HTMLButtonElement;
    retryBtn?.click();
    expect(retrySpy).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("disables retry button while checking", async () => {
    const { root, unmount } = mountComponent({ checking: true });
    await nextTick();

    const retryBtn = root.querySelector("button.btn-primary") as HTMLButtonElement;
    expect(retryBtn?.disabled).toBe(true);

    unmount();
  });
});
