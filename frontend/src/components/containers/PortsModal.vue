<script setup lang="ts">
import { computed } from "vue";
import { Network, X } from "lucide-vue-next";

type PortsItem = { container: string; ports: string[] };

const props = defineProps<{
  host: string | null;
  items: PortsItem[];
  filter: string;
}>();

const emit = defineEmits<{
  (e: "update:filter", value: string): void;
  (e: "close"): void;
}>();

const portsFilter = computed({
  get: () => props.filter,
  set: (val: string) => emit("update:filter", val),
});

const highlightSegments = (text: string, filter: string) => {
  if (!filter) return [{ text, highlighted: false }];
  const lowerFilter = filter.toLowerCase();
  const lowerText = text.toLowerCase();
  const segments: { text: string; highlighted: boolean }[] = [];
  let start = 0;
  let idx = lowerText.indexOf(lowerFilter, start);
  while (idx !== -1) {
    if (idx > start) {
      segments.push({ text: text.slice(start, idx), highlighted: false });
    }
    segments.push({
      text: text.slice(idx, idx + filter.length),
      highlighted: true,
    });
    start = idx + filter.length;
    idx = lowerText.indexOf(lowerFilter, start);
  }
  if (start < text.length) {
    segments.push({ text: text.slice(start), highlighted: false });
  }
  return segments;
};
</script>

<template>
  <Teleport to="body">
    <dialog v-if="host" class="modal modal-open">
      <div class="modal-box max-w-4xl">
        <div class="flex items-center justify-between mb-4">
          <h3 class="font-bold text-lg flex items-center gap-2">
            <Network class="w-5 h-5" /> Port Mapping: {{ host }}
          </h3>
          <button class="btn btn-sm btn-circle btn-ghost" @click="$emit('close')">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="mb-4">
          <div class="form-control w-full">
            <div class="relative">
              <input
                type="text"
                placeholder="Filter by container name or port..."
                class="input input-bordered w-full pr-10 rounded-xl"
                v-model="portsFilter"
              />
              <button
                v-if="portsFilter"
                class="btn btn-ghost btn-xs absolute right-2 top-1/2 -translate-y-1/2"
                @click="portsFilter = ''"
              >
                <X class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <div class="overflow-x-auto max-h-[60vh]">
          <table class="table table-zebra w-full">
            <thead>
              <tr>
                <th>Container</th>
                <th>Ports</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, idx) in items" :key="idx">
                <td class="font-mono text-sm">
                  <span v-for="(seg, segIdx) in highlightSegments(item.container, portsFilter)" :key="segIdx">
                    <span v-if="seg.highlighted" class="bg-warning/30 font-bold">{{ seg.text }}</span>
                    <span v-else>{{ seg.text }}</span>
                  </span>
                </td>
                <td>
                  <div class="flex flex-wrap gap-1">
                    <span
                      v-for="p in item.ports"
                      :key="p"
                      class="badge badge-outline font-mono text-xs"
                    >
                      <span v-for="(seg, segIdx) in highlightSegments(p, portsFilter)" :key="segIdx">
                        <span v-if="seg.highlighted" class="bg-warning/30 font-bold">{{ seg.text }}</span>
                        <span v-else>{{ seg.text }}</span>
                      </span>
                    </span>
                  </div>
                </td>
              </tr>
              <tr v-if="items.length === 0">
                <td colspan="2" class="text-center py-8 text-base-content/60">
                  {{ portsFilter ? "No matching ports found." : "No ports exposed." }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="modal-action">
          <button class="btn" @click="$emit('close')">Close</button>
        </div>
      </div>
      <form method="dialog" class="modal-backdrop" @click="$emit('close')">
        <button aria-label="close"></button>
      </form>
    </dialog>
  </Teleport>
</template>
