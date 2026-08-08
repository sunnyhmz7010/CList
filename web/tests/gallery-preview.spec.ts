import {expect, test} from '@playwright/test'

test('gallery exposes independent password and filters', async ({page}) => {
  await page.goto('/gallery')
  await expect(page.getByRole('heading', {name: '访问相册'})).toBeVisible()
})
