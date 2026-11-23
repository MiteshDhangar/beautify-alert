import pluginJson from './plugin.json';

export const PLUGIN_ID = pluginJson.id;
export const PLUGIN_BASE_URL = `/a/${PLUGIN_ID}`;

export enum ROUTES {
  Templates = 'templates',
  History = 'history',
  Setup = 'setup',
}

export const CONTACT_POINT_NAME = 'Beautify Alert';
export const SERVICE_ACCOUNT_NAME = 'beautify-alert';
export const ISSUES_URL = 'https://github.com/miteshdhangar76/beautify-alert/issues/new';
