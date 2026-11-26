import { useCallback, useEffect, useState } from 'react';
import { pluginSettings } from '../api';
import { settingsBytes } from '../storage';
import { Settings } from '../types';

export const DEFAULT_SETTINGS: Settings = {
  delivery: {
    provider: 'smtp',
    fromAddress: '',
    fromName: 'Grafana Alerts',
    replyTo: '',
    smtp: { host: '', port: 587, username: '', security: 'starttls', skipVerify: false },
  },
  defaultRecipients: '',
  routing: { templateLabel: 'email_template', recipientsLabel: 'email_to' },
  templates: [],
  threading: true,
  maxAlertsPerEmail: 40,
};

/** Fills defaults into saved jsonData so pages never deal with missing keys. */
export function withDefaults(json: Partial<Settings> | undefined): Settings {
  const j = json ?? {};
  return {
    ...DEFAULT_SETTINGS,
    ...j,
    delivery: {
      ...DEFAULT_SETTINGS.delivery,
      ...(j.delivery ?? {}),
      smtp: { ...DEFAULT_SETTINGS.delivery.smtp, ...(j.delivery?.smtp ?? {}) },
    },
    routing: { ...DEFAULT_SETTINGS.routing, ...(j.routing ?? {}) },
    templates: j.templates ?? [],
  };
}

interface State {
  settings?: Settings;
  /** Bytes the saved jsonData takes, out of Grafana's 64 KB. */
  storageBytes?: number;
  secureFields: Record<string, boolean>;
  enabled: boolean;
  loading: boolean;
  error?: string;
}

/** Loads the plugin settings and saves parts of them. */
export function useSettings() {
  const [state, setState] = useState<State>({ secureFields: {}, enabled: false, loading: true });

  const reload = useCallback(async () => {
    try {
      const s = await pluginSettings.get();
      const templates = await pluginSettings.templates(s.jsonData);
      setState({
        settings: { ...withDefaults(s.jsonData), templates, templatesGz: undefined },
        storageBytes: settingsBytes(s.jsonData ?? {}),
        secureFields: s.secureJsonFields ?? {},
        enabled: s.enabled,
        loading: false,
      });
    } catch (e) {
      setState((prev) => ({ ...prev, loading: false, error: (e as Error).message }));
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const save = useCallback(
    async (patch: Partial<Settings>, secure?: Record<string, string>) => {
      await pluginSettings.save(patch, secure);
      await reload();
    },
    [reload]
  );

  return { ...state, reload, save };
}
