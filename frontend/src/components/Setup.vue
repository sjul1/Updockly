<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ApiError, api } from "../services/api";
import { Lock, Shield, Key, Download, Moon, Sun, Copy } from "lucide-vue-next";
import { useToast } from "vue-toastification";

const emit = defineEmits<{
  (e: "setup-complete"): void;
  (e: "toggle-theme"): void;
}>();

const props = withDefaults(
  defineProps<{
    settings: {
      databaseUrl?: string;
      jwtSecret?: string;
      vaultKey?: string;
      recoveryCodes?: string[];
    };
    theme?: "light" | "dark";
  }>(),
  {
    theme: "light",
  }
);

const toast = useToast();

const step = ref<"config" | "admin" | "recovery" | "secrets">("config");

const configForm = reactive({
  databaseUrl: props.settings.databaseUrl || "",
});

watch(
  () => props.settings.databaseUrl,
  (next) => {
    if (next && !configForm.databaseUrl) {
      configForm.databaseUrl = next;
    }
  }
);

const adminForm = reactive({
  username: "",
  email: "",
  password: "",
  name: "",
  totpSecret: "",
  totpCode: "",
});

const fieldTouched = reactive({
  username: false,
  email: false,
  password: false,
});

const isUsernameValid = computed(() => {
  const value = adminForm.username.trim();
  return /^[A-Za-z][A-Za-z0-9-]{2,29}$/.test(value);
});
const isEmailValid = computed(() => {
  const value = adminForm.email.trim();
  return value === "" ? false : /\S+@\S+\.\S+/.test(value);
});
const isPasswordValid = computed(() => {
  const value = adminForm.password.trim();
  return /^(?=.*[0-9])(?=.*[a-z])(?=.*[A-Z]).{8,}$/.test(value);
});

const tfa = reactive({
  qrCode: "",
  secret: "",
  generated: false,
  loading: false,
  error: "",
});

const formError = ref("");
const loading = ref(false);
const configError = ref("");
const secretsCopied = ref(false);
const showSecretsWarning = ref(false);

const recoveryCodes = ref<string[]>(props.settings.recoveryCodes || []);
const generatedSecrets = ref<{ jwtSecret?: string; vaultKey?: string } | null>(
  props.settings.jwtSecret || props.settings.vaultKey
    ? { jwtSecret: props.settings.jwtSecret, vaultKey: props.settings.vaultKey }
    : null
);
const preloadedSecrets = ref(
  Boolean(props.settings.jwtSecret || props.settings.vaultKey)
);

watch(
  () => props.settings,
  (next) => {
    if (next.databaseUrl && !configForm.databaseUrl) {
      configForm.databaseUrl = next.databaseUrl;
    }
    if (Array.isArray(next.recoveryCodes) && next.recoveryCodes.length > 0) {
      recoveryCodes.value = next.recoveryCodes;
    }
    if (next.jwtSecret || next.vaultKey) {
      generatedSecrets.value = {
        jwtSecret: next.jwtSecret,
        vaultKey: next.vaultKey,
      };
      preloadedSecrets.value = true;
      secretsCopied.value = false;
      showSecretsWarning.value = false;
    }
  },
  { deep: true, immediate: true }
);

watch(
  () => generatedSecrets.value,
  () => {
    secretsCopied.value = false;
    showSecretsWarning.value = false;
  }
);

const autoStepResolved = ref(false);

watch(
  () => recoveryCodes.value.length,
  (recoveryCount) => {
    if (autoStepResolved.value) return;
    if (recoveryCount > 0) {
      step.value = "recovery";
      autoStepResolved.value = true;
      return;
    }
  },
  { immediate: true }
);

const downloadRecoveryCodes = () => {
  const text = recoveryCodes.value.join("\n");
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

const handleConfigContinue = async () => {
  loading.value = true;
  configError.value = "";
  try {
    await api.setupTestDb(configForm.databaseUrl);
    step.value = "admin";
  } catch (error) {
    const isForbidden = error instanceof ApiError && error.status === 403;
    const errorMessage =
      error instanceof Error ? error.message : "Configuration test failed";
    const displayMessage = isForbidden
      ? errorMessage ||
        "Backend responded, but the browser blocked it due to CORS. Verify ALLOWED_ORIGIN on the server."
      : errorMessage;
    configError.value = displayMessage;
    if (isForbidden) {
      toast.error(displayMessage);
    }
  } finally {
    loading.value = false;
  }
};

const generate2FA = async () => {
  tfa.loading = true;
  tfa.error = "";
  try {
    const response = await api.setupGenerate();
    tfa.qrCode = response.qrCode;
    tfa.secret = response.secret;
    adminForm.totpSecret = response.secret;
    tfa.generated = true;
  } catch (error) {
    tfa.error =
      error instanceof Error ? error.message : "Failed to generate 2FA";
  } finally {
    tfa.loading = false;
  }
};

const maskSecret = (value: string) => {
  if (!value) return "";
  if (value.length <= 8) return "*".repeat(value.length);
  return `${value.slice(0, 4)}****${value.slice(-4)}`;
};

const copyAllSecrets = async () => {
  if (!generatedSecrets.value) return;
  const lines = [];
  if (generatedSecrets.value.jwtSecret) {
    lines.push(`JWT_SECRET=${generatedSecrets.value.jwtSecret}`);
  }
  if (generatedSecrets.value.vaultKey) {
    lines.push(`VAULT_KEY=${generatedSecrets.value.vaultKey}`);
  }
  if (!lines.length) return;
  const payload = lines.join("\n");
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(payload);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = payload;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    secretsCopied.value = true;
    showSecretsWarning.value = false;
    toast.success("Secrets copied");
  } catch (err) {
    toast.error("Unable to copy secrets");
  }
};

const createAdmin = async () => {
  loading.value = true;
  formError.value = "";
  try {
    const response = await api.setupCreate(adminForm);
    recoveryCodes.value = response.recoveryCodes;
    step.value = "recovery";
    generatedSecrets.value =
      response.jwtSecret || response.vaultKey
        ? { jwtSecret: response.jwtSecret, vaultKey: response.vaultKey }
        : null;
    if (generatedSecrets.value) {
      preloadedSecrets.value = true;
      secretsCopied.value = false;
      showSecretsWarning.value = false;
    }
  } catch (error) {
    formError.value =
      error instanceof Error ? error.message : "Failed to create admin account";
  } finally {
    loading.value = false;
  }
};

const handleFinishSecrets = () => {
  if (!secretsCopied.value) {
    showSecretsWarning.value = true;
    return;
  }
  emit("setup-complete");
};

const forceFinishSecrets = () => {
  showSecretsWarning.value = false;
  emit("setup-complete");
};

const confirmRecoveryCodes = () => {
  if (preloadedSecrets.value && generatedSecrets.value) {
    step.value = "secrets";
    return;
  }
  emit("setup-complete");
};

const themeIcon = computed(() => (props.theme === "dark" ? Sun : Moon));
const themeLabel = computed(() =>
  props.theme === "dark" ? "Switch to light theme" : "Switch to dark theme"
);
const toggleTheme = () => emit("toggle-theme");

const stepTitles = computed(() => {
  const base = [
    { id: "config", label: "Database URL" },
    { id: "admin", label: "Admin Account" },
    { id: "recovery", label: "Recovery Codes" },
  ] as const;

  if (preloadedSecrets.value) {
    return [...base, { id: "secrets", label: "Generated Secrets" } as const];
  }

  return base;
});

const currentStepIndex = computed(() =>
  Math.max(
    0,
    stepTitles.value.findIndex(({ id }) => id === step.value)
  )
);
const stepClass = (index: number) =>
  index <= currentStepIndex.value ? "step-primary" : "";
</script>

<template>
  <div
    class="flex min-h-screen flex-col items-center bg-base-200 relative pt-8 md:pt-10"
  >
    <button
      class="btn btn-ghost btn-sm absolute right-6 top-6 rounded-full shadow-sm"
      type="button"
      :aria-label="themeLabel"
      @click="toggleTheme"
    >
      <component :is="themeIcon" class="w-4 h-4" />
    </button>
    <div class="w-full flex flex-col flex-1 items-center gap-4">
      <div class="w-full max-w-2xl hidden md:block">
        <div
          class="rounded-2xl bg-base-100/90 shadow-lg backdrop-blur px-4 py-3 md:px-6"
        >
          <ul class="steps w-full">
            <li
              v-for="(item, index) in stepTitles"
              :key="item.id"
              class="step flex-1"
              :class="stepClass(index)"
            >
              {{ item.label }}
            </li>
          </ul>
        </div>
      </div>

      <div class="w-full flex-1 flex items-center justify-center px-4 -mt-12">
        <div
          class="w-full max-w-lg rounded-3xl bg-base-100/90 p-8 shadow-xl backdrop-blur"
        >
          <!-- Step 4 (optional): Generated Secrets -->
          <div
            v-if="step === 'secrets' && generatedSecrets && preloadedSecrets"
          >
            <div class="flex items-center gap-3">
              <Shield class="text-warning" :size="28" />
              <div>
                <p class="text-xs uppercase tracking-[0.4em] text-primary">
                  Initial Setup
                </p>
                <p class="text-2xl font-bold">Persist Generated Secrets</p>
              </div>
            </div>
            <div class="mt-6 space-y-4">
              <div
                class="alert alert-info shadow-lg border border-info/40 flex flex-col items-stretch space-y-2"
              >
                <div class="space-y-1">
                  <h3 class="font-bold">Add these to your backend .env</h3>
                  <p class="text-xs">
                    JWT and Vault keys were generated because none were
                    provided. Add them to the environment loaded by the backend
                    container so they survive restarts.
                  </p>
                </div>
              </div>
              <div class="space-y-3 font-mono text-xs">
                <div v-if="generatedSecrets.jwtSecret" class="space-y-1">
                  <p
                    class="text-[0.68rem] font-semibold uppercase tracking-wide text-info"
                  >
                    JWT_SECRET
                  </p>
                  <div
                    class="rounded-md bg-base-100 px-3 py-2 break-all border border-info/40 shadow-sm text-base-content"
                  >
                    {{ maskSecret(generatedSecrets.jwtSecret) }}
                  </div>
                </div>
                <div v-if="generatedSecrets.vaultKey" class="space-y-1">
                  <p
                    class="text-[0.68rem] font-semibold uppercase tracking-wide text-info"
                  >
                    VAULT_KEY
                  </p>
                  <div
                    class="rounded-md bg-base-100 px-3 py-2 break-all border border-info/40 shadow-sm text-base-content"
                  >
                    {{ maskSecret(generatedSecrets.vaultKey) }}
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-outline btn-info btn-xs w-full justify-center gap-2"
                  @click="copyAllSecrets"
                >
                  <Copy class="w-4 h-4" />
                  Copy both keys
                </button>
              </div>
              <div
                v-if="showSecretsWarning"
                class="alert alert-warning border border-warning/40 shadow-sm"
              >
                <div class="space-y-2">
                  <p class="text-sm font-semibold">
                    Save these keys before continuing.
                  </p>
                  <p class="text-xs">
                    If the backend reloads before you store them, you will not
                    be able to log in again.
                  </p>
                  <button
                    type="button"
                    class="btn btn-error btn-xs"
                    @click="forceFinishSecrets"
                  >
                    I understand the risk, continue
                  </button>
                </div>
              </div>
              <button
                v-if="!showSecretsWarning"
                class="btn btn-primary btn-sm rounded-full w-full mt-2"
                @click="handleFinishSecrets"
              >
                Finish Setup
              </button>
            </div>
          </div>

          <!-- Step 3: Recovery Codes -->
          <div v-else-if="step === 'recovery' && recoveryCodes.length > 0">
            <div class="flex items-center gap-3">
              <Shield class="text-warning" :size="28" />
              <div>
                <p class="text-xs uppercase tracking-[0.4em] text-primary">
                  Initial Setup
                </p>
                <p class="text-2xl font-bold">Save Recovery Codes</p>
              </div>
            </div>
            <div class="mt-6 space-y-4">
              <div class="alert alert-warning shadow-lg">
                <div>
                  <h3 class="font-bold">Important!</h3>
                  <p class="text-xs">
                    Save these codes securely. They are the only way to restore
                    access if you lose your authenticator device.
                  </p>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-2 font-mono text-sm">
                <span
                  v-for="code in recoveryCodes"
                  :key="code"
                  class="bg-base-200 px-2 py-1 rounded text-center"
                  >{{ code }}</span
                >
              </div>
              <button
                class="btn btn-outline btn-sm rounded-full w-full mt-2 flex items-center gap-2"
                @click="downloadRecoveryCodes"
              >
                <Download class="w-4 h-4" />
                Download as .txt
              </button>
              <button
                class="btn btn-primary btn-sm rounded-full w-full mt-2"
                @click="confirmRecoveryCodes"
              >
                I have saved them
              </button>
            </div>
          </div>

          <!-- Step 1: Config review -->
          <div v-else-if="step === 'config'">
            <div class="flex items-center gap-3">
              <Key class="text-primary" :size="28" />
              <div>
                <p class="text-xs uppercase tracking-[0.4em] text-primary">
                  Initial Setup
                </p>
                <p class="text-2xl font-bold">Database Configuration</p>
              </div>
            </div>
            <div class="mt-6 space-y-4 text-sm">
              <p>
                Please provide the database connection URL. Updockly will use
                this database to store its own configuration.
              </p>
              <input
                v-model="configForm.databaseUrl"
                type="text"
                class="input input-bordered rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                placeholder="Database URL (postgres://user:pass@host:5432/updockly)"
              />
              <button
                class="btn btn-primary btn-sm rounded-full w-full mt-4"
                @click="handleConfigContinue"
              >
                <span v-if="loading" class="flex items-center gap-2">
                  <span class="loading loading-spinner" /> Checking…
                </span>
                <span v-else>Test & Continue</span>
              </button>
              <p v-if="configError" class="text-error text-center text-sm pt-2">
                {{ configError }}
              </p>
            </div>
          </div>

          <!-- Step 2: Admin Creation -->
          <div v-else-if="step === 'admin'">
            <div class="flex items-center gap-3">
              <Lock class="text-primary" :size="28" />
              <div>
                <p class="text-xs uppercase tracking-[0.4em] text-primary">
                  Initial Setup
                </p>
                <p class="text-2xl font-bold">Create Admin Account</p>
              </div>
            </div>

            <form @submit.prevent="createAdmin" class="mt-6 space-y-3">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3 md:gap-4">
                <label class="form-control w-full">
                  <label
                    class="input input-bordered validator rounded-xl bg-base-100/70 focus-within:ring-2 focus-within:ring-primary/40 w-full flex items-center gap-2"
                  >
                    <svg
                      class="h-[1em] opacity-50"
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 24 24"
                    >
                      <g
                        stroke-linejoin="round"
                        stroke-linecap="round"
                        stroke-width="2.5"
                        fill="none"
                        stroke="currentColor"
                      >
                        <path
                          d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"
                        ></path>
                        <circle cx="12" cy="7" r="4"></circle>
                      </g>
                    </svg>
                    <input
                      v-model="adminForm.username"
                      type="text"
                      required
                      placeholder="Username"
                      pattern="[A-Za-z][A-Za-z0-9\\-]*"
                      minlength="3"
                      maxlength="30"
                      title="Only letters, numbers or dash"
                      class="grow text-sm"
                      @blur="fieldTouched.username = true"
                      @input="fieldTouched.username = true"
                    />
                  </label>
                  <p
                    v-show="fieldTouched.username && !isUsernameValid"
                    class="text-xs text-error mt-1"
                  >
                    3-30 chars. Letters, numbers, dash.
                  </p>
                </label>
                <label class="form-control w-full">
                  <label
                    class="input input-bordered validator rounded-xl bg-base-100/70 focus-within:ring-2 focus-within:ring-primary/40 w-full flex items-center gap-2"
                  >
                    <input
                      v-model="adminForm.name"
                      type="text"
                      class="grow text-sm"
                      placeholder="Full name"
                    />
                    <span class="badge badge-neutral badge-xs">Optional</span>
                  </label>
                </label>
              </div>
              <label class="form-control w-full">
                <label
                  class="input input-bordered validator rounded-xl bg-base-100/70 flex items-center gap-2 focus-within:ring-2 focus-within:ring-primary/40 w-full"
                >
                  <svg
                    class="h-[1em] opacity-50"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                  >
                    <g
                      stroke-linejoin="round"
                      stroke-linecap="round"
                      stroke-width="2.5"
                      fill="none"
                      stroke="currentColor"
                    >
                      <rect width="20" height="16" x="2" y="4" rx="2"></rect>
                      <path
                        d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"
                      ></path>
                    </g>
                  </svg>
                  <input
                    v-model="adminForm.email"
                    type="email"
                    required
                    placeholder="admin@example.com"
                    class="grow text-sm"
                    @blur="fieldTouched.email = true"
                    @input="fieldTouched.email = true"
                  />
                </label>
                <p
                  v-show="fieldTouched.email && !isEmailValid"
                  class="text-xs text-error mt-1"
                >
                  Enter a valid email address.
                </p>
              </label>
              <label class="form-control w-full">
                <label
                  class="input input-bordered validator rounded-xl bg-base-100/70 flex items-center gap-2 focus-within:ring-2 focus-within:ring-primary/40 w-full mt-3"
                >
                  <svg
                    class="h-[1em] opacity-50"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                  >
                    <g
                      stroke-linejoin="round"
                      stroke-linecap="round"
                      stroke-width="2.5"
                      fill="none"
                      stroke="currentColor"
                    >
                      <path
                        d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"
                      ></path>
                      <circle
                        cx="16.5"
                        cy="7.5"
                        r=".5"
                        fill="currentColor"
                      ></circle>
                    </g>
                  </svg>
                  <input
                    v-model="adminForm.password"
                    type="password"
                    required
                    placeholder="Password"
                    minlength="8"
                    pattern="(?=.*[0-9])(?=.*[a-z])(?=.*[A-Z]).{8,}"
                    title="Must be more than 8 characters, including number, lowercase letter, uppercase letter"
                    class="grow text-sm"
                    @blur="fieldTouched.password = true"
                    @input="fieldTouched.password = true"
                  />
                </label>
                <p
                  v-show="fieldTouched.password && !isPasswordValid"
                  class="text-xs text-error mt-1"
                >
                  Must be 8+ chars with upper, lower, and a number.
                </p>
              </label>

              <div class="divider"></div>

              <div v-if="!tfa.generated" class="text-center">
                <button
                  type="button"
                  class="btn btn-info btn-sm rounded-full"
                  @click="generate2FA"
                >
                  <span v-if="loading" class="flex items-center gap-2">
                    <span class="loading loading-spinner" /> Generating…
                  </span>
                  <span v-else class="flex items-center gap-2">
                    <Shield class="w-4 h-4" />
                    Generate QR Code for 2FA
                  </span>
                </button>
                <p v-if="tfa.error" class="text-error text-center text-xs pt-2">
                  {{ tfa.error }}
                </p>
              </div>

              <div v-if="tfa.generated" class="space-y-3">
                <div class="flex justify-center">
                  <img :src="tfa.qrCode" alt="2FA QR Code" class="rounded-lg" />
                </div>
                <p class="text-center text-sm mt-2">
                  Scan the QR code, then enter the code below.
                </p>
                <p class="text-center text-xs mt-1">
                  Or enter this secret manually:
                  <code class="font-mono p-1 bg-base-200 rounded-md">{{
                    tfa.secret
                  }}</code>
                </p>
                <label class="form-control w-full">
                  <input
                    v-model="adminForm.totpCode"
                    type="text"
                    class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                    required
                    placeholder="123456"
                  />
                </label>
              </div>

              <button
                class="btn btn-primary btn-sm rounded-full w-full !mt-6"
                type="submit"
                :disabled="!tfa.generated"
              >
                <span v-if="loading" class="flex items-center gap-2">
                  <span class="loading loading-spinner" /> Creating account…
                </span>
                <span v-else>Create Account</span>
              </button>
              <p v-if="formError" class="text-error text-center text-sm pt-2">
                {{ formError }}
              </p>
            </form>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
