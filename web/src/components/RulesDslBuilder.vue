<script setup lang="ts">
import { ref, watch } from "vue";
import {
  CLIENT_KINDS,
  RULE_ACTIONS,
  emptyAction,
  emptyRule,
  parseRulesText,
  serializeRules,
  type RulesRule,
  type RulesThenAction,
} from "@/lib/rulesDsl";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
  parseError: [message: string];
}>();

const { t } = useI18n();
const rules = ref<RulesRule[]>([]);
const suppress = ref(false);

function loadFromText(text: string) {
  const { rules: parsed, error } = parseRulesText(text);
  if (error) {
    emit("parseError", error);
    return false;
  }
  emit("parseError", "");
  suppress.value = true;
  rules.value = parsed.length ? parsed : [];
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
  rules,
  () => {
    if (suppress.value) return;
    emit("update:modelValue", serializeRules(rules.value));
  },
  { deep: true },
);

function addRule() {
  rules.value.push(emptyRule(rules.value.length + 1));
}

function duplicateRule(i: number) {
  const copy = structuredClone(rules.value[i]);
  copy.id = `${copy.id}-copy`;
  rules.value.splice(i + 1, 0, copy);
}

function removeRule(i: number) {
  rules.value.splice(i, 1);
}

function moveRule(i: number, delta: number) {
  const j = i + delta;
  if (j < 0 || j >= rules.value.length) return;
  const [row] = rules.value.splice(i, 1);
  rules.value.splice(j, 0, row);
}

function addAction(rule: RulesRule) {
  rule.then.push(emptyAction());
}

function removeAction(rule: RulesRule, ai: number) {
  rule.then.splice(ai, 1);
  if (!rule.then.length) rule.then.push(emptyAction());
}

function triBool(v: boolean | null | undefined): string {
  if (v === true) return "true";
  if (v === false) return "false";
  return "";
}

function setTriBool(rule: RulesRule, key: "private" | "complete", raw: string) {
  if (raw === "true") rule.when[key] = true;
  else if (raw === "false") rule.when[key] = false;
  else rule.when[key] = null;
}

function statusInText(rule: RulesRule): string {
  return (rule.when.statusIn ?? []).join(", ");
}

function setStatusIn(rule: RulesRule, raw: string) {
  const parts = raw
    .split(/[,\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
  rule.when.statusIn = parts.length ? parts : undefined;
}

function actionNeeds(a: RulesThenAction, field: string): boolean {
  const act = a.action;
  if (field === "label") return act === "setLabel";
  if (field === "dest") return act === "moveTo";
  if (field === "name") return act === "throttle";
  if (field === "rates") return act === "throttle";
  if (field === "url") return act === "notify";
  if (field === "body") return act === "notify";
  return false;
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
        {{ t("dsl.rules.add") }}
      </button>
      <span
        v-if="!rules.length"
        class="fleet-ext-note"
      >{{ t("dsl.rules.empty") }}</span>
    </div>

    <article
      v-for="(rule, i) in rules"
      :key="i"
      class="fleet-dsl-card"
    >
      <header class="fleet-dsl-card-head">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.id") }}</span>
          <input
            v-model="rule.id"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.priority") }}</span>
          <input
            v-model.number="rule.priority"
            class="fleet-decl-control"
            type="number"
          >
        </label>
        <label class="fleet-decl-field fleet-decl-field-check">
          <input
            v-model="rule.enabled"
            class="fleet-decl-check"
            type="checkbox"
          >
          <span>{{ t("dsl.rules.enabled") }}</span>
        </label>
        <label class="fleet-decl-field fleet-decl-field-check">
          <input
            v-model="rule.stopOnMatch"
            class="fleet-decl-check"
            type="checkbox"
          >
          <span>{{ t("dsl.rules.stopOnMatch") }}</span>
        </label>
        <div class="fleet-dsl-card-actions">
          <button
            type="button"
            class="fleet-btn"
            :disabled="i === 0"
            @click="moveRule(i, -1)"
          >
            ↑
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="i >= rules.length - 1"
            @click="moveRule(i, 1)"
          >
            ↓
          </button>
          <button
            type="button"
            class="fleet-btn"
            @click="duplicateRule(i)"
          >
            {{ t("dsl.duplicate") }}
          </button>
          <button
            type="button"
            class="fleet-btn fleet-btn-danger"
            @click="removeRule(i)"
          >
            {{ t("dsl.remove") }}
          </button>
        </div>
      </header>

      <h4 class="fleet-dsl-section-title">
        {{ t("dsl.rules.when") }}
      </h4>
      <div class="fleet-decl-fields">
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.whenLabel") }}</span>
          <input
            v-model="rule.when.label"
            class="fleet-decl-control"
            type="text"
            placeholder="tv"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.nameRegex") }}</span>
          <input
            v-model="rule.when.nameRegex"
            class="fleet-decl-control"
            type="text"
            placeholder="(?i)1080p"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.labelRegex") }}</span>
          <input
            v-model="rule.when.labelRegex"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.complete") }}</span>
          <select
            class="fleet-decl-control"
            :value="triBool(rule.when.complete)"
            @change="setTriBool(rule, 'complete', ($event.target as HTMLSelectElement).value)"
          >
            <option value="">
              —
            </option>
            <option value="true">
              {{ t("dsl.yes") }}
            </option>
            <option value="false">
              {{ t("dsl.no") }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.private") }}</span>
          <select
            class="fleet-decl-control"
            :value="triBool(rule.when.private)"
            @change="setTriBool(rule, 'private', ($event.target as HTMLSelectElement).value)"
          >
            <option value="">
              —
            </option>
            <option value="true">
              {{ t("dsl.yes") }}
            </option>
            <option value="false">
              {{ t("dsl.no") }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.clientKind") }}</span>
          <select
            v-model="rule.when.clientKind"
            class="fleet-decl-control"
          >
            <option
              v-for="k in CLIENT_KINDS"
              :key="k || 'any'"
              :value="k"
            >
              {{ k || "—" }}
            </option>
          </select>
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.clientId") }}</span>
          <input
            v-model="rule.when.clientId"
            class="fleet-decl-control"
            type="text"
            placeholder="local-rtorrent"
          >
        </label>
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.statusIn") }}</span>
          <input
            class="fleet-decl-control"
            type="text"
            :value="statusInText(rule)"
            placeholder="seeding, downloading"
            @input="setStatusIn(rule, ($event.target as HTMLInputElement).value)"
          >
        </label>
      </div>

      <h4 class="fleet-dsl-section-title">
        {{ t("dsl.rules.then") }}
      </h4>
      <div
        v-for="(action, ai) in rule.then"
        :key="ai"
        class="fleet-dsl-action-row"
      >
        <label class="fleet-decl-field">
          <span class="fleet-decl-label">{{ t("dsl.rules.action") }}</span>
          <select
            v-model="action.action"
            class="fleet-decl-control"
          >
            <option
              v-for="a in RULE_ACTIONS"
              :key="a"
              :value="a"
            >
              {{ a }}
            </option>
          </select>
        </label>
        <label
          v-if="actionNeeds(action, 'label')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">label</span>
          <input
            v-model="action.label"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'dest')"
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">dest</span>
          <input
            v-model="action.dest"
            class="fleet-decl-control"
            type="text"
            placeholder="{label}/{name}"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'name')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">profile</span>
          <input
            v-model="action.name"
            class="fleet-decl-control"
            type="text"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'rates')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">upRate</span>
          <input
            v-model.number="action.upRate"
            class="fleet-decl-control"
            type="number"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'rates')"
          class="fleet-decl-field"
        >
          <span class="fleet-decl-label">downRate</span>
          <input
            v-model.number="action.downRate"
            class="fleet-decl-control"
            type="number"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'url')"
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">url</span>
          <input
            v-model="action.url"
            class="fleet-decl-control"
            type="url"
          >
        </label>
        <label
          v-if="actionNeeds(action, 'body')"
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">body</span>
          <input
            v-model="action.body"
            class="fleet-decl-control"
            type="text"
            placeholder="{name}"
          >
        </label>
        <button
          type="button"
          class="fleet-btn fleet-btn-danger"
          @click="removeAction(rule, ai)"
        >
          {{ t("dsl.remove") }}
        </button>
      </div>
      <button
        type="button"
        class="fleet-btn"
        @click="addAction(rule)"
      >
        {{ t("dsl.rules.addAction") }}
      </button>
    </article>
  </div>
</template>
