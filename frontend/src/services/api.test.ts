import { describe, it, expect, vi, afterEach } from "vitest";
import { api, ApiError, setOfflineMode, setOnUnauthorized } from "./api";

describe("api request error handling", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    if (originalFetch) {
      global.fetch = originalFetch;
    }
    setOnUnauthorized(null as any);
    setOfflineMode(false);
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

  it("surfaces backend error messages", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ error: "boom" }), {
          status: 500,
          headers: { "content-type": "application/json" },
        })
      );
    global.fetch = mockFetch as any;

    await expect(api.getDashboard()).rejects.toMatchObject({
      message: "boom",
      status: 500,
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("prevents non-public requests when offline mode is enabled", async () => {
    const mockFetch = vi.fn();
    global.fetch = mockFetch as any;
    setOfflineMode(true);

    await expect(api.getDashboard()).rejects.toBeInstanceOf(ApiError);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("refreshes the session on 401 and retries the original request", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "expired" }), {
          status: 401,
          headers: { "content-type": "application/json" },
        })
      )
      .mockResolvedValueOnce(new Response(null, { status: 200 })) // refresh success
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ username: "demo", name: "Demo", role: "admin" }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          }
        )
      );
    global.fetch = mockFetch as any;

    const user = await api.getProfile();

    expect(user).toEqual({ username: "demo", name: "Demo", role: "admin" });
    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect((mockFetch as any).mock.calls[1][0]).toContain("/auth/refresh");
  });

  it("notifies unauthorized when refresh fails after 401", async () => {
    const mockFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: "expired" }), {
          status: 401,
          headers: { "content-type": "application/json" },
        })
      )
      .mockResolvedValueOnce(new Response("fail", { status: 500 }));
    global.fetch = mockFetch as any;
    const spy = vi.fn();
    setOnUnauthorized(spy);

    await expect(api.getProfile()).rejects.toBeInstanceOf(ApiError);
    expect(spy).toHaveBeenCalledTimes(2); // once in refresh catch, once on final 401 handling
  });
});
