import { test, expect } from '@playwright/test';

test.describe('数据分析', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/analytics');
  });

  test('分析页面加载', async ({ page }) => {
    await expect(page).not.toHaveURL('/login');
    const mainContent = page.locator('main, .ant-layout-content, #root').first();
    await expect(mainContent).toBeVisible();
  });

  test('图表区域渲染', async ({ page }) => {
    const chart = page.locator('canvas, .echarts-for-react, [data-testid="analytics-chart"]').first();
    await expect(chart).toBeVisible();
  });

  test('时间区间选择', async ({ page }) => {
    const picker = page.locator('.ant-picker-range, .ant-picker, [data-testid="date-range"]').first();
    await expect(picker).toBeVisible();
  });

  test('维度切换', async ({ page }) => {
    const tabs = page.locator('.ant-tabs, .ant-segmented, [data-testid="dimension-tabs"]').first();
    await expect(tabs).toBeVisible();
  });

  test('导出报表按钮', async ({ page }) => {
    const exportBtn = page.locator('button:has-text("导出"), button:has-text("Export"), [data-testid="export-btn"]').first();
    if (await exportBtn.count() > 0) {
      await expect(exportBtn).toBeEnabled();
    }
  });

  test('汇总统计数字显示', async ({ page }) => {
    const statEl = page.locator('.ant-statistic, .ant-card-meta, [data-testid="summary-stat"]').first();
    await expect(statEl).toBeVisible();
  });

  test('数据表格渲染', async ({ page }) => {
    const table = page.locator('.ant-table').first();
    await expect(table).toBeVisible();
  });
});
