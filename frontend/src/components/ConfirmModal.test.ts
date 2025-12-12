import { describe, it, expect, vi } from "vitest";
import { createApp, defineComponent, h, nextTick, ref } from "vue";
import ConfirmModal from "./ConfirmModal.vue";

const mountModal = (options?: {
  open?: boolean;
  hideCancel?: boolean;
  confirmLabel?: string;
  cancelLabel?: string;
}) => {
  const confirmSpy = vi.fn();
  const cancelSpy = vi.fn();
  const open = ref(options?.open ?? true);
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(ConfirmModal, {
            open: open.value,
            title: "Confirm Action",
            message: "Proceed with change?",
            hideCancel: options?.hideCancel,
            confirmLabel: options?.confirmLabel,
            cancelLabel: options?.cancelLabel,
            onConfirm: confirmSpy,
            onCancel: cancelSpy,
          });
      },
    })
  );

  app.mount(root);

  return {
    confirmSpy,
    cancelSpy,
    open,
    root,
    unmount: () => {
      app.unmount();
      root.remove();
    },
  };
};

describe("ConfirmModal", () => {
  it("renders labels and emits confirm/cancel from buttons", async () => {
    const { root, confirmSpy, cancelSpy, unmount } = mountModal({
      confirmLabel: "Remove",
      cancelLabel: "Back",
    });
    await nextTick();

    const confirmBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Remove")
    ) as HTMLButtonElement | undefined;
    const cancelBtn = Array.from(root.querySelectorAll("button")).find((btn) =>
      btn.textContent?.includes("Back")
    ) as HTMLButtonElement | undefined;

    expect(confirmBtn?.textContent).toContain("Remove");
    expect(cancelBtn?.textContent ?? "").toContain("Back");

    confirmBtn?.click();
    cancelBtn?.click();

    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(cancelSpy).toHaveBeenCalledTimes(1);

    unmount();
  });

  it("responds to keyboard shortcuts only when open", async () => {
    const { confirmSpy, cancelSpy, open, unmount } = mountModal();
    await nextTick();

    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(cancelSpy).toHaveBeenCalledTimes(1);

    open.value = false;
    await nextTick();
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    expect(confirmSpy).toHaveBeenCalledTimes(1);

    unmount();
  });
});
