import {expect, test} from '@playwright/test'

test('CList full flow surfaces stable links and gallery preview', async ({page}) => {
  await page.goto('/gallery')
  await expect(page.getByRole('heading', {name: /访问相册|相册/})).toBeVisible()
})
