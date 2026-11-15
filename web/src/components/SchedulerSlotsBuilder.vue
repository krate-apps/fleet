<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import {
  bytesPerSecToKib,
  emptySchedulerSlot,
  hourInSlot,
  kibToBytesPerSec,
  weekHeatmap,
  type SchedulerSlot,
} from "@/lib/schedulerSlots";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{
  modelValue: SchedulerSlot[];
}>();

const emit = defineEmits<{
  "update:modelValue": [value: SchedulerSlot[]];
}>();

const { t } = useI18n();

const DAY_KEYS = [
  "scheduler.day.sun",
  "scheduler.day.mon",
  "scheduler.day.tue",
  "scheduler.day.wed",
  "scheduler.day.thu",
  "scheduler.day.fri",
  "scheduler.day.sat",
] as const;

const HOURS = Array.from({ length: 24 }, (_, h) => h);
const START_HOUR_OPTIONS = HOURS.map((h) => ({
  value: h,
  label: `${String(h).padStart(2, "0")}:00`,
}));
const END_HOUR_OPTIONS = [
  ...START_HOUR_OPTIONS,
  { value: 24, label: "24:00" },
];

const slots = ref<SchedulerSlot[]>([]);
const suppress = ref(false);
const pickEndFor = ref<number | null>(null);
let emitTimer: ReturnType<typeof setTimeout> | null = null;

function cloneSlots(list: SchedulerSlot[] | undefined | null): SchedulerSlot[] {
  return (list ?? []).map((s) => ({
    days: [...(s.days ?? [])],
    startHour: s.startHour,
    endHour: s.endHour,
    downRate: s.downRate,
    upRate: s.upRate,
    ...(s.throttleProfile ? { throttleProfile: s.throttleProfile } : {}),
  }));
}

function fingerprint(list: SchedulerSlot[]): string {
  return JSON.stringify(
    list.map((s) => [
      s.days,
      s.startHour,
      s.endHour,
      s.downRate,
      s.upRate,
      s.throttleProfile ?? "",
    ]),
  );
}

function scheduleEmit() {
  if (suppress.value) return;
  if (emitTimer != null) clearTimeout(emitTimer);
  emitTimer = setTimeout(() => {
    emitTimer = null;
    if (suppress.value) return;
    const next = cloneSlots(slots.value);
    if (fingerprint(next) === fingerprint(props.modelValue ?? [])) return;
    emit("update:modelValue", next);
  }, 80);
}

watch(
  () => fingerprint(props.modelValue ?? []),
  async () => {
    const incoming = cloneSlots(props.modelValue);
    if (fingerprint(incoming) === fingerprint(slots.value)) return;
    suppress.value = true;
    slots.value = incoming;
    await nextTick();
    suppress.value = false;
  },
  { immediate: true },
);

watch(slots, scheduleEmit, { deep: true });

const heatmap = computed(() => weekHeatmap(slots.value));

function hourStripGradient(
  on: (hour: number) => boolean,
  onColor: string,
  offColor: string,
): string {
  const stops: string[] = [];
  for (let h = 0; h < 24; h++) {
    const color = on(h) ? onColor : offColor;
    const a = `${((h / 24) * 100).toFixed(4)}%`;
    const b = `${(((h + 1) / 24) * 100).toFixed(4)}%`;
    stops.push(`${color} ${a}`, `${color} ${b}`);
  }
  return `linear-gradient(to right, ${stops.join(", ")})`;
}

const heatmapStrips = computed(() => {
  const grid = heatmap.value;
  const onC = "color-mix(in srgb, var(--fleet-primary) 65%, transparent)";
  const offC = "color-mix(in srgb, var(--fleet-border) 40%, transparent)";
  return grid.map((row) =>
    hourStripGradient((h) => row[h], onC, offC),
  );
});

/** Compact CSS bar (one element, no 24 buttons). */
function barStyle(slot: SchedulerSlot): Record<string, string> {
  return {
    backgroundImage: hourStripGradient(
      (h) => hourInSlot(h, slot.startHour, slot.endHour),
      "color-mix(in srgb, var(--fleet-primary) 55%, transparent)",
      "transparent",
    ),
  };
}

function addSlot() {
  slots.value.push(emptySchedulerSlot());
}

function removeSlot(i: number) {
  slots.value.splice(i, 1);
  if (pickEndFor.value === i) pickEndFor.value = null;
}

function toggleDay(slot: SchedulerSlot, day: number) {
  const set = new Set(slot.days);
  if (set.has(day)) set.delete(day);
  else set.add(day);
  slot.days = [...set].sort((a, b) => a - b);
}

function dayActive(slot: SchedulerSlot, day: number): boolean {
  return slot.days.includes(day);
}

function downKib(slot: SchedulerSlot): number {
  return bytesPerSecToKib(slot.downRate);
}

function setDownKib(slot: SchedulerSlot, kib: number) {
  slot.downRate = kibToBytesPerSec(kib);
}

function upKib(slot: SchedulerSlot): number {
  return bytesPerSecToKib(slot.upRate);
}

function setUpKib(slot: SchedulerSlot, kib: number) {
  slot.upRate = kibToBytesPerSec(kib);
}

function onBarPointer(slotIndex: number, ev: MouseEvent) {
  const el = ev.currentTarget as HTMLElement | null;
  if (!el) return;
  const rect = el.getBoundingClientRect();
  if (rect.width <= 0) return;
  const ratio = Math.min(1, Math.max(0, (ev.clientX - rect.left) / rect.width));
  const hour = Math.min(23, Math.floor(ratio * 24));
  const slot = slots.value[slotIndex];
  if (!slot) return;
  if (ev.altKey) {
    slot.endHour = Math.min(24, hour + 1);
    pickEndFor.value = null;
    return;
  }
  if (pickEndFor.value === slotIndex) {
    slot.endHour = Math.min(24, hour + 1);
    pickEndFor.value = null;
    return;
  }
  slot.startHour = hour;
  pickEndFor.value = slotIndex;
}
</script>

<template>
  <div class="fleet-dsl-builder fleet-scheduler-builder">
    <div class="fleet-dsl-toolbar">
      <button
        type="button"
        class="fleet-btn-primary"
        @click="addSlot"
      >
        {{ t("scheduler.addSlot") }}
      </button>
      <span
        v-if="!slots.length"
        class="fleet-ext-note"
      >{{ t("scheduler.empty") }}</span>
    </div>

    <div
      v-if="slots.length"
      class="fleet-scheduler-heatmap"
      role="img"
      :aria-label="t('scheduler.heatmap')"
    >
      <div class="fleet-scheduler-heatmap-label">
        {{ t("scheduler.heatmap") }}
      </div>
      <div class="fleet-scheduler-heatmap-grid">
        <div
          v-for="day in 7"
          :key="day"
          class="fleet-scheduler-heatmap-row"
        >
          <span class="fleet-scheduler-heatmap-day">{{ t(DAY_KEYS[day - 1]) }}</span>
          <span
            class="fleet-scheduler-heatmap-strip"
            :style="{ backgroundImage: heatmapStrips[day - 1] }"
          />
        </div>
      </div>
    </div>

    <article
      v-for="(slot, i) in slots"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <span class="fleet-dsl-section-title">{{ t("scheduler.slot", { n: i + 1 }) }}</span>
        <div class="fleet-dsl-card-actions">
          <button
            type="button"
            class="fleet-btn fleet-btn-danger"
            @click="removeSlot(i)"
          >
            {{ t("dsl.remove") }}
          </button>
        </div>
      </header>

      <div
        class="fleet-scheduler-days"
        role="group"
        :aria-label="t('scheduler.days')"
      >
        <button
          v-for="day in 7"
          :key="day"
          type="button"
          class="fleet-scheduler-day"
          :aria-pressed="dayActive(slot, day - 1)"
          @click="toggleDay(slot, day - 1)"
        >
          {{ t(DAY_KEYS[day - 1]) }}
        </button>
      </div>

      <div class="fleet-scheduler-hours">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("scheduler.start") }}</span>
          <select
            v-model.number="slot.startHour"
            class="fleet-decl-control"
          >
            <option
              v-for="opt in START_HOUR_OPTIONS"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("scheduler.end") }}</span>
          <select
            v-model.number="slot.endHour"
            class="fleet-decl-control"
          >
            <option
              v-for="opt in END_HOUR_OPTIONS"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </option>
          </select>
        </label>
      </div>

      <button
        type="button"
        class="fleet-scheduler-bar"
        :aria-label="t('scheduler.barHelp')"
        :style="barStyle(slot)"
        @click="onBarPointer(i, $event)"
      />
      <p class="fleet-ext-note">
        {{ t("scheduler.barHelp") }}
      </p>

      <div class="fleet-scheduler-rates">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("scheduler.downKib") }}</span>
          <input
            class="fleet-decl-control"
            type="number"
            min="0"
            step="1"
            :value="downKib(slot)"
            @input="setDownKib(slot, Number(($event.target as HTMLInputElement).value) || 0)"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("scheduler.upKib") }}</span>
          <input
            class="fleet-decl-control"
            type="number"
            min="0"
            step="1"
            :value="upKib(slot)"
            @input="setUpKib(slot, Number(($event.target as HTMLInputElement).value) || 0)"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("scheduler.profile") }}</span>
          <input
            v-model="slot.throttleProfile"
            class="fleet-decl-control"
            type="text"
            :placeholder="t('scheduler.profileAny')"
          >
        </label>
      </div>
    </article>
  </div>
</template>
