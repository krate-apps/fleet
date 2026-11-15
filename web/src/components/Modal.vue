<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import { useFocusTrap } from "@/composables/useFocusTrap";
import { useI18n } from "@/i18n/useI18n";

defineProps<{
  title: string;
  wide?: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();

const { t } = useI18n();
const panelRef = ref<HTMLElement | null>(null);
const trapActive = ref(true);
useFocusTrap(panelRef, trapActive);

function onKey(ev: KeyboardEvent) {
  if (ev.key === "Escape") {
    ev.preventDefault();
    ev.stopPropagation();
    emit("close");
  }
}

onMounted(() => {
  document.addEventListener("keydown", onKey, true);
  document.body.style.overflow = "hidden";
});

onUnmounted(() => {
  trapActive.value = false;
  document.removeEventListener("keydown", onKey, true);
  document.body.style.overflow = "";
});
</script>

<template>
  <div
    class="fleet-modal-root"
    role="presentation"
    @click.self="emit('close')"
  >
    <div
      ref="panelRef"
      class="fleet-modal"
      :class="{ wide }"
      role="dialog"
      aria-modal="true"
      :aria-label="title"
      @click.stop
    >
      <header class="fleet-modal-head">
        <h2>{{ title }}</h2>
        <div class="fleet-modal-head-end">
          <div
            v-if="$slots.actions"
            class="fleet-modal-head-actions"
          >
            <slot name="actions" />
          </div>
          <button
            type="button"
            class="fleet-icon-btn"
            :aria-label="t('modal.close')"
            @click="emit('close')"
          >
            <ToolbarIcon name="close" />
          </button>
        </div>
      </header>
      <div class="fleet-modal-body">
        <slot />
      </div>
      <footer
        v-if="$slots.footer"
        class="fleet-modal-foot"
      >
        <slot name="footer" />
      </footer>
    </div>
  </div>
</template>
