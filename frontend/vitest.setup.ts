import { vi } from "vitest";

// Mock global fetch if needed in components/services tests
if (typeof fetch === "undefined") {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
  global.fetch = vi.fn();
}
