import { computed, ref, watch } from "vue";

export type UiDensity = "compact" | "comfort";

const STORAGE_KEY = "fleet.uiDensity.v1";

function readStored(): UiDensity {
  if (typeof localStorage === "undefined") return "comfort";
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw === "compact" || raw === "comfort" ? raw : "comfort";
  } catch {
    return "comfort";
  }
}

function applyDom(d: UiDensity) {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.fleetDensity = d;
}

const density = ref<UiDensity>(readStored());
applyDom(density.value);

watch(density, (d) => {
  applyDom(d);
  try {
    localStorage.setItem(STORAGE_KEY, d);
  } catch {
    // ignore
  }
});

export function useUiDensity() {
  function setDensity(next: UiDensity) {
    density.value = next;
  }

  const rowHeightPx = computed(() => (density.value === "compact" ? 40 : 52));

  return {
    density,
    setDensity,
    rowHeightPx,
  };
}
