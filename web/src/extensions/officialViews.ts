/** Allowlist of official extensions with first-party Vue Configure (OPEN-07). */
export const OFFICIAL_FIRST_PARTY_VIEWS = new Set<string>([
  "org.fleet.autodownload",
  "org.fleet.traffic",
  "org.fleet.websearch",
  "org.fleet.labelbridge",
]);

export function hasOfficialFirstPartyView(extensionId: string): boolean {
  return OFFICIAL_FIRST_PARTY_VIEWS.has(extensionId);
}
