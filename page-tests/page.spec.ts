// The default landing page in a real browser: its landmarks, and the shared accessibility sweep — WCAG 2.2 AA and AAA (7:1 text) in light and
// dark, reflow at 320 px, 200% text, forced colours, reduced motion, and the keyboard with focus kept visible and unobscured.
import { expect, test } from '@playwright/test'
import { defineA11ySuite } from '@meddleware/frontend-config/a11y'

test('has its landmarks, one heading level 1 and named tables', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('banner')).toHaveCount(1)
  await expect(page.getByRole('main')).toHaveCount(1)
  await expect(page.getByRole('contentinfo')).toHaveCount(1)
  await expect(page.getByRole('heading', { level: 1 })).toHaveCount(1)
  for (const table of await page.getByRole('table').all()) await expect(table).toHaveAccessibleName(/.+/)
})

defineA11ySuite({ page: async (page) => { await page.goto('/') } }, { aaa: 'gate' })
