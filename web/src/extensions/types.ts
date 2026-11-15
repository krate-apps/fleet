/** Declarative UI contribution types mirrored from docs/developer/schemas/ui-contribution.schema.json */

export type UIColumn = {
  id: string;
  label: string;
  field: string;
  width?: number;
};

export type UIAction = {
  id: string;
  label: string;
  target: "torrent" | "selection" | "global";
  confirm?: boolean;
  primary?: boolean;
  danger?: boolean;
};

export type UIPage = {
  id: string;
  title: string;
  route: string;
};

export type UIPanel = {
  id: string;
  title: string;
  placement: "torrent-details" | "sidebar" | "settings";
};

export type UIFormFieldOption = {
  value: string;
  label: string;
};

export type UIFormField = {
  name: string;
  label: string;
  kind: "text" | "number" | "boolean" | "select" | "url" | "textarea";
  required?: boolean;
  help?: string;
  placeholder?: string;
  advanced?: boolean;
  rows?: number;
  options?: UIFormFieldOption[];
};

export type UIForm = {
  id: string;
  title: string;
  description?: string;
  submitLabel?: string;
  fields: UIFormField[];
  loadMethod?: string;
  submitMethod?: string;
};

export type UIContribution = {
  schema: number;
  extensionId: string;
  columns?: UIColumn[];
  actions?: UIAction[];
  pages?: UIPage[];
  panels?: UIPanel[];
  forms?: UIForm[];
};

export type ExtensionRecord = {
  id: string;
  name: string;
  description?: string;
  version: string;
  path: string;
  state: "discovered" | "inactive" | "active" | "error";
  error?: string;
  ui?: UIContribution;
  updatedAt: string;
  official?: boolean;
  consentRequired?: boolean;
  consented?: boolean;
  consentWarning?: string;
};
