import type {
  SettingsFormState,
} from "../types/formTypes";

const API_BASE_URL = "/api";

let authToken: string | null = null;
let offlineMode = false;

export const setAuthToken = (token: string | null) => {
  authToken = token;
};

export const setOfflineMode = (offline: boolean) => {
  offlineMode = offline;
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

async function request<T>(
  endpoint: string,
  options: RequestInit = {},
  publicEndpoint = false,
  timeout = 15000
): Promise<T> {
  const isHealthCheck = endpoint === "/health";
  if (offlineMode && !isHealthCheck) {
    throw new ApiError("Backend offline", 503);
  }

  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...options.headers,
  } as HeadersInit;

  if (authToken && !publicEndpoint) {
    (headers as any)["Authorization"] = `Bearer ${authToken}`;
  }

  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), timeout);

  try {
    const response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
      signal: controller.signal,
    });
    clearTimeout(id);

    if (response.status === 204) {
      return {} as T;
    }

    const contentType = response.headers.get("content-type");
    if (!response.ok) {
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
  healthCheck: () => request<HealthResponse>("/health", {}, true, 5000),

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

  regenerateRecoveryCodes: () =>
    request<{ recoveryCodes: string[] }>("/2fa/regenerate", {
      method: "POST",
    }),

  disable2FA: (code: string, password: string) =>
    request<unknown>("/2fa/disable", {
      method: "POST",
      body: JSON.stringify({ code, password }),
    }),

  getDashboard: () => request<DashboardStats>("/dashboard"),

  getContainers: () => request<Container[]>("/containers"),
  getHostInfo: () =>
    request<{ dockerVersion: string; platform: string; hostname: string; lastSeen: string }>(
      "/containers/host-info"
    ),
  checkContainerUpdate: (id: string) =>
    request<{ updateAvailable: boolean }>(`/containers/${id}/check-update`, {
      method: "POST",
    }),
  updateContainer: async (
    id: string,
    onMessage: (message: Record<string, any>) => void
  ) => {
    const headers: HeadersInit = { "Content-Type": "application/json" };
    if (authToken) {
      (headers as any)["Authorization"] = `Bearer ${authToken}`;
    }
    const response = await fetch(`${API_BASE_URL}/containers/${id}/update`, {
      method: "POST",
      headers,
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

  getPublicRuntimeSettings: () =>
    request<{
      databaseUrl?: string;
      clientOrigin?: string;
      secretKey?: string;
      timezone?: string;
      needsSetup?: boolean;
      ssoEnabled?: boolean;
    }>("/runtime-settings", {}, true, 5000),
  updatePublicRuntimeSettings: (payload: {
    databaseUrl: string;
    clientOrigin: string;
    secretKey: string;
    timezone: string;
  }) =>
    request<unknown>("/runtime-settings", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  setupGenerate: (secretKey?: string) =>
    request<{ secret: string; qrCode: string }>("/auth/setup/generate", {
      method: "POST",
      body: JSON.stringify({ secretKey }),
    }),

  setupCreate: (payload: unknown) =>
    request<{ message: string; recoveryCodes: string[] }>("/auth/setup/create", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  setupTestDb: (databaseUrl: string) =>
    request<unknown>("/auth/setup/test-db", {
      method: "POST",
      body: JSON.stringify({ databaseUrl }),
    }),

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
    if (authToken) {
      (headers as any)["Authorization"] = `Bearer ${authToken}`;
    }
    const res = await fetch(`${API_BASE_URL}/settings/ca-cert`, { headers });
    if (!res.ok) {
      const message = `Failed to download cert (${res.status})`;
      throw new ApiError(message, res.status);
    }
    return res.blob();
  },
};

interface loginRequest {
  username: string;
  password: string;
}
