/** Weekly bandwidth slots — mirrors org.fleet.scheduler worker JSON. */

export type SchedulerSlot = {
  days: number[];
  startHour: number;
  endHour: number;
  /** Bytes/sec; 0 = unlimited. */
  downRate: number;
  /** Bytes/sec; 0 = unlimited. */
  upRate: number;
  throttleProfile?: string;
};

export type SchedulerSchedule = {
  enabled: boolean;
  slots: SchedulerSlot[];
};

/** Go weekday: 0=Sunday … 6=Saturday. */
export const WEEKDAY_IDS = [0, 1, 2, 3, 4, 5, 6] as const;

export function emptySchedulerSlot(): SchedulerSlot {
  return {
    days: [...WEEKDAY_IDS],
    startHour: 0,
    endHour: 24,
    downRate: 0,
    upRate: 0,
  };
}

export function bytesPerSecToKib(bps: number): number {
  if (!Number.isFinite(bps) || bps <= 0) return 0;
  return Math.round(bps / 1024);
}

export function kibToBytesPerSec(kib: number): number {
  if (!Number.isFinite(kib) || kib <= 0) return 0;
  return Math.round(kib * 1024);
}

export function normalizeSlot(raw: Partial<SchedulerSlot> | null | undefined): SchedulerSlot {
  const days = Array.isArray(raw?.days)
    ? [...new Set(raw!.days.map((d) => Number(d)).filter((d) => d >= 0 && d <= 6))].sort(
        (a, b) => a - b,
      )
    : [...WEEKDAY_IDS];
  let startHour = Number(raw?.startHour);
  let endHour = Number(raw?.endHour);
  if (!Number.isFinite(startHour)) startHour = 0;
  if (!Number.isFinite(endHour)) endHour = 24;
  startHour = Math.max(0, Math.min(23, Math.trunc(startHour)));
  endHour = Math.max(0, Math.min(24, Math.trunc(endHour)));
  const downRate = Math.max(0, Math.trunc(Number(raw?.downRate) || 0));
  const upRate = Math.max(0, Math.trunc(Number(raw?.upRate) || 0));
  const throttleProfile = String(raw?.throttleProfile ?? "").trim();
  return {
    days: days.length ? days : [...WEEKDAY_IDS],
    startHour,
    endHour,
    downRate,
    upRate,
    ...(throttleProfile ? { throttleProfile } : {}),
  };
}

export function parseSchedule(raw: unknown): SchedulerSchedule {
  if (!raw || typeof raw !== "object") {
    return { enabled: false, slots: [] };
  }
  const o = raw as Record<string, unknown>;
  const slotsRaw = Array.isArray(o.slots) ? o.slots : [];
  return {
    enabled: Boolean(o.enabled),
    slots: slotsRaw.map((s) => normalizeSlot(s as Partial<SchedulerSlot>)),
  };
}

export function validateSchedule(st: SchedulerSchedule): string | undefined {
  for (let i = 0; i < st.slots.length; i++) {
    const s = st.slots[i];
    if (s.startHour < 0 || s.startHour > 23 || s.endHour < 0 || s.endHour > 24) {
      return `Slot ${i + 1}: invalid hours`;
    }
    if (s.downRate < 0 || s.upRate < 0) {
      return `Slot ${i + 1}: rates must be >= 0`;
    }
    for (const d of s.days) {
      if (d < 0 || d > 6) return `Slot ${i + 1}: invalid day ${d}`;
    }
  }
  return undefined;
}

/** True if hour (0–23) falls inside the slot window (supports overnight; matches worker). */
export function hourInSlot(hour: number, startHour: number, endHour: number): boolean {
  let end = endHour;
  if (end === 0 && startHour !== 0) end = 24;
  if (startHour < end) return hour >= startHour && hour < end;
  if (startHour > end) return hour >= startHour || hour < end;
  return false;
}

/** 7×24 boolean matrix: cell [day][hour] active if any slot covers it. */
export function weekHeatmap(slots: SchedulerSlot[]): boolean[][] {
  const grid = Array.from({ length: 7 }, () => Array.from({ length: 24 }, () => false));
  for (const s of slots) {
    const days = s.days.length ? s.days : [...WEEKDAY_IDS];
    for (const d of days) {
      if (d < 0 || d > 6) continue;
      for (let h = 0; h < 24; h++) {
        if (hourInSlot(h, s.startHour, s.endHour)) grid[d][h] = true;
      }
    }
  }
  return grid;
}

export function toSavePayload(st: SchedulerSchedule): SchedulerSchedule {
  return {
    enabled: Boolean(st.enabled),
    slots: st.slots.map((s) => {
      const n = normalizeSlot(s);
      return {
        days: n.days,
        startHour: n.startHour,
        endHour: n.endHour,
        downRate: n.downRate,
        upRate: n.upRate,
        ...(n.throttleProfile ? { throttleProfile: n.throttleProfile } : {}),
      };
    }),
  };
}
