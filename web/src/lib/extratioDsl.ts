export type ExtratioMatch =
  | "label_contain"
  | "tracker_contain"
  | "private"
  | "public";

export type ExtratioRule = {
  name: string;
  match: ExtratioMatch;
  pattern: string;
  targetLabel: string;
  enabled: boolean;
};

const MATCHES: ExtratioMatch[] = [
  "label_contain",
  "tracker_contain",
  "private",
  "public",
];

export function emptyExtratioRule(): ExtratioRule {
  return {
    name: "",
    match: "label_contain",
    pattern: "",
    targetLabel: "",
    enabled: true,
  };
}

function isMatch(v: string): v is ExtratioMatch {
  return (MATCHES as string[]).includes(v);
}

export function parseExtratioRulesText(text: string): {
  rules: ExtratioRule[];
  error?: string;
} {
  const rules: ExtratioRule[] = [];
  const seen = new Set<string>();
  const lines = String(text ?? "").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith("#")) continue;
    const parts = line.split("|");
    if (parts.length < 4) {
      return {
        rules: [],
        error: `Line ${i + 1}: expected Name|match|pattern|targetLabel[|on]`,
      };
    }
    const name = parts[0].trim();
    const matchRaw = parts[1].trim().toLowerCase();
    const pattern = parts[2];
    const targetLabel = parts[3].trim();
    let enabled = true;
    if (parts.length >= 5) {
      const flag = parts[4].trim().toLowerCase();
      if (["", "on", "1", "true", "yes"].includes(flag)) enabled = true;
      else if (["off", "0", "false", "no"].includes(flag)) enabled = false;
      else {
        return {
          rules: [],
          error: `Line ${i + 1}: enabled flag must be on/off`,
        };
      }
    }
    if (!name || !targetLabel) {
      return {
        rules: [],
        error: `Line ${i + 1}: name and targetLabel required`,
      };
    }
    if (!isMatch(matchRaw)) {
      return {
        rules: [],
        error: `Line ${i + 1}: match must be label_contain|tracker_contain|private|public`,
      };
    }
    if (matchRaw === "tracker_contain" && !pattern.trim()) {
      return {
        rules: [],
        error: `Line ${i + 1}: pattern required for tracker_contain`,
      };
    }
    const key = name.toLowerCase();
    if (seen.has(key)) {
      return { rules: [], error: `Line ${i + 1}: duplicate rule name ${JSON.stringify(name)}` };
    }
    seen.add(key);
    rules.push({
      name,
      match: matchRaw,
      pattern,
      targetLabel,
      enabled,
    });
  }
  return { rules };
}

export function serializeExtratioRules(rules: ExtratioRule[]): string {
  return rules
    .map((r) => {
      const flag = r.enabled ? "on" : "off";
      return `${r.name.trim()}|${r.match}|${r.pattern}|${r.targetLabel.trim()}|${flag}`;
    })
    .join("\n");
}
