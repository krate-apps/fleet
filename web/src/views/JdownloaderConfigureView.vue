<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import DownloadQueuePanel from "@/components/DownloadQueuePanel.vue";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.jdownloader";
const { t } = useI18n();

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const connectionMode = ref<"local" | "myjd">("local");
const baseUrl = ref("http://127.0.0.1:3128");
const email = ref("");
const password = ref("");
const passwordConfigured = ref(false);
const deviceName = ref("");
const devices = ref<string[]>([]);
const showQueue = ref(false);

const isLocal = computed(() => connectionMode.value === "local");

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.jdownloader.getSettings", {});
  const r = (res.result ?? {}) as {
    connectionMode?: string;
    baseUrl?: string;
    email?: string;
    deviceName?: string;
    passwordConfigured?: boolean;
  };
  connectionMode.value = r.connectionMode === "myjd" ? "myjd" : "local";
  baseUrl.value = r.baseUrl || "http://127.0.0.1:3128";
  email.value = r.email || "";
  deviceName.value = r.deviceName || "";
  passwordConfigured.value = !!r.passwordConfigured;
  password.value = "";
  showQueue.value = connectionMode.value === "local" || passwordConfigured.value;
}

async function saveSettings() {
  busy.value = true;
  try {
    const payload: Record<string, unknown> = {
      enabled: true,
      connectionMode: connectionMode.value,
      baseUrl: baseUrl.value.trim(),
      email: email.value.trim(),
      deviceName: deviceName.value.trim(),
    };
    if (password.value) payload.password = password.value;
    await extensionRPC(EXT, "ext.jdownloader.saveSettings", payload);
    setNotice(t("jdownloader.saved"));
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
    const res = await extensionRPC(EXT, "ext.jdownloader.testConnection", {});
    const r = (res.result ?? {}) as { devices?: string[]; mode?: string };
    devices.value = r.devices ?? [];
    if (r.mode === "local" || connectionMode.value === "local") {
      setNotice(t("jdownloader.testOkLocal"));
    } else {
      setNotice(t("jdownloader.testOk", { n: devices.value.length }));
    }
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
  <div class="jd-cfg">
    <header class="jd-cfg__head">
      <h2>{{ t("jdownloader.title") }}</h2>
      <p class="jd-cfg__hint">
        {{ t("jdownloader.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="jd-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="jd-cfg__card">
      <h3>{{ t("jdownloader.settings") }}</h3>
      <label class="jd-cfg__row">
        {{ t("jdownloader.mode") }}
        <select
          v-model="connectionMode"
          class="fleet-control"
          :disabled="busy"
        >
          <option value="local">
            {{ t("jdownloader.modeLocal") }}
          </option>
          <option value="myjd">
            {{ t("jdownloader.modeMyjd") }}
          </option>
        </select>
      </label>
      <label
        v-if="isLocal"
        class="jd-cfg__row"
      >
        {{ t("jdownloader.baseUrl") }}
        <input
          v-model="baseUrl"
          class="fleet-control"
          type="url"
          :disabled="busy"
        >
        <span class="jd-cfg__help">{{ t("jdownloader.baseUrlHelp") }}</span>
      </label>
      <template v-else>
        <label class="jd-cfg__row">
          {{ t("jdownloader.email") }}
          <input
            v-model="email"
            class="fleet-control"
            type="email"
            autocomplete="username"
            :disabled="busy"
          >
        </label>
        <label class="jd-cfg__row">
          {{ t("jdownloader.password") }}
          <input
            v-model="password"
            class="fleet-control"
            type="password"
            autocomplete="current-password"
            :placeholder="passwordConfigured ? t('jdownloader.passwordKeep') : ''"
            :disabled="busy"
          >
        </label>
        <label class="jd-cfg__row">
          {{ t("jdownloader.device") }}
          <input
            v-model="deviceName"
            class="fleet-control"
            type="text"
            :placeholder="t('jdownloader.devicePlaceholder')"
            :disabled="busy"
          >
        </label>
        <p
          v-if="devices.length"
          class="jd-cfg__devices"
        >
          {{ t("jdownloader.devices") }}: {{ devices.join(", ") }}
        </p>
      </template>
      <div class="jd-cfg__actions">
        <button
          type="button"
          class="fleet-btn fleet-btn-primary"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("jdownloader.save") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="testConnection"
        >
          {{ t("jdownloader.test") }}
        </button>
      </div>
    </section>

    <section
      v-if="showQueue"
      class="jd-cfg__card"
    >
      <h3>{{ t("jdownloader.queue") }}</h3>
      <DownloadQueuePanel
        :extension-id="EXT"
        list-method="ext.jdownloader.listQueue"
        control-method="ext.jdownloader.control"
        :actions="['pause', 'resume', 'remove']"
        @notice="(msg, kind) => setNotice(msg, kind)"
      />
    </section>
  </div>
</template>

<style scoped>
.jd-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 56rem;
}
.jd-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.jd-cfg__hint {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.jd-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--fleet-accent, #3b82f6) 18%, transparent);
}
.jd-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #ef4444 22%, transparent);
}
.jd-cfg__card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.jd-cfg__card h3 {
  margin: 0;
  font-size: 1rem;
}
.jd-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
}
.jd-cfg__help {
  font-size: 0.8rem;
  opacity: 0.7;
}
.jd-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.jd-cfg__devices {
  margin: 0;
  font-size: 0.85rem;
  opacity: 0.8;
}
</style>
