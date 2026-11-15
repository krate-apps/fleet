<script setup lang="ts" generic="T">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    items: T[];
    rowHeight?: number;
    overscan?: number;
    getKey: (item: T, index: number) => string | number;
  }>(),
  {
    rowHeight: 44,
    overscan: 8,
  },
);

const scroller = ref<HTMLElement | null>(null);
const scrollTop = ref(0);
const viewportH = ref(600);

const totalHeight = computed(() => props.items.length * props.rowHeight);
const startIndex = computed(() =>
  Math.max(0, Math.floor(scrollTop.value / props.rowHeight) - props.overscan),
);
const endIndex = computed(() =>
  Math.min(
    props.items.length,
    Math.ceil((scrollTop.value + viewportH.value) / props.rowHeight) + props.overscan,
  ),
);
const visible = computed(() => {
  const items = props.items;
  const start = startIndex.value;
  const end = endIndex.value;
  const out = new Array(Math.max(0, end - start));
  for (let i = start; i < end; i++) {
    out[i - start] = {
      item: items[i],
      index: i,
      top: i * props.rowHeight,
      // Include index so reorder always remounts rows in visual order.
      key: `${props.getKey(items[i] as T, i)}:${i}`,
    };
  }
  return out;
});

function onScroll() {
  if (!scroller.value) return;
  scrollTop.value = scroller.value.scrollTop;
}

function measure() {
  if (!scroller.value) return;
  viewportH.value = scroller.value.clientHeight || 600;
}

let ro: ResizeObserver | null = null;
onMounted(() => {
  measure();
  ro = new ResizeObserver(() => measure());
  if (scroller.value) ro.observe(scroller.value);
});
onBeforeUnmount(() => ro?.disconnect());
watch(
  () => props.items.length,
  () => {
    // Keep scroll within bounds after filter changes.
    if (!scroller.value) return;
    const max = Math.max(0, totalHeight.value - scroller.value.clientHeight);
    if (scroller.value.scrollTop > max) scroller.value.scrollTop = max;
  },
);
</script>

<template>
  <div
    ref="scroller"
    class="fleet-vtable"
    @scroll="onScroll"
  >
    <div
      class="fleet-vtable-space"
      :style="{ height: `${totalHeight}px` }"
    >
      <div
        v-for="row in visible"
        :key="row.key"
        class="fleet-vtable-row"
        :style="{ top: `${row.top}px`, height: `${rowHeight}px` }"
      >
        <slot
          name="row"
          :item="row.item"
          :index="row.index"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.fleet-vtable {
  position: relative;
  overflow: auto;
  min-height: 0;
  height: 100%;
}

.fleet-vtable-space {
  position: relative;
  width: 100%;
  min-width: max(100%, var(--fleet-table-w, 100%));
}

.fleet-vtable-row {
  position: absolute;
  left: 0;
  right: 0;
  width: 100%;
  min-width: var(--fleet-table-w, 100%);
  display: flex;
  align-items: stretch;
}
</style>
