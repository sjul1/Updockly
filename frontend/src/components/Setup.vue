<script setup lang="ts">
import { reactive, ref } from "vue";
import { api } from "../services/api";
import { Lock, Shield, Key, Download } from "lucide-vue-next";

const emit = defineEmits<{
  (e: "setup-complete"): void;
}>();

const props = defineProps<{
  settings: {
    databaseUrl?: string;
    secretKey?: string;
  };
}>();

const step = ref<"config" | "admin">("config");

const configForm = reactive({
  databaseUrl: props.settings.databaseUrl || "",
});

const adminForm = reactive({
  username: "admin",
  email: "",
  password: "",
  name: "Platform Admin",
  secretKey: "",
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
    await api.updatePublicRuntimeSettings({
      databaseUrl: configForm.databaseUrl,
      clientOrigin: window.location.origin,
      secretKey: "", // Not updated here anymore
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    });

    // Re-check runtime settings to see if setup is now complete (e.g., accounts found)
    const runtime = await api.getPublicRuntimeSettings();
    if (!runtime.needsSetup) {
      emit("setup-complete"); // Tell App.vue to redirect to login
    } else {
      step.value = "admin"; // Otherwise, continue to admin creation step
    }
  } catch (error) {
    configError.value =
      error instanceof Error ? error.message : "Configuration test failed";
  } finally {
    loading.value = false;
  }
};

const generate2FA = async () => {
  tfa.loading = true;
  tfa.error = "";
  try {
    const response = await api.setupGenerate(adminForm.secretKey);
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

const createAdmin = async () => {
  loading.value = true;
  formError.value = "";
  try {
    const response = await api.setupCreate(adminForm);
    recoveryCodes.value = response.recoveryCodes;
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
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-base-200">
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
          <label class="form-control w-full">
            <div class="label">
              <span class="label-text">Secret Key (Optional)</span>
            </div>
            <input
              v-model="adminForm.secretKey"
              type="password"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              placeholder="Leave empty to auto-generate"
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
