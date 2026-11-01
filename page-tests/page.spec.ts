// The default landing page in a real browser: its landmarks, WCAG 2.2 AA and AAA (7:1 text) in light and dark, and reflow at 320 px.
import { expect, test } from '@playwright/test'
import { expectAccessible } from '@meddleware/frontend-config/axe'

test('has its landmarks, one heading level 1 and named tables', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('banner')).toHaveCount(1)
  await expect(page.getByRole('main')).toHaveCount(1)
  await expect(page.getByRole('contentinfo')).toHaveCount(1)
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1)
  for (const table of await page.getByRole('table').all()) await expect(table).toHaveAccessibleName(/.+/)
})

for (const scheme of ['light', 'dark'] as const) {
  test(`has no axe violations, AA or AAA (${scheme})`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: scheme })
    await page.goto('/')
    await expectAccessible(page, { aaa: 'gate' })
  })
}

test('reflows at 320 CSS pixels without scrolling sideways', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 640 })
  await page.goto('/')
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBeLessThanOrEqual(0)
})
