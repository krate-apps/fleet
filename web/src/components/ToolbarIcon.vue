<script setup lang="ts">
defineProps<{
  name:
    | "menu"
    | "filters"
    | "play"
    | "pause"
    | "stop"
    | "hash"
    | "remove"
    | "extensions"
    | "settings"
    | "files"
    | "close"
    | "github";
}>();
</script>

<template>
  <svg
    class="fleet-toolbar-icon"
    viewBox="0 0 24 24"
    width="18"
    height="18"
    aria-hidden="true"
    :fill="name === 'github' ? 'currentColor' : 'none'"
    :stroke="name === 'github' ? 'none' : 'currentColor'"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
  >
    <!-- Sidebar toggle -->
    <template v-if="name === 'menu'">
      <path d="M4 7h16M4 12h16M4 17h16" />
    </template>

    <!-- Funnel = filters / views -->
    <template v-else-if="name === 'filters'">
      <path d="M4 5h16l-6 7.5V19l-4 2v-8.5L4 5z" />
    </template>

    <!-- Start -->
    <template v-else-if="name === 'play'">
      <path
        d="M8 5.5v13l11-6.5L8 5.5z"
        fill="currentColor"
        stroke="none"
      />
    </template>

    <!-- Pause -->
    <template v-else-if="name === 'pause'">
      <path
        d="M7 5h3.5v14H7V5zm6.5 0H17v14h-3.5V5z"
        fill="currentColor"
        stroke="none"
      />
    </template>

    <!-- Stop -->
    <template v-else-if="name === 'stop'">
      <path
        d="M6.5 6.5h11v11h-11z"
        fill="currentColor"
        stroke="none"
      />
    </template>

    <!-- Recheck / hash-check (not a # glyph) -->
    <template v-else-if="name === 'hash'">
      <path d="M4 12a8 8 0 0 1 13.5-5.8" />
      <path d="M20 12a8 8 0 0 1-13.5 5.8" />
      <path d="M17 3.5v4.5h-4.5" />
      <path d="M7 20.5V16H11.5" />
    </template>

    <!-- Remove torrent (trash) -->
    <template v-else-if="name === 'remove'">
      <path d="M4 7h16" />
      <path d="M9 7V5h6v2" />
      <path d="M7 7l1 13h8l1-13" />
      <path d="M10 11v6M14 11v6" />
    </template>

    <!-- Extensions (puzzle piece) -->
    <template v-else-if="name === 'extensions'">
      <path d="M12 3v3.5a1.5 1.5 0 1 0 0 3V12h2.5a1.5 1.5 0 1 1 3 0H21v9H3V12h3.5a1.5 1.5 0 1 0 0-3H3V3h9z" />
    </template>

    <!-- Settings gear -->
    <template v-else-if="name === 'settings'">
      <circle
        cx="12"
        cy="12"
        r="3"
      />
      <path
        d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09A1.65 1.65 0 0 0 15 4.6a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
      />
    </template>

    <!-- Files / folder -->
    <template v-else-if="name === 'files'">
      <path d="M3 7.5A1.5 1.5 0 0 1 4.5 6H9l2 2h8.5A1.5 1.5 0 0 1 21 9.5v8A1.5 1.5 0 0 1 19.5 19h-15A1.5 1.5 0 0 1 3 17.5v-10z" />
    </template>

    <!-- Close / dismiss -->
    <template v-else-if="name === 'close'">
      <path d="M6 6l12 12M18 6L6 18" />
    </template>

    <!-- GitHub mark -->
    <template v-else-if="name === 'github'">
      <path
        d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844a9.59 9.59 0 0 1 2.504.337c1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.02 10.02 0 0 0 22 12.017C22 6.484 17.522 2 12 2z"
      />
    </template>
  </svg>
</template>
