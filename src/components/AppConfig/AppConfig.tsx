import React, { useState } from 'react';
import { css } from '@emotion/css';
import { AppPluginMeta, GrafanaTheme2, PluginConfigPageProps } from '@grafana/data';
import {
  Alert,
  Button,
  Field,
  FieldSet,
  Input,
  RadioButtonGroup,
  SecretInput,
  Stack,
  Switch,
  Text,
  TextArea,
  useStyles2,
} from '@grafana/ui';
import { api, pluginSettings } from '../../api';
import { withDefaults } from '../../hooks/useSettings';
import { Settings, SmtpSecurity } from '../../types';
import { testIds } from '../testIds';

export interface AppConfigProps extends PluginConfigPageProps<AppPluginMeta<Partial<Settings>>> {}

const SECURITY_OPTIONS: Array<{ label: string; value: SmtpSecurity; description: string }> = [
  { label: 'STARTTLS', value: 'starttls', description: 'Usually port 587' },
  { label: 'TLS', value: 'tls', description: 'Usually port 465' },
  { label: 'None', value: 'none', description: 'Unencrypted, only for local relays' },
];

const DEFAULT_PORT: Record<SmtpSecurity, number> = { starttls: 587, tls: 465, none: 25 };

type Notice = { severity: 'success' | 'error' | 'info'; title: string; text?: string };

const AppConfig = ({ plugin }: AppConfigProps) => {
  const s = useStyles2(getStyles);
  const [form, setForm] = useState<Settings>(withDefaults(plugin.meta.jsonData));
  const [passwordSet, setPasswordSet] = useState(Boolean(plugin.meta.secureJsonFields?.smtpPassword));
  const [password, setPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [notice, setNotice] = useState<Notice>();

  const d = form.delivery;
  const setDelivery = (patch: Partial<Settings['delivery']>) =>
    setForm((f) => ({ ...f, delivery: { ...f.delivery, ...patch } }));
  const setSmtp = (patch: Partial<Settings['delivery']['smtp']>) =>
    setForm((f) => ({ ...f, delivery: { ...f.delivery, smtp: { ...f.delivery.smtp, ...patch } } }));

  const missing = [!d.fromAddress && 'from address', !d.smtp.host && 'SMTP host'].filter(Boolean);

  const save = async () => {
    setSaving(true);
    setNotice(undefined);
    try {
      // Templates are edited on the Templates page; keep whatever is saved.
      const { templates: _ignored, templatesGz: _ignoredGz, ...rest } = form;
      // "Skip certificate check" was removed in 0.1.6; drop the old value.
      const { skipVerify: _removed, ...smtp } = rest.delivery.smtp;
      rest.delivery = { ...rest.delivery, smtp };
      const secure = password ? { smtpPassword: password } : passwordSet ? undefined : { smtpPassword: '' };
      await pluginSettings.save(rest, secure);
      // Grafana keeps the plugin settings in memory per page load.
      window.location.reload();
    } catch (e) {
      setNotice({ severity: 'error', title: 'Could not save', text: (e as Error).message });
      setSaving(false);
    }
  };

  const verify = async () => {
    setVerifying(true);
    setNotice(undefined);
    try {
      await api.verify(d, password || undefined);
      setNotice({
        severity: 'success',
        title: `Connected to ${d.smtp.host}:${d.smtp.port}`,
        text: 'The server accepted the connection and login. Nothing was sent.',
      });
    } catch (e) {
      setNotice({ severity: 'error', title: 'Connection failed', text: (e as Error).message });
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className={s.root}>
      <FieldSet label="Sender">
        <div className={s.grid}>
          <Field label="From address" required>
            <Input
              value={d.fromAddress}
              placeholder="alerts@example.com"
              onChange={(e) => setDelivery({ fromAddress: e.currentTarget.value.trim() })}
              data-testid={testIds.appConfig.fromAddress}
            />
          </Field>
          <Field label="From name">
            <Input
              value={d.fromName}
              placeholder="Grafana Alerts"
              onChange={(e) => setDelivery({ fromName: e.currentTarget.value })}
            />
          </Field>
          <Field label="Reply-to" description="Optional, where replies go">
            <Input
              value={d.replyTo ?? ''}
              placeholder="oncall@example.com"
              onChange={(e) => setDelivery({ replyTo: e.currentTarget.value.trim() })}
            />
          </Field>
        </div>
      </FieldSet>

      <FieldSet label="SMTP server">
        <Text color="secondary" element="p">
          More providers (Microsoft 365 with OAuth, Amazon SES, SendGrid, Mailgun) are planned. This page stores its own
          SMTP settings; the <code>[smtp]</code> section of the Grafana configuration file is not used.
        </Text>
        <div className={s.grid}>
          <Field label="Host" required>
            <Input
              value={d.smtp.host}
              placeholder="smtp.example.com"
              onChange={(e) => setSmtp({ host: e.currentTarget.value.trim() })}
              data-testid={testIds.appConfig.smtpHost}
            />
          </Field>
          <Field label="Port">
            <Input
              type="number"
              value={d.smtp.port}
              onChange={(e) => setSmtp({ port: Number(e.currentTarget.value) || 0 })}
              data-testid={testIds.appConfig.smtpPort}
            />
          </Field>
          <Field label="Security">
            <RadioButtonGroup
              options={SECURITY_OPTIONS}
              value={d.smtp.security}
              onChange={(security) => {
                // Move the port along when it is still the previous default.
                const port = d.smtp.port === DEFAULT_PORT[d.smtp.security] ? DEFAULT_PORT[security] : d.smtp.port;
                setSmtp({ security, port });
              }}
            />
          </Field>
          <Field label="Username" description="Leave empty for servers without login">
            <Input
              value={d.smtp.username}
              autoComplete="off"
              onChange={(e) => setSmtp({ username: e.currentTarget.value.trim() })}
            />
          </Field>
          <Field label="Password" description="Stored encrypted by Grafana">
            <SecretInput
              value={password}
              isConfigured={passwordSet}
              autoComplete="new-password"
              onChange={(e) => setPassword(e.currentTarget.value)}
              onReset={() => {
                setPasswordSet(false);
                setPassword('');
              }}
            />
          </Field>
        </div>
        <Field
          label="Trusted certificate"
          description="Optional. For a server with a self-signed or private certificate: paste its certificate, or the CA that issued it, in PEM format. Certificates are always checked."
        >
          <TextArea
            rows={3}
            value={d.smtp.caCertificate ?? ''}
            placeholder={'-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----'}
            onChange={(e) => setSmtp({ caCertificate: e.currentTarget.value })}
            data-testid={testIds.appConfig.caCertificate}
          />
        </Field>
        {d.smtp.skipVerify && !d.smtp.caCertificate && (
          <Alert severity="warning" title="Skip certificate check was removed">
            Version 0.1.6 always checks the server certificate, because skipping the check let anyone on the network read
            the SMTP password. If this server has a self-signed certificate, paste it above and save.
          </Alert>
        )}
        <Button
          variant="secondary"
          icon={verifying ? 'spinner' : 'plug'}
          onClick={verify}
          disabled={verifying || missing.length > 0}
        >
          Test connection
        </Button>
      </FieldSet>

      <FieldSet label="Recipients and routing">
        <Field
          label="Default recipients"
          description="Used when neither the alert nor the template names recipients. Separate with commas."
        >
          <Input
            value={form.defaultRecipients}
            placeholder="ops@example.com, oncall@example.com"
            onChange={(e) => setForm({ ...form, defaultRecipients: e.currentTarget.value })}
            data-testid={testIds.appConfig.defaultRecipients}
          />
        </Field>
        <div className={s.grid}>
          <Field label="Template label" description="Alert label or annotation that picks a template">
            <Input
              value={form.routing.templateLabel}
              onChange={(e) =>
                setForm({ ...form, routing: { ...form.routing, templateLabel: e.currentTarget.value.trim() } })
              }
            />
          </Field>
          <Field label="Recipients label" description="Alert label or annotation that overrides recipients">
            <Input
              value={form.routing.recipientsLabel}
              onChange={(e) =>
                setForm({ ...form, routing: { ...form.routing, recipientsLabel: e.currentTarget.value.trim() } })
              }
            />
          </Field>
        </div>
        <Field
          label="Alerts per email"
          description="Longer groups list the first ones and say how many more there are. Gmail cuts off emails larger than about 102 KB, roughly 40 to 50 alerts with the default template."
        >
          <Input
            type="number"
            width={16}
            min={1}
            max={1000}
            value={form.maxAlertsPerEmail ?? 40}
            onChange={(e) =>
              setForm({ ...form, maxAlertsPerEmail: Math.min(1000, Math.max(1, Number(e.currentTarget.value) || 40)) })
            }
          />
        </Field>
        <Field
          label="Thread emails"
          description="Show the firing and resolved emails of one alert group as one conversation in mail clients"
        >
          <Switch
            value={form.threading !== false}
            onChange={(e) => setForm({ ...form, threading: e.currentTarget.checked })}
          />
        </Field>
      </FieldSet>

      <FieldSet label="Limits">
        <Field
          label="Allowed recipient domains"
          description="Every email, alert or test, only goes to addresses in these domains. Separate with commas; list subdomains separately. Leave empty to allow any domain."
        >
          <Input
            value={form.allowedRecipientDomains ?? ''}
            placeholder="example.com, example.org"
            onChange={(e) => setForm({ ...form, allowedRecipientDomains: e.currentTarget.value })}
            data-testid={testIds.appConfig.allowedDomains}
          />
        </Field>
        <div className={s.grid}>
          <Field
            label="Max recipients per email"
            description="Extra recipients are skipped with a warning in the delivery log. Many providers allow 50 to 100."
          >
            <Input
              type="number"
              width={16}
              min={1}
              max={500}
              value={form.maxRecipientsPerEmail ?? 50}
              onChange={(e) =>
                setForm({
                  ...form,
                  maxRecipientsPerEmail: Math.min(500, Math.max(1, Number(e.currentTarget.value) || 50)),
                })
              }
            />
          </Field>
          <Field
            label="Max emails per notification"
            description="One Grafana notification becomes one email per template and recipient list. The rest are not sent and the delivery log says so."
          >
            <Input
              type="number"
              width={16}
              min={1}
              max={200}
              value={form.maxEmailsPerNotification ?? 20}
              onChange={(e) =>
                setForm({
                  ...form,
                  maxEmailsPerNotification: Math.min(200, Math.max(1, Number(e.currentTarget.value) || 20)),
                })
              }
            />
          </Field>
        </div>
      </FieldSet>

      {notice && (
        <Alert severity={notice.severity} title={notice.title} onRemove={() => setNotice(undefined)}>
          {notice.text}
        </Alert>
      )}
      <Stack alignItems="center">
        <Button
          onClick={save}
          disabled={saving}
          icon={saving ? 'spinner' : 'save'}
          data-testid={testIds.appConfig.submit}
        >
          Save settings
        </Button>
        {missing.length > 0 && (
          <Text color="secondary" variant="bodySmall">
            Emails can be sent once you add the {missing.join(' and ')}.
          </Text>
        )}
      </Stack>
    </div>
  );
};

export default AppConfig;

const getStyles = (theme: GrafanaTheme2) => ({
  root: css({ maxWidth: 960 }),
  grid: css({
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
    columnGap: theme.spacing(2),
  }),
});
