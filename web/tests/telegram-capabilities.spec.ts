import {expect, test} from '@playwright/test'

test('streaming profile declares that resume is unavailable', async ({page}) => {
  await page.goto('/admin')
  await page.getByLabel('类型').selectOption('telegram_streaming')
  await expect(page.getByText('当前后端不支持断点续传')).toBeVisible()
})
