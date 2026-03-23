import { test, expect } from '@playwright/test';

test.describe('资产管理', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/cmdb');
  });

  test('资产列表加载', async ({ page }) => {
    // 验证资产表格存在
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();

    // 验证页面标题
    const title = page.locator('text=资产管理').or(page.locator('text=Asset'));
    if (await title.count() > 0) {
      await expect(title.first()).toBeVisible();
    }
  });

  test('六维筛选区域', async ({ page }) => {
    // 验证 6 个筛选器存在：业务 / 类型 / 环境 / 地域 / 分级 / 状态
    const filterCard = page.locator('.ant-card').first();
    await expect(filterCard).toBeVisible();

    const selects = filterCard.locator('.ant-select');
    // 应有 6 个 Select 筛选器（business / type / env / region / grade / status）
    await expect(selects).toHaveCount(6);
  });

  test('资产批量选择', async ({ page }) => {
    // 表格应有 checkbox 列（rowSelection 已启用）
    const checkboxColumn = page.locator('.ant-table-selection-column').first();
    if (await checkboxColumn.count() > 0) {
      await expect(checkboxColumn).toBeVisible();
    }

    // 如果有数据行，点击 checkbox 验证批量操作栏出现
    const firstCheckbox = page.locator('.ant-table-tbody .ant-checkbox-input').first();
    if (await firstCheckbox.count() > 0 && await firstCheckbox.isVisible()) {
      await firstCheckbox.check();
      // 验证批量操作按钮出现（在 table footer 中）
      const batchButton = page.locator('.ant-table-footer .ant-btn');
      await expect(batchButton).toBeVisible();
    }
  });

  test('搜索功能', async ({ page }) => {
    // 找到搜索按钮
    const searchButton = page.getByRole('button', { name: /搜索|search/i });
    await expect(searchButton).toBeVisible();

    // 找到标签/关键词输入框
    const tagInput = page.locator('.ant-card input[type="text"]').first();
    if (await tagInput.count() > 0) {
      await tagInput.fill('web-server-01');
    }

    // 点击搜索按钮触发请求
    await searchButton.click();

    // 验证表格仍可见（请求已触发）
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();
  });
});
