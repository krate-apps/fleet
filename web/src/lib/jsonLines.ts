/** Parse JSON-lines (# comments and blank lines ignored). */
export function parseJsonLines<T>(text: string): { items: T[]; error?: string } {
  const items: T[] = [];
  const lines = String(text ?? "").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith("#")) continue;
    try {
      items.push(JSON.parse(line) as T);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      return { items: [], error: `Line ${i + 1}: ${msg}` };
    }
  }
  return { items };
}

/** Serialize objects as one compact JSON object per line. */
export function toJsonLines(items: unknown[]): string {
  if (!items.length) return "";
  return items.map((item) => JSON.stringify(item)).join("\n");
}
