import React, { useEffect, useMemo, useRef, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import {
  Alert,
  Button,
  ClipboardButton,
  CodeEditor,
  ConfirmModal,
  Dropdown,
  Field,
  Input,
  Menu,
  RadioButtonGroup,
  Stack,
  Switch,
  Text,
  useStyles2,
} from '@grafana/ui';
import { api, RequestError } from '../../api';
import { EmailTemplate, Rendered, Settings } from '../../types';
import { EmailPreview } from '../../components/EmailPreview';
import { SNIPPETS, suggestions } from '../../components/snippets';

type MonacoEditor = Parameters<NonNullable<React.ComponentProps<typeof CodeEditor>['onEditorDidMount']>>[0];

interface Props {
  template: EmailTemplate;
  /** True when the template is the built-in default that has not been saved yet. */
  builtin: boolean;
  settings: Settings;
  canSave: boolean;
  nameTaken: (name: string) => boolean;
  onSave: (t: EmailTemplate) => Promise<void>;
  onDelete: () => Promise<void>;
  onDuplicate: (t: EmailTemplate) => void;
  onDirtyChange: (dirty: boolean) => void;
}

const NAME_RE = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export function TemplateEditor(props: Props) {
  const { template, builtin, settings, canSave, nameTaken, onSave, onDelete, onDuplicate, onDirtyChange } = props;
  const s = useStyles2(getStyles);
  const [draft, setDraft] = useState<EmailTemplate>(template);
  const [status, setStatus] = useState<'firing' | 'resolved'>('firing');
  const [rendered, setRendered] = useState<Rendered>();
  const [renderError, setRenderError] = useState<string>();
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ severity: 'success' | 'error'; text: string }>();
  const [testTo, setTestTo] = useState('');
  const [testing, setTesting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const editorRef = useRef<MonacoEditor | null>(null);

  const dirty = builtin || JSON.stringify(draft) !== JSON.stringify(template);
  useEffect(() => onDirtyChange(dirty && !builtin), [dirty, builtin, onDirtyChange]);

  // Live preview, debounced.
  useEffect(() => {
    const handle = setTimeout(async () => {
      try {
        setRendered(await api.preview(draft, status));
        setRenderError(undefined);
      } catch (e) {
        setRenderError((e as Error).message);
      }
    }, 300);
    return () => clearTimeout(handle);
  }, [draft, status]);

  const nameError = !NAME_RE.test(draft.name)
    ? 'Use 1 to 64 lowercase letters, digits, - or _'
    : draft.name !== template.name && nameTaken(draft.name)
      ? 'Another template has this name'
      : undefined;

  const recipients = draft.recipients || settings.defaultRecipients;
  const label = `${settings.routing.templateLabel}=${draft.name}`;

  const update = (patch: Partial<EmailTemplate>) => setDraft((d) => ({ ...d, ...patch }));

  const insert = (text: string) => {
    const ed = editorRef.current;
    if (!ed) {
      update({ html: draft.html + text });
      return;
    }
    const sel = ed.getSelection();
    if (sel) {
      ed.executeEdits('beautify-alert', [{ range: sel, text, forceMoveMarkers: true }]);
    }
    ed.focus();
  };

  const save = async () => {
    setSaving(true);
    setNotice(undefined);
    try {
      await onSave(draft);
      setNotice({ severity: 'success', text: 'Template saved. Alerts with this label use it from now on.' });
    } catch (e) {
      setNotice({ severity: 'error', text: (e as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const sendTest = async () => {
    setTesting(true);
    setNotice(undefined);
    try {
      const entry = await api.test(draft, testTo);
      setNotice({ severity: 'success', text: `Test email sent to ${entry.to.join(', ')}.` });
    } catch (e) {
      const err = e as RequestError;
      setNotice({ severity: 'error', text: `Test email failed: ${err.message}` });
    } finally {
      setTesting(false);
    }
  };

  const insertMenu = (
    <Menu>
      {SNIPPETS.map((sn) => (
        <Menu.Item key={sn.label} label={sn.label} description={sn.detail} onClick={() => insert(sn.insert)} />
      ))}
    </Menu>
  );

  return (
    <div className={s.root}>
      {builtin && (
        <Alert severity="info" title="Built-in default template">
          Alerts without the <code>{settings.routing.templateLabel}</code> label use this template. Save it to customize
          it.
        </Alert>
      )}
      <div className={s.fields}>
        <Field
          label="Name"
          description="Alerts pick this template with a label"
          invalid={!!nameError}
          error={nameError}
        >
          <Input
            value={draft.name}
            disabled={builtin || template.name === 'default'}
            onChange={(e) => update({ name: e.currentTarget.value.trim() })}
            data-testid="template-name"
          />
        </Field>
        <Field label="Use it on an alert rule" description="Add this label to the rule">
          <Stack alignItems="center">
            <code className={s.label}>{label}</code>
            <ClipboardButton icon="copy" variant="secondary" size="sm" getText={() => label}>
              Copy
            </ClipboardButton>
          </Stack>
        </Field>
        <Field label="Description">
          <Input
            value={draft.description ?? ''}
            placeholder="Who gets this and why"
            onChange={(e) => update({ description: e.currentTarget.value })}
          />
        </Field>
        <Field
          label="Recipients"
          description={`Used when an alert has no ${settings.routing.recipientsLabel} label. Empty: the default recipients.`}
        >
          <Input
            value={draft.recipients ?? ''}
            placeholder={settings.defaultRecipients || 'team@example.com, oncall@example.com'}
            onChange={(e) => update({ recipients: e.currentTarget.value })}
          />
        </Field>
      </div>
      <Field label="Subject" description="Go template, one line">
        <Input
          className={s.mono}
          value={draft.subject}
          onChange={(e) => update({ subject: e.currentTarget.value })}
          data-testid="template-subject"
        />
      </Field>

      <div className={s.split}>
        <div className={s.pane}>
          <Stack justifyContent="space-between" alignItems="center">
            <Text weight="medium">HTML</Text>
            <Stack alignItems="center">
              <Stack alignItems="center" gap={1}>
                <Switch
                  value={draft.inlineCss !== false}
                  onChange={(e) => update({ inlineCss: e.currentTarget.checked })}
                  id="inline-css"
                />
                <label htmlFor="inline-css">
                  <Text variant="bodySmall" color="secondary">
                    Inline CSS for email clients
                  </Text>
                </label>
              </Stack>
              <Dropdown overlay={insertMenu} placement="bottom-end">
                <Button size="sm" variant="secondary" icon="plus" fill="outline">
                  Insert field
                </Button>
              </Dropdown>
            </Stack>
          </Stack>
          <CodeEditor
            value={draft.html}
            language="html"
            height={560}
            showMiniMap={false}
            showLineNumbers
            wordWrap
            onChange={(html) => update({ html })}
            onEditorDidMount={(ed) => {
              editorRef.current = ed;
            }}
            getSuggestions={suggestions}
          />
        </div>
        <div className={s.pane}>
          <Stack justifyContent="space-between" alignItems="center">
            <Text weight="medium">Preview with sample alerts</Text>
            <RadioButtonGroup
              size="sm"
              value={status}
              onChange={(v) => setStatus(v as 'firing' | 'resolved')}
              options={[
                { label: 'Firing', value: 'firing' },
                { label: 'Resolved', value: 'resolved' },
              ]}
            />
          </Stack>
          <EmailPreview
            subject={rendered?.subject}
            html={rendered?.html}
            from={settings.delivery.fromAddress}
            to={recipients}
            error={renderError}
          />
        </div>
      </div>

      {notice && <Alert severity={notice.severity} title={notice.text} onRemove={() => setNotice(undefined)} />}

      <div className={s.actions}>
        <Stack alignItems="center" wrap="wrap">
          <Button
            onClick={save}
            disabled={!canSave || saving || !!nameError || !dirty}
            icon={saving ? 'spinner' : 'save'}
            data-testid="template-save"
          >
            {builtin ? 'Save as my default' : 'Save template'}
          </Button>
          <Input
            width={32}
            value={testTo}
            placeholder={recipients || 'you@example.com'}
            onChange={(e) => setTestTo(e.currentTarget.value)}
            aria-label="Test recipients"
          />
          <Button
            variant="secondary"
            onClick={sendTest}
            disabled={testing || !!renderError}
            icon={testing ? 'spinner' : 'envelope'}
          >
            Send test email
          </Button>
        </Stack>
        <Stack>
          {!builtin && (
            <Button variant="secondary" fill="text" icon="copy" onClick={() => onDuplicate(draft)}>
              Duplicate
            </Button>
          )}
          {!builtin && (
            <Button
              variant="destructive"
              fill="text"
              icon="trash-alt"
              disabled={!canSave}
              onClick={() => setConfirmDelete(true)}
            >
              Delete
            </Button>
          )}
        </Stack>
      </div>
      {!canSave && (
        <Text variant="bodySmall" color="secondary">
          Only Grafana admins can save templates. You can still preview and send test emails.
        </Text>
      )}
      <ConfirmModal
        isOpen={confirmDelete}
        title={`Delete ${template.name}?`}
        body={`Alerts labeled ${settings.routing.templateLabel}=${template.name} will use the default template instead.`}
        confirmText="Delete"
        onConfirm={async () => {
          setConfirmDelete(false);
          await onDelete();
        }}
        onDismiss={() => setConfirmDelete(false)}
      />
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2), minWidth: 0 }),
  fields: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
    columnGap: theme.spacing(2),
    rowGap: theme.spacing(1),
    '& > div': { marginBottom: 0 },
  }),
  split: css({
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
    gap: theme.spacing(2),
    [theme.breakpoints.down('xl')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  pane: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), minWidth: 0 }),
  actions: css({ display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: theme.spacing(1) }),
  label: css({
    fontFamily: theme.typography.fontFamilyMonospace,
    padding: theme.spacing(0.5, 1),
    background: theme.colors.background.secondary,
    borderRadius: theme.shape.radius.default,
  }),
  mono: css({ input: { fontFamily: theme.typography.fontFamilyMonospace } }),
});

export function useNameTaken(templates: EmailTemplate[]) {
  return useMemo(() => {
    const names = new Set(templates.map((t) => t.name));
    return (name: string) => names.has(name);
  }, [templates]);
}
