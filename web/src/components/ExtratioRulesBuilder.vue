<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  emptyExtratioRule,
  parseExtratioRulesText,
  serializeExtratioRules,
  type ExtratioRule,
} from "@/lib/extratioDsl";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  parseError: [message: string];
}>();

const { t } = useI18n();
const rules = ref<ExtratioRule[]>([]);
const suppress = ref(false);

watch(
  () => props.modelValue,
  (text) => {
    const { rules: parsed, error } = parseExtratioRulesText(text ?? "");
    if (error) {
      emit("parseError", error);
      return;
    }
    emit("parseError", "");
    suppress.value = true;
    rules.value = parsed;
    suppress.value = false;
  },
  { immediate: true },
);

watch(
  rules,
  () => {
    if (suppress.value) return;
    emit("update:modelValue", serializeExtratioRules(rules.value));
  },
  { deep: true },
);

function addRule() {
  rules.value.push(emptyExtratioRule());
}

function removeRule(i: number) {
  rules.value.splice(i, 1);
}

function needsPattern(r: ExtratioRule): boolean {
  return r.match === "label_contain" || r.match === "tracker_contain";
}

const matchOptions = computed(() =>
  [
    ["label_contain", "dsl.extratio.matchLabel"],
    ["tracker_contain", "dsl.extratio.matchTracker"],
    ["private", "dsl.extratio.matchPrivate"],
    ["public", "dsl.extratio.matchPublic"],
  ] as const,
);
</script>

<template>
  <div class="fleet-dsl-builder">
    <div class="fleet-dsl-toolbar">
      <button
        type="button"
        class="fleet-btn-primary"
        @click="addRule"
      >
        {{ t("dsl.extratio.add") }}
      </button>
      <span
        v-if="!rules.length"
        class="fleet-ext-note"
      >{{ t("dsl.extratio.empty") }}</span>
    </div>

    <article
      v-for="(r, i) in rules"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.extratio.name") }}</span>
          <input
            v-model="r.name"
            class="fleet-decl-control"
            type="text"
            placeholder="priv"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.extratio.match") }}</span>
          <select
            v-model="r.match"
            class="fleet-decl-control"
          >
            <option
              v-for="[value, key] in matchOptions"
              :key="value"
              :value="value"
            >
              {{ t(key) }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.extratio.target") }}</span>
          <input
            v-model="r.targetLabel"
            class="fleet-decl-control"
            type="text"
            placeholder="seed"
          >
        </label>
        <label class="fleet-decl-field fleet-dsl-check">
          <span class="fleet-decl-label">{{ t("dsl.extratio.enabled") }}</span>
          <input
            v-model="r.enabled"
            type="checkbox"
          >
        </label>
        <div class="fleet-dsl-card-actions">
          <button
            type="button"
            class="fleet-btn fleet-btn-danger"
            @click="removeRule(i)"
          >
            {{ t("dsl.remove") }}
          </button>
        </div>
      </header>
      <label
        v-if="needsPattern(r)"
        class="fleet-decl-field"
        data-wide="true"
      >
        <span class="fleet-decl-label">{{ t("dsl.extratio.pattern") }}</span>
        <input
          v-model="r.pattern"
          class="fleet-decl-control"
          type="text"
          :placeholder="r.match === 'tracker_contain' ? 'example.org' : 'tv'"
        >
      </label>
    </article>
  </div>
</template>
