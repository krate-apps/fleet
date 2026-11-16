<script setup lang="ts">
import ExtratioRulesBuilder from "@/components/ExtratioRulesBuilder.vue";
import HooksDslBuilder from "@/components/HooksDslBuilder.vue";
import NotifyChannelsBuilder from "@/components/NotifyChannelsBuilder.vue";
import RcloneRulesBuilder from "@/components/RcloneRulesBuilder.vue";
import RulesDslBuilder from "@/components/RulesDslBuilder.vue";
import { useI18n } from "@/i18n/useI18n";

export type DslKind = "rules" | "notify" | "hooks" | "rclone" | "extratio";

defineProps<{
  kind: DslKind;
  mode: "builder" | "expert";
  parseError: string;
  expertLabel: string;
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
  "parse-error": [msg: string];
  "set-mode": [mode: "builder" | "expert"];
}>();

const { t } = useI18n();
</script>

<template>
  <div class="fleet-dsl-chrome">
    <div
      class="fleet-dsl-mode"
      role="group"
      :aria-label="t('dsl.modeLabel')"
    >
      <button
        type="button"
        class="fleet-btn"
        :aria-pressed="mode === 'builder'"
        @click="emit('set-mode', 'builder')"
      >
        {{ t("dsl.modeBuilder") }}
      </button>
      <button
        type="button"
        class="fleet-btn"
        :aria-pressed="mode === 'expert'"
        @click="emit('set-mode', 'expert')"
      >
        {{ t("dsl.modeExpert") }}
      </button>
    </div>
    <p
      v-if="parseError"
      class="fleet-dsl-parse-err"
    >
      {{ t("dsl.parseError") }}: {{ parseError }}
    </p>
    <RulesDslBuilder
      v-if="kind === 'rules' && mode === 'builder'"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      @parse-error="emit('parse-error', $event)"
    />
    <NotifyChannelsBuilder
      v-if="kind === 'notify' && mode === 'builder'"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      @parse-error="emit('parse-error', $event)"
    />
    <HooksDslBuilder
      v-if="kind === 'hooks' && mode === 'builder'"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      @parse-error="emit('parse-error', $event)"
    />
    <RcloneRulesBuilder
      v-if="kind === 'rclone' && mode === 'builder'"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      @parse-error="emit('parse-error', $event)"
    />
    <ExtratioRulesBuilder
      v-if="kind === 'extratio' && mode === 'builder'"
      :model-value="modelValue"
      @update:model-value="emit('update:modelValue', $event)"
      @parse-error="emit('parse-error', $event)"
    />
    <label
      v-if="mode === 'expert'"
      class="fleet-decl-field"
      data-wide="true"
    >
      <span class="fleet-decl-label">{{ expertLabel }}</span>
      <textarea
        class="fleet-decl-control fleet-decl-textarea"
        :rows="kind === 'hooks' ? 8 : 12"
        :value="modelValue"
        @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
      />
    </label>
  </div>
</template>
