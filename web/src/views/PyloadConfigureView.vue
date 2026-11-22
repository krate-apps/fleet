<script setup lang="ts">
import { onMounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import DownloadQueuePanel from "@/components/DownloadQueuePanel.vue";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.pyload";
const { t } = useI18n();

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const baseUrl = ref("http://127.0.0.1:8000");
const username = ref("");
const password = ref("");
const passwordConfigured = ref(false);
const showQueue = ref(false);

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.pyload.getSettings", {});
  const r = (res.result ?? {}) as {
    baseUrl?: string;
    username?: string;
    passwordConfigured?: boolean;
  };
  baseUrl.value = r.baseUrl || "http://127.0.0.1:8000";
  username.value = r.username || "";
  passwordConfigured.value = !!r.passwordConfigured;
  password.value = "";
  showQueue.value = passwordConfigured.value;
}

async function saveSettings() {
  busy.value = true;
  try {
    const payload: Record<string, unknown> = {
      enabled: true,
      baseUrl: baseUrl.value.trim(),
      username: username.value.trim(),
    };
    if (password.value) payload.password = password.value;
    await extensionRPC(EXT, "ext.pyload.saveSettings", payload);
    setNotice(t("pyload.saved"));
    password.value = "";
    await loadSettings();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function testConnection() {
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.pyload.testConnection", {});
    setNotice(t("pyload.testOk"));
    showQueue.value = true;
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

onMounted(async () => {
  busy.value = true;
  try {
    await loadSettings();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
});
</script>

<template>
  <div class="py-cfg">
    <header class="py-cfg__head">
      <h2>{{ t("pyload.title") }}</h2>
      <p class="py-cfg__hint">
        {{ t("pyload.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="py-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="py-cfg__card">
      <h3>{{ t("pyload.settings") }}</h3>
      <label class="py-cfg__row">
        {{ t("pyload.baseUrl") }}
        <input
          v-model="baseUrl"
          class="fleet-control"
          type="url"
          :disabled="busy"
        >
        <span class="py-cfg__help">{{ t("pyload.baseUrlHelp") }}</span>
      </label>
      <label class="py-cfg__row">
        {{ t("pyload.username") }}
        <input
          v-model="username"
          class="fleet-control"
          type="text"
          autocomplete="username"
          :disabled="busy"
        >
      </label>
      <label class="py-cfg__row">
        {{ t("pyload.password") }}
        <input
          v-model="password"
          class="fleet-control"
          type="password"
          autocomplete="current-password"
          :placeholder="passwordConfigured ? t('pyload.passwordKeep') : ''"
          :disabled="busy"
        >
      </label>
      <div class="py-cfg__actions">
        <button
          type="button"
          class="fleet-btn fleet-btn-primary"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("pyload.save") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="testConnection"
        >
          {{ t("pyload.test") }}
        </button>
      </div>
    </section>

    <section
      v-if="showQueue"
      class="py-cfg__card"
    >
      <h3>{{ t("pyload.queue") }}</h3>
      <DownloadQueuePanel
        :extension-id="EXT"
        list-method="ext.pyload.listQueue"
        control-method="ext.pyload.control"
        :actions="['pause', 'resume', 'abort', 'restart', 'delete']"
        @notice="(msg, kind) => setNotice(msg, kind)"
      />
    </section>
  </div>
</template>

<style scoped>
.py-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 56rem;
}
.py-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.py-cfg__hint {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.py-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--fleet-accent, #3b82f6) 18%, transparent);
}
.py-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #ef4444 22%, transparent);
}
.py-cfg__card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.py-cfg__card h3 {
  margin: 0;
  font-size: 1rem;
}
.py-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
}
.py-cfg__help {
  font-size: 0.8rem;
  opacity: 0.7;
}
.py-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
</style>
