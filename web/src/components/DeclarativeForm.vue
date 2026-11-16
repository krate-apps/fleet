<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import type { UIForm } from "@/extensions/types";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{
  form: UIForm;
  initialValues?: Record<string, unknown> | null;
}>();

const emit = defineEmits<{
  submit: [values: Record<string, unknown>];
}>();

const { t } = useI18n();
const values = reactive<Record<string, unknown>>({});

function defaultForKind(kind: string): unknown {
  return kind === "boolean" ? false : "";
}

function applyDefaults(form: UIForm) {
  for (const key of Object.keys(values)) {
    delete values[key];
  }
  for (const field of form.fields) {
    values[field.name] = defaultForKind(field.kind);
  }
}

function applyInitial(form: UIForm, initial?: Record<string, unknown> | null) {
  applyDefaults(form);
  if (!initial) return;
  for (const field of form.fields) {
    if (!(field.name in initial)) continue;
    const raw = initial[field.name];
    if (field.kind === "boolean") {
      values[field.name] = Boolean(raw);
    } else if (field.kind === "number") {
      values[field.name] = typeof raw === "number" ? raw : Number(raw) || 0;
    } else {
      values[field.name] = raw == null ? "" : String(raw);
    }
  }
}

watch(
  () => [props.form, props.initialValues] as const,
  ([form, initial]) => {
    applyInitial(form, initial);
  },
  { immediate: true, deep: true },
);

function isWide(kind: string): boolean {
  // Booleans (Enabled, …) always span the full row so paired fields stay aligned on the next line.
  return kind === "text" || kind === "url" || kind === "textarea" || kind === "boolean";
}

/** Default `any` so ratio thresholds and similar fields accept decimals (and integers). */
function numberStep(field: { step?: number | "any" }): number | "any" {
  return field.step ?? "any";
}

function helpLooksLikeExample(help: string): boolean {
  return /[|{]/.test(help) || help.includes("JSON") || help.includes("event=");
}

const essentialFields = computed(() => props.form.fields.filter((f) => !f.advanced));
const advancedFields = computed(() => props.form.fields.filter((f) => f.advanced));

const submitLabel = computed(
  () => props.form.submitLabel?.trim() || t("extensions.apply"),
);

function onSubmit() {
  emit("submit", { ...values, __formId: props.form.id });
}
</script>

<template>
  <form
    class="fleet-decl-form"
    @submit.prevent="onSubmit"
  >
    <header class="fleet-decl-form-head">
      <h3 class="fleet-decl-form-title">
        {{ form.title }}
      </h3>
      <p
        v-if="form.description"
        class="fleet-decl-form-desc"
      >
        {{ form.description }}
      </p>
    </header>

    <div class="fleet-decl-fields">
      <label
        v-for="field in essentialFields"
        :key="field.name"
        class="fleet-decl-field"
        :data-kind="field.kind"
        :data-wide="isWide(field.kind) ? 'true' : 'false'"
      >
        <template v-if="field.kind === 'boolean'">
          <input
            v-model="values[field.name]"
            class="fleet-decl-check"
            type="checkbox"
          >
          <span>{{ field.label }}{{ field.required ? " *" : "" }}</span>
        </template>
        <template v-else>
          <span class="fleet-decl-label">{{ field.label }}{{ field.required ? " *" : "" }}</span>
          <input
            v-if="field.kind === 'number'"
            v-model.number="values[field.name]"
            class="fleet-decl-control"
            type="number"
            :step="numberStep(field)"
            :placeholder="field.placeholder"
            :required="field.required"
          >
          <input
            v-else-if="field.kind === 'url'"
            v-model="values[field.name]"
            class="fleet-decl-control"
            type="url"
            :placeholder="field.placeholder"
            :required="field.required"
          >
          <select
            v-else-if="field.kind === 'select'"
            v-model="values[field.name]"
            class="fleet-decl-control"
            :required="field.required"
          >
            <option
              v-if="!field.required"
              value=""
            >
              —
            </option>
            <option
              v-for="opt in field.options ?? []"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </option>
          </select>
          <textarea
            v-else-if="field.kind === 'textarea'"
            :value="String(values[field.name] ?? '')"
            class="fleet-decl-control fleet-decl-textarea"
            :rows="field.rows ?? 8"
            :placeholder="field.placeholder"
            :required="field.required"
            @input="values[field.name] = ($event.target as HTMLTextAreaElement).value"
          />
          <input
            v-else
            :value="String(values[field.name] ?? '')"
            class="fleet-decl-control"
            type="text"
            :placeholder="field.placeholder"
            :required="field.required"
            @input="values[field.name] = ($event.target as HTMLInputElement).value"
          >
          <span
            v-if="field.help"
            class="fleet-decl-help"
            :data-example="helpLooksLikeExample(field.help) ? 'true' : 'false'"
          >{{ field.help }}</span>
        </template>
      </label>
    </div>

    <details
      v-if="advancedFields.length"
      class="fleet-decl-advanced"
    >
      <summary>{{ t("extensions.moreOptions") }}</summary>
      <div class="fleet-decl-fields">
        <label
          v-for="field in advancedFields"
          :key="field.name"
          class="fleet-decl-field"
          :data-kind="field.kind"
          :data-wide="isWide(field.kind) ? 'true' : 'false'"
        >
          <template v-if="field.kind === 'boolean'">
            <input
              v-model="values[field.name]"
              class="fleet-decl-check"
              type="checkbox"
            >
            <span>{{ field.label }}{{ field.required ? " *" : "" }}</span>
          </template>
          <template v-else>
            <span class="fleet-decl-label">{{ field.label }}{{ field.required ? " *" : "" }}</span>
            <input
              v-if="field.kind === 'number'"
              v-model.number="values[field.name]"
              class="fleet-decl-control"
              type="number"
              :step="numberStep(field)"
              :placeholder="field.placeholder"
              :required="field.required"
            >
            <input
              v-else-if="field.kind === 'url'"
              v-model="values[field.name]"
              class="fleet-decl-control"
              type="url"
              :placeholder="field.placeholder"
              :required="field.required"
            >
            <select
              v-else-if="field.kind === 'select'"
              v-model="values[field.name]"
              class="fleet-decl-control"
              :required="field.required"
            >
              <option
                v-if="!field.required"
                value=""
              >
                —
              </option>
              <option
                v-for="opt in field.options ?? []"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </option>
            </select>
            <textarea
              v-else-if="field.kind === 'textarea'"
              :value="String(values[field.name] ?? '')"
              class="fleet-decl-control fleet-decl-textarea"
              :rows="field.rows ?? 8"
              :placeholder="field.placeholder"
              :required="field.required"
              @input="values[field.name] = ($event.target as HTMLTextAreaElement).value"
            />
            <input
              v-else
              :value="String(values[field.name] ?? '')"
              class="fleet-decl-control"
              type="text"
              :placeholder="field.placeholder"
              :required="field.required"
              @input="values[field.name] = ($event.target as HTMLInputElement).value"
            >
            <span
              v-if="field.help"
              class="fleet-decl-help"
              :data-example="helpLooksLikeExample(field.help) ? 'true' : 'false'"
            >{{ field.help }}</span>
          </template>
        </label>
      </div>
    </details>

    <!-- Optional chrome between fields and Apply (e.g. Editor / Expert DSL). -->
    <slot name="before-submit" />

    <button
      type="submit"
      class="fleet-btn-primary fleet-form-apply"
    >
      {{ submitLabel }}
    </button>
  </form>
</template>
