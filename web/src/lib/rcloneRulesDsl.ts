export type RcloneRule = {
  action: "copy" | "move";
  remote: string;
  dest: string;
  label: string;
};

export function emptyRcloneRule(): RcloneRule {
  return { action: "copy", remote: "", dest: "{name}", label: "" };
}

export function parseRcloneRulesText(text: string): { rules: RcloneRule[]; error?: string } {
  const rules: RcloneRule[] = [];
  const lines = String(text ?? "").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith("#")) continue;
    const r: RcloneRule = { action: "copy", remote: "", dest: "{name}", label: "" };
    for (const part of line.split(/\s+/).filter(Boolean)) {
      const eq = part.indexOf("=");
      if (eq <= 0) return { rules: [], error: `Line ${i + 1}: expected key=value tokens` };
      const k = part.slice(0, eq).trim().toLowerCase();
      const v = part.slice(eq + 1).trim();
      if (k === "label") r.label = v;
      else if (k === "action") r.action = v.toLowerCase() === "move" ? "move" : "copy";
      else if (k === "remote") r.remote = v;
      else if (k === "dest") r.dest = v || "{name}";
      else return { rules: [], error: `Line ${i + 1}: unknown key ${JSON.stringify(k)}` };
    }
    if (!r.remote) return { rules: [], error: `Line ${i + 1}: remote required` };
    if (r.action !== "copy" && r.action !== "move") {
      return { rules: [], error: `Line ${i + 1}: action must be copy or move` };
    }
    rules.push(r);
  }
  return { rules };
}

export function serializeRcloneRules(rules: RcloneRule[]): string {
  return rules
    .map((r) => {
      let line = `action=${r.action} remote=${r.remote.trim()} dest=${(r.dest || "{name}").trim()}`;
      if (r.label.trim()) line += ` label=${r.label.trim()}`;
      return line;
    })
    .join("\n");
}
