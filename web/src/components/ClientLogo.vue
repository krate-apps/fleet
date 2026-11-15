<script setup lang="ts">
import { computed } from "vue";
import { clientFamily, clientLabel } from "@/lib/clientFamily";
import logoDeluge from "@/assets/clients/deluge.png";
import logoQBittorrent from "@/assets/clients/qbittorrent.png";
import logoRTorrent from "@/assets/clients/rtorrent.png";
import logoTransmission from "@/assets/clients/transmission.png";

const props = defineProps<{
  client?: string | null;
  title?: string;
}>();

const family = computed(() => clientFamily(props.client ?? ""));
const label = computed(() => clientLabel(family.value, props.client ?? "") || props.title || "");
const titleText = computed(() => props.title || label.value);

const logoSrc = computed(() => {
  switch (family.value) {
    case "qbittorrent":
      return logoQBittorrent;
    case "transmission":
      return logoTransmission;
    case "deluge":
      return logoDeluge;
    case "rtorrent":
      return logoRTorrent;
    default:
      return null;
  }
});

const mark = computed(() => {
  switch (family.value) {
    case "utorrent":
      return "µ";
    case "aria2":
      return "a2";
    case "libtorrent":
      return "lt";
    case "bittorrent":
      return "BT";
    case "vuze":
      return "Vz";
    case "tixati":
      return "Tx";
    case "bitcomet":
      return "BC";
    default:
      return props.client?.trim() ? "?" : "—";
  }
});
</script>

<template>
  <span
    class="fleet-client-logo"
    :data-client="family"
    :data-image="logoSrc ? '1' : '0'"
    :title="titleText"
    role="img"
    :aria-label="titleText"
  >
    <img
      v-if="logoSrc"
      class="fleet-client-logo-img"
      :src="logoSrc"
      alt=""
      draggable="false"
    >
    <span
      v-else
      class="fleet-client-logo-mark"
      aria-hidden="true"
    >{{ mark }}</span>
  </span>
</template>
