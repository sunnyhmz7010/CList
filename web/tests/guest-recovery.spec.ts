import {expect, test} from '@playwright/test'
test('recovery page keeps the key out of the URL', async ({page}) => {
  await page.goto('/'); await expect(page).not.toHaveURL(/recovery_key=/)
})
