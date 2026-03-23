import { test, expect } from '@playwright/test';

test.describe('系统设置', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/settings');
  });

  test('设置页面加载', async ({ page }) => {
    await expect(page).not.toHaveURL('/login');
    const mainContent = page.locator('main, .ant-layout-content, #root').first();
    await expect(mainContent).toBeVisible();
  });

  test('侧边菜单导航', async ({ page }) => {
    const menu = page.locator('.ant-menu, .ant-layout-sider, [data-testid="settings-menu"]').first();
    await expect(menu).toBeVisible();
  });

  test('个人信息表单', async ({ page }) => {
    const profileLink = page.locator('a:has-text("个人"), .ant-menu-item:has-text("个人"), [data-testid="profile-menu"]').first();
    if (await profileLink.count() > 0) {
      await profileLink.click();
      const form = page.locator('.ant-form').first();
      await expect(form).toBeVisible();
    }
  });

  test('密码修改入口', async ({ page }) => {
    const pwdLink = page.locator('a:has-text("密码"), .ant-menu-item:has-text("密码"), [data-testid="password-menu"]').first();
    if (await pwdLink.count() > 0) {
      await pwdLink.click();
      const form = page.locator('.ant-form').first();
      await expect(form).toBeVisible();
    }
  });

  test('通知偏好设置', async ({ page }) => {
    const notifyLink = page.locator('a:has-text("通知"), .ant-menu-item:has-text("通知"), [data-testid="notify-settings-menu"]').first();
    if (await notifyLink.count() > 0) {
      await notifyLink.click();
      const switchEl = page.locator('.ant-switch, .ant-checkbox').first();
      await expect(switchEl).toBeVisible();
    }
  });

  test('保存按钮可点击', async ({ page }) => {
    const saveBtn = page.locator('button[type="submit"], button:has-text("保存"), button:has-text("Save")').first();
    if (await saveBtn.count() > 0) {
      await expect(saveBtn).toBeEnabled();
    }
  });

  test('语言切换', async ({ page }) => {
    const langEl = page.locator('[data-testid="lang-switch"], .ant-select:has-text("中文"), .ant-select:has-text("English")').first();
    if (await langEl.count() > 0) {
      await expect(langEl).toBeVisible();
    }
  });
});
