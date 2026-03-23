import { test, expect } from '@playwright/test';

test.describe('日志检索', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/log');
  });

  test('日志检索页面加载', async ({ page }) => {
    // 验证 Lucene 搜索框存在（Input.Search 组件）
    const searchInput = page.locator('.ant-input-search input');
    await expect(searchInput).toBeVisible();

    // 验证页面标题存在
    const title = page.locator('text=日志检索').or(page.locator('text=Log Search'));
    if (await title.count() > 0) {
      await expect(title.first()).toBeVisible();
    }
  });

  test('执行日志搜索', async ({ page }) => {
    // 在搜索框中输入查询词
    const searchInput = page.locator('.ant-input-search input');
    await searchInput.fill('error AND service:gateway');

    // 点击搜索按钮
    const searchButton = page.locator('.ant-input-search .ant-input-search-button');
    await searchButton.click();

    // 验证结果区域（表格）存在
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();
  });

  test('时间范围快捷选择', async ({ page }) => {
    // 找到时间预设 Select 组件
    const timePresetSelect = page.locator('.ant-select').first();
    await timePresetSelect.click();

    // 等待下拉面板出现
    await page.waitForSelector('.ant-select-dropdown');

    // 选择 "1h"（最近1小时）
    await page.getByTitle('1h').click();

    // 验证选中值
    await expect(timePresetSelect.locator('.ant-select-selection-item')).toHaveText('1h');
  });

  test('导出对话框', async ({ page }) => {
    // 点击导出按钮
    const exportButton = page.getByRole('button', { name: /导出|export/i });
    await exportButton.click();

    // 验证导出 Modal 弹出
    const modal = page.locator('.ant-modal');
    await expect(modal).toBeVisible();

    // 验证 Modal 中有 CSV / JSON 格式选项
    await expect(page.getByText('CSV')).toBeVisible();
    await expect(page.getByText('JSON')).toBeVisible();
  });
});
