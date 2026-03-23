import { test, expect } from '@playwright/test';

test.describe('运维驾驶舱', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/cockpit');
  });

  test('驾驶舱页面加载', async ({ page }) => {
    await expect(page).not.toHaveURL('/login');
    const mainContent = page.locator('main, .ant-layout-content, #root').first();
    await expect(mainContent).toBeVisible();
  });

  test('健康度评分显示', async ({ page }) => {
    const scoreEl = page.locator('[data-testid="health-score"], .health-score, .ant-statistic').first();
    await expect(scoreEl).toBeVisible();
  });

  test('服务依赖图渲染', async ({ page }) => {
    const graphEl = page.locator('canvas, svg, [data-testid="topology-graph"]').first();
    await expect(graphEl).toBeVisible();
  });

  test('时间范围筛选', async ({ page }) => {
    const pickerEl = page.locator('.ant-picker, [data-testid="time-range-picker"]').first();
    await expect(pickerEl).toBeVisible();
    await pickerEl.click();
    const dropdown = page.locator('.ant-picker-dropdown, .ant-dropdown').first();
    await expect(dropdown).toBeVisible();
  });

  test('告警摘要列表', async ({ page }) => {
    const alertList = page.locator('.ant-list, .ant-table, [data-testid="alert-summary"]').first();
    await expect(alertList).toBeVisible();
  });

  test('SLA 仪表盘', async ({ page }) => {
    const slaEl = page.locator('[data-testid="sla-gauge"], .sla-panel, .ant-progress').first();
    await expect(slaEl).toBeVisible();
  });
});
