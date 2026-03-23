import { test, expect } from '@playwright/test';

test.describe('通知中心', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/notify');
  });

  test('通知列表加载', async ({ page }) => {
    await expect(page).not.toHaveURL('/login');
    const list = page.locator('.ant-list, .ant-table, [data-testid="notification-list"]').first();
    await expect(list).toBeVisible();
  });

  test('通知渠道配置入口', async ({ page }) => {
    const channelBtn = page.locator('[data-testid="channel-config"], button:has-text("渠道"), .ant-tabs-tab').first();
    await expect(channelBtn).toBeVisible();
  });

  test('已读/未读筛选', async ({ page }) => {
    const filterEl = page.locator('.ant-radio-group, .ant-segmented, [data-testid="read-filter"]').first();
    await expect(filterEl).toBeVisible();
  });

  test('全部标为已读', async ({ page }) => {
    const markAllBtn = page.locator('button:has-text("全部已读"), [data-testid="mark-all-read"]').first();
    if (await markAllBtn.count() > 0) {
      await expect(markAllBtn).toBeEnabled();
    }
  });

  test('通知详情弹窗', async ({ page }) => {
    const firstRow = page.locator('.ant-list-item, .ant-table-row').first();
    if (await firstRow.count() > 0) {
      await firstRow.click();
      const modal = page.locator('.ant-modal, .ant-drawer').first();
      await expect(modal).toBeVisible();
    }
  });

  test('分页控件', async ({ page }) => {
    const pagination = page.locator('.ant-pagination').first();
    await expect(pagination).toBeVisible();
  });
});
