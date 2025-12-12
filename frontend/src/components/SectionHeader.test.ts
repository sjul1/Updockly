import { describe, it, expect } from "vitest";
import { createApp, defineComponent, h, nextTick } from "vue";
import SectionHeader from "./SectionHeader.vue";

const FakeIcon = defineComponent({
  name: "FakeIcon",
  setup: () => () => h("span", { class: "fake-icon" }, "icon"),
});

const mountHeader = () => {
  const root = document.createElement("div");
  document.body.appendChild(root);

  const app = createApp(
    defineComponent({
      setup() {
        return () =>
          h(
            SectionHeader,
            { title: "Dashboard", icon: FakeIcon },
            {
              eyebrow: () => "Live",
              subtitle: () => "Overview of your containers",
              badges: () => h("span", { class: "badge" }, "Badge"),
              meta: () => h("span", { class: "meta" }, "Meta info"),
              actions: () => h("button", { class: "action" }, "Add"),
            }
          );
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

describe("SectionHeader", () => {
  it("renders title, icon, and all provided slots", async () => {
    const { root, unmount } = mountHeader();
    await nextTick();

    expect(root.textContent).toContain("Dashboard");
    expect(root.textContent).toContain("Live");
    expect(root.textContent).toContain("Overview of your containers");
    expect(root.textContent).toContain("Badge");
    expect(root.textContent).toContain("Meta info");
    expect(root.textContent).toContain("Add");

    const icon = root.querySelector(".fake-icon");
    expect(icon?.textContent).toBe("icon");

    unmount();
  });
});
