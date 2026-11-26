import { lastValueFrom } from 'rxjs';
import { FetchError, getBackendSrv } from '@grafana/runtime';
import { PLUGIN_ID } from './constants';
import { ApiError, EmailTemplate, HistoryEntry, Rendered, Settings, Starter } from './types';
import { MAX_SETTINGS_BYTES, SettingsTooLargeError, decodeTemplates, encodeTemplates, settingsBytes } from './storage';

const RESOURCES = `/api/plugins/${PLUGIN_ID}/resources`;

/** Error thrown by API calls, with the backend's message. */
export class RequestError extends Error {
  constructor(
    message: string,
    public status: number,
    public part?: ApiError['part']
  ) {
    super(message);
  }
}

async function request<T>(method: string, url: string, data?: unknown, headers?: Record<string, string>): Promise<T> {
  try {
    const res = await lastValueFrom(
      getBackendSrv().fetch<T>({ method, url, data, headers, showErrorAlert: false, showSuccessAlert: false })
    );
    return res.data;
  } catch (e) {
    const err = e as FetchError<ApiError & { message?: string }>;
    const msg = err.data?.error || err.data?.message || err.statusText || 'Request failed';
    throw new RequestError(msg, err.status ?? 0, err.data?.part);
  }
}

export const api = {
  starters: () => request<{ starters: Starter[] }>('GET', `${RESOURCES}/starters`),
  preview: (template: EmailTemplate, status?: 'firing' | 'resolved') =>
    request<Rendered>('POST', `${RESOURCES}/preview`, { template, status }),
  test: (template: EmailTemplate, to: string) => request<HistoryEntry>('POST', `${RESOURCES}/test`, { template, to }),
  history: () => request<{ entries: HistoryEntry[]; capacity: number }>('GET', `${RESOURCES}/history`),
  historyEntry: (id: string) => request<HistoryEntry>('GET', `${RESOURCES}/history/${encodeURIComponent(id)}`),
  resend: (id: string) => request<HistoryEntry>('POST', `${RESOURCES}/history/${encodeURIComponent(id)}/resend`),
  verify: (delivery: Settings['delivery'], password?: string) =>
    request<{ ok: boolean }>('POST', `${RESOURCES}/verify`, { delivery, password }),
};

interface PluginSettingsDTO {
  enabled: boolean;
  pinned: boolean;
  jsonData?: Partial<Settings>;
  secureJsonFields?: Record<string, boolean>;
}

/** Plugin settings stored by Grafana (jsonData and which secrets are set). */
export const pluginSettings = {
  get: () => request<PluginSettingsDTO>('GET', `/api/plugins/${PLUGIN_ID}/settings`),
  /**
   * Saves jsonData. It reads the current settings first and replaces only the
   * given keys, so two pages saving different parts do not overwrite each other.
   */
  async save(patch: Partial<Settings>, secureJsonData?: Record<string, string>) {
    const current = await pluginSettings.get();
    const jsonData = await pluginSettings.prepare(current.jsonData ?? {}, patch);
    await request('POST', `/api/plugins/${PLUGIN_ID}/settings`, {
      enabled: true,
      pinned: current.pinned,
      jsonData,
      secureJsonData,
    });
  },
  /**
   * Merges patch into the saved jsonData. Templates are stored compressed in
   * templatesGz; it throws SettingsTooLargeError above Grafana's 64 KB limit.
   */
  async prepare(current: Partial<Settings>, patch: Partial<Settings>): Promise<Partial<Settings>> {
    const { templates, templatesGz: _ignored, ...rest } = patch;
    const next: Partial<Settings> = { ...current, ...rest };
    if (templates) {
      next.templatesGz = await encodeTemplates(templates);
      delete next.templates;
    }
    const bytes = settingsBytes(next);
    if (bytes > MAX_SETTINGS_BYTES) {
      throw new SettingsTooLargeError(bytes);
    }
    return next;
  },
  /** The saved templates, from templatesGz or plain templates. */
  async templates(jsonData: Partial<Settings> | undefined): Promise<EmailTemplate[]> {
    if (jsonData?.templatesGz) {
      return decodeTemplates(jsonData.templatesGz);
    }
    return jsonData?.templates ?? [];
  },
};

/** Grafana HTTP API calls the Setup page makes with the admin's session. */
export const grafanaApi = {
  findServiceAccount: (name: string) =>
    request<{ serviceAccounts: Array<{ id: number; name: string; role: string; tokens: number }> }>(
      'GET',
      `/api/serviceaccounts/search?query=${encodeURIComponent(name)}`
    ),
  createServiceAccount: (name: string) =>
    request<{ id: number }>('POST', '/api/serviceaccounts', { name, role: 'Editor', isDisabled: false }),
  createToken: (id: number, name: string) =>
    request<{ key: string }>('POST', `/api/serviceaccounts/${id}/tokens`, { name }),
  contactPoints: () =>
    request<Array<{ uid: string; name: string; type: string; settings: Record<string, unknown> }>>(
      'GET',
      '/api/v1/provisioning/contact-points'
    ),
  createContactPoint: (body: unknown) =>
    request<{ uid: string }>('POST', '/api/v1/provisioning/contact-points', body, { 'X-Disable-Provenance': 'true' }),
  updateContactPoint: (uid: string, body: unknown) =>
    request('PUT', `/api/v1/provisioning/contact-points/${uid}`, body, { 'X-Disable-Provenance': 'true' }),
};
