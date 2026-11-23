import { test, expect } from './fixtures';

test('saves SMTP settings', async ({ appConfigPage, page }) => {
  await page.getByTestId('data-testid ac-from-address').fill('alerts@example.com');
  await page.getByTestId('data-testid ac-smtp-host').fill('mailpit');
  await page.getByTestId('data-testid ac-smtp-port').fill('1025');
  await page.getByTestId('data-testid ac-default-recipients').fill('ops@example.com');

  const saveResponse = appConfigPage.waitForSettingsResponse();
  await page.getByTestId('data-testid ac-submit-form').click();
  await expect(saveResponse).toBeOK();
});
