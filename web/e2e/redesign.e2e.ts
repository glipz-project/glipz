import { test, expect, type Page } from '@playwright/test';
import { readFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';

const fixture = () => JSON.parse(readFileSync('../data/ux-browser-fixture.json', 'utf8'));
const errors = new WeakMap<Page, string[]>();
test.beforeEach(async ({ page }) => {
  const messages: string[] = [];
  errors.set(page, messages);
  page.on('pageerror', error => messages.push(error.message));
  await page.addInitScript(() => localStorage.setItem('glipz-theme-mode', 'light'));
});
test.afterEach(({ page }) => expect(errors.get(page)).toEqual([]));

async function login(page: Page) {
  const user = fixture();
  await page.goto('/login');
  await page.locator('#login-email').fill(user.email);
  await page.locator('#login-password').fill(user.password);
  await page.getByRole('button', { name: 'ログイン', exact: true }).click();
  await expect(page).toHaveURL(/\/feed$/);
}
async function checkScreen(page: Page, name: string) {
  await expect(page.locator('#main-content')).toBeVisible();
  await expect(page.locator('#main-content')).not.toHaveText('');
  await expect(page.locator('vite-error-overlay')).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), name).toBe(true);
  // Content must contribute to the shell height rather than overflow below it.
  const bounds = await page.evaluate(() => ({
    document: document.documentElement.scrollHeight,
    shell: document.querySelector('.ui-app-shell')!.getBoundingClientRect().bottom + scrollY,
  }));
  expect(bounds.document, `${name}: content below shell`).toBeLessThanOrEqual(Math.ceil(bounds.shell) + 1);
  if (process.env.UI_SCREENSHOT_DIR) {
    mkdirSync(process.env.UI_SCREENSHOT_DIR, { recursive: true });
    await page.screenshot({ path: join(process.env.UI_SCREENSHOT_DIR, `${name}.png`) });
  }
}

for (const width of [390, 1440]) {
  test(`all secondary screens render at ${width}`, async ({ page }) => {
    test.setTimeout(120000);
    await page.setViewportSize({ width, height: 900 });
    await login(page);
    const handle = fixture().handle;
    for (const path of [
      '/feed', '/bookmarks', '/feed/scheduled', '/settings', '/settings/appearance',
      '/compose', '/remote/profile', '/posts/ui-missing-post', '/posts/federated/ui-missing-post',
      '/communities/ui-missing-community', '/messages/ui-missing-thread', '/about',
      '/settings/language', '/settings/timeline', '/settings/plugins', '/settings/mfa',
      '/settings/identity-portability', '/settings/notifications', '/settings/account-deletion',
      '/settings/direct-messages', '/settings/custom-emojis', '/developer/api',
      '/developer/oauth/authorize', `/@${handle}`, `/@${handle}/following`, `/@${handle}/followers`,
      '/communities', '/communities/new', '/messages', '/notifications', '/search',
      '/legal/terms', '/legal/privacy', '/legal/nsfw-guidelines', '/legal/law-enforcement',
      '/federation/guidelines', '/legal/api-guidelines',
    ]) {
      await page.goto(path);
      await page.setViewportSize({ width, height: 844 });
      await checkScreen(page, `${width}-${path.replaceAll('/', '-').replaceAll('@', '')}`);
      if (width >= 768 && await page.locator('.ui-sidebar').isVisible()) {
        await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
        expect(Math.abs(await page.locator('.ui-sidebar').evaluate(el => el.getBoundingClientRect().top)), `${path}: sidebar after scroll`).toBeLessThanOrEqual(1);
      }
      await page.setViewportSize({ width, height: 900 });
    }
  });
}

test('responsive navigation, inline composer and drawer keyboard behavior', async ({ page }) => {
  await login(page);
  for (const width of [320, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(page.getByRole('textbox', { name: 'キャプション', exact: true })).toBeVisible();
    if (width < 768) {
      await expect(page.locator('.ui-bottom-nav')).toBeVisible();
      await expect(page.locator('.ui-bottom-label')).toHaveCount(4);
      await page.locator('#mobile-account-button').click();
      await expect(page.locator('.ui-drawer-close')).toBeFocused();
      await page.keyboard.press('Escape');
      await expect(page.locator('#mobile-account-button')).toBeFocused();
      await expect(page.locator('#app-sidebar')).not.toBeVisible();
    } else {
      await expect(page.locator('.ui-bottom-nav')).not.toBeVisible();
      await expect(page.locator('#app-sidebar')).toBeVisible();
    }
    expect(await page.locator('.ui-right-sidebar').isVisible()).toBe(width >= 1280);
    await checkScreen(page, `navigation-${width}`);
  }
});

test('SPA navigation preserves page headers after leaving messages', async ({ page }) => {
  await login(page);
  await page.locator('.ui-primary-nav a[href="/messages"]').click();
  await expect(page.locator('.ui-messages')).toBeVisible();
  await page.locator('.ui-primary-nav a[href="/feed"]').click();
  await expect(page.locator('.ui-feed-heading')).toBeVisible();
  await expect(page.locator('#app-view-header-slot-desktop button')).not.toHaveCount(0);
  await page.locator('.ui-primary-nav a[href="/settings"]').click();
  await expect(page.locator('#app-view-header-slot-desktop h1')).toHaveText('設定');
});

test('desktop canvas has no extra bottom band or empty vertical overflow', async ({ page }) => {
  await page.setViewportSize({ width: 1905, height: 1109 });
  await login(page);
  for (const mode of ['light', 'dark']) {
    await page.goto('/settings/appearance');
    await page.locator(`[data-theme-mode="${mode}"]`).click();
    await page.locator('.ui-primary-nav a[href="/feed"]').click();
    const geometry = await page.evaluate(() => ({
      body: getComputedStyle(document.body).backgroundColor,
      canvas: getComputedStyle(document.querySelector('.ui-app-shell')!).backgroundColor,
      height: document.documentElement.scrollHeight,
      viewport: innerHeight,
      top: document.querySelector('.ui-main')!.getBoundingClientRect().top,
    }));
    expect(geometry.body).toBe(geometry.canvas);
    expect(geometry.top).toBe(0);
    expect(geometry.height).toBeLessThanOrEqual(geometry.viewport + 1);
    await checkScreen(page, `desktop-canvas-${mode}`);
    // A resize resolves percentage minimum heights differently from first layout.
    for (const height of [945, 780, 1109]) {
      await page.setViewportSize({ width: 1905, height });
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollHeight - innerHeight)).toBeLessThanOrEqual(1);
    }
  }
});

// Admin responses are isolated browser fixtures: no real accounts receive privileges.
for (const width of [390, 1440]) {
  test(`admin surfaces and user actions at ${width}`, async ({ page }) => {
    test.setTimeout(90000);
    await page.setViewportSize({ width, height: 900 });
    await login(page);
    await page.route('**/api/v1/me', async route => {
      const response = await route.fetch();
      await route.fulfill({ json: { ...await response.json(), is_site_admin: true } });
    });
    const user = { id: 'redesign-user', handle: 'sample', email: 'sample@example.test', display_name: '表示確認ユーザー', badges: [], created_at: '2026-09-01T00:00:00Z', suspended_at: null as string | null };
    await page.route('**/api/v1/admin/**', async route => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith('/suspension')) {
        user.suspended_at = route.request().postDataJSON().suspended ? '2026-09-19T00:00:00Z' : null;
        return route.fulfill({ json: { user } });
      }
      if (path.endsWith('/users')) return route.fulfill({ json: { items: [user], total: 1 } });
      if (path.endsWith('/badges')) return route.fulfill({ json: { available_badges: ['verified'], user: { ...user, visible_badges: [] } } });
      if (path.endsWith('/overview')) return route.fulfill({ json: { users_total: 1, users_suspended: 0, reports_open_local: 0, reports_open_federated: 0, federation_pending: 0, federation_dead: 0, registrations_enabled: true, federation_policy_summary: '', operator_announcements_count: 0 } });
      return route.fulfill({ json: { items: [], total: 0, pending: 0, dead: 0, operator_announcements: [] } });
    });
    for (const path of ['/admin', '/admin/reports', '/admin/federation', '/admin/legal-requests', '/admin/custom-emojis', '/admin/instance-settings', '/admin/users']) {
      await page.goto(path);
      await expect(page.locator('.ui-admin main h1')).toBeVisible();
      await checkScreen(page, `${width}-${path.replaceAll('/', '-')}`);
    }
    const row = page.locator('.ui-admin-list article');
    await expect(row.getByRole('heading', { name: user.display_name })).toBeVisible();
    await row.locator('summary').click();
    await expect(row.getByText(user.email, { exact: true })).toBeVisible();
    await row.getByRole('button', { name: 'バッジ編集', exact: true }).click();
    await expect(row.locator('input[type=checkbox]')).toBeVisible();
  });
}
