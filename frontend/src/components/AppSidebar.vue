<script setup lang="ts">
import type { Component } from "vue";
import {
  Moon,
  Sun,
  LogOut,
  Server,
  User,
  Activity,
  Coffee,
  X,
} from "lucide-vue-next";

export type Panel =
  | "login"
  | "settings"
  | "containers"
  | "history"
  | "schedule"
  | "agents";
export interface NavItem {
  id: Panel;
  label: string;
  icon: Component;
}

const props = defineProps<{
  navItems: NavItem[];
  active: Panel;
  theme: "light" | "dark";
  backendVersion: string;
  healthStatus: string;
  userName?: string;
  isAuthenticated: boolean;
  hideSupportButton?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:panel", panel: Panel): void;
  (e: "toggleTheme"): void;
  (e: "logout"): void;
  (e: "close"): void;
}>();

const toggleTheme = () => emit("toggleTheme");
const handleLogout = () => emit("logout");
const handleNav = (id: Panel) => emit("update:panel", id);
</script>

<template>
  <aside
    class="flex h-screen w-72 flex-col border-r border-base-300 bg-base-100/50 backdrop-blur-md transition-all duration-300"
  >
    <!-- Header -->
    <div class="p-6">
      <div class="flex items-center justify-between">
        <div
          class="flex items-center gap-3 select-none cursor-pointer"
          @click="handleNav('login')"
        >
          <div
            class="flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-content shadow-lg shadow-primary/30"
          >
            <Activity class="h-6 w-6" />
          </div>
          <div class="flex flex-col leading-none">
            <span class="text-lg font-bold tracking-tight">Updockly</span>
            <span
              class="text-[0.65rem] font-medium text-base-content/50 uppercase tracking-wider"
              >Container Mgr</span
            >
          </div>
        </div>

        <div class="flex items-center gap-1">
          <button
            class="btn btn-circle btn-ghost btn-sm text-base-content/70 hover:bg-base-200 lg:hidden"
            @click="$emit('close')"
          >
            <X :size="18" />
          </button>
          <button
            class="btn btn-circle btn-ghost btn-sm text-base-content/70 hover:bg-base-200"
            @click="toggleTheme"
            :title="
              props.theme === 'dark'
                ? 'Switch to light mode'
                : 'Switch to dark mode'
            "
          >
            <Sun v-if="props.theme === 'dark'" :size="18" />
            <Moon v-else :size="18" />
          </button>
        </div>
      </div>
    </div>

    <!-- Navigation -->
    <div class="flex-1 px-4">
      <nav class="space-y-1.5">
        <button
          v-for="item in props.navItems"
          :key="item.id"
          class="group flex w-full items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-all duration-200"
          :class="
            props.active === item.id
              ? 'bg-primary text-primary-content shadow-md shadow-primary/20'
              : 'text-base-content/70 hover:bg-base-200 hover:text-base-content'
          "
          @click="handleNav(item.id)"
        >
          <component
            :is="item.icon"
            :size="20"
            class="transition-transform group-hover:scale-110"
            :class="{ 'opacity-70': props.active !== item.id }"
          />
          <span>{{ item.label }}</span>

          <div
            v-if="props.active === item.id"
            class="ml-auto h-1.5 w-1.5 rounded-full bg-white/50"
          ></div>
        </button>
      </nav>
    </div>

    <!-- Support -->
    <div v-if="!props.hideSupportButton" class="px-4 pb-4">
      <a
        href="https://buymeacoffee.com/joul"
        target="_blank"
        rel="noreferrer"
        class="group relative flex items-center gap-3 rounded-xl border border-amber-200/80 bg-amber-50/80 px-3 py-3 shadow-sm transition hover:-translate-y-0.5 hover:shadow-md hover:border-amber-300 overflow-hidden"
      >
        <div
          class="absolute inset-0 z-0 flex items-center justify-center animate-shimmer-loop pointer-events-none"
        >
          <div
            class="h-full w-2/3 -skew-x-12 bg-gradient-to-r from-transparent via-white/50 to-transparent"
          ></div>
        </div>

        <div
          class="relative z-10 flex h-10 w-10 items-center justify-center rounded-full bg-amber-100 text-amber-700"
        >
          <Coffee class="h-5 w-5" />
        </div>
        <div class="relative z-10 flex-1 min-w-0">
          <p
            class="text-xs uppercase tracking-wide text-amber-700/80 font-semibold"
          >
            Support the project
          </p>
          <p class="text-sm text-amber-800 leading-snug">
            Buy me a coffee to keep Updockly brewing ☕
          </p>
        </div>
      </a>
    </div>

    <!-- Footer Status -->
    <div
      class="mt-auto border-t border-base-300 bg-base-200/30 p-4 backdrop-blur-sm"
    >
      <div class="space-y-4">
        <!-- System Status -->
        <div
          class="rounded-lg border border-base-200 bg-base-100 p-3 shadow-sm"
        >
          <div class="flex items-center justify-between text-xs">
            <div class="flex items-center gap-2 text-base-content/70">
              <Server class="h-3.5 w-3.5" />
              <span class="font-mono"
                >v{{ props.backendVersion || "..." }}</span
              >
            </div>
            <div
              class="flex items-center gap-1.5"
              :class="
                props.healthStatus === 'OK' ? 'text-success' : 'text-error'
              "
            >
              <div class="relative flex h-2 w-2">
                <span
                  v-if="props.healthStatus === 'OK'"
                  class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-75"
                ></span>
                <span
                  class="relative inline-flex h-2 w-2 rounded-full bg-current"
                ></span>
              </div>
              <span class="font-semibold">{{
                props.healthStatus === "OK" ? "Online" : "Offline"
              }}</span>
            </div>
          </div>
        </div>

        <!-- User Profile -->
        <div
          v-if="props.isAuthenticated && props.userName"
          class="flex items-center justify-between gap-3 pt-1"
        >
          <div class="flex items-center gap-3 overflow-hidden">
            <div
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-base-300 text-base-content/70 ring-2 ring-base-100"
            >
              <User class="h-4 w-4" />
            </div>
            <div class="flex flex-col overflow-hidden">
              <span class="truncate text-sm font-semibold text-base-content">{{
                props.userName
              }}</span>
              <span class="truncate text-xs text-base-content/50"
                >Administrator</span
              >
            </div>
          </div>

          <button
            class="btn btn-square btn-ghost btn-sm text-error hover:bg-error/10"
            title="Sign Out"
            @click="handleLogout"
          >
            <LogOut class="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@keyframes shimmer-loop {
  0% {
    transform: translateX(-150%);
  }
  80% {
    transform: translateX(200%);
  }
  100% {
    transform: translateX(200%);
  }
}

.animate-shimmer-loop {
  animation: shimmer-loop 15s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}
</style>
