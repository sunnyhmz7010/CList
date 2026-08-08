import {expect, test} from '@playwright/test'

test('trash page requires explicit permanent delete', async ({page}) => {
  await page.goto('/trash')
  await expect(page.getByRole('heading', {name: '回收站'})).toBeVisible()
})
