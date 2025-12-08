import { describe, it, expect, vi, afterEach } from "vitest";
import { api, ApiError, setOnUnauthorized } from "./api";

describe("api request error handling", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    if (originalFetch) {
      global.fetch = originalFetch;
    }
    setOnUnauthorized(null as any);
  });

  it("retries on 503 and eventually succeeds", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(new Response(null, { status: 503 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ value: 42 }), {
          status: 200,
          headers: { "content-type": "application/json" },
        })
      );
    global.fetch = mockFetch as any;

    const result = await api.healthCheck();
    expect(result).toBeDefined();
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it("calls unauthorized callback on 401", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ error: "unauth" }), { status: 401 }));
    global.fetch = mockFetch as any;
    const spy = vi.fn();
    setOnUnauthorized(spy);

    await expect(api.getProfile()).rejects.toBeInstanceOf(ApiError);
    expect(spy).toHaveBeenCalled();
  });
});
