<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { authSetup, fetchAuthStatus } from "@/api/client";

const router = useRouter();
const username = ref("");
const password = ref("");
const confirm = ref("");
const error = ref("");
const busy = ref(false);

onMounted(async () => {
  try {
    const st = await fetchAuthStatus();
    if (!st.enabled) {
      await router.replace({ name: "torrents" });
      return;
    }
    if (!st.needsSetup) {
      await router.replace({ name: "login" });
    }
  } catch {
    /* ignore */
  }
});

async function submit() {
  if (password.value !== confirm.value) {
    error.value = "Passwords do not match";
    return;
  }
  if (password.value.length < 8) {
    error.value = "Password must be at least 8 characters";
    return;
  }
  busy.value = true;
  error.value = "";
  try {
    await authSetup(username.value.trim(), password.value);
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
      <h1>Create administrator</h1>
      <p class="fleet-ext-note">
        First-run setup for this Unix-user instance. Choose a strong password — nothing is shipped by default.
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
          autocomplete="new-password"
          required
          minlength="8"
        >
      </label>
      <label>
        Confirm password
        <input
          v-model="confirm"
          type="password"
          autocomplete="new-password"
          required
          minlength="8"
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
        Create admin
      </button>
    </form>
  </div>
</template>
