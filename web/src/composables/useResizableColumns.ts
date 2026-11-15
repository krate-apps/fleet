import { onBeforeUnmount, reactive } from "vue";

export type ColumnSpec = {
  id: string;
  label: string;
  width: number;
  minWidth?: number;
  sortable?: boolean;
  /** When true, column is omitted from header/body (mobile defaults). */
  hidden?: boolean;
};

/**
 * Draggable column widths shared by sticky header + virtualized body colgroups.
 */
export function useResizableColumns(initial: ColumnSpec[]) {
  const columns = reactive(
    initial.map((c) => ({ ...c, minWidth: c.minWidth ?? 48, hidden: Boolean(c.hidden) })),
  );

  let drag: { index: number; startX: number; startW: number } | null = null;

  function onMove(ev: MouseEvent) {
    if (!drag) return;
    const col = columns[drag.index];
    if (!col) return;
    const next = Math.max(col.minWidth ?? 48, drag.startW + (ev.clientX - drag.startX));
    col.width = Math.round(next);
  }

  function onUp() {
    drag = null;
    window.removeEventListener("mousemove", onMove);
    window.removeEventListener("mouseup", onUp);
  }

  function startResize(index: number, ev: MouseEvent) {
    ev.preventDefault();
    ev.stopPropagation();
    const col = columns[index];
    if (!col) return;
    drag = { index, startX: ev.clientX, startW: col.width };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  onBeforeUnmount(() => {
    window.removeEventListener("mousemove", onMove);
    window.removeEventListener("mouseup", onUp);
  });

  return { columns, startResize };
}
