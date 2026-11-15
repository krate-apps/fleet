<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { authLogin, fetchAuthStatus } from "@/api/client";

const router = useRouter();
const username = ref("");
const password = ref("");
const error = ref("");
const busy = ref(false);

onMounted(async () => {
  try {
    const st = await fetchAuthStatus();
    if (st.enabled && st.needsSetup) {
      await router.replace({ name: "setup" });
    }
  } catch {
    /* ignore */
  }
});

async function submit() {
  busy.value = true;
  error.value = "";
  try {
    await authLogin(username.value.trim(), password.value);
    await router.replace({ name: "torrents" });
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fleet-auth-page">
    <form
      class="fleet-auth-card"
      @submit.prevent="submit"
    >
      <p class="fleet-auth-brand">
        Fleet
      </p>
      <h1>Sign in</h1>
      <p class="fleet-ext-note">
        Native authentication for this instance. No default password.
      </p>
      <label>
        Username
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          required
        >
      </label>
      <label>
        Password
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
        >
      </label>
      <p
        v-if="error"
        class="fleet-ext-err"
      >
        {{ error }}
      </p>
      <button
        type="submit"
        class="fleet-btn-primary"
        :disabled="busy"
      >
        Sign in
      </button>
    </form>
  </div>
</template>
