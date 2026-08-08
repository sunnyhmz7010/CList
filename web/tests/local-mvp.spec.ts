import {expect, test} from '@playwright/test'

test('shows the CList local workspace', async ({page}) => {
  await page.goto('/')
  await expect(page.getByRole('heading', {name: 'CList'})).toBeVisible()
  await expect(page.getByRole('heading', {name: '上传'})).toBeVisible()
})
