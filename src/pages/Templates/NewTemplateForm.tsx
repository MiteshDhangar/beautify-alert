import React, { useState } from 'react';
import { css, cx } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { Button, Field, Input, Stack, Text, useStyles2 } from '@grafana/ui';
import { EmailTemplate, Starter } from '../../types';

interface Props {
  starters: Starter[];
  templateLabel: string;
  nameTaken: (name: string) => boolean;
  canSave: boolean;
  onCreate: (t: EmailTemplate) => Promise<void>;
  onCancel: () => void;
}

const NAME_RE = /^[a-z0-9][a-z0-9_-]{0,63}$/;

export function NewTemplateForm({ starters, templateLabel, nameTaken, canSave, onCreate, onCancel }: Props) {
  const s = useStyles2(getStyles);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [recipients, setRecipients] = useState('');
  const [starter, setStarter] = useState(starters[0]?.name ?? '');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string>();

  const nameError = !name
    ? undefined
    : !NAME_RE.test(name)
      ? 'Use lowercase letters, digits, - and _ only'
      : nameTaken(name)
        ? 'A template with this name already exists'
        : undefined;

  const create = async () => {
    const from = starters.find((st) => st.name === starter) ?? starters[0];
    setCreating(true);
    setError(undefined);
    try {
      await onCreate({ name, description, recipients, subject: from.subject, html: from.html });
    } catch (e) {
      setError((e as Error).message);
      setCreating(false);
    }
  };

  return (
    <div className={s.root} data-testid="new-template-form">
      <Stack direction="column" gap={0.5}>
        <Text element="h3">New template</Text>
        <Text color="secondary">Name it, pick a starting point, then edit the HTML.</Text>
      </Stack>
      <Field
        label="Name"
        invalid={!!nameError}
        error={nameError}
        description={
          name && !nameError
            ? `Alerts use it with the label ${templateLabel}=${name}`
            : `Alerts pick it with the label ${templateLabel}=<name>`
        }
      >
        <Input
          value={name}
          placeholder="stock-low"
          autoFocus
          onChange={(e) => setName(e.currentTarget.value.trim())}
          data-testid="new-template-name"
        />
      </Field>
      <Field label="Description" description="Optional">
        <Input
          value={description}
          placeholder="Low stock alerts for the warehouse team"
          onChange={(e) => setDescription(e.currentTarget.value)}
        />
      </Field>
      <Field label="Recipients" description="Optional. An alert's recipients label overrides it.">
        <Input
          value={recipients}
          placeholder="warehouse@example.com"
          onChange={(e) => setRecipients(e.currentTarget.value)}
        />
      </Field>
      <Field label="Start from">
        <div className={s.starters} role="radiogroup">
          {starters.map((st) => (
            <button
              key={st.name}
              type="button"
              role="radio"
              aria-checked={starter === st.name}
              className={cx(s.starter, starter === st.name && s.selected)}
              onClick={() => setStarter(st.name)}
            >
              <Text weight="medium">{st.title}</Text>
              <Text variant="bodySmall" color="secondary">
                {st.description ?? ''}
              </Text>
            </button>
          ))}
        </div>
      </Field>
      {error && <Text color="error">{error}</Text>}
      <Stack>
        <Button
          onClick={create}
          disabled={!canSave || !name || !!nameError || creating}
          icon={creating ? 'spinner' : 'plus'}
          data-testid="new-template-create"
        >
          Create template
        </Button>
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </Stack>
      {!canSave && (
        <Text variant="bodySmall" color="secondary">
          Only Grafana admins can create templates.
        </Text>
      )}
    </div>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), maxWidth: 820 }),
  starters: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
    gap: theme.spacing(1),
  }),
  starter: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    textAlign: 'left',
    padding: theme.spacing(1.5),
    background: theme.colors.background.primary,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    cursor: 'pointer',
    color: theme.colors.text.primary,
    '&:hover': { background: theme.colors.action.hover },
  }),
  selected: css({ borderColor: theme.colors.primary.border, boxShadow: `0 0 0 1px ${theme.colors.primary.border}` }),
});
