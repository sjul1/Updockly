<script setup lang="ts">
import { Lock, Shield, KeyRound, Mail, Download } from "lucide-vue-next";
import { reactive, ref, onMounted } from "vue";
import { useToast } from "vue-toastification";
import { api } from "../services/api";

export interface LoginPayload {
  username: string;
  password: string;
}

const props = defineProps({
  loading: Boolean,
  authenticated: Boolean,
  userName: String,
  onSubmit: Function,
  onLogout: Function,
  twoFactorRequired: Boolean,
  onVerify2fa: Function,
  ssoEnabled: Boolean,
});

const form = reactive({
  username: "",
  password: "",
  error: "",
});

const resetForm = reactive({
  username: "",
  recoveryCode: "",
  newPassword: "",
});

const reset2FAForm = reactive({
  username: "",
  recoveryCode: "",
  password: "",
});

const twoFactorCode = ref("");
const failedAttempts = ref(0);
const isResetMode = ref(false);
const isForgotEmailMode = ref(false);
const isTokenResetMode = ref(false);
const isReset2FAMode = ref(false);
const resetToken = ref("");
const submitting = ref(false);

const reset2FAResult = reactive({
  secret: "",
  qrCode: "",
  recoveryCodes: [] as string[],
});
const reset2FATempToken = ref("");
const reset2FAStep = ref<"init" | "verify" | "complete">("init");
const isReloading = ref(false);
const reset2FAVerifyCode = ref("");

const toast = useToast();

onMounted(() => {
  const params = new URLSearchParams(window.location.search);
  const errorMsg = params.get("error");
  if (errorMsg) {
    form.error = errorMsg;
    toast.error(errorMsg);
    window.history.replaceState({}, document.title, window.location.pathname);
  }
  const token = params.get("token");
  if (token) {
    resetToken.value = token;
    isTokenResetMode.value = true;
    window.history.replaceState({}, document.title, window.location.pathname);
  }
});

const handleSubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    if (props.onSubmit) {
      await props.onSubmit({
        username: form.username,
        password: form.password,
      });
    }
    form.password = "";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Unable to sign in";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handle2FASubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    if (props.onVerify2fa) {
      await props.onVerify2fa(twoFactorCode.value);
    }
    twoFactorCode.value = "";
    failedAttempts.value = 0;
  } catch (error) {
    failedAttempts.value++;
    const message =
      error instanceof Error ? error.message : "Unable to verify 2FA code";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleResetSubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    await api.resetPassword({
      username: resetForm.username,
      recoveryCode: resetForm.recoveryCode,
      newPassword: resetForm.newPassword,
    });
    toast.success("Password reset successfully. Please log in.");
    isResetMode.value = false;
    resetForm.username = "";
    resetForm.recoveryCode = "";
    resetForm.newPassword = "";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Failed to reset password";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleForgotEmailSubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    await api.forgotPassword(form.username);
    toast.success("If the email exists, a reset link has been sent.");
    isForgotEmailMode.value = false;
    form.username = "";
    form.password = "";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Failed to request reset";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleTokenResetSubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    await api.resetPasswordWithToken({
      token: resetToken.value,
      newPassword: resetForm.newPassword,
    });
    toast.success("Password reset successfully. Please log in.");
    isTokenResetMode.value = false;
    resetToken.value = "";
    resetForm.newPassword = "";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Failed to reset password";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleInitiateReset2FA = async () => {
  submitting.value = true;
  form.error = "";
  try {
    const result = await api.initiateReset2FA(
      reset2FAForm.username,
      reset2FAForm.recoveryCode,
      reset2FAForm.password
    );
    reset2FAResult.secret = result.secret;
    reset2FAResult.qrCode = result.qrCode;
    reset2FATempToken.value = result.tempToken;
    reset2FAStep.value = "verify";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Failed to initiate 2FA reset";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleFinalizeReset2FA = async () => {
  if (!reset2FAVerifyCode.value) return;
  submitting.value = true;
  form.error = "";
  try {
    const result = await api.finalizeReset2FA(
      reset2FATempToken.value,
      reset2FAVerifyCode.value
    );
    reset2FAResult.recoveryCodes = result.recoveryCodes;
    reset2FAStep.value = "complete";
    reset2FAForm.password = "";
    reset2FAVerifyCode.value = "";
  } catch (error) {
    const message =
      error instanceof Error ? error.message : "Failed to verify code";
    form.error = message;
  } finally {
    submitting.value = false;
  }
};

const handleResetDone = () => {
  isReloading.value = true;
  setTimeout(() => {
    window.location.reload();
  }, 800);
};

const downloadRecoveryCodes = () => {
  const text = reset2FAResult.recoveryCodes.join("\n");
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

const toggleResetMode = () => {
  isResetMode.value = !isResetMode.value;
  isForgotEmailMode.value = false;
  form.error = "";
};

const toggleForgotEmailMode = () => {
  isForgotEmailMode.value = !isForgotEmailMode.value;
  isResetMode.value = false;
  form.error = "";
  form.username = "";
};

const toggleReset2FAMode = () => {
  isReset2FAMode.value = !isReset2FAMode.value;
  if (isReset2FAMode.value) {
    reset2FAStep.value = "init";
    form.error = "";
    reset2FAForm.username = form.username;
    reset2FAForm.recoveryCode = "";
    reset2FAForm.password = "";
  }
};
</script>

<template>
  <div id="login-wrapper" class="w-full">
    <!-- Loading Overlay -->
    <div
      v-if="isReloading"
      class="fixed inset-0 z-50 flex h-screen items-center justify-center bg-base-200"
    >
      <div class="flex flex-col items-center gap-4">
        <span class="loading loading-spinner loading-lg text-primary"></span>
      </div>
    </div>

    <!-- Main Content -->
    <div
      v-else
      class="w-full max-w-[520px] rounded-3xl bg-base-100/90 p-10 shadow-xl backdrop-blur mx-auto"
    >
      <div
        v-if="
          !twoFactorRequired &&
          !isResetMode &&
          !isForgotEmailMode &&
          !isTokenResetMode &&
          !isReset2FAMode
        "
        class="flex items-center gap-3"
      >
        <Lock class="text-primary" :size="28" />
        <div>
          <p class="text-xs uppercase tracking-[0.4em] text-primary">
            Updockly
          </p>
          <p class="text-2xl font-bold">Sign in</p>
        </div>
      </div>
      <div v-else-if="isForgotEmailMode" class="flex items-center gap-3">
        <Mail class="text-primary" :size="28" />
        <div>
          <p class="text-xs uppercase tracking-[0.4em] text-primary">
            Recovery
          </p>
          <p class="text-2xl font-bold">Reset Password</p>
        </div>
      </div>
      <div
        v-else-if="isResetMode || isTokenResetMode"
        class="flex items-center gap-3"
      >
        <KeyRound class="text-primary" :size="28" />
        <div>
          <p class="text-xs uppercase tracking-[0.4em] text-primary">
            Recovery
          </p>
          <p class="text-2xl font-bold">Reset Password</p>
        </div>
      </div>
      <div v-else-if="isReset2FAMode" class="flex items-center gap-3">
        <Shield class="text-primary" :size="28" />
        <div>
          <p class="text-xs uppercase tracking-[0.4em] text-primary">
            2FA Recovery
          </p>
          <p class="text-2xl font-bold">Reset 2FA</p>
        </div>
      </div>
      <div v-else class="flex items-center gap-3">
        <Shield class="text-primary" :size="28" />
        <div>
          <p class="text-xs uppercase tracking-[0.4em] text-primary">
            Two-Factor Authentication
          </p>
          <p class="text-2xl font-bold">Enter Code</p>
        </div>
      </div>

      <form
        v-if="
          !authenticated &&
          !twoFactorRequired &&
          !isResetMode &&
          !isForgotEmailMode &&
          !isTokenResetMode
        "
        @submit.prevent="handleSubmit"
        class="mt-6 w-full max-w-md mx-auto space-y-6"
      >
        <label class="form-control w-full">
          <label
            class="input input-bordered rounded-xl bg-base-100/70 focus-within:ring-2 focus-within:ring-primary/40 w-full flex items-center gap-2"
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
                <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"></path>
                <circle cx="12" cy="7" r="4"></circle>
              </g>
            </svg>
            <input
              v-model.trim="form.username"
              id="username"
              name="username"
              type="text"
              class="grow text-sm"
              placeholder="Username"
              autocomplete="username"
              required
            />
          </label>
        </label>

        <label class="form-control w-full">
          <label
            class="input input-bordered rounded-xl bg-base-100/70 flex items-center gap-2 focus-within:ring-2 focus-within:ring-primary/40 w-full mt-3"
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
                <circle cx="16.5" cy="7.5" r=".5" fill="currentColor"></circle>
              </g>
            </svg>
            <input
              v-model="form.password"
              id="password"
              name="password"
              type="password"
              class="grow text-sm"
              placeholder="Password"
              autocomplete="current-password"
              required
            />
          </label>
          <div class="label">
            <span class="label-text-alt"></span>
            <button
              v-if="form.error"
              type="button"
              class="label-text-alt link link-primary no-underline hover:underline"
              @click="toggleForgotEmailMode"
            >
              Forgot password?
            </button>
          </div>
        </label>

        <button
          class="btn btn-primary w-full mt-4 rounded-xl"
          type="submit"
          :disabled="loading || submitting"
          :aria-busy="loading || submitting"
        >
          <span v-if="loading || submitting" class="flex items-center gap-2">
            <span class="loading loading-spinner" /> Signing in…
          </span>
          <span v-else>Sign in</span>
        </button>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>

        <div
          v-if="props.ssoEnabled"
          class="divider text-xs text-base-content/50"
        >
          OR
        </div>

        <a
          v-if="props.ssoEnabled"
          href="/api/auth/sso/login"
          class="btn btn-outline w-full rounded-xl gap-2"
        >
          <Shield class="w-4 h-4" />
          Sign in with SSO
        </a>
      </form>

      <form
        v-else-if="isTokenResetMode"
        @submit.prevent="handleTokenResetSubmit"
        class="mt-6 w-full max-w-sm mx-auto space-y-4"
      >
        <div class="alert alert-info shadow-sm text-xs">
          <span>Enter your new password to complete the reset process.</span>
        </div>

        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide"
              >New Password</span
            >
          </div>
          <input
            v-model="resetForm.newPassword"
            type="password"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            required
          />
        </label>

        <div class="flex gap-2 mt-4">
          <button
            class="btn btn-ghost w-1/3 rounded-xl"
            type="button"
            @click="
              isTokenResetMode = false;
              resetToken = '';
            "
          >
            Cancel
          </button>
          <button
            class="btn btn-primary w-2/3 rounded-xl"
            :class="{ loading: submitting }"
            type="submit"
            :disabled="submitting"
          >
            Reset Password
          </button>
        </div>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>
      </form>

      <form
        v-else-if="isForgotEmailMode"
        @submit.prevent="handleForgotEmailSubmit"
        class="mt-6 w-full max-w-sm mx-auto space-y-4"
      >
        <div class="alert alert-info shadow-sm text-xs">
          <span
            >Enter your email address to receive a password reset link.</span
          >
        </div>

        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide"
              >Email</span
            >
          </div>
          <input
            v-model.trim="form.username"
            type="email"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            placeholder="admin@example.com"
            required
          />
        </label>

        <button
          class="btn btn-primary w-full mt-4 rounded-xl"
          type="submit"
          :disabled="submitting"
        >
          <span v-if="submitting" class="flex items-center gap-2">
            <span class="loading loading-spinner" /> Sending…
          </span>
          <span v-else>Send Reset Link</span>
        </button>

        <div class="text-center mt-2">
          <button
            type="button"
            class="link link-primary text-xs no-underline hover:underline"
            @click="toggleResetMode"
          >
            Have a recovery code?
          </button>
        </div>
        <div class="text-center mt-1">
          <button
            type="button"
            class="link link-ghost text-xs no-underline hover:underline"
            @click="toggleForgotEmailMode"
          >
            Cancel
          </button>
        </div>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>
      </form>

      <form
        v-else-if="isResetMode"
        @submit.prevent="handleResetSubmit"
        class="mt-6 w-full max-w-sm mx-auto space-y-4"
      >
        <div class="alert alert-info shadow-sm text-xs">
          <span
            >Use a recovery code generated during 2FA setup to reset your
            password.</span
          >
        </div>

        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide"
              >Username</span
            >
          </div>
          <input
            v-model.trim="resetForm.username"
            type="text"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            required
          />
        </label>

        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide"
              >Recovery Code</span
            >
          </div>
          <input
            v-model.trim="resetForm.recoveryCode"
            type="text"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            placeholder="1234567890"
            required
          />
        </label>

        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide"
              >New Password</span
            >
          </div>
          <input
            v-model="resetForm.newPassword"
            type="password"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            required
          />
        </label>

        <div class="flex gap-2 mt-4">
          <button
            class="btn btn-ghost w-1/3 rounded-xl"
            type="button"
            @click="toggleResetMode"
          >
            Cancel
          </button>
          <button
            class="btn btn-primary w-2/3 rounded-xl"
            :class="{ loading: submitting }"
            type="submit"
            :disabled="submitting"
          >
            Reset Password
          </button>
        </div>

        <div class="text-center mt-2">
          <button
            type="button"
            class="link link-primary text-xs no-underline hover:underline"
            @click="toggleForgotEmailMode"
          >
            Back to Email Reset
          </button>
        </div>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>
      </form>

      <div v-else-if="isReset2FAMode" class="w-full max-w-sm mx-auto space-y-4">
        <!-- Step 1: Init -->
        <form
          v-if="reset2FAStep === 'init'"
          @submit.prevent="handleInitiateReset2FA"
          class="space-y-4"
        >
          <div class="alert alert-warning shadow-sm text-xs mt-6">
            <span
              >To reset 2FA, enter your username, a 2FA recovery code, and your
              current password.</span
            >
          </div>

          <label class="form-control w-full">
            <div class="label">
              <span class="text-xs font-semibold uppercase tracking-wide"
                >Username</span
              >
            </div>
            <input
              v-model.trim="reset2FAForm.username"
              type="text"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              required
            />
          </label>

          <label class="form-control w-full">
            <div class="label">
              <span class="text-xs font-semibold uppercase tracking-wide"
                >Recovery Code</span
              >
            </div>
            <input
              v-model.trim="reset2FAForm.recoveryCode"
              type="text"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              placeholder="1234567890"
              required
            />
          </label>

          <label class="form-control w-full">
            <div class="label">
              <span class="text-xs font-semibold uppercase tracking-wide"
                >Current Password</span
              >
            </div>
            <input
              v-model="reset2FAForm.password"
              type="password"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              required
            />
          </label>

          <div class="flex gap-2 mt-4">
            <button
              class="btn btn-ghost w-1/3 rounded-xl"
              type="button"
              @click="toggleReset2FAMode"
            >
              Cancel
            </button>
            <button
              class="btn btn-warning w-2/3 rounded-xl"
              type="submit"
              :disabled="submitting"
            >
              <span v-if="submitting" class="flex items-center gap-2">
                <span class="loading loading-spinner" /> Reseting…
              </span>
              <span v-else>Reset 2FA</span>
            </button>
          </div>
        </form>

        <!-- Step 2: Verify -->
        <div v-else-if="reset2FAStep === 'verify'" class="space-y-4">
          <div class="alert alert-info shadow-sm text-xs mt-6">
            <span
              >Credentials verified. Scan the new QR code and enter a code to
              finalize.</span
            >
          </div>

          <div class="flex flex-col items-center space-y-2">
            <div class="bg-white p-2 rounded-lg">
              <img
                :src="reset2FAResult.qrCode"
                alt="QR Code"
                class="w-40 h-40"
              />
            </div>
            <p
              class="text-xs font-mono bg-base-200 p-2 rounded select-all break-all text-center"
            >
              {{ reset2FAResult.secret }}
            </p>
          </div>

          <label class="form-control w-full">
            <div class="label">
              <span class="text-xs font-semibold uppercase tracking-wide"
                >Verification Code</span
              >
            </div>
            <input
              v-model.trim="reset2FAVerifyCode"
              type="text"
              class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
              placeholder="123456"
              required
            />
          </label>

          <button
            class="btn btn-primary w-full rounded-xl mt-6"
            @click="handleFinalizeReset2FA"
            :disabled="submitting || !reset2FAVerifyCode"
          >
            <span v-if="submitting" class="flex items-center gap-2">
              <span class="loading loading-spinner" /> Proceeding…
            </span>
            <span v-else>Verify & Enable</span>
          </button>
        </div>

        <!-- Step 3: Complete -->
        <div v-else-if="reset2FAStep === 'complete'" class="space-y-4">
          <div class="alert alert-success shadow-sm mt-6">
            <span
              >2FA Reset Complete! Please save your new recovery codes.</span
            >
          </div>

          <div class="divider text-xs font-bold text-base-content/50">
            NEW RECOVERY CODES
          </div>

          <div
            class="grid grid-cols-2 gap-2 text-xs font-mono bg-base-200 p-3 rounded-lg"
          >
            <div
              v-for="code in reset2FAResult.recoveryCodes"
              :key="code"
              class="select-all text-center"
            >
              {{ code }}
            </div>
          </div>

          <button
            class="btn btn-outline btn-sm rounded-full w-full mt-2 flex items-center gap-2"
            @click="downloadRecoveryCodes"
          >
            <Download class="w-4 h-4" />
            Download as .txt
          </button>

          <p class="text-[10px] text-error text-center">
            Save these codes! Old codes are now invalid.
          </p>

          <button
            class="btn btn-primary w-full mt-2 rounded-xl"
            @click="handleResetDone"
          >
            I've saved these, Log In
          </button>
        </div>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>
      </div>

      <form
        v-else-if="twoFactorRequired"
        @submit.prevent="handle2FASubmit"
        class="mt-6 w-full max-w-sm mx-auto space-y-4"
      >
        <label class="form-control w-full">
          <div class="label">
            <span class="text-xs font-semibold uppercase tracking-wide">
              <span>Authentication</span>
            </span>
          </div>
          <input
            v-model.trim="twoFactorCode"
            id="2fa-code"
            name="2fa-code"
            type="text"
            class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
            :placeholder="'123456'"
            autocomplete="one-time-code"
            required
          />
        </label>

        <button
          class="btn btn-primary w-full mt-4 rounded-xl"
          type="submit"
          :disabled="loading || submitting"
          :aria-busy="loading || submitting"
        >
          <span v-if="loading || submitting" class="flex items-center gap-2">
            <span class="loading loading-spinner" /> Verifying…
          </span>
          <span v-else>Verify</span>
        </button>

        <div
          v-if="failedAttempts > 0 && !isReset2FAMode"
          class="text-center mt-2"
        >
          <button
            type="button"
            class="link link-warning text-xs no-underline hover:underline"
            @click="toggleReset2FAMode"
          >
            Reset 2FA with password & recovery code?
          </button>
        </div>

        <p
          v-if="form.error"
          class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
          role="alert"
        >
          {{ form.error }}
        </p>
      </form>

      <div v-else class="rounded-2xl bg-base-200/80 p-6 text-base">
        <p class="font-semibold">Welcome back, {{ userName }}</p>
        <p class="text-sm text-base-content/70">You are authenticated.</p>
        <button
          class="btn btn-ghost btn-sm mt-4"
          @click="props.onLogout && props.onLogout()"
        >
          Sign out
        </button>
      </div>
    </div>
  </div>
</template>
