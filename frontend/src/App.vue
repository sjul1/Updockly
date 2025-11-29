<script setup lang="ts">
import {
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
  computed,
  provide,
} from "vue";
import {
  Gauge,
  Lock,
  SlidersHorizontal,
  Boxes,
  CalendarClock,
  ServerCog,
  History as HistoryIcon,
  Menu,
  Activity,
} from "lucide-vue-next";
import {
  ApiError,
  api,
  setAuthToken,
  setOfflineMode,
  type ApiUser,
  type DashboardStats,
} from "./services/api";
import LoginPanel from "./components/Login.vue";
import AppSidebar, {
  type Panel as SidebarPanel,
  type NavItem,
} from "./components/AppSidebar.vue";
import SettingsPanel from "./components/Settings.vue";
import DashboardPanel from "./components/Dashboard.vue";
import ContainersPanel from "./components/Containers.vue";
import HistoryPanel from "./components/History.vue";
import SchedulePanel from "./components/Schedule.vue";
import AgentsPanel from "./components/Agents.vue";
import BackendOffline from "./components/BackendOffline.vue";
import Setup from "./components/Setup.vue";
import { useToast } from "vue-toastification";
import type {
  SettingsFormState,
} from "./types/formTypes";

const storedTheme =
  (localStorage.getItem("replicore_theme") as "light" | "dark") || "light";
const theme = ref<"light" | "dark">(storedTheme);
const applyTheme = () => {
  document.documentElement.setAttribute("data-theme", theme.value);
};
const toggleTheme = () => {
  theme.value = theme.value === "light" ? "dark" : "light";
  localStorage.setItem("replicore_theme", theme.value);
  applyTheme();
};

const browserTimezone =
  Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";

const healthStatus = ref("Checking...");
const backendVersion = ref("");
const backendOffline = ref(false);
const backendErrorMessage = ref("");
const healthPollTimer = ref<number | null>(null);
const HEALTH_CHECK_INTERVAL_MS = 30000;
const dataPollTimer = ref<number | null>(null);
const DATA_POLL_INTERVAL_MS = 30000;
const currentUser = ref<ApiUser | null>(null);
const sessionToken = ref<string | null>(
  localStorage.getItem("replicore_token")
);
const tempToken = ref("");
const twoFactorRequired = ref(false);
const needsSetup = ref(false);
const ssoEnabled = ref(false);
if (sessionToken.value) {
  setAuthToken(sessionToken.value);
}

const PANEL_STORAGE_KEY = "replicore_active_panel";
const panelOptions: SidebarPanel[] = [
  "login",
  "containers",
  "history",
  "schedule",
  "agents",
  "settings",
];
const isSidebarPanel = (value: string | null): value is SidebarPanel =>
  Boolean(value && panelOptions.includes(value as SidebarPanel));

const storedPanel = localStorage.getItem(PANEL_STORAGE_KEY);
const activePanel = ref<SidebarPanel>(
  isSidebarPanel(storedPanel) ? storedPanel : "login"
);

const isSidebarOpen = ref(false);

watch(activePanel, (panel) => {
  localStorage.setItem(PANEL_STORAGE_KEY, panel);
  isSidebarOpen.value = false;
});

const createDefaultSettings = (): SettingsFormState => ({
  databaseUrl: "",
  clientOrigin: "",
  secretKey: "",
  timezone: browserTimezone,
  autoPruneImages: false,
  hideSupportButton: false,
  backupDestination: {
    type: "local",
    webdavUrl: "",
    webdavUsername: "",
    webdavPassword: "",
    googleCredentials: "",
    onedriveTenant: "",
    onedriveClientId: "",
    onedriveClientSecret: "",
  },
  notifications: {
    webhookUrl: "",
    discordToken: "",
    discordChannel: "",
    onSuccess: false,
    onFailure: false,
    recapTime: "",
    notificationCron: "",
    smtp: {
      host: "",
      port: 587,
      user: "",
      password: "",
      from: "",
      tls: false,
      enabled: false,
    },
  },
  sso: {
    enabled: false,
    provider: "",
    issuerUrl: "",
    clientId: "",
    clientSecret: "",
    redirectUrl: "",
  },
});
const settingsForm = reactive<SettingsFormState>(createDefaultSettings());

type FormatTimeFn = (value: string | number | Date | null | undefined) => string;
const formatTime: FormatTimeFn = (value) => {
  if (!value) return "--:--:--";
  const date = value instanceof Date ? value : new Date(value);
  try {
    return new Intl.DateTimeFormat(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
      timeZone: settingsForm.timezone || browserTimezone,
    }).format(date);
  } catch {
    const iso = date.toISOString().split("T")[1] ?? "";
    return iso.slice(0, 8) || "--:--:--";
  }
};

const formatDateTime = (value: string | number | Date | null | undefined) => {
  if (!value) return "";
  const date = value instanceof Date ? value : new Date(value);
  try {
    return new Intl.DateTimeFormat(undefined, {
      dateStyle: "medium",
      timeStyle: "medium",
      hour12: false,
      timeZone: settingsForm.timezone || browserTimezone,
    }).format(date);
  } catch {
    return date.toISOString();
  }
};

provide("formatAppTime", formatTime);
provide("formatAppDateTime", formatDateTime);
provide(
  "appTimezone",
  computed(() => settingsForm.timezone || browserTimezone)
);

const baselineState = ref("");
const dirtyRuntime = ref(false);

const syncRuntimeBaseline = () => {
  baselineState.value = JSON.stringify({
    databaseUrl: settingsForm.databaseUrl,
    clientOrigin: settingsForm.clientOrigin,
    secretKey: settingsForm.secretKey,
    timezone: settingsForm.timezone,
  });
  dirtyRuntime.value = false;
};

const dashboard = ref<DashboardStats | null>(null);

const loading = reactive({
  health: false,
  login: false,
  bootstrap: false,
  settings: false,
  notificationsTest: false,
  userUpdate: false,
});

const isAuthenticated = computed(() => Boolean(currentUser.value));
const navItems = computed<NavItem[]>(() => {
  const items: NavItem[] = [
    {
      id: "login",
      label: isAuthenticated.value ? "Dashboard" : "Login",
      icon: isAuthenticated.value ? Gauge : Lock,
    },
  ];
  if (isAuthenticated.value) {
    items.push({ id: "containers", label: "Containers", icon: Boxes });
    items.push({ id: "history", label: "History", icon: HistoryIcon });
    items.push({ id: "schedule", label: "Schedule", icon: CalendarClock });
    items.push({ id: "agents", label: "Agents", icon: ServerCog });
    items.push({ id: "settings", label: "Settings", icon: SlidersHorizontal });
  }
  return items;
});

watch(
  () => ({ authed: isAuthenticated.value, token: sessionToken.value }),
  ({ authed, token }) => {
    if (!authed && !token && activePanel.value !== "login") {
      activePanel.value = "login";
    }
  }
);

const toast = useToast();
const notify = (type: "success" | "error", message: string) => {
  if (type === "success") {
    toast.success(message);
  } else {
    toast.error(message);
  }
};

const persistToken = (value: string | null) => {
  sessionToken.value = value;
  if (value) {
    localStorage.setItem("replicore_token", value);
    setAuthToken(value);
  } else {
    localStorage.removeItem("replicore_token");
    setAuthToken(null);
  }
};

watch(
  () => [
    settingsForm.databaseUrl,
    settingsForm.clientOrigin,
    settingsForm.secretKey,
    settingsForm.timezone,
  ],
  (next) => {
    if (isAuthenticated.value) {
      return;
    }
    const current = JSON.stringify({
      databaseUrl: next[0],
      clientOrigin: next[1],
      secretKey: next[2],
      timezone: next[3],
    });
    dirtyRuntime.value = current !== baselineState.value;
  },
  { deep: true }
);

const handleApiError = (error: unknown, fallbackMessage: string) => {
  if (error instanceof ApiError && error.status === 401) {
    notify("error", "Session expired. Please sign in again.");
    logout();
    return;
  }
  if (
    (error instanceof ApiError && error.status === 503) ||
    error instanceof TypeError
  ) {
    const message =
      (error instanceof Error && error.message) || "Unable to reach the backend";
    backendErrorMessage.value = message;
    backendOffline.value = true;
    healthStatus.value = "Offline";
    backendVersion.value = "";
    setOfflineMode(true);
    stopDataPolling();
    return;
  }
  console.error(error);
  const message =
    (error instanceof Error && error.message) || fallbackMessage || "An error occurred";
  notify("error", message);
};

const handleSetupComplete = async () => {
  needsSetup.value = false;
  activePanel.value = "login";
  notify("success", "Setup complete. Please log in.");
};

const lastHealthCheckAt = ref(0);
let healthRequestInFlight: Promise<boolean> | null = null;

type HealthOptions = { silent?: boolean; force?: boolean };

const checkHealth = async (options?: HealthOptions) => {
  const { silent = false, force = false } = options ?? {};
  if (!force && healthRequestInFlight) {
    return healthRequestInFlight;
  }
  const now = Date.now();
  if (!force && now - lastHealthCheckAt.value < HEALTH_CHECK_INTERVAL_MS) {
    return true;
  }
  lastHealthCheckAt.value = now;
  const request = (async () => {
    if (!silent) {
      loading.health = true;
    }
    const wasOffline = backendOffline.value;
    try {
      const result = await api.healthCheck();
      healthStatus.value = result.status;
      backendVersion.value = result.version;
      backendOffline.value = false;
      backendErrorMessage.value = "";
      setOfflineMode(false);
      if (wasOffline) {
        startDataPolling();
      }
      return true;
    } catch (error) {
      const message =
        (error instanceof Error && error.message) || "Backend health check failed";
      console.error(error);
      healthStatus.value = "Offline";
      backendVersion.value = "";
      backendOffline.value = true;
      backendErrorMessage.value = message;
      setOfflineMode(true);
      stopDataPolling();
      if (!silent) {
        notify("error", message);
      }
      return false;
    } finally {
      if (!silent) {
        loading.health = false;
      }
      healthRequestInFlight = null;
    }
  })();
  healthRequestInFlight = request;
  return request;
};

const stopHealthPolling = () => {
  if (healthPollTimer.value) {
    window.clearTimeout(healthPollTimer.value);
    healthPollTimer.value = null;
  }
};

const scheduleHealthPolling = () => {
  if (healthPollTimer.value) {
    return;
  }
  healthPollTimer.value = window.setTimeout(async () => {
    healthPollTimer.value = null;
    await checkHealth({ silent: true });
    scheduleHealthPolling();
  }, HEALTH_CHECK_INTERVAL_MS);
};

const ensureSession = async () => {
  if (!sessionToken.value || backendOffline.value) return;
  try {
    currentUser.value = await api.getProfile();
    await loadAllData();
    await fetchSettings();
  } catch (error) {
    handleApiError(error, "Failed to restore session");
  }
};

const loadAllData = async () => {
  if (!isAuthenticated.value) return;
  loading.bootstrap = true;
  try {
    const [stats] = await Promise.all([
      api.getDashboard(),
    ]);
    dashboard.value = stats;
  } catch (error) {
    handleApiError(error, "Unable to load data");
  } finally {
    loading.bootstrap = false;
  }
};

const logout = () => {
  persistToken(null);
  currentUser.value = null;
  dashboard.value = null;
  hydrateSettings(createDefaultSettings());
  activePanel.value = "login";
  void fetchPublicRuntimeSettings();
};

const handleLogin = async (payload: { username: string; password: string }) => {
  loading.login = true;
  try {
    const response = await api.login(payload);
    if (response.twoFactorRequired && response.tempToken) {
      currentUser.value = null;
      tempToken.value = response.tempToken;
      twoFactorRequired.value = true;
      return;
    }
    if (response.token && response.user) {
      persistToken(response.token);
      currentUser.value = response.user;
      notify("success", `Welcome back ${response.user.name}`);
      await loadAllData();
      await fetchSettings();
    }
  } catch (error) {
    throw error;
  } finally {
    loading.login = false;
  }
};

const handle2FAVerify = async (code: string) => {
  loading.login = true;
  try {
    const response = await api.verify2FA(tempToken.value, code);
    if (response.token && response.user) {
      persistToken(response.token);
      currentUser.value = response.user;
      notify("success", `Welcome back ${response.user.name}`);
      await loadAllData();
      await fetchSettings();
      twoFactorRequired.value = false;
      tempToken.value = "";
    }
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      notify("error", "Invalid code. Please try again.");
      throw error; // Re-throw so the Login component can handle the failure count
    }
    handleApiError(error, "Failed to verify 2FA code");
  } finally {
    loading.login = false;
  }
};

const hydrateSettings = (next: SettingsFormState) => {
  const defaults = createDefaultSettings();
  const nextNotifications = next.notifications ?? defaults.notifications;
  const nextSSO = next.sso ?? defaults.sso;
  Object.assign(settingsForm, defaults, next, {
    backupDestination: {
      ...defaults.backupDestination,
      ...next.backupDestination,
    },
    notifications: {
      ...defaults.notifications,
      ...nextNotifications,
      smtp: {
        ...defaults.notifications.smtp,
        ...(nextNotifications.smtp ?? {}),
      },
    },
    sso: {
      ...defaults.sso,
      ...nextSSO,
    },
    autoPruneImages:
      typeof next.autoPruneImages === "boolean"
        ? next.autoPruneImages
        : defaults.autoPruneImages,
    hideSupportButton:
      typeof next.hideSupportButton === "boolean"
        ? next.hideSupportButton
        : defaults.hideSupportButton,
  });
  if (!settingsForm.timezone) {
    settingsForm.timezone = browserTimezone;
  }
};

const fetchSettings = async () => {
  if (!isAuthenticated.value) return;
  loading.settings = true;
  try {
    const response = await api.getSettings();
    hydrateSettings(response);
  } catch (error) {
    handleApiError(error, "Unable to load settings");
  } finally {
    loading.settings = false;
  }
};
const loadSettings = fetchSettings;

const fetchUserProfile = ensureSession;


const fetchPublicRuntimeSettings = async () => {
  try {
    const runtime = await api.getPublicRuntimeSettings();
    if (runtime.databaseUrl) {
      settingsForm.databaseUrl = runtime.databaseUrl;
    }
    if (runtime.clientOrigin) {
      settingsForm.clientOrigin = runtime.clientOrigin;
    }
    if (runtime.timezone) {
      settingsForm.timezone = runtime.timezone;
    }
    if (runtime.secretKey) {
      settingsForm.secretKey = runtime.secretKey;
    }
    if (runtime.needsSetup) {
      needsSetup.value = true;
    }
    if (runtime.ssoEnabled) {
      ssoEnabled.value = true;
    }
    syncRuntimeBaseline();
  } catch (error) {
    console.error("Unable to load runtime settings", error);
  }
};

const saveSettings = async () => {
  if (
    settingsForm.notifications.recapTime &&
    !/^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/.test(settingsForm.notifications.recapTime)
  ) {
    notify("error", "Recap time must be in HH:mm (24h) format");
    return;
  }
  loading.settings = true;
  try {
    if (!isAuthenticated.value) {
      await api.updatePublicRuntimeSettings({
        databaseUrl: settingsForm.databaseUrl,
        clientOrigin: settingsForm.clientOrigin,
        secretKey: settingsForm.secretKey,
        timezone: settingsForm.timezone,
      });
      notify("success", "Runtime settings saved");
      syncRuntimeBaseline();
      return;
    }
    const response = await api.updateSettings(settingsForm);
    hydrateSettings(response);
    notify("success", "Settings saved");
  } catch (error) {
    handleApiError(error, "Unable to save settings");
  } finally {
    loading.settings = false;
  }
};

const testNotification = async () => {
  loading.notificationsTest = true;
  try {
    await api.testNotification();
    notify("success", "Test notification sent!");
  } catch (error) {
    notify("error", error instanceof Error ? error.message : "Test failed");
  } finally {
    loading.notificationsTest = false;
  }
};

const testEmailNotification = async () => {
  loading.notificationsTest = true;
  try {
    await api.testEmailNotification();
    notify("success", "Test email sent! Check your admin email inbox.");
  } catch (error) {
    notify("error", error instanceof Error ? error.message : "Email test failed");
  } finally {
    loading.notificationsTest = false;
  }
};

const updateUserProfile = async (payload: {
  name?: string;
  email?: string;
  currentPassword?: string;
  newPassword?: string;
}) => {
  loading.userUpdate = true;
  try {
    await api.updateProfile(payload);
    await fetchUserProfile();
    notify("success", "User settings updated");
  } catch (error) {
    handleApiError(error, "Failed to update user settings");
  } finally {
    loading.userUpdate = false;
  }
};

const loadDashboard = async () => {
  if (!isAuthenticated.value) return;
  try {
    dashboard.value = await api.getDashboard();
  } catch (error) {
    handleApiError(error, "Unable to refresh dashboard");
  }
};

const stopDataPolling = () => {
  if (dataPollTimer.value) {
    window.clearInterval(dataPollTimer.value);
    dataPollTimer.value = null;
  }
};

const startDataPolling = () => {
  stopDataPolling();
  if (!isAuthenticated.value || backendOffline.value) return;
  const tick = async () => {
    if (activePanel.value === "login") {
      await loadDashboard();
    }
  };
  void tick();
  dataPollTimer.value = window.setInterval(tick, DATA_POLL_INTERVAL_MS);
};

onMounted(async () => {
  applyTheme();
  
  // Check for SSO token in URL
  const url = new URL(window.location.href);
  const token = url.searchParams.get("token");
  if (token) {
    persistToken(token);
    window.history.replaceState({}, document.title, "/");
    // ensureSession will be called below
  }

  await fetchPublicRuntimeSettings();
  if (needsSetup.value) {
    return;
  }
  const healthy = await checkHealth({ force: true });
  scheduleHealthPolling();
  if (healthy) {
    await ensureSession();
  }
  startDataPolling();
});

onBeforeUnmount(() => {
  stopHealthPolling();
  stopDataPolling();
});

watch(
  () => [activePanel.value, isAuthenticated.value],
  () => {
    startDataPolling();
  }
);

watch(backendOffline, (isOffline) => {
  setOfflineMode(isOffline);
  if (isOffline) {
    stopDataPolling();
    return;
  }
  if (isAuthenticated.value) {
    startDataPolling();
  }
});
</script>

<template>
  <Setup
    v-if="needsSetup"
    :settings="settingsForm"
    @setup-complete="handleSetupComplete"
  />
  <div v-else class="flex min-h-screen bg-base-200 text-base-content">
    <!-- Mobile Header -->
    <div class="lg:hidden fixed top-0 left-0 right-0 z-40 flex items-center justify-between border-b border-base-300 bg-base-100/80 px-4 py-3 backdrop-blur-md">
              <div class="flex items-center gap-3">
                <button class="btn btn-square btn-ghost btn-sm" @click="isSidebarOpen = !isSidebarOpen">
                  <Menu class="h-5 w-5" />
                </button>
                <div
                  class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-content shadow-lg shadow-primary/30"
                >
                  <Activity class="h-6 w-6" />
                </div>
                <span class="font-bold text-lg">Updockly</span>
              </div>    </div>

    <!-- Backdrop -->
    <div 
      v-if="isSidebarOpen" 
      class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm lg:hidden"
      @click="isSidebarOpen = false"
    ></div>

    <AppSidebar
      v-if="!backendOffline"
      class="fixed inset-y-0 left-0 z-50 transform transition-transform duration-300 lg:sticky lg:top-0 lg:flex lg:translate-x-0"
      :class="isSidebarOpen ? 'translate-x-0' : '-translate-x-full'"
      :nav-items="navItems"
      :active="activePanel"
      :theme="theme"
      :backend-version="backendVersion"
      :health-status="healthStatus"
      :user-name="currentUser?.name"
      :is-authenticated="isAuthenticated"
      :hide-support-button="settingsForm.hideSupportButton"
      @update:panel="(panel) => (activePanel = panel)"
      @toggle-theme="toggleTheme"
      @logout="logout"
      @close="isSidebarOpen = false"
    />

    <main class="flex-1 overflow-y-auto relative pt-16 lg:pt-0">
      <div v-if="backendOffline" class="mx-auto max-w-3xl px-4 py-16">
        <BackendOffline
          :checking="loading.health"
          :error-message="backendErrorMessage"
          @retry="checkHealth({ force: true })"
        />
      </div>
      <div v-else class="mx-auto max-w-6xl px-4 py-10 space-y-8 relative">
        <transition name="fade-slide" mode="out-in">
          <section
            v-if="activePanel === 'login'"
            :key="'login'"
            class="space-y-8"
          >
            <div
              v-if="!isAuthenticated"
              class="flex min-h-[60vh] items-center justify-center"
            >
              <LoginPanel
                :loading="loading.login"
                :authenticated="isAuthenticated"
                :user-name="currentUser?.name"
                :on-submit="handleLogin"
                :on-logout="logout"
                :two-factor-required="twoFactorRequired"
                :on-verify-2fa="handle2FAVerify"
                :sso-enabled="ssoEnabled"
              />
            </div>
            <DashboardPanel
              v-if="isAuthenticated"
              :dashboard="dashboard"
              :loading-bootstrap="loading.bootstrap"
              @refresh-dashboard="loadDashboard"
            />
          </section>

          <section
            v-else-if="activePanel === 'containers' && isAuthenticated"
            :key="'containers'"
            class="space-y-6"
          >
            <ContainersPanel />
          </section>

          <section
            v-else-if="activePanel === 'history' && isAuthenticated"
            :key="'history'"
            class="space-y-6"
          >
            <HistoryPanel />
          </section>

          <section
            v-else-if="activePanel === 'schedule' && isAuthenticated"
            :key="'schedule'"
            class="space-y-6"
          >
            <SchedulePanel />
          </section>

          <section
            v-else-if="activePanel === 'agents' && isAuthenticated"
            :key="'agents'"
            class="space-y-6"
          >
            <AgentsPanel />
          </section>

          <section v-else :key="'settings'" class="space-y-6">
            <SettingsPanel
              :form="settingsForm"
              :loading="loading.settings"
              :testing-notification="loading.notificationsTest"
              :is-authenticated="isAuthenticated"
              :dirty="dirtyRuntime"
              :current-user="currentUser"
        @save="saveSettings"
        @reset="loadSettings"
        @test-notification="testNotification"
        @test-email="testEmailNotification"
        @refresh-user="fetchUserProfile"
        @update-user="updateUserProfile"
        :updating-user="loading.userUpdate"
      />
          </section>
        </transition>
      </div>
    </main>
  </div>
  <ConfirmModal />
</template>

<style scoped>
.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.35s ease;
}
.fade-slide-enter-from,
.fade-slide-leave-to {
  opacity: 0;
  transform: translate3d(0, 20px, 0);
}
</style>
