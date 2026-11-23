import { DEFAULT_SETTINGS, withDefaults } from './useSettings';

describe('withDefaults', () => {
  it('fills every key for empty settings', () => {
    expect(withDefaults(undefined)).toEqual(DEFAULT_SETTINGS);
  });

  it('keeps saved values and fills nested defaults', () => {
    const s = withDefaults({
      delivery: { fromAddress: 'a@x.com', smtp: { host: 'smtp.x.com' } } as never,
      templates: [{ name: 'p', subject: '', html: '' }],
    });
    expect(s.delivery.fromAddress).toBe('a@x.com');
    expect(s.delivery.smtp.host).toBe('smtp.x.com');
    expect(s.delivery.smtp.port).toBe(587);
    expect(s.routing.templateLabel).toBe('email_template');
    expect(s.templates).toHaveLength(1);
  });
});
