<script setup lang="ts">
import { ref, watch } from "vue";
import {
  HOOK_EVENTS,
  argsToText,
  emptyHook,
  parseHooksText,
  serializeHooks,
  textToArgs,
  type HookDef,
} from "@/lib/hooksDsl";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  parseError: [message: string];
}>();

const { t } = useI18n();
const hooks = ref<HookDef[]>([]);
const suppress = ref(false);

watch(
  () => props.modelValue,
  (text) => {
    const { hooks: parsed, error } = parseHooksText(text ?? "");
    if (error) {
      emit("parseError", error);
      return;
    }
    emit("parseError", "");
    suppress.value = true;
    hooks.value = parsed;
    suppress.value = false;
  },
  { immediate: true },
);

watch(
  hooks,
  () => {
    if (suppress.value) return;
    emit("update:modelValue", serializeHooks(hooks.value));
  },
  { deep: true },
);

function addHook() {
  hooks.value.push(emptyHook());
}

function removeHook(i: number) {
  hooks.value.splice(i, 1);
}

function argsText(h: HookDef): string {
  return argsToText(h.args);
}

function setArgs(h: HookDef, raw: string) {
  h.args = textToArgs(raw);
}
</script>

<template>
  <div class="fleet-dsl-builder">
    <div class="fleet-dsl-toolbar">
      <button
        type="button"
        class="fleet-btn-primary"
        @click="addHook"
      >
        {{ t("dsl.hooks.add") }}
      </button>
      <span
        v-if="!hooks.length"
        class="fleet-ext-note"
      >{{ t("dsl.hooks.empty") }}</span>
    </div>

    <article
      v-for="(h, i) in hooks"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.hooks.event") }}</span>
          <select
            v-model="h.event"
            class="fleet-decl-control"
          >
            <option
              v-for="ev in HOOK_EVENTS"
              :key="ev"
              :value="ev"
            >
              {{ ev }}
            </option>
          </select>
        </label>
        <label
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">{{ t("dsl.hooks.path") }}</span>
          <input
            v-model="h.path"
            class="fleet-decl-control"
            type="text"
            placeholder="/usr/local/bin/on-finish"
          >
        </label>
        <div class="fleet-dsl-card-actions">
          <button
            type="button"
            class="fleet-btn fleet-btn-danger"
            @click="removeHook(i)"
          >
            {{ t("dsl.remove") }}
          </button>
        </div>
      </header>
      <label
        class="fleet-decl-field"
        data-wide="true"
      >
        <span class="fleet-decl-label">{{ t("dsl.hooks.args") }}</span>
        <input
          class="fleet-decl-control"
          type="text"
          :value="argsText(h)"
          placeholder="{hash} {name} {path}"
          @input="setArgs(h, ($event.target as HTMLInputElement).value)"
        >
        <span class="fleet-decl-help">{{ t("dsl.hooks.argsHelp") }}</span>
      </label>
    </article>
  </div>
</template>
