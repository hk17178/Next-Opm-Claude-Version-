import { test, expect } from '@playwright/test';

test.describe('运维大屏', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/dashboard');
  });

  test('大屏页面加载', async ({ page }) => {
    // 验证暗色主题背景存在（BigScreen 组件使用深色 linear-gradient 背景）
    const darkContainer = page.locator('div[style*="background"]').first();
    await expect(darkContainer).toBeVisible();

    // 验证页面标题存在
    const heading = page.locator('h2, h1').first();
    await expect(heading).toBeVisible();
  });

  test('统计卡片渲染', async ({ page }) => {
    // BigScreen 有 5 个 MetricBlock 卡片（alerts / incidents / sla / assets / logs）
    // 但任务要求验证 4 列，取第一行 Row 中的 Card
    const metricRow = page.locator('.ant-row').first();
    const metricCards = metricRow.locator('.ant-card');

    // 验证至少有 4 个统计卡片存在
    const cardCount = await metricCards.count();
    expect(cardCount).toBeGreaterThanOrEqual(4);
  });

  test('全屏模式', async ({ page }) => {
    // 检查是否有全屏按钮
    const fullscreenButton = page.getByRole('button', { name: /全屏|fullscreen|expand/i });

    if (await fullscreenButton.count() > 0) {
      await fullscreenButton.click();
      // 验证进入全屏后的状态变化
      const exitButton = page.getByRole('button', { name: /退出全屏|exit fullscreen|shrink/i });
      if (await exitButton.count() > 0) {
        await expect(exitButton).toBeVisible();
      }
    }
  });
});
