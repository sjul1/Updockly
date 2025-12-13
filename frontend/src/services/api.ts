import type {
  SettingsFormState,
} from "../types/formTypes";

const API_BASE_URL = "/api";

let offlineMode = false;
type UnauthorizedCallback = () => void;
let onUnauthorized: UnauthorizedCallback | null = null;
let refreshing: Promise<void> | null = null;
const MAX_RETRIES = 2;
const RETRY_DELAYS = [300, 800];
const RETRYABLE_STATUSES = new Set([502, 503, 504]);

export const setOfflineMode = (offline: boolean) => {
  offlineMode = offline;
};

export const setOnUnauthorized = (cb: UnauthorizedCallback) => {
  onUnauthorized = cb;
};

const refreshSession = async () => {
  if (!refreshing) {
    refreshing = (async () => {
      const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) {
        throw new ApiError("refresh failed", res.status);
      }
    })().finally(() => {
      refreshing = null;
    });
  }
  return refreshing;
};

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

export interface ApiUser {
  username: string;
  name: string;
  role: string;
  email?: string;
  twoFactorEnabled?: boolean;
}

export interface LoginResponse {
  token?: string;
  user?: ApiUser;
  twoFactorRequired?: boolean;
  tempToken?: string;
}

export interface HealthResponse {
  status: string;
  time: string;
  version: string;
}

export interface DashboardStats {
  message: string;
  time: string;
  totalContainers: number;
  runningContainers: number;
  autoUpdateEnabled: number;
  scheduleCount: number;
  agentCount?: number;
  agentOnline?: number;
}

export interface Container {
  ID: string;
  Name: string;
  Image: string;
  State: string;
  Status: string;
  AutoUpdate: boolean;
  UpdateAvailable?: boolean;
  Ports?: string[];
}

export interface UpdateHistory {
  id: string;
  containerId: string;
  containerName: string;
  image: string;
  imageDigest?: string;
  agentId?: string;
  agentName?: string;
  source: string;
  status: string;
  message: string;
  createdAt: string;
  updatedAt?: string;
}

export interface RunningHistoryEntry {
  id: string;
  date: string;
  running: number;
  total: number;
}

export interface Schedule {
  ID: string;
  Name: string;
  CronExpression: string;
  CreatedAt?: string;
  UpdatedAt?: string;
}

export interface Agent {
  id: string;
  name: string;
  hostname: string;
  notes: string;
  tlsEnabled?: boolean;
  agentVersion?: string;
  dockerVersion?: string;
  platform?: string;
  lastSeen?: string;
  createdAt?: string;
  updatedAt?: string;
  containers?: AgentContainer[];
  cpu?: number;
  memory?: number;
  tokenBound?: boolean;
}

export type ApiAgent = Agent;

export interface AgentWithToken extends Agent {
  token?: string;
}

export interface AgentContainer {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  autoUpdate?: boolean;
  updateAvailable?: boolean;
  ports?: string[];
  labels?: string[];
  checkedAt?: string;
}

export interface AgentCommand {
  id: string;
  agentId: string;
  type: string;
  status: string;
  payload?: Record<string, unknown>;
  createdAt?: string;
}

export interface AuditLog {
  id: number;
  userId: string;
  username: string;
  action: string;
  details: string;
  ipAddress: string;
  createdAt: string;
}

async function request<T>(
  endpoint: string,
  options: RequestInit = {},
  publicEndpoint = false,
  timeout = 15000
): Promise<T> {
  const isHealthCheck = endpoint === "/health";
  const isSetupCheck = endpoint === "/auth/setup/status" || endpoint === "/auth/setup/runtime-settings";
  if (offlineMode && !isHealthCheck && !isSetupCheck) {
    throw new ApiError("Backend offline", 503);
  }

  const getCookie = (name: string) => {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop()?.split(";").shift();
  };

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...options.headers,
  } as HeadersInit;

  const csrfToken = getCookie("csrf_token");
  if (csrfToken) {
    (headers as Record<string, string>)["X-CSRF-Token"] = csrfToken;
  }

  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), timeout);

  const makeRequest = async (): Promise<Response> =>
    fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
      credentials: "include",
      signal: controller.signal,
    });

  const fetchWithRetry = async (): Promise<Response> => {
    let attempt = 0;
    let lastError: any;
    while (attempt <= MAX_RETRIES) {
      try {
        const res = await makeRequest();
        if (RETRYABLE_STATUSES.has(res.status) && attempt < MAX_RETRIES) {
          lastError = new ApiError(`transient ${res.status}`, res.status);
          await new Promise((resolve) =>
            setTimeout(resolve, (RETRY_DELAYS[attempt] || 500) + Math.random() * 100)
          );
          attempt++;
          continue;
        }
        return res;
      } catch (err) {
        lastError = err;
        if (attempt === MAX_RETRIES) {
          throw lastError;
        }
        await new Promise((resolve) =>
          setTimeout(resolve, (RETRY_DELAYS[attempt] || 500) + Math.random() * 100)
        );
        attempt++;
      }
    }
    throw lastError;
  };

  try {
    let response = await fetchWithRetry();
    if (!publicEndpoint && endpoint !== "/auth/refresh" && response.status === 401) {
      try {
        await refreshSession();
        response = await fetchWithRetry();
      } catch {
        onUnauthorized?.();
      }
    }
    clearTimeout(id);

    if (response.status === 204) {
      return {} as T;
    }

    const contentType = response.headers.get("content-type");
    if (!response.ok) {
      if (response.status === 401) {
        onUnauthorized?.();
      }
      let message = "An error occurred";
      if (contentType && contentType.includes("application/json")) {
        const errorData = await response.json();
        message = errorData.error || message;
      } else {
        message = await response.text();
      }
      throw new ApiError(message, response.status);
    }

    if (contentType && contentType.includes("application/json")) {
      return (await response.json()) as T;
    }
    return {} as T;
  } catch (error) {
    clearTimeout(id);
    throw error;
  }
}

export const api = {
  setOnUnauthorized,
  healthCheck: () => request<HealthResponse>("/health", {}, true, 500),

  login: (credentials: loginRequest) =>
    request<LoginResponse>(
      "/auth/login",
      {
        method: "POST",
        body: JSON.stringify(credentials),
      },
      true,
    ),

  getProfile: () => request<ApiUser>("/auth/me"),
  updateProfile: (payload: { name?: string; email?: string; currentPassword?: string; newPassword?: string }) =>
    request<ApiUser>("/auth/me", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  verify2FA: (tempToken: string, code: string) =>
    request<LoginResponse>("/auth/2fa/verify", {
      method: "POST",
      body: JSON.stringify({ tempToken, code }),
    }),

  resetPassword: (payload: { username: string; recoveryCode: string; newPassword: string }) =>
    request<LoginResponse>("/auth/reset-password", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  forgotPassword: (email: string) =>
    request<{ message: string }>("/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),

  resetPasswordWithToken: (payload: { token: string; newPassword: string }) =>
    request<{ message: string }>("/auth/reset-password-token", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  generate2FA: () =>
    request<{ secret: string; qrCode: string }>("/2fa/generate", {
      method: "POST",
    }),

  enable2FA: (code: string) =>
    request<{ message: string; recoveryCodes: string[] }>("/2fa/enable", {
      method: "POST",
      body: JSON.stringify({ code }),
    }),

  regenerateRecoveryCodes: (password: string) =>
    request<{ recoveryCodes: string[] }>("/2fa/regenerate", {
      method: "POST",
      body: JSON.stringify({ password }),
    }),

  disable2FA: (code: string, password: string) =>
    request<unknown>("/2fa/disable", {
      method: "POST",
      body: JSON.stringify({ code, password }),
    }),

  initiateReset2FA: (username: string, recoveryCode: string, password: string) =>
    request<{
      secret: string;
      qrCode: string;
      tempToken: string;
    }>("/auth/2fa/reset/init", {
      method: "POST",
      body: JSON.stringify({ username, recoveryCode, password }),
    }, true),

  finalizeReset2FA: (tempToken: string, code: string) =>
    request<{
      message: string;
      recoveryCodes: string[];
    }>("/auth/2fa/reset/finalize", {
      method: "POST",
      body: JSON.stringify({ tempToken, code }),
    }, true),

  getDashboard: () => request<DashboardStats>("/dashboard"),

  getContainers: () => request<Container[]>("/containers"),
  getHostInfo: () =>
    request<{
      dockerVersion: string;
      platform: string;
      hostname: string;
      lastSeen: string;
      cpu?: number;
      memory?: number;
    }>("/containers/host-info", {}, false, 8000),
  checkContainerUpdate: (id: string) =>
    request<{ updateAvailable: boolean }>(`/containers/${id}/check-update`, {
      method: "POST",
    }),
  updateContainer: async (
    id: string,
    onMessage: (message: Record<string, any>) => void
  ) => {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    const response = await fetch(`${API_BASE_URL}/containers/${id}/update`, {
      method: "POST",
      headers,
      credentials: "include",
    });
    if (!response.body) {
      onMessage({ error: "No response body received from update endpoint." });
      return;
    }
    if (!response.ok) {
      const errorText = await response.text();
      onMessage({ error: errorText || "Container update failed." });
      return;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let newlineIndex = buffer.indexOf("\n");
      while (newlineIndex !== -1) {
        const line = buffer.slice(0, newlineIndex).trim();
        buffer = buffer.slice(newlineIndex + 1);
        if (line) {
          try {
            onMessage(JSON.parse(line));
          } catch (error) {
            console.error("Failed to parse update stream message", error, line);
          }
        }
        newlineIndex = buffer.indexOf("\n");
      }
    }
  },
  toggleAutoUpdate: (id: string, enabled: boolean) =>
    request<unknown>(`/containers/${id}/auto-update`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),
  startContainer: (id: string) =>
    request<{ message: string }>(`/containers/${id}/start`, { method: "POST" }),
  stopContainer: (id: string) =>
    request<{ message: string }>(`/containers/${id}/stop`, { method: "POST" }),
  restartContainer: (id: string) =>
    request<{ message: string }>(`/containers/${id}/restart`, { method: "POST" }),
  getContainerLogs: (id: string, tail = 200) =>
    request<{ logs: string }>(`/containers/${id}/logs?tail=${tail}`),
  rollbackContainer: (id: string, image: string, historyId?: string) =>
    request<{ message: string; newId?: string }>(`/containers/${id}/rollback`, {
      method: "POST",
      body: JSON.stringify({ image, historyId }),
    }),
  deleteHistoryEntry: (id: string) =>
    request<unknown>(`/history/${id}`, {
      method: "DELETE",
    }),
  getAutoUpdateCount: () =>
    request<{ count: number }>("/containers/auto-update/count"),
  getUpdateHistory: (limit = 200) =>
    request<UpdateHistory[]>(`/history?limit=${limit}`),
  getRunningHistory: () => request<RunningHistoryEntry[]>(`/metrics/running-history`),

  getSetupStatus: () =>
    request<{ needsSetup: boolean }>("/auth/setup/status", {}, true, 500),
  getSetupRuntimeSettings: () =>
    request<{
      databaseUrl?: string;
      jwtSecret?: string;
      vaultKey?: string;
      recoveryCodes?: string[];
    }>("/auth/setup/runtime-settings", {}, true, 500),

  setupGenerate: () =>
    request<{ secret: string; qrCode: string }>("/auth/setup/generate", {
      method: "POST",
    }),

  setupCreate: (payload: unknown) =>
    request<{ message: string; recoveryCodes: string[]; jwtSecret?: string; vaultKey?: string }>("/auth/setup/create", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  logout: (options: RequestInit = {}) =>
    request<{ message: string }>("/auth/logout", { ...options, method: "POST" }),

  setupTestDb: (databaseUrl: string) =>
    request<unknown>("/auth/setup/test-db", {
      method: "POST",
      body: JSON.stringify({ databaseUrl }),
    }),

  getAuditLogs: (limit = 100) => request<AuditLog[]>(`/audit-logs?limit=${limit}`),

  getSettings: () => request<SettingsFormState>("/settings"),

  updateSettings: (payload: SettingsFormState) =>
    request<SettingsFormState>("/settings", {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  testNotification: () =>
    request<{ message: string }>("/notifications/test", {
      method: "POST",
    }),

  testEmailNotification: () =>
    request<{ message: string }>("/notifications/test-email", {
      method: "POST",
    }),

  getAgents: () => request<ApiAgent[]>("/agents"),
  createAgent: (payload: { name: string; hostname?: string; notes?: string; tlsEnabled?: boolean }) =>
    request<AgentWithToken>("/agents", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  rotateAgentToken: (id: string) =>
    request<AgentWithToken>(`/agents/${id}/rotate-token`, { method: "POST" }),
  updateAgent: (id: string, payload: { name: string; hostname?: string; notes?: string; tlsEnabled?: boolean }) =>
    request<Agent>(`/agents/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  toggleAgentContainerAutoUpdate: (
    agentId: string,
    containerId: string,
    enabled: boolean
  ) =>
    request<unknown>(`/agents/${agentId}/containers/${containerId}/auto-update`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    }),
  startAgentContainer: (agentId: string, containerId: string) =>
    request<unknown>(`/agents/${agentId}/containers/${containerId}/start`, {
      method: "POST",
    }),
  stopAgentContainer: (agentId: string, containerId: string) =>
    request<unknown>(`/agents/${agentId}/containers/${containerId}/stop`, {
      method: "POST",
    }),
  restartAgentContainer: (agentId: string, containerId: string) =>
    request<unknown>(`/agents/${agentId}/containers/${containerId}/restart`, {
      method: "POST",
    }),
  rollbackAgentContainer: (
    agentId: string,
    containerId: string,
    image: string,
    historyId?: string
  ) =>
    request<{ message: string; newId?: string }>(
      `/agents/${agentId}/containers/${containerId}/rollback`,
      {
        method: "POST",
        body: JSON.stringify({ image, historyId }),
      }
    ),
  getAgentContainerLogs: (agentId: string, containerId: string, tail = 200) =>
    request<{ logs?: string; message?: string }>(
      `/agents/${agentId}/containers/${containerId}/logs?tail=${tail}`
    ),
  createAgentCommand: (id: string, type: string, containerId: string) =>
    request<AgentCommand>(`/agents/${id}/commands`, {
      method: "POST",
      body: JSON.stringify({ type, containerId }),
    }),
  deleteAgent: (id: string) => request<unknown>(`/agents/${id}`, { method: "DELETE" }),

  getSchedules: () => request<Schedule[]>("/schedules"),
  createSchedule: (name: string, cronExpression: string) =>
    request<Schedule>("/schedules", {
      method: "POST",
      body: JSON.stringify({ name, cronExpression }),
    }),
  updateSchedule: (id: string, name: string, cronExpression: string) =>
    request<Schedule>(`/schedules/${id}`, {
      method: "PUT",
      body: JSON.stringify({ name, cronExpression }),
    }),
  deleteSchedule: (id: string) =>
    request<unknown>(`/schedules/${id}`, {
      method: "DELETE",
    }),

  downloadCACert: async () => {
    const headers: HeadersInit = { Accept: "application/x-x509-ca-cert" };
    const res = await fetch(`${API_BASE_URL}/settings/ca-cert`, { headers, credentials: "include" });
    if (!res.ok) {
      const message = `Failed to download cert (${res.status})`;
      throw new ApiError(message, res.status);
    }
    return res.blob();
  },

  getPublicConfig: () =>
    request<{ sso: { enabled: boolean; provider: string } }>("/auth/config", {}, true),
};

interface loginRequest {
  username: string;
  password: string;
}
