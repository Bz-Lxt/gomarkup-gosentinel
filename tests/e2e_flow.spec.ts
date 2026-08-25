import { expect, test, type Page } from "@playwright/test";

const base = process.env.E2E_BASE || "http://127.0.0.1:31481";
const gin = process.env.GIN_BASE || "http://127.0.0.1:31483";

async function waitDashboard(page: Page) {
  await page.goto(base);
  await expect(page.getByRole("heading", { name: "阻断雷达控制室" })).toBeVisible();
  await expect(page.getByTestId("radar")).toBeVisible();
  await expect(page.getByTestId("rule-editor")).toBeVisible({ timeout: 15_000 });
}

test("首屏雷达与规则画布", async ({ page }) => {
  await waitDashboard(page);
  await expect(page.getByTestId("conn-status")).toBeVisible();
});

test("规则校验拦截非法 QPS", async ({ page }) => {
  await waitDashboard(page);
  const qps = page.getByRole("spinbutton", { name: "QPS 上限" });
  await qps.fill("0");
  await page.getByTestId("save-rule").click();
  await expect(page.getByTestId("form-summary")).toContainText("标红");
});

test("保存合法规则并看到收敛卡", async ({ page }) => {
  await waitDashboard(page);
  const qps = page.getByRole("spinbutton", { name: "QPS 上限" });
  await qps.fill("60");
  await page.getByTestId("save-rule").click();
  await page.getByRole("button", { name: "确认" }).click();
  await expect(page.getByTestId("convergence")).toContainText("v");
});

test("制造流量后雷达仍可见", async ({ request, page }) => {
  for (let i = 0; i < 20; i++) {
    await request.get(`${gin}/work`).catch(() => undefined);
  }
  await waitDashboard(page);
  await expect(page.getByTestId("radar")).toBeVisible();
});
