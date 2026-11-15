export type HookEvent = "added" | "finished";

export type HookDef = {
  event: HookEvent;
  path: string;
  args: string[];
};

export const HOOK_EVENTS: HookEvent[] = ["added", "finished"];
export const DEFAULT_HOOK_ARGS = ["{hash}", "{name}", "{path}"];

export function emptyHook(): HookDef {
  return { event: "finished", path: "", args: [...DEFAULT_HOOK_ARGS] };
}

export function parseHooksText(text: string): { hooks: HookDef[]; error?: string } {
  const hooks: HookDef[] = [];
  const lines = String(text ?? "").split("\n");
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith("#")) continue;
    const parsed = parseHookLine(line);
    if (parsed.error) return { hooks: [], error: `Line ${i + 1}: ${parsed.error}` };
    hooks.push(parsed.hook!);
  }
  return { hooks };
}

function parseHookLine(line: string): { hook?: HookDef; error?: string } {
  const parts = line.split(/\s+/).filter(Boolean);
  let event = "";
  let path = "";
  const args: string[] = [];
  let seeingArgs = false;
  for (const p of parts) {
    if (seeingArgs) {
      args.push(p);
      continue;
    }
    if (p.startsWith("event=")) {
      event = p.slice("event=".length).trim().toLowerCase();
    } else if (p.startsWith("path=")) {
      path = p.slice("path=".length).trim();
    } else if (p === "args" || p.startsWith("args=")) {
      seeingArgs = true;
      const rest = p === "args" ? "" : p.slice("args=".length);
      if (rest) args.push(rest);
    } else {
      return { error: `unexpected token ${JSON.stringify(p)}` };
    }
  }
  if (event !== "added" && event !== "finished") {
    return { error: "event must be added or finished" };
  }
  if (!path.startsWith("/")) {
    return { error: "path must be absolute" };
  }
  return {
    hook: {
      event,
      path,
      args: args.length ? args : [...DEFAULT_HOOK_ARGS],
    },
  };
}

export function serializeHooks(hooks: HookDef[]): string {
  return hooks
    .map((h) => {
      const args = (h.args.length ? h.args : DEFAULT_HOOK_ARGS).join(" ");
      return `event=${h.event} path=${h.path.trim()} args=${args}`;
    })
    .join("\n");
}

export function argsToText(args: string[]): string {
  return args.join(" ");
}

export function textToArgs(raw: string): string[] {
  return raw
    .split(/\s+/)
    .map((s) => s.trim())
    .filter(Boolean);
}
