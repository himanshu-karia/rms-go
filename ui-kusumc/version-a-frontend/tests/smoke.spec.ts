import { test, expect } from './fixtures/auth';

test('dashboard loads for authenticated users', async ({ page, authenticateAs }) => {
  await authenticateAs('superAdmin');
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Fleet Overview' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Open Telemetry Monitor' })).toBeVisible();
});
