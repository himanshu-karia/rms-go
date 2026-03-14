import { test, expect } from './fixtures/auth';
test.describe('sidebar navigation RBAC', () => {
    test('super admin sees administration and operations menus', async ({ page, authenticateAs }) => {
        await authenticateAs('superAdmin');
        await page.goto('/');
        await expect(page.getByRole('link', { name: 'RMS Dashboard' })).toBeVisible();
        const sidebar = page.locator('aside nav').first();
        await sidebar.getByRole('button', { name: /Administration/ }).click();
        const adminLinks = [
            'States',
            'State Authorities',
            'Projects',
            'User Groups',
            'Server Vendors',
            'Protocol Versions',
            'Drive Manufacturers',
            'Pump Vendors',
            'RMS Manufacturers',
        ];
        for (const label of adminLinks) {
            await expect(sidebar.getByRole('link', { name: label, exact: true })).toBeVisible();
        }
        await sidebar.getByRole('button', { name: /Operations/ }).click();
        const operationsLinks = ['Enroll Device', 'Manage Government Credentials', 'Import CSVs'];
        for (const label of operationsLinks) {
            await expect(sidebar.getByRole('link', { name: label, exact: true })).toBeVisible();
        }
        await sidebar.getByRole('button', { name: /Live/ }).click();
        await expect(sidebar.getByRole('link', { name: 'Telemetry Monitor', exact: true })).toBeVisible();
        await expect(sidebar.getByRole('link', { name: 'Telemetry v2', exact: true })).toBeVisible();
        await expect(sidebar.getByRole('link', { name: 'Simulator', exact: true })).toBeVisible();
    });
    test('operations manager hides administration links but keeps operations menu', async ({ page, authenticateAs, }) => {
        await authenticateAs('operationsManager');
        await page.goto('/');
        await expect(page.getByRole('link', { name: 'RMS Dashboard' })).toBeVisible();
        const sidebar = page.locator('aside nav').first();
        await expect(sidebar.getByRole('button', { name: /Administration/ })).toHaveCount(0);
        await expect(sidebar.getByRole('button', { name: /Operations/ })).toBeVisible();
        await sidebar.getByRole('button', { name: /Operations/ }).click();
        const operationsLinks = [
            'Enroll Device',
            'Manage Government Credentials',
            'Import CSVs',
            'Import History',
        ];
        for (const label of operationsLinks) {
            await expect(sidebar.getByRole('link', { name: label, exact: true })).toBeVisible();
        }
        await sidebar.getByRole('button', { name: /Live/ }).click();
        await expect(sidebar.getByRole('link', { name: 'Telemetry Monitor', exact: true })).toBeVisible();
        await expect(sidebar.getByRole('link', { name: 'Simulator', exact: true })).toBeVisible();
    });
    test('telemetry viewer only sees live telemetry links', async ({ page, authenticateAs }) => {
        await authenticateAs('telemetryViewer');
        await page.goto('/');
        await expect(page.getByRole('link', { name: 'RMS Dashboard' })).toBeVisible();
        const sidebar = page.locator('aside nav').first();
        await expect(sidebar.getByRole('button', { name: /Administration/ })).toHaveCount(0);
        await expect(sidebar.getByRole('button', { name: /Operations/ })).toHaveCount(0);
        await sidebar.getByRole('button', { name: /Live/ }).click();
        const blockedLinks = ['Device Inventory', 'Simulator'];
        for (const label of blockedLinks) {
            await expect(sidebar.getByRole('link', { name: label, exact: true })).toHaveCount(0);
        }
        const liveLinks = ['Telemetry Monitor', 'Telemetry v2'];
        for (const label of liveLinks) {
            await expect(sidebar.getByRole('link', { name: label, exact: true })).toBeVisible();
        }
    });
});
