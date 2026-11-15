<script setup lang="ts">
import { ref, watch } from "vue";
import {
  NOTIFY_KINDS,
  emptyChannel,
  parseChannelsText,
  serializeChannels,
  type NotifyChannel,
  type NotifyChannelKind,
} from "@/lib/notifyChannels";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
  parseError: [message: string];
}>();

const { t } = useI18n();
const channels = ref<NotifyChannel[]>([]);
const suppress = ref(false);

function loadFromText(text: string) {
  const { channels: parsed, error } = parseChannelsText(text);
  if (error) {
    emit("parseError", error);
    return false;
  }
  emit("parseError", "");
  suppress.value = true;
  channels.value = parsed.length ? parsed : [];
  suppress.value = false;
  return true;
}

watch(
  () => props.modelValue,
  (text) => {
    loadFromText(text ?? "");
  },
  { immediate: true },
);

watch(
  channels,
  () => {
    if (suppress.value) return;
    emit("update:modelValue", serializeChannels(channels.value));
  },
  { deep: true },
);

function addChannel() {
  channels.value.push(emptyChannel(channels.value.length + 1));
}

function removeChannel(i: number) {
  channels.value.splice(i, 1);
}

function onKindChange(ch: NotifyChannel, kind: NotifyChannelKind) {
  ch.kind = kind;
  if (!ch.id || /^(ntfy|gotify|slack|pushover|email)-?\d*$/.test(ch.id)) {
    ch.id = `${kind}-${channels.value.indexOf(ch) + 1}`;
  }
}

function show(ch: NotifyChannel, field: string): boolean {
  const k = ch.kind;
  if (field === "url") return k === "ntfy" || k === "gotify" || k === "slack" || k === "pushover";
  if (field === "token") return k === "gotify" || k === "pushover" || k === "slack";
  if (field === "user") return k === "pushover";
  if (field === "title") return k === "ntfy" || k === "gotify" || k === "pushover";
  if (field === "email") return k === "email";
  return false;
}
</script>

<template>
  <div class="fleet-dsl-builder">
    <div class="fleet-dsl-toolbar">
      <button
        type="button"
        class="fleet-btn-primary"
        @click="addChannel"
      >
        {{ t("dsl.notify.add") }}
      </button>
      <span
        v-if="!channels.length"
        class="fleet-ext-note"
      >{{ t("dsl.notify.empty") }}</span>
    </div>

    <article
      v-for="(ch, i) in channels"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.notify.kind") }}</span>
          <select
            class="fleet-decl-control"
            :value="ch.kind"
            @change="onKindChange(ch, ($event.target as HTMLSelectElement).value as NotifyChannelKind)"
          >
            <option
              v-for="k in NOTIFY_KINDS"
              :key="k"
              :value="k"
            >
              {{ k }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.notify.id") }}</span>
          <input
            v-model="ch.id"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label class="fleet-decl-field fleet-decl-field-check">
          <input
            v-model="ch.enabled"
            class="fleet-decl-check"
            type="checkbox"
          >
          <span>{{ t("dsl.notify.enabled") }}</span>
        </label>
        <div class="fleet-dsl-card-actions">
          <button
            type="button"
            class="fleet-btn fleet-btn-danger"
            @click="removeChannel(i)"
          >
            {{ t("dsl.remove") }}
          </button>
        </div>
      </header>

      <div class="fleet-decl-fields">
        <label
          v-if="show(ch, 'url')"
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">URL</span>
          <input
            v-model="ch.url"
            class="fleet-decl-control"
            type="url"
            :placeholder="ch.kind === 'ntfy' ? 'https://ntfy.sh/topic' : 'https://…'"
          >
        </label>
        <label
          v-if="show(ch, 'token')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">{{ t("dsl.notify.token") }}</span>
          <input
            v-model="ch.token"
            class="fleet-decl-control"
            type="password"
            autocomplete="off"
          >
        </label>
        <label
          v-if="show(ch, 'user')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">{{ t("dsl.notify.user") }}</span>
          <input
            v-model="ch.user"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label
          v-if="show(ch, 'title')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">{{ t("dsl.notify.title") }}</span>
          <input
            v-model="ch.title"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <template v-if="show(ch, 'email')">
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">smtpHost</span>
            <input
              v-model="ch.smtpHost"
              class="fleet-decl-control"
              type="text"
              placeholder="smtp.example.com"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">smtpPort</span>
            <input
              v-model.number="ch.smtpPort"
              class="fleet-decl-control"
              type="number"
              placeholder="587"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">from</span>
            <input
              v-model="ch.from"
              class="fleet-decl-control"
              type="email"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">to</span>
            <input
              v-model="ch.to"
              class="fleet-decl-control"
              type="email"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("dsl.notify.user") }}</span>
            <input
              v-model="ch.user"
              class="fleet-decl-control"
              type="text"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("dsl.notify.password") }}</span>
            <input
              v-model="ch.password"
              class="fleet-decl-control"
              type="password"
              autocomplete="off"
            >
          </label>
        </template>
      </div>
    </article>
  </div>
</template>
