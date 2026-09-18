import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';
const runtimeErrors = new WeakMap<Page, string[]>();
test.beforeEach(({page}) => {
  const errors: string[] = []; runtimeErrors.set(page, errors);
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error' && /(?:TypeError|ReferenceError|SyntaxError)/.test(message.text())) errors.push(message.text()); });
});
test.afterEach(({page}) => { expect(runtimeErrors.get(page)).toEqual([]); });
async function login(page: Page) {
  const user = JSON.parse(readFileSync('../data/ux-browser-fixture.json', 'utf8'));
  await page.goto('/login');
  await page.getByRole('textbox', {name:'メール',exact:true}).fill(user.email);
  await page.locator('#login-password').fill(user.password);
  await page.getByRole('button', {name:'ログイン',exact:true}).click();
  await expect(page).toHaveURL(/\/feed$/);
}
async function noHorizontalOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
}
for (const width of [320,390,768,1024,1440]) {
  test(`public pages reflow at ${width}px`, async ({page}) => {
    await page.setViewportSize({width,height:844});
    for (const path of ['/about','/register','/login']) {
      await page.goto(path); await expect(page.locator('h1')).toBeVisible(); await noHorizontalOverflow(page);
      if (path === '/about' && width === 390) {
        const button = page.getByRole('link',{name:'アカウント作成',exact:true}).first();
        expect((await button.boundingBox())!.y + (await button.boundingBox())!.height).toBeLessThan(844);
      }
    }
  });
  test(`signed-in pages reflow at ${width}px`, async ({page}) => {
    await login(page); await page.setViewportSize({width,height:844});
    for (const path of ['/feed','/compose','/settings','/notifications','/messages','/communities','/search']) {
      await page.goto(path); await expect(page.locator('main')).toBeVisible(); await noHorizontalOverflow(page);
    }
  });
}
test('registration has labels, first-invalid focus and password visibility', async ({page}) => {
  await page.goto('/register');
  await page.getByRole('button',{name:'登録する',exact:true}).click();
  await expect(page.getByLabel('メール',{exact:true})).toBeFocused();
  const password = page.getByLabel('パスワード',{exact:true});
  await password.fill('temporary-fixture-password');
  await page.getByRole('button',{name:'表示',exact:true}).first().click();
  await expect(password).toHaveAttribute('type','text');
  await page.getByRole('button',{name:'隠す',exact:true}).click();
  await expect(password).toHaveAttribute('type','password');
});
test('composer exposes audience and restores keyboard focus after Escape', async ({page}) => {
  await login(page);
  const trigger = page.getByRole('button',{name:'投稿する',exact:true});
  await trigger.click();
  const dialog = page.getByRole('dialog'); await expect(dialog).toBeVisible();
  await dialog.getByRole('button',{name:'追加設定',exact:true}).click();
  await expect(dialog.getByRole('button',{name:'閲覧パスワード設定を開く',exact:true})).toBeVisible();
  await dialog.getByRole('button',{name:/公開範囲を設定/}).click();
  await dialog.locator('select').filter({visible:true}).first().selectOption('private');
  await expect(dialog.getByRole('button',{name:/公開範囲を設定/})).not.toContainText('パブリック');
  await page.keyboard.press('Escape'); await expect(dialog).not.toBeVisible(); await expect(trigger).toBeFocused();
});
test('mobile compose publishes through the shared form', async ({page}) => {
  await login(page); await page.setViewportSize({width:390,height:844}); await page.goto('/compose');
  const caption = `UI browser publication ${Date.now()}`;
  await page.getByRole('textbox',{name:'キャプション',exact:true}).fill(caption);
  await page.getByRole('button',{name:'投稿',exact:true}).click();
  await expect(page).toHaveURL(/\/posts\/[^/]+$/); await expect(page.getByText(caption,{exact:true})).toBeVisible();
});
test('all themes and modes retain layout and save feedback', async ({page}) => {
  await login(page); await page.goto('/settings/appearance');
  for (const mode of ['light','dark']) {
    await page.locator(`[data-theme-mode="${mode}"]`).click();
    for (const theme of ['default','pink','orange','blue','violet']) {
      const button = page.locator(`[data-theme-preset="${theme}"]`); await button.click();
      await expect(button).toHaveAttribute('aria-pressed','true');
      await expect(page.getByRole('status')).toContainText('保存しました'); await noHorizontalOverflow(page);
    }
  }
});
test('200% layout zoom and long translations keep primary controls reachable', async ({page}) => {
  await page.goto('/register');
  await page.evaluate(() => { document.documentElement.style.zoom = '2'; });
  await noHorizontalOverflow(page); await expect(page.getByRole('button',{name:'登録する',exact:true})).toBeVisible();
  await page.goto('/about');
  await page.getByRole('combobox').selectOption('ru');
  await page.setViewportSize({width:320,height:844}); await noHorizontalOverflow(page);
});
test('tablet search remains available when the right column is hidden', async ({page}) => {
  await login(page); await page.setViewportSize({width:1024,height:844}); await page.goto('/search');
  const search = page.getByRole('searchbox',{name:'検索',exact:true}).filter({visible:true});
  await search.fill('UI'); await search.press('Enter'); await expect(page).toHaveURL(/q=UI/);
});
test('network failure has retry and recovers without a reload', async ({page}) => {
  await login(page);
  await page.route('**/api/v1/notifications', route => route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Temporarily unavailable'})}));
  await page.goto('/notifications');
  const retry = page.getByRole('button',{name:'再試行'}); await expect(retry).toBeVisible();
  await page.unroute('**/api/v1/notifications'); await retry.click(); await expect(retry).not.toBeVisible();
});
test('main navigation and settings have no runtime errors', async ({page}) => {
  const errors: string[] = []; page.on('pageerror', error => errors.push(error.message));
  await login(page); await page.getByRole('link',{name:'設定',exact:true}).first().click();
  for (const label of ['プロフィール','表示・言語','セキュリティ','連携・開発者','アカウント削除']) await expect(page.getByRole('heading',{name:label,exact:true})).toBeVisible();
  expect(errors).toEqual([]);
});

test('loading and empty notifications remain distinct', async ({page}) => {
  await login(page);
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/api/v1/notifications', async route => {
    await gate;
    await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({items:[]})});
  });
  await page.goto('/notifications');
  await expect(page.locator('main .ui-spinner')).toBeVisible();
  release();
  await expect(page.locator('main .ui-spinner')).not.toBeVisible();
  await expect(page.locator('main .ui-empty')).toBeVisible();
  await expect(page.locator('main .ui-empty button')).toHaveCount(0);
});

test('hub detail uses page scrolling without nested tab scrollbars', async ({page}) => {
  await login(page);
  await page.route('**/api/v1/communities/ui-scroll-fixture/posts', route => route.fulfill({
    status:200,contentType:'application/json',body:JSON.stringify({community:{
      id:'ui-scroll-fixture',name:'Scroll fixture',description:'Long description\n'.repeat(40),details:'Details',tags:[],approved_member_count:1
    },items:[]})
  }));
  for (const width of [390,1440]) {
    await page.setViewportSize({width,height:844}); await page.goto('/communities/ui-scroll-fixture');
    await expect(page.getByRole('heading',{name:'Scroll fixture'})).toBeVisible();
    await noHorizontalOverflow(page);
    expect(await page.locator('main').evaluate(el => getComputedStyle(el).overflowY)).toBe('visible');
    const tabs = page.locator('.ui-hub-tabs');
    expect(await tabs.evaluate(el => el.scrollHeight <= el.clientHeight + 1)).toBe(true);
    await tabs.getByRole('button',{name:'詳細情報',exact:true}).focus();
    await page.keyboard.press('Enter');
    await expect(tabs.getByRole('button',{name:'詳細情報',exact:true})).toHaveAttribute('aria-current','page');
    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(0);
    if (width >= 1280) {
      for (const selector of ['.ui-sidebar', '.ui-right-sidebar']) {
        expect(await page.locator(selector).evaluate(el => el.getBoundingClientRect().top)).toBe(0);
      }
    }
  }
});
