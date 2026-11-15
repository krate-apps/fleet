<script setup lang="ts">
import { ref, watch } from "vue";
import {
  emptyRcloneRule,
  parseRcloneRulesText,
  serializeRcloneRules,
  type RcloneRule,
} from "@/lib/rcloneRulesDsl";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  parseError: [message: string];
}>();

const { t } = useI18n();
const rules = ref<RcloneRule[]>([]);
const suppress = ref(false);

watch(
  () => props.modelValue,
  (text) => {
    const { rules: parsed, error } = parseRcloneRulesText(text ?? "");
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
    emit("update:modelValue", serializeRcloneRules(rules.value));
  },
  { deep: true },
);

function addRule() {
  rules.value.push(emptyRcloneRule());
}

function removeRule(i: number) {
  rules.value.splice(i, 1);
}
</script>

<template>
  <div class="fleet-dsl-builder">
    <div class="fleet-dsl-toolbar">
      <button
        type="button"
        class="fleet-btn-primary"
        @click="addRule"
      >
        {{ t("dsl.rclone.add") }}
      </button>
      <span
        v-if="!rules.length"
        class="fleet-ext-note"
      >{{ t("dsl.rclone.empty") }}</span>
    </div>

    <article
      v-for="(r, i) in rules"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rclone.action") }}</span>
          <select
            v-model="r.action"
            class="fleet-decl-control"
          >
            <option value="copy">
              copy
            </option>
            <option value="move">
              move
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rclone.remote") }}</span>
          <input
            v-model="r.remote"
            class="fleet-decl-control"
            type="text"
            placeholder="gdrive"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rclone.label") }}</span>
          <input
            v-model="r.label"
            class="fleet-decl-control"
            type="text"
            :placeholder="t('dsl.rclone.labelAny')"
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
        class="fleet-decl-field"
        data-wide="true"
      >
        <span class="fleet-decl-label">{{ t("dsl.rclone.dest") }}</span>
        <input
          v-model="r.dest"
          class="fleet-decl-control"
          type="text"
          placeholder="{label}/{name}"
        >
      </label>
    </article>
  </div>
</template>
