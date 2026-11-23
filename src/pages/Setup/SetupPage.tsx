import React, { useCallback, useEffect, useState } from 'react';
import { css } from '@emotion/css';
import { GrafanaTheme2 } from '@grafana/data';
import { config, PluginPage } from '@grafana/runtime';
import {
  Alert,
  Badge,
  Button,
  Field,
  Icon,
  Input,
  LinkButton,
  LoadingPlaceholder,
  Text,
  TextLink,
  useStyles2,
} from '@grafana/ui';
import { grafanaApi } from '../../api';
import { CONTACT_POINT_NAME, ISSUES_URL, PLUGIN_ID, ROUTES, SERVICE_ACCOUNT_NAME } from '../../constants';
import { useSettings } from '../../hooks/useSettings';
import { prefixRoute } from '../../utils/utils.routing';

type ContactPoint = Awaited<ReturnType<typeof grafanaApi.contactPoints>>[number];

export function webhookPath() {
  return `api/plugins/${PLUGIN_ID}/resources/alert`;
}

function defaultWebhookUrl() {
  const base = config.appUrl.endsWith('/') ? config.appUrl : `${config.appUrl}/`;
  return base + webhookPath();
}

/**
 * Creates (or updates) the webhook contact point that sends alerts to the
 * plugin, with a fresh token of a dedicated Editor service account. Runs with
 * the signed-in admin's session.
 */
async function connect(url: string, existing?: ContactPoint) {
  const found = await grafanaApi.findServiceAccount(SERVICE_ACCOUNT_NAME);
  let sa = found.serviceAccounts.find((a) => a.name === SERVICE_ACCOUNT_NAME);
  if (!sa) {
    const created = await grafanaApi.createServiceAccount(SERVICE_ACCOUNT_NAME);
    sa = { id: created.id, name: SERVICE_ACCOUNT_NAME, role: 'Editor', tokens: 0 };
  }
  const token = await grafanaApi.createToken(sa.id, `webhook-${Date.now()}`);
  const body = {
    name: CONTACT_POINT_NAME,
    type: 'webhook',
    disableResolveMessage: false,
    settings: { url, httpMethod: 'POST', authorization_scheme: 'Bearer', authorization_credentials: token.key },
  };
  if (existing) {
    await grafanaApi.updateContactPoint(existing.uid, { ...body, uid: existing.uid });
  } else {
    await grafanaApi.createContactPoint(body);
  }
}

export default function SetupPage() {
  const s = useStyles2(getStyles);
  const { settings, loading } = useSettings();
  const [points, setPoints] = useState<ContactPoint[]>();
  const [url, setUrl] = useState(defaultWebhookUrl());
  const [working, setWorking] = useState(false);
  const [result, setResult] = useState<{ ok: boolean; text: string }>();

  const loadPoints = useCallback(
    () =>
      grafanaApi
        .contactPoints()
        .then(setPoints)
        .catch((e: Error) => {
          setResult({ ok: false, text: `Could not read contact points: ${e.message}` });
          setPoints([]);
        }),
    []
  );

  useEffect(() => {
    loadPoints();
  }, [loadPoints]);

  if (loading || !settings || !points) {
    return (
      <PluginPage>
        <LoadingPlaceholder text="Checking setup" />
      </PluginPage>
    );
  }

  const deliveryReady = !!settings.delivery.fromAddress && !!settings.delivery.smtp.host;
  const ours = points.filter((p) => p.type === 'webhook' && String(p.settings?.url ?? '').includes(webhookPath()));
  const named = points.find((p) => p.name === CONTACT_POINT_NAME && p.type === 'webhook');
  const connected = ours.length > 0;
  const { templateLabel, recipientsLabel } = settings.routing;

  const onConnect = async () => {
    setWorking(true);
    setResult(undefined);
    try {
      await connect(url, named);
      setResult({
        ok: true,
        text: `Contact point "${CONTACT_POINT_NAME}" is ready. Pick it in an alert rule or a notification policy.`,
      });
      await loadPoints();
    } catch (e) {
      setResult({ ok: false, text: `Setup failed: ${(e as Error).message}` });
    } finally {
      setWorking(false);
    }
  };

  return (
    <PluginPage subTitle="Three steps connect Grafana alerting to your email templates.">
      <ol className={s.steps}>
        <li className={s.step}>
          <Status done={deliveryReady} />
          <div className={s.body}>
            <Text element="h4">Set up email delivery</Text>
            <Text color="secondary">
              {deliveryReady
                ? `Sending as ${settings.delivery.fromAddress} through ${settings.delivery.smtp.host}:${settings.delivery.smtp.port}.`
                : 'Add your SMTP server and the address emails come from.'}
            </Text>
            <div>
              <LinkButton variant="secondary" size="sm" href={`/plugins/${PLUGIN_ID}`} icon="cog">
                {deliveryReady ? 'Change' : 'Open Configuration'}
              </LinkButton>
            </div>
          </div>
        </li>

        <li className={s.step}>
          <Status done={connected} />
          <div className={s.body}>
            <Text element="h4">Connect Grafana alerting</Text>
            <Text color="secondary">
              Grafana sends alerts to this plugin through a webhook contact point. This creates a service account named{' '}
              <code>{SERVICE_ACCOUNT_NAME}</code> with the Editor role, a token for it, and the contact point{' '}
              <code>{CONTACT_POINT_NAME}</code>.
            </Text>
            {connected && <Text color="secondary">Connected through: {ours.map((p) => p.name).join(', ')}.</Text>}
            <Field
              label="Webhook URL"
              description="Grafana's own address followed by the plugin path. Change it if your Grafana server cannot reach its public URL, for example http://localhost:3000/."
            >
              <Input value={url} onChange={(e) => setUrl(e.currentTarget.value.trim())} data-testid="webhook-url" />
            </Field>
            <div>
              <Button
                onClick={onConnect}
                disabled={working || !url}
                icon={working ? 'spinner' : 'link'}
                data-testid="connect"
              >
                {named ? 'Reconnect with a new token' : 'Create contact point'}
              </Button>
            </div>
            {result && (
              <Alert
                severity={result.ok ? 'success' : 'error'}
                title={result.text}
                onRemove={() => setResult(undefined)}
              />
            )}
            <details className={s.details}>
              <summary>Set it up by hand instead</summary>
              <ol>
                <li>
                  Administration, Users and access, Service accounts: add one with the Editor role, then add a token.
                </li>
                <li>
                  Alerting, Contact points: add a Webhook integration with the URL above, HTTP method POST, and under
                  Optional settings the authorization scheme <code>Bearer</code> with the token as credentials.
                </li>
              </ol>
            </details>
          </div>
        </li>

        <li className={s.step}>
          <Status done={false} neutral />
          <div className={s.body}>
            <Text element="h4">Route alerts and pick templates</Text>
            <Text color="secondary">
              Choose the <code>{CONTACT_POINT_NAME}</code> contact point in an alert rule, or route to it in{' '}
              <TextLink href="/alerting/routes" inline>
                notification policies
              </TextLink>
              . Then add labels to pick a template and recipients:
            </Text>
            <table className={s.table}>
              <thead>
                <tr>
                  <th>Label on the alert rule</th>
                  <th>Effect</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>
                    <code>{templateLabel}=payments</code>
                  </td>
                  <td>Uses the template named payments</td>
                </tr>
                <tr>
                  <td>
                    <code>{recipientsLabel}=finance@example.com</code>
                  </td>
                  <td>Sends to this address instead of the template recipients or default recipients</td>
                </tr>
                <tr>
                  <td>none</td>
                  <td>Uses the default template and the default recipients</td>
                </tr>
              </tbody>
            </table>
            <Text variant="bodySmall" color="secondary">
              The webhook URL also accepts <code>?template=name</code> and <code>?to=address</code>, which override
              labels for every alert sent through that contact point.
            </Text>
            <div>
              <LinkButton variant="secondary" size="sm" href={prefixRoute(ROUTES.Templates)} icon="envelope">
                Edit templates
              </LinkButton>
            </div>
          </div>
        </li>
      </ol>
      <Text variant="bodySmall" color="secondary">
        Something missing or broken?{' '}
        <TextLink href={ISSUES_URL} external inline>
          Open an issue
        </TextLink>
        .
      </Text>
    </PluginPage>
  );
}

function Status({ done, neutral }: { done: boolean; neutral?: boolean }) {
  const s = useStyles2(getStyles);
  if (neutral) {
    return <Icon name="info-circle" size="lg" className={s.neutral} />;
  }
  return done ? (
    <Icon name="check-circle" size="lg" className={s.done} aria-label="Done" />
  ) : (
    <Badge text="To do" color="orange" />
  );
}

const getStyles = (theme: GrafanaTheme2) => ({
  steps: css({
    listStyle: 'none',
    padding: 0,
    margin: theme.spacing(0, 0, 3, 0),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    maxWidth: 900,
  }),
  step: css({
    display: 'grid',
    gridTemplateColumns: '64px minmax(0, 1fr)',
    gap: theme.spacing(2),
    alignItems: 'start',
  }),
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1), '& h4': { margin: 0 } }),
  done: css({ color: theme.colors.success.text }),
  neutral: css({ color: theme.colors.text.secondary }),
  details: css({
    color: theme.colors.text.secondary,
    summary: { cursor: 'pointer' },
    ol: { marginTop: theme.spacing(1) },
  }),
  table: css({
    borderCollapse: 'collapse',
    th: {
      textAlign: 'left',
      color: theme.colors.text.secondary,
      fontWeight: theme.typography.fontWeightMedium,
      padding: theme.spacing(0.5, 2, 0.5, 0),
      borderBottom: `1px solid ${theme.colors.border.weak}`,
    },
    td: { padding: theme.spacing(0.75, 2, 0.75, 0), borderBottom: `1px solid ${theme.colors.border.weak}` },
  }),
});
