import React, { useCallback, useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { dateTimeFormat, dateTimeFormatTimeAgo, GrafanaTheme2 } from '@grafana/data';
import { PluginPage } from '@grafana/runtime';
import {
  Alert,
  Badge,
  Button,
  Drawer,
  EmptyState,
  Icon,
  InteractiveTable,
  LoadingPlaceholder,
  Stack,
  Text,
  Tooltip,
  useStyles2,
} from '@grafana/ui';
import { api } from '../../api';
import { EmailPreview } from '../../components/EmailPreview';
import { HistoryEntry } from '../../types';

type Cell = { row: { original: HistoryEntry } };

const SOURCE_LABEL: Record<HistoryEntry['source'], string> = { alert: 'Alert', test: 'Test', resend: 'Resend' };

export default function HistoryPage() {
  const s = useStyles2(getStyles);
  const [entries, setEntries] = useState<HistoryEntry[]>();
  const [capacity, setCapacity] = useState(0);
  const [error, setError] = useState<string>();
  const [open, setOpen] = useState<HistoryEntry>();
  const [busy, setBusy] = useState<string>();
  const [notice, setNotice] = useState<{ ok: boolean; text: string }>();

  const load = useCallback(
    () =>
      api
        .history()
        .then((r) => {
          setEntries(r.entries);
          setCapacity(r.capacity);
          setError(undefined);
        })
        .catch((e: Error) => setError(e.message)),
    []
  );

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  const view = async (id: string) => {
    try {
      setOpen(await api.historyEntry(id));
    } catch (e) {
      setNotice({ ok: false, text: (e as Error).message });
    }
  };

  const resend = async (id: string) => {
    setBusy(id);
    try {
      const e = await api.resend(id);
      setNotice({ ok: e.ok, text: e.ok ? `Resent to ${e.to.join(', ')}` : `Resend failed: ${e.error}` });
    } catch (e) {
      setNotice({ ok: false, text: `Resend failed: ${(e as Error).message}` });
    } finally {
      setBusy(undefined);
      load();
    }
  };

  const columns = [
    {
      id: 'time',
      header: 'Time',
      cell: ({ row: { original: e } }: Cell) => (
        <Tooltip content={dateTimeFormat(e.time)}>
          <span>{dateTimeFormatTimeAgo(e.time)}</span>
        </Tooltip>
      ),
    },
    {
      id: 'ok',
      header: 'Result',
      cell: ({ row: { original: e } }: Cell) =>
        e.skipped ? (
          <Tooltip content={e.skipped}>
            <span>
              <Badge text="Skipped" color="blue" icon="info-circle" />
            </span>
          </Tooltip>
        ) : e.ok ? (
          e.warnings?.length ? (
            <Tooltip content={e.warnings.join('. ')}>
              <span>
                <Badge text="Sent, with warnings" color="orange" icon="exclamation-triangle" />
              </span>
            </Tooltip>
          ) : (
            <Badge text="Sent" color="green" icon="check" />
          )
        ) : (
          <Tooltip content={e.error ?? ''}>
            <span>
              <Badge text="Failed" color="red" icon="exclamation-triangle" />
            </span>
          </Tooltip>
        ),
    },
    {
      id: 'status',
      header: 'Alerts',
      cell: ({ row: { original: e } }: Cell) => (
        <Stack gap={1} alignItems="center">
          <Icon name="circle-mono" size="xs" className={e.alertStatus === 'firing' ? s.firing : s.resolved} />
          <span>
            {e.alerts} {e.alertStatus}
          </span>
        </Stack>
      ),
    },
    { id: 'source', header: 'From', cell: ({ row: { original: e } }: Cell) => SOURCE_LABEL[e.source] ?? e.source },
    { id: 'template', header: 'Template', cell: ({ row: { original: e } }: Cell) => <code>{e.template}</code> },
    {
      id: 'to',
      header: 'To',
      cell: ({ row: { original: e } }: Cell) => <span className={s.clip}>{e.to.join(', ')}</span>,
    },
    {
      id: 'subject',
      header: 'Subject',
      cell: ({ row: { original: e } }: Cell) => <span className={s.clip}>{e.subject}</span>,
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row: { original: e } }: Cell) => (
        <Stack gap={1} justifyContent="flex-end">
          <Button size="sm" variant="secondary" fill="text" icon="eye" onClick={() => view(e.id)}>
            View
          </Button>
          <Button
            size="sm"
            variant="secondary"
            fill="text"
            icon={busy === e.id ? 'spinner' : 'repeat'}
            disabled={!!busy}
            onClick={() => resend(e.id)}
          >
            Resend
          </Button>
        </Stack>
      ),
    },
  ];

  return (
    <PluginPage
      subTitle={`Recent emails this Grafana server sent or failed to send. Keeps the last ${capacity || 500} and clears when Grafana restarts.`}
      actions={
        <Button variant="secondary" icon="sync" onClick={load}>
          Refresh
        </Button>
      }
    >
      {error && (
        <Alert severity="error" title="Could not load the delivery log">
          {error}
        </Alert>
      )}
      {notice && (
        <Alert severity={notice.ok ? 'success' : 'error'} title={notice.text} onRemove={() => setNotice(undefined)} />
      )}
      {!entries ? (
        <LoadingPlaceholder text="Loading" />
      ) : entries.length === 0 ? (
        <EmptyState variant="call-to-action" message="No emails yet">
          <Text color="secondary">
            Emails appear here when an alert fires or when you send a test email from a template.
          </Text>
        </EmptyState>
      ) : (
        <InteractiveTable columns={columns} data={entries} getRowId={(e: HistoryEntry) => e.id} pageSize={25} />
      )}
      {open && (
        <Drawer
          title={open.subject || '(no subject)'}
          subtitle={`${SOURCE_LABEL[open.source]} · ${dateTimeFormat(open.time)}`}
          onClose={() => setOpen(undefined)}
          size="lg"
        >
          <Stack direction="column" gap={2}>
            {!open.ok && (
              <Alert severity="error" title="Delivery failed">
                {open.error}
              </Alert>
            )}
            {open.warnings?.length ? (
              <Alert severity="warning" title="Sent with warnings">
                <ul>
                  {open.warnings.map((w) => (
                    <li key={w}>{w}</li>
                  ))}
                </ul>
              </Alert>
            ) : null}
            {open.message ? (
              <EmailPreview
                subject={open.message.subject}
                html={open.message.html}
                to={open.message.to.join(', ')}
                height={700}
              />
            ) : (
              <Text color="secondary">The body of this email was not kept because it was too large.</Text>
            )}
            <Stack>
              <Button icon="repeat" disabled={!open.message || !!busy} onClick={() => resend(open.id)}>
                Resend
              </Button>
            </Stack>
          </Stack>
        </Drawer>
      )}
    </PluginPage>
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  firing: css({ color: theme.colors.error.text }),
  resolved: css({ color: theme.colors.success.text }),
  clip: css({
    display: 'inline-block',
    maxWidth: 280,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    verticalAlign: 'bottom',
  }),
});
