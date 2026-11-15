<script setup lang="ts">
import { useI18n } from "@/i18n/useI18n";
import type { MessageKey } from "@/i18n/messages";

const { t } = useI18n();

const rows: { keys: string; action: MessageKey }[] = [
  { keys: "j / ↓", action: "shortcuts.moveDown" },
  { keys: "k / ↑", action: "shortcuts.moveUp" },
  { keys: "Space", action: "shortcuts.toggleSelect" },
  { keys: "a", action: "shortcuts.add" },
  { keys: "/", action: "shortcuts.search" },
  { keys: "s", action: "shortcuts.start" },
  { keys: "p", action: "shortcuts.pause" },
  { keys: "Delete", action: "shortcuts.remove" },
  { keys: "Esc", action: "shortcuts.escape" },
  { keys: "?", action: "shortcuts.thisHelp" },
];
</script>

<template>
  <div class="fleet-shortcuts-help">
    <p class="fleet-ext-note">
      {{ t("shortcuts.intro") }}
    </p>
    <table class="fleet-shortcuts-table">
      <thead>
        <tr>
          <th>{{ t("shortcuts.keys") }}</th>
          <th>{{ t("shortcuts.action") }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.keys"
        >
          <td><kbd>{{ row.keys }}</kbd></td>
          <td>{{ t(row.action) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
