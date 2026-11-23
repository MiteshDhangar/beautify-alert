import { config } from '@grafana/runtime';

/** Whether the signed-in user can change plugin settings (org admin or Grafana server admin). */
export function isAdmin(): boolean {
  const user = config.bootData.user;
  return user.orgRole === 'Admin' || user.isGrafanaAdmin;
}
