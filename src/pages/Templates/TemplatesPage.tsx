import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { css, cx } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { PluginPage } from '@grafana/runtime';
import { Alert, Badge, Button, LinkButton, LoadingPlaceholder, Stack, Text, useStyles2 } from '@grafana/ui';
import { api } from '../../api';
import { useSettings } from '../../hooks/useSettings';
import { EmailTemplate, Starter } from '../../types';
import { prefixRoute } from '../../utils/utils.routing';
import { PLUGIN_ID, ROUTES } from '../../constants';
import { isAdmin } from '../../utils/roles';
import { NewTemplateForm } from './NewTemplateForm';
import { TemplateEditor, useNameTaken } from './TemplateEditor';

type Selection = { kind: 'new' } | { kind: 'template'; name: string };

export default function TemplatesPage() {
  const s = useStyles2(getStyles);
  const { settings, loading, error, save } = useSettings();
  const [starters, setStarters] = useState<Starter[]>([]);
  const [selection, setSelection] = useState<Selection>({ kind: 'template', name: 'default' });
  const [dirty, setDirty] = useState(false);
  const canSave = isAdmin();

  useEffect(() => {
    api.starters().then((r) => setStarters(r.starters));
  }, []);

  const saved = useMemo(() => settings?.templates ?? [], [settings]);
  const builtinDefault = starters.find((st) => st.name === 'default');
  const hasSavedDefault = saved.some((t) => t.name === 'default');

  // The list shows the built-in default until the user saves their own.
  const list: Array<{ template: EmailTemplate; builtin: boolean }> = useMemo(() => {
    const items = saved.map((t) => ({ template: t, builtin: false }));
    if (!hasSavedDefault && builtinDefault) {
      items.unshift({
        template: { ...builtinDefault, description: 'Built-in. Used when an alert names no template.' },
        builtin: true,
      });
    }
    return items;
  }, [saved, hasSavedDefault, builtinDefault]);

  const nameTaken = useNameTaken(saved);

  const select = useCallback(
    (next: Selection) => {
      if (dirty && !window.confirm('Discard your unsaved changes to this template?')) {
        return;
      }
      setDirty(false);
      setSelection(next);
    },
    [dirty]
  );

  const saveTemplates = (templates: EmailTemplate[]) => save({ templates });

  if (loading || !settings) {
    return (
      <PluginPage>
        {error ? (
          <Alert severity="error" title="Could not load settings">
            {error}
          </Alert>
        ) : (
          <LoadingPlaceholder text="Loading templates" />
        )}
      </PluginPage>
    );
  }

  const current =
    selection.kind === 'template' ? (list.find((i) => i.template.name === selection.name) ?? list[0]) : undefined;
  const deliveryReady = !!settings.delivery.fromAddress && !!settings.delivery.smtp.host;

  return (
    <PluginPage>
      {!deliveryReady && (
        <Alert severity="warning" title="Email delivery is not set up yet">
          <Stack direction="column" alignItems="flex-start">
            <span>Add your SMTP server on the Configuration page before alerts can be emailed.</span>
            <LinkButton size="sm" href={`/plugins/${PLUGIN_ID}`} variant="secondary">
              Open Configuration
            </LinkButton>
          </Stack>
        </Alert>
      )}
      <div className={s.layout}>
        <nav className={s.list} aria-label="Templates">
          {list.map(({ template, builtin }) => {
            const active = selection.kind === 'template' && current?.template.name === template.name;
            return (
              <button
                key={template.name}
                type="button"
                className={cx(s.item, active && s.active)}
                onClick={() => select({ kind: 'template', name: template.name })}
                data-testid={`template-item-${template.name}`}
              >
                <Stack alignItems="center" gap={1}>
                  <span className={s.name}>{template.name}</span>
                  {builtin && <Badge text="built-in" color="blue" />}
                </Stack>
                {template.description && (
                  <Text variant="bodySmall" color="secondary" truncate>
                    {template.description}
                  </Text>
                )}
              </button>
            );
          })}
          <Button
            variant={selection.kind === 'new' ? 'primary' : 'secondary'}
            fill={selection.kind === 'new' ? 'solid' : 'outline'}
            icon="plus"
            className={s.newBtn}
            onClick={() => select({ kind: 'new' })}
            data-testid="new-template"
          >
            New template
          </Button>
          <LinkButton variant="secondary" fill="text" size="sm" icon="book" href={prefixRoute(ROUTES.Setup)}>
            How to use templates
          </LinkButton>
        </nav>
        <section className={s.main}>
          {selection.kind === 'new' ? (
            <NewTemplateForm
              starters={starters}
              templateLabel={settings.routing.templateLabel}
              nameTaken={nameTaken}
              canSave={canSave}
              onCancel={() => setSelection({ kind: 'template', name: 'default' })}
              onCreate={async (t) => {
                await saveTemplates([...saved, t]);
                setSelection({ kind: 'template', name: t.name });
              }}
            />
          ) : current ? (
            <TemplateEditor
              key={current.template.name}
              template={current.template}
              builtin={current.builtin}
              settings={settings}
              canSave={canSave}
              nameTaken={nameTaken}
              onDirtyChange={setDirty}
              onSave={async (t) => {
                const others = saved.filter((x) => x.name !== current.template.name);
                await saveTemplates(
                  current.builtin ? [t, ...others] : saved.map((x) => (x.name === current.template.name ? t : x))
                );
                setDirty(false);
                setSelection({ kind: 'template', name: t.name });
              }}
              onDelete={async () => {
                await saveTemplates(saved.filter((x) => x.name !== current.template.name));
                setSelection({ kind: 'template', name: 'default' });
              }}
              onDuplicate={(t) => {
                let n = 2;
                while (nameTaken(`${t.name}-${n}`)) {
                  n++;
                }
                const copy = { ...t, name: `${t.name}-${n}` };
                saveTemplates([...saved, copy]).then(() => setSelection({ kind: 'template', name: copy.name }));
              }}
            />
          ) : (
            <LoadingPlaceholder text="Loading starters" />
          )}
        </section>
      </div>
    </PluginPage>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  layout: css({
    display: 'grid',
    gridTemplateColumns: '240px minmax(0, 1fr)',
    gap: theme.spacing(3),
    [theme.breakpoints.down('md')]: { gridTemplateColumns: 'minmax(0, 1fr)' },
  }),
  list: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), alignItems: 'stretch' }),
  item: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.25),
    textAlign: 'left',
    padding: theme.spacing(1, 1.5),
    border: 0,
    borderRadius: theme.shape.radius.default,
    background: 'transparent',
    color: theme.colors.text.primary,
    cursor: 'pointer',
    minWidth: 0,
    '&:hover': { background: theme.colors.action.hover },
  }),
  active: css({ background: theme.colors.action.selected, '&:hover': { background: theme.colors.action.selected } }),
  name: css({ fontFamily: theme.typography.fontFamilyMonospace }),
  newBtn: css({ marginTop: theme.spacing(1), justifyContent: 'center' }),
  main: css({ minWidth: 0 }),
});
