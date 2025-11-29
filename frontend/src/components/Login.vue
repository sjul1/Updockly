<script setup lang="ts">
import { Lock, Shield, KeyRound, Mail } from "lucide-vue-next";
import { reactive, ref, onMounted } from "vue";
import { useToast } from "vue-toastification";
import { api } from "../services/api";

export interface LoginPayload {
  username: string;
  password: string;
}

const props = defineProps<{
  loading: boolean;
  authenticated: boolean;
  userName?: string;
  onSubmit: (payload: LoginPayload) => Promise<void>;
  onLogout: () => void;
  twoFactorRequired: boolean;
  onVerify2fa: (code: string) => Promise<void>;
  ssoEnabled?: boolean;
}>();

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

const twoFactorCode = ref("");
const failedAttempts = ref(0);
const isRecoveryMode = ref(false);
const isResetMode = ref(false);
const isForgotEmailMode = ref(false);
const isTokenResetMode = ref(false);
const resetToken = ref("");
const submitting = ref(false);
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
    // Clear token from URL to be clean
    window.history.replaceState({}, document.title, window.location.pathname);
  }
});

const handleSubmit = async () => {
  submitting.value = true;
  form.error = "";
  try {
    await props.onSubmit({ username: form.username, password: form.password });
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
    await props.onVerify2fa(twoFactorCode.value);
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
    await api.forgotPassword(form.username); // abusing form.username field for email input to reuse
    toast.success("If the email exists, a reset link has been sent.");
    isForgotEmailMode.value = false;
    form.username = "";
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

const toggleRecoveryMode = () => {
  isRecoveryMode.value = !isRecoveryMode.value;
  form.error = "";
  twoFactorCode.value = "";
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
  form.username = ""; // Reset input
};
</script>

<template>
  <div
    class="w-full max-w-md rounded-3xl bg-base-100/90 p-8 shadow-xl backdrop-blur"
  >
    <div v-if="!twoFactorRequired && !isResetMode && !isForgotEmailMode && !isTokenResetMode" class="flex items-center gap-3">
      <Lock class="text-primary" :size="28" />
      <div>
        <p class="text-xs uppercase tracking-[0.4em] text-primary">Updockly</p>
        <p class="text-2xl font-bold">Sign in</p>
      </div>
    </div>
    <div v-else-if="isForgotEmailMode" class="flex items-center gap-3">
      <Mail class="text-primary" :size="28" />
      <div>
        <p class="text-xs uppercase tracking-[0.4em] text-primary">Recovery</p>
        <p class="text-2xl font-bold">Reset Password</p>
      </div>
    </div>
    <div v-else-if="isResetMode || isTokenResetMode" class="flex items-center gap-3">
      <KeyRound class="text-primary" :size="28" />
      <div>
        <p class="text-xs uppercase tracking-[0.4em] text-primary">Recovery</p>
        <p class="text-2xl font-bold">Reset Password</p>
      </div>
    </div>
    <div v-else class="flex items-center gap-3">
      <Shield class="text-primary" :size="28" />
      <div>
        <p class="text-xs uppercase tracking-[0.4em] text-primary">
          Two-Factor Authentication
        </p>
        <p class="text-2xl font-bold">
          {{ isRecoveryMode ? "Enter Recovery Code" : "Enter Code" }}
        </p>
      </div>
    </div>

    <form
      v-if="!authenticated && !twoFactorRequired && !isResetMode && !isForgotEmailMode && !isTokenResetMode"
      @submit.prevent="handleSubmit"
      class="mt-6 w-full max-w-sm mx-auto space-y-4"
    >
      <label class="form-control w-full">
        <div class="label">
          <span class="text-xs font-semibold uppercase tracking-wide"
            >Username</span
          >
        </div>
        <input
          v-model.trim="form.username"
          id="username"
          name="username"
          type="text"
          class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
          autocomplete="username"
          required
        />
      </label>

      <label class="form-control w-full">
        <div class="label">
          <span class="text-xs font-semibold uppercase tracking-wide"
            >Password</span
          >
        </div>
        <input
          v-model="form.password"
          id="password"
          name="password"
          type="password"
          class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
          autocomplete="current-password"
          required
        />
        <div class="label">
          <span class="label-text-alt"></span>
          <button
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
        :class="{ loading: loading || submitting }"
        type="submit"
        :disabled="loading || submitting"
        :aria-busy="loading || submitting"
      >
        <span v-if="!(loading || submitting)">Sign in</span>
        <span v-else>Signing in…</span>
      </button>

      <p
        v-if="form.error"
        class="text-error text-sm text-center bg-error/10 border border-error/30 rounded-md px-3 py-2"
        role="alert"
      >
        {{ form.error }}
      </p>

      <div v-if="props.ssoEnabled" class="divider text-xs text-base-content/50">OR</div>

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
          <span class="text-xs font-semibold uppercase tracking-wide">New Password</span>
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
          @click="isTokenResetMode = false; resetToken = '';"
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
        <span>Enter your email address to receive a password reset link.</span>
      </div>

      <label class="form-control w-full">
        <div class="label">
          <span class="text-xs font-semibold uppercase tracking-wide">Email</span>
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
        :class="{ loading: submitting }"
        type="submit"
        :disabled="submitting"
      >
        Send Reset Link
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
        <span>Use a recovery code generated during 2FA setup to reset your password.</span>
      </div>

      <label class="form-control w-full">
        <div class="label">
          <span class="text-xs font-semibold uppercase tracking-wide">Username</span>
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
          <span class="text-xs font-semibold uppercase tracking-wide">Recovery Code</span>
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
          <span class="text-xs font-semibold uppercase tracking-wide">New Password</span>
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

    <form
      v-else-if="twoFactorRequired"
      @submit.prevent="handle2FASubmit"
      class="mt-6 w-full max-w-sm mx-auto space-y-4"
    >
      <label class="form-control w-full">
        <div class="label">
          <span class="text-xs font-semibold uppercase tracking-wide">
            {{ isRecoveryMode ? "Recovery Code" : "Authentication" }}
          </span>
        </div>
        <input
          v-model.trim="twoFactorCode"
          id="2fa-code"
          name="2fa-code"
          type="text"
          class="input input-bordered input-sm rounded-xl bg-base-100/70 focus:outline-none focus:ring-2 focus:ring-primary/40 w-full"
          :placeholder="isRecoveryMode ? 'Enter recovery code' : '123456'"
          autocomplete="one-time-code"
          required
        />
      </label>

      <button
        class="btn btn-primary w-full mt-4 rounded-xl"
        :class="{ loading: loading || submitting }"
        type="submit"
        :disabled="loading || submitting"
        :aria-busy="loading || submitting"
      >
        <span v-if="!(loading || submitting)">Verify</span>
        <span v-else>Verifying…</span>
      </button>

      <div v-if="failedAttempts > 0 || isRecoveryMode" class="text-center mt-2">
        <button
          type="button"
          class="link link-primary text-xs no-underline hover:underline"
          @click="toggleRecoveryMode"
        >
          {{
            isRecoveryMode
              ? "Use Authenticator App code?"
              : "Use your recovery code instead?"
          }}
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
      <button class="btn btn-ghost btn-sm mt-4" @click="onLogout">
        Sign out
      </button>
    </div>
  </div>
</template>
