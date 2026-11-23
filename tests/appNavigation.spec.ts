import { test, expect } from './fixtures';
import { ROUTES } from '../src/constants';

test.describe('app pages', () => {
  test('templates page previews the built-in default', async ({ gotoPage, page }) => {
    await gotoPage(`/${ROUTES.Templates}`);
    await expect(page.getByTestId('template-item-default')).toBeVisible();
    const preview = page.getByTestId('email-preview');
    await expect(preview).toBeVisible();
    await expect(preview.getByText('High CPU usage', { exact: false }).first()).toBeVisible();
  });

  test('creates a template from a starter', async ({ gotoPage, page }) => {
    const name = `e2e-${Date.now()}`;
    await gotoPage(`/${ROUTES.Templates}`);
    await page.getByTestId('new-template').click();
    await page.getByTestId('new-template-name').fill(name);
    await page.getByRole('radio', { name: /Compact table/ }).click();
    await page.getByTestId('new-template-create').click();
    await expect(page.getByTestId(`template-item-${name}`)).toBeVisible();
    await expect(page.getByText(`email_template=${name}`)).toBeVisible();
  });

  test('delivery log renders', async ({ gotoPage, page }) => {
    await gotoPage(`/${ROUTES.History}`);
    await expect(page.getByRole('heading', { name: 'Delivery log' })).toBeVisible();
  });

  test('setup page shows the three steps', async ({ gotoPage, page }) => {
    await gotoPage(`/${ROUTES.Setup}`);
    for (const step of ['Set up email delivery', 'Connect Grafana alerting', 'Route alerts and pick templates']) {
      await expect(page.getByRole('heading', { name: step })).toBeVisible();
    }
  });
});
