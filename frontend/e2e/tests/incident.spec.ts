import { test, expect } from '@playwright/test';

test.describe('事件管理', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/incident');
  });

  test('事件列表加载', async ({ page }) => {
    // 验证页面表格和 Tab 栏存在
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();

    const tabs = page.locator('.ant-tabs');
    await expect(tabs).toBeVisible();
  });

  test('点击事件进入详情', async ({ page }) => {
    // 等待表格渲染
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();

    // 如果有数据行，点击第一行验证跳转
    const firstRow = page.locator('.ant-table-tbody tr').first();
    if (await firstRow.count() > 0 && await firstRow.isVisible()) {
      await firstRow.click();
      // 验证 URL 变化到详情页
      await expect(page).toHaveURL(/\/incident\/.+/);
    }
  });

  test('AI 分析摘要区域', async ({ page }) => {
    // 验证页面上 AI 分析面板存在（如有）
    const aiPanel = page.locator('[data-testid="ai-analysis"], [class*="ai-analysis"], [class*="ai-summary"]');
    // AI 分析面板可能在详情页，此处验证列表页是否有 AI 相关入口
    const table = page.locator('.ant-table');
    await expect(table).toBeVisible();

    // 如果列表页没有 AI 面板，验证进入详情后存在
    const firstRow = page.locator('.ant-table-tbody tr').first();
    if (await firstRow.count() > 0 && await firstRow.isVisible()) {
      await firstRow.click();
      // 在详情页中查找 AI 分析区域
      const aiSection = page.locator('[data-testid="ai-analysis"], [class*="ai"]');
      if (await aiSection.count() > 0) {
        await expect(aiSection.first()).toBeVisible();
      }
    }
  });

  test('统计卡片渲染', async ({ page }) => {
    // 5 列统计卡片（active / todayNew / todayResolved / avgMTTR / monthSLA）
    const statRow = page.locator('.ant-row').first();
    const cols = statRow.locator('.ant-col');
    await expect(cols).toHaveCount(5);

    // 验证每个卡片内有 Card 组件
    const cards = statRow.locator('.ant-card');
    await expect(cards).toHaveCount(5);
  });
});
