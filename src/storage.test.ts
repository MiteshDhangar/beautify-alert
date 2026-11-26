/**
 * @jest-environment node
 */
import { MAX_SETTINGS_BYTES, SettingsTooLargeError, decodeTemplates, encodeTemplates, settingsBytes } from './storage';

describe('template storage', () => {
  it('round-trips templates through gzip and base64, including non-ASCII text', async () => {
    const templates = [
      { name: 'tamil', subject: 'ஆச்சிரியமான கிப்ட்', html: '<p>₹65 🥬</p>'.repeat(200) },
      { name: 'plain', subject: 's', html: '<h1>{{ .Name }}</h1>' },
    ];
    const gz = await encodeTemplates(templates);
    expect(await decodeTemplates(gz)).toEqual(templates);
    expect(gz.length).toBeLessThan(JSON.stringify(templates).length / 3);
  });

  it('counts bytes the way Grafana does', () => {
    expect(settingsBytes({ a: 'é' })).toBe(10);
    expect(new SettingsTooLargeError(MAX_SETTINGS_BYTES + 1).message).toContain('64 KB');
  });
});
