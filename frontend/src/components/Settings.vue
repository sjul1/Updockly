<script setup lang="ts">
import {
  SlidersHorizontal,
  ChevronDown,
  Clock,
  Shield,
  HelpCircle,
  Bell,
  Download,
  Mail,
  User,
} from "lucide-vue-next";
import {
  computed,
  onMounted,
  reactive,
  ref,
  watch,
  inject,
  type ComputedRef,
  type Ref,
} from "vue";
import type { SettingsFormState } from "../types/formTypes";
import { api, type ApiUser } from "../services/api";

type SectionKey =
  | "runtime"
  | "notifications"
  | "2fa"
  | "sso"
  | "smtp"
  | "account";

const props = defineProps<{
  form: SettingsFormState;
  loading: boolean;
  testingNotification?: boolean;
  isAuthenticated: boolean;
  dirty?: boolean;
  currentUser: ApiUser | null;
  updatingUser?: boolean;
}>();

const emit = defineEmits<{
  (e: "save"): void;
  (e: "reset"): void;
  (e: "test-notification"): void;
  (e: "test-email"): void;
  (e: "refreshUser"): void;
  (
    e: "update-user",
    payload: {
      name: string;
      email: string;
      currentPassword?: string;
      newPassword?: string;
    }
  ): void;
}>();

const appTimezone = inject<ComputedRef<string>>(
  "appTimezone",
  computed(
    () =>
      props.form.timezone ||
      Intl.DateTimeFormat().resolvedOptions().timeZone ||
      "UTC"
  )
);

const twoFactor = reactive({
  setup: false,
  qrCode: "",
  secret: "",
  code: "",
  password: "",
  disablingStep: "idle" as "idle" | "code" | "password",
  loading: false,
  error: "",
  recoveryCodes: [] as string[],
});

const confirmRegenerate = ref(false);

const start2FASetup = async () => {
  twoFactor.loading = true;
  twoFactor.error = "";
  try {
    const response = await api.generate2FA();
    twoFactor.qrCode = response.qrCode;
    twoFactor.secret = response.secret;
    twoFactor.setup = true;
  } catch (error) {
    twoFactor.error =
      error instanceof Error ? error.message : "Failed to start 2FA setup";
  } finally {
    twoFactor.loading = false;
  }
};

const enable2FA = async () => {
  if (!twoFactor.code) return;
  twoFactor.loading = true;
  twoFactor.error = "";
  try {
    const response = await api.enable2FA(twoFactor.code);
    twoFactor.recoveryCodes = response.recoveryCodes;
    emit("refreshUser");
    twoFactor.setup = false;
    twoFactor.code = "";
  } catch (error) {
    twoFactor.error =
      error instanceof Error ? error.message : "Failed to enable 2FA";
  } finally {
    twoFactor.loading = false;
  }
};

const regenerateCodes = async () => {
  confirmRegenerate.value = true;
  twoFactor.password = "";
  twoFactor.error = "";
};

const confirmRegeneration = async () => {
  if (!twoFactor.password) return;
  twoFactor.loading = true;
  twoFactor.error = "";
  try {
    const response = await api.regenerateRecoveryCodes(twoFactor.password);
    twoFactor.recoveryCodes = response.recoveryCodes;
    confirmRegenerate.value = false;
    twoFactor.password = "";
  } catch (error) {
    twoFactor.error =
      error instanceof Error ? error.message : "Failed to regenerate codes";
  } finally {
    twoFactor.loading = false;
  }
};

const downloadRecoveryCodes = () => {
  const text = twoFactor.recoveryCodes.join("\n");
  const blob = new Blob([text], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "updockly-recovery-codes.txt";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
};

const disable2FA = async () => {
  if (!twoFactor.code || !twoFactor.password) return;
  twoFactor.loading = true;
  twoFactor.error = "";
  try {
    await api.disable2FA(twoFactor.code, twoFactor.password);
    emit("refreshUser");
    twoFactor.code = "";
    twoFactor.password = "";
    twoFactor.disablingStep = "idle";
  } catch (error) {
    twoFactor.error =
      error instanceof Error ? error.message : "Failed to disable 2FA";
  } finally {
    twoFactor.loading = false;
  }
};

const sectionVisibility = reactive<Record<SectionKey, boolean>>({
  runtime: false,
  notifications: false,
  "2fa": false,
  sso: false,
  smtp: false,
  account: false,
});

const appTheme = inject<Ref<string>>("appTheme", ref("light"));
const setAppTheme = inject<(theme: string) => void>(
  "setAppTheme",
  (value: string) => {
    localStorage.setItem("updockly_theme", value);
    document.documentElement.setAttribute("data-theme", value);
  }
);
const autoPruneId = "auto-prune-images-toggle";
const hideSupportId = "hide-support-toggle";
const availableThemes = [
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
  { value: "cupcake", label: "Cupcake" },
  { value: "bumblebee", label: "Bumblebee" },
  { value: "emerald", label: "Emerald" },
  { value: "corporate", label: "Corporate" },
  { value: "synthwave", label: "Synthwave" },
  { value: "retro", label: "Retro" },
  { value: "cyberpunk", label: "Cyberpunk" },
  { value: "valentine", label: "Valentine" },
  { value: "halloween", label: "Halloween" },
  { value: "garden", label: "Garden" },
  { value: "forest", label: "Forest" },
  { value: "aqua", label: "Aqua" },
  { value: "lofi", label: "Lofi" },
  { value: "pastel", label: "Pastel" },
  { value: "business", label: "Business" },
  { value: "dracula", label: "Dracula" },
];
const selectedTheme = ref<string>(appTheme?.value || "light");
watch(
  () => appTheme?.value,
  (value) => {
    if (value) {
      selectedTheme.value = value;
    }
  },
  { immediate: true }
);
watch(selectedTheme, (value) => {
  setAppTheme?.(value);
});
const selectTheme = (value: string) => {
  selectedTheme.value = value;
  setAppTheme?.(value);
};

const toggleSection = (key: SectionKey) => {
  sectionVisibility[key] = !sectionVisibility[key];
};

const baselineState = ref(JSON.stringify(props.form));
const dirty = ref(false);
const updateDirty = () => {
  dirty.value = JSON.stringify(props.form) !== baselineState.value;
};
watch(
  () => props.form,
  () => {
    updateDirty();
  },
  { deep: true }
);
const syncBaseline = () => {
  baselineState.value = JSON.stringify(props.form);
  dirty.value = false;
};
const loadingWasActive = ref(props.loading);
watch(
  () => props.loading,
  (next) => {
    if (!next && loadingWasActive.value) {
      syncBaseline();
    }
    loadingWasActive.value = next;
  }
);
onMounted(() => {
  syncBaseline();
  loadingWasActive.value = props.loading;
});

const isDirty = computed(() =>
  !props.isAuthenticated ? props.dirty : dirty.value
);

const handleSave = () => emit("save");
const handleReset = () => emit("reset");

const fallbackTimezones = [
  "UTC",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "Europe/London",
  "Europe/Paris",
  "Europe/Berlin",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Australia/Sydney",
];

const getSupportedTimezones = (): string[] => {
  const intlRef = Intl as unknown as {
    supportedValuesOf?: (input: string) => string[];
  };
  if (typeof intlRef.supportedValuesOf === "function") {
    try {
      return intlRef.supportedValuesOf("timeZone");
    } catch {
      return fallbackTimezones;
    }
  }
  return fallbackTimezones;
};

const baseTimezoneOptions = getSupportedTimezones();
const timezoneOptions = computed(() => {
  const set = new Set(baseTimezoneOptions);
  if (props.form.timezone) {
    set.add(props.form.timezone);
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b));
});

const recapTimePattern = /^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$/;
const isRecapTimeValid = computed(() => {
  const value = props.form.notifications.recapTime?.trim();
  if (!value) return true;
  return recapTimePattern.test(value);
});

const recapTimeHelper = computed(() =>
  isRecapTimeValid.value
    ? `Daily recap uses timezone: ${appTimezone?.value || "UTC"}`
    : "Use HH:mm in 24h format (e.g. 17:00)"
);

const isCronValid = computed(() => {
  const cron = props.form.notifications.notificationCron?.trim();
  if (!cron) return true;
  const segments = cron.split(/\s+/);
  return segments.length >= 5 && segments.length <= 6;
});

const cronHelper = computed(() =>
  isCronValid.value
    ? "Alternative schedule (e.g. 0 7 * * *)"
    : "Cron needs 5-6 parts: min hour day month weekday"
);

const canTestDiscord = computed(() => {
  const notifications = props.form.notifications;
  return (
    Boolean(notifications.discordToken && notifications.discordChannel) &&
    !props.testingNotification
  );
});

const isSmtpConfigured = computed(() => {
  const smtp = props.form.notifications.smtp;
  return Boolean(smtp.host && smtp.port && smtp.from);
});

const canTestEmail = computed(() => {
  const smtp = props.form.notifications.smtp;
  return Boolean(smtp.host && smtp.from);
});

const userForm = reactive({
  name: props.currentUser?.name || "",
  email: props.currentUser?.email || "",
  currentPassword: "",
  newPassword: "",
  confirmPassword: "",
});

const userError = ref("");

const syncUserForm = () => {
  userForm.name = props.currentUser?.name || "";
  userForm.email = props.currentUser?.email || "";
  userForm.currentPassword = "";
  userForm.newPassword = "";
  userForm.confirmPassword = "";
  userError.value = "";
};

watch(
  () => props.currentUser,
  () => syncUserForm(),
  { deep: true }
);

const userHasChanges = computed(() => {
  return (
    userForm.name !== (props.currentUser?.name || "") ||
    userForm.email !== (props.currentUser?.email || "") ||
    Boolean(userForm.newPassword)
  );
});

const submitUserForm = () => {
  userError.value = "";
  if (!userHasChanges.value) return;
  if (
    userForm.newPassword &&
    userForm.newPassword !== userForm.confirmPassword
  ) {
    userError.value = "Passwords do not match.";
    return;
  }
  emit("update-user", {
    name: userForm.name,
    email: userForm.email,
    currentPassword: userForm.currentPassword || undefined,
    newPassword: userForm.newPassword || undefined,
  });
};
</script>

<template>
  <div class="space-y-8">
    <!-- RUNTIME & SECRETS -->
    <section
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <!-- Header -->
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('runtime')"
        >
            <div class="flex items-start gap-3">
              <div
                class="flex h-10 w-10 items-center justify-center rounded-2xl bg-primary/10 text-primary"
              >
                <SlidersHorizontal class="w-5 h-5" />
              </div>
              <div>
                <h3 class="card-title text-lg">Runtime</h3>
                <p class="text-xs text-base-content/70 mt-1">
                  Configure runtime behavior for scheduling, notifications, and
                  UI preferences.
                </p>
              </div>
            </div>
            <button
              type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility.runtime }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility.runtime ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility.runtime" class="space-y-5">
          <!-- Form -->
          <form class="grid gap-4 md:grid-cols-2">
            <!-- TIMEZONE -->
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Timezone
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="The timezone used for displaying dates and times throughout the application."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
              </div>
              <select
                v-model="props.form.timezone"
                class="select select-bordered select-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              >
                <option
                  v-for="zone in timezoneOptions"
                  :key="zone"
                  :value="zone"
                >
                  {{ zone }}
                </option>
              </select>
            </label>

            <!-- AUTO PRUNE IMAGES -->
            <div class="form-control w-full md:col-span-2">
              <div class="label">
                <label
                  :for="autoPruneId"
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Auto-prune images after updates
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="After auto-update cycles finish, remove dangling images to reclaim disk space."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </label>
                <input
                  type="checkbox"
                  class="toggle toggle-primary"
                  :id="autoPruneId"
                  v-model="props.form.autoPruneImages"
                />
              </div>
              <p class="text-xs text-base-content/60">
                When enabled, Updockly will call Docker image prune after
                auto-update runs complete.
              </p>
            </div>

            <!-- HIDE SUPPORT BUTTON -->
            <div class="form-control w-full md:col-span-2">
              <div class="label">
                <label
                  :for="hideSupportId"
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Hide sidebar support button
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="Remove the 'Support the project' call-to-action from the sidebar."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </label>
                <input
                  type="checkbox"
                  class="toggle toggle-primary"
                  :id="hideSupportId"
                  v-model="props.form.hideSupportButton"
                />
              </div>
              <p class="text-xs text-base-content/60">
                Turn this on to mask the support banner in the sidebar.
              </p>
            </div>
          </form>
        </div>
      </div>
    </section>

    <!-- ACCOUNT -->
    <section
      v-if="props.isAuthenticated"
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('account')"
        >
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-2xl bg-base-200 text-base-content"
            >
              <User class="w-5 h-5" />
            </div>
            <div>
              <h3 class="card-title text-lg">User Settings</h3>
              <p class="text-xs text-base-content/70 mt-1">
                Update your display name, email address, and password.
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility.account }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility.account ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility.account" class="space-y-4">
          <div class="grid gap-4 md:grid-cols-2">
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >Full name</span
                >
              </div>
              <input
                v-model="userForm.name"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="Your name"
              />
            </label>
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >Email</span
                >
              </div>
              <input
                v-model="userForm.email"
                type="email"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="you@example.com"
              />
            </label>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >Current password</span
                >
              </div>
              <input
                v-model="userForm.currentPassword"
                type="password"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="Required if changing password"
              />
            </label>
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >New password</span
                >
              </div>
              <input
                v-model="userForm.newPassword"
                type="password"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="Leave blank to keep current"
              />
            </label>
            <label class="form-control w-full md:col-span-2">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >Confirm new password</span
                >
              </div>
              <input
                v-model="userForm.confirmPassword"
                type="password"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="Re-enter new password"
              />
            </label>
          </div>

          <div class="flex items-center justify-between">
            <div class="text-xs text-error" v-if="userError">
              {{ userError }}
            </div>
            <div class="flex items-center gap-2">
              <button
                class="btn btn-primary btn-sm rounded-full"
                type="button"
                @click="submitUserForm"
                :disabled="props.updatingUser || !userHasChanges"
                :class="{ loading: props.updatingUser }"
              >
                Save user settings
              </button>
              <button
                class="btn btn-ghost btn-sm rounded-full"
                type="button"
                @click="syncUserForm"
              >
                Reset
              </button>
            </div>
          </div>
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <p class="text-sm font-semibold text-base-content/80">Theme</p>
              <span class="text-xs text-base-content/60"
                >Pick your look. Stored locally.</span
              >
            </div>
            <div
              class="rounded-box grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5"
            >
              <button
                v-for="themeOption in availableThemes"
                :key="themeOption.value"
                type="button"
                class="border-base-content/20 hover:border-base-content/40 overflow-hidden rounded-lg border outline-2 outline-offset-2 outline-transparent transition"
                :class="{
                  'outline-base-content': selectedTheme === themeOption.value,
                }"
                :data-set-theme="themeOption.value"
                @click.stop="selectTheme(themeOption.value)"
              >
                <div
                  class="bg-base-100 text-base-content w-full cursor-pointer font-sans"
                  :data-theme="themeOption.value"
                >
                  <div class="grid grid-cols-5 grid-rows-3">
                    <div
                      class="bg-base-200 col-start-1 row-span-2 row-start-1"
                    ></div>
                    <div class="bg-base-300 col-start-1 row-start-3"></div>
                    <div
                      class="bg-base-100 col-span-4 col-start-2 row-span-3 row-start-1 flex flex-col gap-1 p-2"
                    >
                      <div class="font-bold capitalize">
                        {{ themeOption.label }}
                      </div>
                      <div class="flex flex-wrap gap-1">
                        <div
                          class="bg-primary flex aspect-square w-5 items-center justify-center rounded lg:w-6"
                        >
                          <div class="text-primary-content text-sm font-bold">
                            A
                          </div>
                        </div>
                        <div
                          class="bg-secondary flex aspect-square w-5 items-center justify-center rounded lg:w-6"
                        >
                          <div class="text-secondary-content text-sm font-bold">
                            A
                          </div>
                        </div>
                        <div
                          class="bg-accent flex aspect-square w-5 items-center justify-center rounded lg:w-6"
                        >
                          <div class="text-accent-content text-sm font-bold">
                            A
                          </div>
                        </div>
                        <div
                          class="bg-neutral flex aspect-square w-5 items-center justify-center rounded lg:w-6"
                        >
                          <div class="text-neutral-content text-sm font-bold">
                            A
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section
      v-if="props.isAuthenticated"
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <!-- Header -->
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('sso')"
        >
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-2xl bg-secondary/10 text-secondary"
            >
              <Shield class="w-5 h-5" />
            </div>
            <div>
              <h3 class="card-title text-lg">Single Sign-On (SSO)</h3>
              <p class="text-xs text-base-content/70 mt-1">
                Configure OpenID Connect (OIDC) authentication with providers
                like Authentik, Keycloak, or Auth0.
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility.sso }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility.sso ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility.sso" class="space-y-5">
          <div class="form-control">
            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                class="checkbox checkbox-sm"
                v-model="props.form.sso.enabled"
              />
              <span class="label-text font-medium">Enable SSO Login</span>
            </label>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Provider Name
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="Internal identifier for the provider (e.g., 'authentik')."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
              </div>
              <input
                v-model="props.form.sso.provider"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-secondary/40 w-full"
                placeholder="authentik"
              />
            </label>

            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Issuer URL
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="The OIDC Issuer URL (e.g., https://authentik.company/application/o/updockly/)."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
              </div>
              <input
                v-model="props.form.sso.issuerUrl"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-secondary/40 w-full"
                placeholder="https://authentik.company/application/o/updockly/"
              />
            </label>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Client ID
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="The Client ID from your OIDC provider application."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
              </div>
              <input
                v-model="props.form.sso.clientId"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-secondary/40 w-full"
              />
            </label>

            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Client Secret
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="The Client Secret from your OIDC provider application."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
              </div>
              <input
                v-model="props.form.sso.clientSecret"
                type="password"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-secondary/40 w-full"
              />
            </label>
          </div>

          <label class="form-control w-full">
            <div class="label">
              <span
                class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
              >
                Redirect URL
                <div
                  class="tooltip tooltip-info normal-case"
                  data-tip="The callback URL (must match provider settings): http://your-domain/api/auth/sso/callback"
                >
                  <HelpCircle class="h-3.5 w-3.5 text-primary" />
                </div>
              </span>
            </div>
            <input
              v-model="props.form.sso.redirectUrl"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-secondary/40 w-full"
              placeholder="http://localhost:8080/api/auth/sso/callback"
            />
          </label>
        </div>
      </div>
    </section>

    <section
      v-if="props.isAuthenticated"
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('notifications')"
        >
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-2xl bg-accent/10 text-accent"
            >
              <Bell class="w-5 h-5" />
            </div>
            <div>
              <h3 class="card-title text-lg">Notifications</h3>
              <p class="text-xs text-base-content/70 mt-1">
                Send webhook and Discord alerts and recap every day after the
                chosen hour.
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility.notifications }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility.notifications ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility.notifications" class="space-y-8">
          <!-- General Section -->
          <div class="space-y-4">
            <div
              class="flex items-center gap-2 pb-2 border-b border-base-content/10"
            >
              <Clock class="w-4 h-4 text-base-content/70" />
              <h4
                class="text-sm font-bold uppercase tracking-wider text-base-content/70"
              >
                General & Schedules
              </h4>
            </div>

            <div class="grid gap-4 md:grid-cols-2">
              <label class="form-control w-full">
                <div class="label items-start">
                  <span
                    class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >
                    Recap time
                    <div
                      class="tooltip tooltip-info normal-case"
                      data-tip="Daily recap time (HH:mm, 24-hour). Sends a 24h summary regardless of instant notification toggles."
                    >
                      <HelpCircle class="h-3.5 w-3.5 text-primary" />
                    </div>
                  </span>
                  <span
                    class="label-text-alt text-[0.7rem] text-base-content/60"
                  >
                    <span :class="{ 'text-error': !isRecapTimeValid }">{{
                      recapTimeHelper
                    }}</span>
                  </span>
                </div>
                <input
                  v-model="props.form.notifications.recapTime"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  :class="{
                    'input-error focus:ring-error/40': !isRecapTimeValid,
                  }"
                  type="text"
                  inputmode="numeric"
                  pattern="[0-2][0-9]:[0-5][0-9]"
                  placeholder="17:00 (24h)"
                />
                <p
                  v-if="!isRecapTimeValid"
                  class="mt-1 text-[0.7rem] text-error"
                >
                  Recap time must be HH:mm in 24-hour time.
                </p>
              </label>

              <label class="form-control w-full">
                <div class="label items-start">
                  <span
                    class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >
                    Cron override
                    <div
                      class="tooltip tooltip-info normal-case"
                      data-tip="Advanced: override daily recap schedule with a custom cron expression."
                    >
                      <HelpCircle class="h-3.5 w-3.5 text-primary" />
                    </div>
                  </span>
                  <span
                    class="label-text-alt text-[0.7rem] text-base-content/60"
                  >
                    <span :class="{ 'text-error': !isCronValid }">{{
                      cronHelper
                    }}</span>
                  </span>
                </div>
                <input
                  v-model="props.form.notifications.notificationCron"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  :class="{ 'input-error focus:ring-error/40': !isCronValid }"
                  placeholder="0 7 * * *"
                />
                <p v-if="!isCronValid" class="mt-1 text-[0.7rem] text-error">
                  Cron expressions should have 5-6 fields (e.g. 0 7 * * *).
                </p>
              </label>
            </div>

            <div class="space-y-2">
              <p
                class="text-[0.65rem] font-semibold uppercase tracking-wide text-base-content/80"
              >
                Instant notifications triggers
              </p>
              <div class="flex flex-wrap gap-4">
                <label
                  class="flex items-center gap-2 text-xs text-base-content/70 cursor-pointer"
                >
                  <input
                    type="checkbox"
                    class="checkbox checkbox-sm checkbox-accent"
                    v-model="props.form.notifications.onSuccess"
                  />
                  <span>On Success</span>
                </label>
                <label
                  class="flex items-center gap-2 text-xs text-base-content/70 cursor-pointer"
                >
                  <input
                    type="checkbox"
                    class="checkbox checkbox-sm checkbox-accent"
                    v-model="props.form.notifications.onFailure"
                  />
                  <span>On Failure</span>
                </label>
              </div>
            </div>

            <div class="space-y-2 pt-4">
              <div
                class="flex items-center justify-between pb-1 border-b border-base-content/10"
              >
                <div
                  class="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  <Mail class="w-4 h-4 text-warning" />
                  Email notifications
                </div>
                <input
                  type="checkbox"
                  class="toggle toggle-warning toggle-sm"
                  v-model="props.form.notifications.smtp.enabled"
                  :disabled="!isSmtpConfigured"
                />
              </div>
              <p class="text-[0.7rem] text-base-content/70">
                Uses SMTP settings below.
                <span v-if="!isSmtpConfigured" class="text-warning"
                  >Add host, port, and from to enable.</span
                >
              </p>
            </div>
          </div>

          <!-- Webhook Section -->
          <div class="space-y-4">
            <div
              class="flex items-center gap-2 pb-2 border-b border-base-content/10"
            >
              <span class="font-bold text-accent">#</span>
              <h4
                class="text-sm font-bold uppercase tracking-wider text-base-content/70"
              >
                Webhook
              </h4>
            </div>
            <label class="form-control w-full">
              <div class="label">
                <span
                  class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                >
                  Webhook URL
                  <div
                    class="tooltip tooltip-info normal-case"
                    data-tip="URL to send JSON payloads to for notifications (e.g., Slack, custom services)."
                  >
                    <HelpCircle class="h-3.5 w-3.5 text-primary" />
                  </div>
                </span>
                <span class="label-text-alt text-[0.7rem] text-base-content/60"
                  >POST JSON payloads</span
                >
              </div>
              <input
                v-model="props.form.notifications.webhookUrl"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                placeholder="https://hooks.example.com/backup"
              />
            </label>
          </div>

          <!-- Discord Section -->
          <div class="space-y-4">
            <div
              class="flex items-center gap-2 pb-2 border-b border-base-content/10"
            >
              <span class="font-bold text-[#5865F2]">D</span>
              <h4
                class="text-sm font-bold uppercase tracking-wider text-base-content/70"
              >
                Discord
              </h4>
            </div>
            <div class="grid gap-4 md:grid-cols-2">
              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >
                    Discord bot token
                    <div
                      class="tooltip tooltip-info normal-case"
                      data-tip="Your Discord bot's authentication token (starts with 'Bot'). Keep this secret."
                    >
                      <HelpCircle class="h-3.5 w-3.5 text-primary" />
                    </div>
                  </span>
                </div>
                <input
                  v-model="props.form.notifications.discordToken"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  type="password"
                  placeholder="Bot xoxb-..."
                />
              </label>

              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="flex items-center gap-2 label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >
                    Discord channel
                    <div
                      class="tooltip tooltip-info normal-case"
                      data-tip="The ID of the Discord channel where notifications will be sent."
                    >
                      <HelpCircle class="h-3.5 w-3.5 text-primary" />
                    </div>
                  </span>
                </div>
                <input
                  v-model="props.form.notifications.discordChannel"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  placeholder="C1234567890"
                />
              </label>
            </div>

            <div class="flex justify-end">
              <button
                class="btn btn-outline btn-sm rounded-full"
                :disabled="!canTestDiscord"
                type="button"
                @click="emit('test-notification')"
                :class="{ loading: props.testingNotification }"
              >
                Test Discord
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- EMAIL (SMTP) -->
    <section
      v-if="props.isAuthenticated"
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('smtp')"
        >
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-2xl bg-warning/10 text-warning"
            >
              <Mail class="w-5 h-5" />
            </div>
            <div>
              <h3 class="card-title text-lg">Email (SMTP)</h3>
              <p class="text-xs text-base-content/70 mt-1">
                Configure an SMTP server to send email notifications and
                password recovery links.
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility.smtp }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility.smtp ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility.smtp" class="space-y-5">
          <div
            class="space-y-4 animate-in fade-in slide-in-from-top-2 duration-300"
          >
            <div class="grid gap-4 md:grid-cols-2">
              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                    >SMTP Host</span
                  >
                </div>
                <input
                  v-model="props.form.notifications.smtp.host"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  placeholder="smtp.example.com"
                />
              </label>
              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                    >SMTP Port</span
                  >
                </div>
                <input
                  v-model.number="props.form.notifications.smtp.port"
                  type="number"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  placeholder="587"
                />
              </label>
            </div>

            <label class="form-control w-full">
              <div class="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  class="checkbox checkbox-sm checkbox-accent"
                  v-model="props.form.notifications.smtp.tls"
                />
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >Use SSL/TLS</span
                >
              </div>
              <p class="text-[0.7rem] text-base-content/60 mt-1">
                Turn on if your SMTP server requires SSL/TLS (implicit TLS or
                STARTTLS).
              </p>
            </label>

            <div class="grid gap-4 md:grid-cols-2">
              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                    >SMTP User</span
                  >
                </div>
                <input
                  v-model="props.form.notifications.smtp.user"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  placeholder="user@example.com"
                />
              </label>
              <label class="form-control w-full">
                <div class="label">
                  <span
                    class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                    >SMTP Password</span
                  >
                </div>
                <input
                  v-model="props.form.notifications.smtp.password"
                  type="password"
                  class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                  placeholder="••••••"
                />
              </label>
            </div>

            <label class="form-control w-full">
              <div class="label">
                <span
                  class="label-text text-xs font-semibold uppercase tracking-wide text-base-content/80"
                  >From Address</span
                >
              </div>
              <input
                v-model="props.form.notifications.smtp.from"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-accent/40 w-full"
                placeholder="no-reply@example.com"
              />
            </label>

            <div class="flex justify-end pt-4">
              <button
                class="btn btn-outline btn-sm rounded-full gap-2"
                type="button"
                @click="emit('test-email')"
                :class="{ loading: props.testingNotification }"
                :disabled="!canTestEmail"
              >
                <Mail class="w-4 h-4" />
                Test Email
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 2FA -->
    <section
      v-if="props.isAuthenticated"
      class="card bg-base-100/80 backdrop-blur border border-base-200/70 shadow-lg"
    >
      <div class="card-body space-y-5">
        <div
          class="flex items-start justify-between gap-3 cursor-pointer"
          @click="toggleSection('2fa')"
        >
          <div class="flex items-start gap-3">
            <div
              class="flex h-10 w-10 items-center justify-center rounded-2xl bg-info/10 text-info"
            >
              <Shield class="w-5 h-5" />
            </div>
            <div>
              <h3 class="card-title text-lg">Two-Factor Authentication</h3>
              <p class="text-xs text-base-content/70 mt-1">
                Add an extra layer of security to your account.
              </p>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-xs rounded-full gap-1"
            tabindex="-1"
          >
            <ChevronDown
              class="w-3 h-3 transition-transform"
              :class="{ 'rotate-180': sectionVisibility['2fa'] }"
            />
            <span class="uppercase text-[0.65rem] tracking-wide">
              {{ sectionVisibility["2fa"] ? "Collapse" : "Expand" }}
            </span>
          </button>
        </div>

        <div v-show="sectionVisibility['2fa']" class="space-y-5">
          <div v-if="!props.currentUser?.twoFactorEnabled">
            <div v-if="!twoFactor.setup">
              <button
                class="btn btn-info btn-sm rounded-full"
                :class="{ loading: twoFactor.loading }"
                @click="start2FASetup"
              >
                Enable 2FA
              </button>
            </div>
            <div v-else class="space-y-4">
              <p>
                Scan the QR code with your authenticator app and enter the code
                to enable 2FA.
              </p>
              <div class="flex justify-center">
                <img :src="twoFactor.qrCode" alt="2FA QR Code" />
              </div>
              <p class="text-center">
                Or enter this secret manually:
                <code class="font-mono">{{ twoFactor.secret }}</code>
              </p>
              <div class="form-control w-full max-w-xs mx-auto">
                <label class="label">
                  <span class="label-text">Verification Code</span>
                </label>
                <input
                  v-model="twoFactor.code"
                  type="text"
                  class="input input-bordered input-sm rounded-xl"
                  placeholder="123456"
                />
              </div>
              <div class="flex justify-center">
                <button
                  class="btn btn-primary btn-sm rounded-full"
                  :class="{ loading: twoFactor.loading }"
                  @click="enable2FA"
                >
                  Verify & Enable
                </button>
              </div>
            </div>
          </div>
          <div v-else class="space-y-4">
            <p>Two-factor authentication is enabled.</p>

            <div
              v-if="twoFactor.recoveryCodes.length > 0"
              class="space-y-4 max-w-md mx-auto"
            >
              <div class="alert alert-warning shadow-lg">
                <div>
                  <h3 class="font-bold">New Recovery Codes</h3>
                  <p class="text-xs">
                    Save these codes securely. They are the only way to restore
                    access if you lose your device.
                  </p>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2 mt-2 font-mono text-sm">
                <span
                  v-for="code in twoFactor.recoveryCodes"
                  :key="code"
                  class="bg-base-200 px-2 py-1 rounded text-center"
                  >{{ code }}</span
                >
              </div>
              <button
                class="btn btn-sm btn-outline mt-2 w-full flex items-center gap-2"
                @click="downloadRecoveryCodes"
              >
                <Download class="w-4 h-4" />
                Download as .txt
              </button>
              <button
                class="btn btn-sm btn-ghost mt-2 w-full"
                @click="twoFactor.recoveryCodes = []"
              >
                I have saved them
              </button>
            </div>

            <div
              v-if="
                twoFactor.disablingStep === 'idle' &&
                !confirmRegenerate &&
                twoFactor.recoveryCodes.length === 0
              "
              class="flex flex-wrap gap-3"
            >
              <button
                class="btn btn-outline btn-sm rounded-full"
                :class="{ loading: twoFactor.loading }"
                @click="regenerateCodes"
              >
                Regenerate Recovery Codes
              </button>
              <button
                class="btn btn-error btn-sm rounded-full"
                @click="twoFactor.disablingStep = 'code'"
              >
                Disable 2FA
              </button>
            </div>

            <div v-if="confirmRegenerate" class="form-control w-full max-w-xs">
              <label class="label">
                <span class="label-text"
                  >Enter password to regenerate codes</span
                >
              </label>
              <input
                v-model="twoFactor.password"
                type="password"
                class="input input-bordered input-sm rounded-xl"
                placeholder="••••••"
              />
              <div class="mt-2 flex gap-2">
                <button
                  class="btn btn-warning btn-sm rounded-full"
                  :class="{ loading: twoFactor.loading }"
                  @click="confirmRegeneration"
                >
                  Confirm Regeneration
                </button>
                <button
                  class="btn btn-ghost btn-sm rounded-full"
                  @click="confirmRegenerate = false"
                >
                  Cancel
                </button>
              </div>
              <p class="text-xs text-warning mt-2">
                Warning: Old recovery codes will be invalidated immediately.
              </p>
            </div>

            <div
              v-else-if="twoFactor.disablingStep === 'code'"
              class="form-control w-full max-w-xs"
            >
              <label class="label">
                <span class="label-text">Enter a code to disable 2FA</span>
              </label>
              <input
                v-model="twoFactor.code"
                type="text"
                class="input input-bordered input-sm rounded-xl"
                placeholder="123456"
              />
              <div class="mt-2">
                <button
                  class="btn btn-primary btn-sm rounded-full"
                  @click="twoFactor.disablingStep = 'password'"
                >
                  Continue
                </button>
              </div>
            </div>
            <div
              v-else-if="twoFactor.disablingStep === 'password'"
              class="form-control w-full max-w-xs"
            >
              <label class="label">
                <span class="label-text">Enter your password to confirm</span>
              </label>
              <input
                v-model="twoFactor.password"
                type="password"
                class="input input-bordered input-sm rounded-xl"
                placeholder="••••••"
              />
              <div class="mt-2">
                <button
                  class="btn btn-error btn-sm rounded-full"
                  :class="{ loading: twoFactor.loading }"
                  @click="disable2FA"
                >
                  Confirm & Disable
                </button>
              </div>
            </div>
          </div>
          <div v-if="twoFactor.error" class="text-error text-sm text-center">
            {{ twoFactor.error }}
          </div>
        </div>
      </div>
    </section>

    <transition name="fade">
      <div
        v-if="isDirty"
        class="fixed bottom-4 right-4 z-50 flex flex-wrap items-center gap-3 rounded-2xl border border-base-200/70 bg-base-100/90 px-4 py-3 shadow-2xl backdrop-blur transition"
      >
        <button
          class="btn btn-primary btn-sm rounded-full"
          :class="{ loading: props.loading }"
          type="button"
          @click="handleSave"
        >
          Save settings
        </button>
        <button
          class="btn btn-ghost btn-sm rounded-full"
          type="button"
          @click="handleReset"
        >
          Reset
        </button>
      </div>
    </transition>
  </div>
</template>
