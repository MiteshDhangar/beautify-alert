import { CodeEditorSuggestionItem, CodeEditorSuggestionItemKind } from '@grafana/ui';

/** A template snippet users can insert. */
export interface Snippet {
  label: string;
  insert: string;
  detail: string;
}

export const SNIPPETS: Snippet[] = [
  { label: 'Alert name', insert: '{{ .CommonLabels.alertname }}', detail: 'Name shared by the alerts in this email' },
  { label: 'Status', insert: '{{ .Status }}', detail: 'firing or resolved' },
  { label: 'Firing count', insert: '{{ len .Alerts.Firing }}', detail: 'Number of firing alerts' },
  { label: 'Loop over alerts', insert: '{{ range .Alerts }}\n  \n{{ end }}', detail: 'Repeat a block for each alert' },
  { label: 'Summary', insert: '{{ .Annotations.summary }}', detail: 'Inside a loop: the summary annotation' },
  {
    label: 'Description',
    insert: '{{ .Annotations.description }}',
    detail: 'Inside a loop: the description annotation',
  },
  { label: 'Severity', insert: '{{ .Labels.severity }}', detail: 'Inside a loop: the severity label' },
  { label: 'Status color', insert: '{{ statusColor .Status }}', detail: 'Red when firing, green when resolved' },
  {
    label: 'Severity color',
    insert: '{{ severityColor .Labels.severity }}',
    detail: 'Color for critical, warning, info, ...',
  },
  {
    label: 'Start time',
    insert: '{{ formatTime .StartsAt "02 Jan 2006 15:04 MST" }}',
    detail: 'Add a time zone: ... "MST" "Europe/Berlin"',
  },
  { label: 'Started ago', insert: '{{ since .StartsAt }}', detail: 'For example 12m or 3h5m' },
  {
    label: 'Values',
    insert: '{{ range $ref, $v := .Values }}{{ $ref }}: {{ humanize $v }} {{ end }}',
    detail: 'Inside a loop: query values',
  },
  {
    label: 'All labels',
    insert: '{{ range .Labels.SortedPairs }}{{ .Name }}={{ .Value }} {{ end }}',
    detail: 'Inside a loop: every label',
  },
  { label: 'Dashboard link', insert: '{{ .DashboardURL }}', detail: 'Inside a loop: link to the dashboard' },
  { label: 'Rule link', insert: '{{ .GeneratorURL }}', detail: 'Inside a loop: link to the alert rule' },
  { label: 'Silence link', insert: '{{ .SilenceURL }}', detail: 'Inside a loop: link to silence the alert' },
  { label: 'Grafana link', insert: '{{ .ExternalURL }}', detail: 'Link to your Grafana' },
  {
    label: 'If firing',
    insert: '{{ if eq .Status "firing" }}\n  \n{{ else }}\n  \n{{ end }}',
    detail: 'Different content when resolved',
  },
];

/** Completions offered by the code editor. */
export function suggestions(): CodeEditorSuggestionItem[] {
  return SNIPPETS.map((s) => ({
    label: s.insert.split('\n')[0],
    insertText: s.insert,
    detail: s.label,
    documentation: s.detail,
    kind: CodeEditorSuggestionItemKind.Field,
  }));
}
