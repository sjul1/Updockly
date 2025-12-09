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
    };
    theme?: "light" | "dark";
  }>(),
  {
    theme: "light",
  }
);

const toast = useToast();

const step = ref<"config" | "admin">("config");

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
  username: "admin",
  email: "",
  password: "",
  name: "Platform Admin",
  totpSecret: "",
  totpCode: "",
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

const recoveryCodes = ref<string[]>([]);
const generatedSecrets = ref<{ jwtSecret?: string; vaultKey?: string } | null>(null);
const maskSecret = (value: string) => {
  if (!value) return "";
  if (value.length <= 8) return "*".repeat(value.length);
  return `${value.slice(0, 4)}****${value.slice(-4)}`;
};

const copySecret = async (key: "jwtSecret" | "vaultKey") => {
  if (!generatedSecrets.value) return;
  const value = generatedSecrets.value[key];
  if (!value) return;
  const line =
    key === "vaultKey" ? `VAULT_KEY=${value}` : `JWT_SECRET=${value}`;

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(line);
    } else {
      const textarea = document.createElement("textarea");
      textarea.value = line;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
    }
    toast.success(`${key === "vaultKey" ? "VAULT_KEY" : "JWT_SECRET"} copied`);
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
    generatedSecrets.value =
      response.jwtSecret || response.vaultKey
        ? { jwtSecret: response.jwtSecret, vaultKey: response.vaultKey }
        : null;
  } catch (error) {
    formError.value =
      error instanceof Error ? error.message : "Failed to create admin account";
  } finally {
    loading.value = false;
  }
};

const confirmRecoveryCodes = () => {
  emit("setup-complete");
};

const themeIcon = computed(() => (props.theme === "dark" ? Sun : Moon));
const themeLabel = computed(() =>
  props.theme === "dark" ? "Switch to light theme" : "Switch to dark theme"
);
const toggleTheme = () => emit("toggle-theme");
</script>

<template>
  <div
    class="flex min-h-screen items-center justify-center bg-base-200 relative"
  >
    <button
      class="btn btn-ghost btn-sm absolute right-6 top-6 rounded-full shadow-sm"
      type="button"
      :aria-label="themeLabel"
      @click="toggleTheme"
    >
      <component :is="themeIcon" class="w-4 h-4" />
    </button>
    <div
      class="w-full max-w-lg rounded-3xl bg-base-100/90 p-8 shadow-xl backdrop-blur"
    >
      <!-- Step 3: Recovery Codes -->
      <div v-if="recoveryCodes.length > 0">
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
          <div
            v-if="generatedSecrets"
            class="alert alert-info shadow-lg border border-info/40"
          >
            <div class="space-y-3">
              <div class="space-y-1">
                <div>
                  <h3 class="font-bold">Persist your generated secrets</h3>
                  <p class="text-xs">
                    JWT and Vault keys were generated because none were provided.
                    Add the following values to the <code>.env</code> file loaded
                    by the backend container so they survive restarts.
                  </p>
                </div>
              </div>
              <div class="space-y-1 font-mono text-xs">
                <div
                  v-if="generatedSecrets.jwtSecret"
                  class="flex items-center gap-2"
                >
                  <span class="font-semibold">JWT_SECRET</span>:
                  <span class="flex-1">{{ maskSecret(generatedSecrets.jwtSecret) }}</span>
                  <button
                    type="button"
                    class="btn btn-ghost btn-xs"
                    @click="copySecret('jwtSecret')"
                    :aria-label="'Copy JWT_SECRET'"
                  >
                    <Copy class="w-4 h-4" />
                  </button>
                </div>
                <div
                  v-if="generatedSecrets.vaultKey"
                  class="flex items-center gap-2"
                >
                  <span class="font-semibold">VAULT_KEY</span>:
                  <span class="flex-1">{{ maskSecret(generatedSecrets.vaultKey) }}</span>
                  <button
                    type="button"
                    class="btn btn-ghost btn-xs"
                    @click="copySecret('vaultKey')"
                    :aria-label="'Copy VAULT_KEY'"
                  >
                    <Copy class="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          </div>
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
            Please provide the database connection URL. Updockly will use this
            database to store its own configuration.
          </p>
          <div class="form-control w-full">
            <label class="label">
              <span class="label-text">Database URL</span>
            </label>
            <input
              v-model="configForm.databaseUrl"
              type="text"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              placeholder="postgres://user:pass@host:5432/updockly"
            />
          </div>
          <button
            class="btn btn-primary btn-sm rounded-full w-full mt-4"
            @click="handleConfigContinue"
            :class="{ loading: loading }"
          >
            Test & Continue
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
          <div class="grid grid-cols-2 gap-4">
            <label class="form-control w-full">
              <div class="label">
                <span class="label-text">Username</span>
              </div>
              <input
                v-model="adminForm.username"
                type="text"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                required
              />
            </label>
            <label class="form-control w-full">
              <div class="label">
                <span class="label-text">Full Name</span>
              </div>
              <input
                v-model="adminForm.name"
                type="text"
                class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
                required
              />
            </label>
          </div>
          <label class="form-control w-full">
            <div class="label">
              <span class="label-text">Email</span>
            </div>
            <input
              v-model="adminForm.email"
              type="email"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              required
              placeholder="admin@example.com"
            />
          </label>
          <label class="form-control w-full">
            <div class="label">
              <span class="label-text">Password</span>
            </div>
            <input
              v-model="adminForm.password"
              type="password"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              required
            />
          </label>

          <div class="divider"></div>

          <div v-if="!tfa.generated" class="text-center">
            <button
              type="button"
              class="btn btn-info btn-sm rounded-full"
              @click="generate2FA"
              :class="{ loading: tfa.loading }"
            >
              <Shield class="w-4 h-4" />
              Generate QR Code for 2FA
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
              <div class="label">
                <span class="label-text">2FA Code</span>
              </div>
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
            :class="{ loading: loading }"
            :disabled="!tfa.generated"
          >
            Create Account
          </button>
          <p v-if="formError" class="text-error text-center text-sm pt-2">
            {{ formError }}
          </p>
        </form>
      </div>
    </div>
  </div>
</template>
